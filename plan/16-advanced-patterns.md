# 高度なパターンと補足事項

## 参考資料

- [Graceful Shutdown in Go - VictoriaMetrics](https://victoriametrics.com/blog/go-graceful-shutdown/)
- [Goroutine Leak Detection - Uber](https://www.uber.com/blog/leakprof-featherlight-in-production-goroutine-leak-detection/)
- [Go Module Versioning](https://go.dev/doc/modules/version-numbers)
- [Designing Go Libraries - Abhinav Gupta](https://abhinavg.net/2022/12/06/designing-go-libraries/)
- [uber-go/goleak](https://github.com/uber-go/goleak)

---

## 1. グレースフルシャットダウン

### CLI でのシグナル処理

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"
)

func main() {
    // シグナルを受け取るコンテキスト
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT,  // Ctrl+C
        syscall.SIGTERM, // kill コマンド
    )
    defer stop()

    // Watch開始
    events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

    // メインループ
    for {
        select {
        case <-ctx.Done():
            fmt.Fprintln(os.Stderr, "\nShutting down...")
            return
        case event, ok := <-events:
            if !ok {
                return
            }
            // イベント処理
        case err, ok := <-errs:
            if !ok {
                return
            }
            // エラー処理
        }
    }
}
```

### バッファ付きシグナルチャネル

```go
// Good: バッファ付きチャネル
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

// Bad: バッファなし（シグナルを見逃す可能性）
sigCh := make(chan os.Signal)
```

### vrclog-go での適用

```go
// cmd/vrclog/tail.go
func runTail(cmd *cobra.Command, args []string) error {
    // signal.NotifyContext を使用
    ctx, stop := signal.NotifyContext(cmd.Context(),
        syscall.SIGINT,
        syscall.SIGTERM,
    )
    defer stop()

    events, errs := vrclog.Watch(ctx, opts)

    for {
        select {
        case <-ctx.Done():
            return nil // 正常終了
        case event, ok := <-events:
            // ...
        case err, ok := <-errs:
            // ...
        }
    }
}
```

---

## 2. Goroutineリーク防止

### リークの原因

1. **チャネルの送信側がブロック**: 受信者がいない
2. **チャネルの受信側がブロック**: 送信者がいない
3. **無限ループ**: キャンセルシグナルをチェックしない
4. **WaitGroupの不一致**: Add/Doneの数が合わない

### 防止パターン

```go
// Good: selectでctx.Done()をチェック
func worker(ctx context.Context, out chan<- Result) {
    defer close(out)

    for {
        select {
        case <-ctx.Done():
            return // キャンセルされたら即座に終了
        case result := <-doWork():
            select {
            case <-ctx.Done():
                return
            case out <- result:
            }
        }
    }
}

// Bad: ctx.Done()をチェックしない
func worker(out chan<- Result) {
    for {
        out <- doWork() // 永遠にブロックする可能性
    }
}
```

### Watch API での実装

```go
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
    eventCh := make(chan Event)
    errCh := make(chan error)

    go func() {
        defer close(eventCh)
        defer close(errCh)

        // 初期化
        tailer, err := setupTailer(opts)
        if err != nil {
            sendError(ctx, errCh, err)
            return
        }
        defer tailer.Stop()

        for {
            select {
            case <-ctx.Done():
                return // 重要: キャンセル時に即座に終了

            case line, ok := <-tailer.Lines():
                if !ok {
                    return
                }

                event, err := parser.Parse(line)
                if err != nil {
                    sendError(ctx, errCh, err)
                    continue
                }

                if event != nil {
                    select {
                    case <-ctx.Done():
                        return
                    case eventCh <- *event:
                    }
                }
            }
        }
    }()

    return eventCh, errCh
}

// ノンブロッキングエラー送信
func sendError(ctx context.Context, errCh chan<- error, err error) {
    select {
    case <-ctx.Done():
    case errCh <- err:
    default:
        // エラーチャネルがブロックされている場合はドロップ
    }
}
```

---

## 3. goleak によるテスト

### 導入

```go
// go get go.uber.org/goleak

import "go.uber.org/goleak"

func TestMain(m *testing.M) {
    goleak.VerifyTestMain(m)
}
```

### 個別テストでの使用

```go
func TestWatch_Cancellation(t *testing.T) {
    defer goleak.VerifyNone(t)

    ctx, cancel := context.WithCancel(context.Background())
    events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

    // キャンセル
    cancel()

    // チャネルが閉じることを確認
    for range events {
    }
    for range errs {
    }

    // goleak がgoroutineリークをチェック
}
```

### vrclog-go での判断

**採用しない**（初期バージョン）

理由:
- 外部依存を増やしたくない
- 手動でgoroutine管理をしっかり行う
- 将来的に問題が発生したら導入を検討

代替策:
```go
func TestWatch_NoLeak(t *testing.T) {
    before := runtime.NumGoroutine()

    ctx, cancel := context.WithCancel(context.Background())
    events, _ := vrclog.Watch(ctx, vrclog.WatchOptions{})
    cancel()

    // チャネルを消費
    for range events {
    }

    // goroutineが増えていないことを確認
    time.Sleep(100 * time.Millisecond) // goroutineの終了を待つ
    after := runtime.NumGoroutine()

    if after > before+1 { // 多少の誤差は許容
        t.Errorf("goroutine leak: before=%d, after=%d", before, after)
    }
}
```

---

## 4. セマンティックバージョニング

### v0 (初期バージョン)

```
v0.1.0 - 初期リリース
v0.2.0 - 破壊的変更を含んでも良い
v0.3.0 - API変更
```

- 破壊的変更が許容される
- ユーザーは互換性の保証がないことを理解している
- 安定するまでv0を維持

### v1.0.0 への移行条件

1. APIが安定している
2. 実際のユースケースでテスト済み
3. ドキュメントが整備されている
4. 破壊的変更の予定がない

### vrclog-go の方針

```
v0.1.0 - 初期リリース（今回）
         - 3つのイベントタイプ
         - Watch API
         - ParseLine API

v0.x.x - 機能追加、バグ修正
         - 新イベントタイプ追加可能
         - API微調整可能

v1.0.0 - 安定版（将来）
         - 十分な実績ができてから
         - 破壊的変更なしを約束
```

---

## 5. ライブラリ設計の追加原則

### テスタビリティ

```go
// Good: 依存を注入可能
func watchWithSource(ctx context.Context, src lineSource) <-chan Event {
    // テスト時にモックを注入可能
}

// 内部インターフェース
type lineSource interface {
    Lines() <-chan string
}
```

### ドキュメント優先

1. 公開APIのドキュメントを先に書く
2. 使用例を先に考える
3. その後に実装する

```go
// 1. まずドキュメントを書く
// ParseLine parses a single VRChat log line into an Event.
//
// Return values:
//   - (*Event, nil): Successfully parsed event
//   - (nil, nil): Line doesn't match any known event pattern
//   - (nil, error): Line is malformed
func ParseLine(line string) (*Event, error)

// 2. 次に使用例を書く
func ExampleParseLine() {
    event, _ := ParseLine("...")
    fmt.Println(event.Type)
}

// 3. 最後に実装
```

### 一般的な操作を簡単に

```go
// Good: 一般的な使い方がシンプル
events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

// 特殊な設定が必要な場合も対応
events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{
    LogDir:         "/custom/path",
    PollInterval:   5 * time.Second,
    IncludeRawLine: true,
})
```

---

## 6. チャネルクローズの順序

### 正しい順序

```go
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
    eventCh := make(chan Event)
    errCh := make(chan error)

    go func() {
        // defer は逆順で実行される
        defer close(errCh)   // 2番目に閉じる
        defer close(eventCh) // 1番目に閉じる

        // ...処理...
    }()

    return eventCh, errCh
}
```

### 理由

1. イベントチャネルを先に閉じる
2. その後でエラーチャネルを閉じる
3. 受信側は両チャネルの終了を検出可能

---

## 7. バッファサイズの選択

### イベントチャネル

```go
// バッファなし（推奨）
eventCh := make(chan Event)
```

理由:
- バックプレッシャーを適用
- メモリを節約
- 受信側が処理できる速度で送信

### エラーチャネル

```go
// バッファなし + ノンブロッキング送信
errCh := make(chan error)

select {
case errCh <- err:
default:
    // 受信者がいなければドロップ
}
```

理由:
- エラーは致命的でなければドロップしても良い
- メインのイベント処理をブロックしない

---

## 8. Windows固有の考慮事項

### パス区切り文字

```go
// filepath.Join を使用（OS依存の区切り文字を使用）
path := filepath.Join(dir, "output_log_*.txt")

// Bad: ハードコード
path := dir + "\\output_log_*.txt"
```

### 環境変数展開

```go
// os.ExpandEnv または os.Getenv を使用
logDir := os.ExpandEnv("$LOCALAPPDATA\\VRChat\\VRChat")

// または
localAppData := os.Getenv("LOCALAPPDATA")
logDir := filepath.Join(localAppData, "VRChat", "VRChat")
```

### ファイルロック

- VRChatがログファイルを開いている間も読み取り可能
- nxadm/tail がこれを適切に処理

---

## 9. チェックリスト（実装時の確認事項）

### Goroutineリーク防止

- [ ] すべてのgoroutineがキャンセル可能か
- [ ] チャネル操作がすべてselectで囲まれているか
- [ ] deferでリソースクリーンアップしているか

### シグナル処理

- [ ] SIGINT/SIGTERMを適切に処理しているか
- [ ] シャットダウン時にリソースを解放しているか

### チャネル設計

- [ ] バッファサイズは適切か
- [ ] チャネルは必ずcloseされるか
- [ ] close済みチャネルへの送信を防いでいるか

### テスト

- [ ] キャンセレーションのテストがあるか
- [ ] 並行処理のrace conditionテストがあるか
- [ ] エッジケース（空ファイル、大きなファイル等）のテストがあるか

---

## 10. 将来の拡張ポイント

### 追加イベント

```go
// 将来追加可能なイベント
const (
    EventScreenshot EventType = "screenshot"
    EventAvatarChange EventType = "avatar_change"
    EventVideoPlayer EventType = "video_player"
)
```

### フィルタリング

```go
// 将来: イベントフィルタリング
type WatchOptions struct {
    // 既存
    LogDir string
    // ...

    // 将来追加
    EventTypes []EventType // 特定タイプのみ
    PlayerFilter func(string) bool // プレイヤー名フィルタ
}
```

### 複数ファイル監視

```go
// 将来: 複数ディレクトリの監視
func WatchMultiple(ctx context.Context, dirs []string) (<-chan Event, <-chan error)
```
