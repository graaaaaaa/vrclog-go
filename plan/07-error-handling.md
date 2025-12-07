# エラーハンドリング方針

## 基本原則

### 1. エラーはラップして返す

```go
if err != nil {
    return fmt.Errorf("finding log directory: %w", err)
}
```

- `%w` でオリジナルエラーを保持
- 呼び出し元で `errors.Is()` / `errors.As()` が使える
- エラーチェーンでコンテキストを追加

### 2. センチネルエラーは最小限に

```go
var (
    ErrLogDirNotFound = errors.New("vrclog: log directory not found")
    ErrNoLogFiles     = errors.New("vrclog: no log files found")
)
```

- 公開APIで判定が必要なエラーのみ
- パッケージプレフィックス付き（`vrclog:`）
- `Err` プレフィックスで始める

### 3. 内部エラーはラップのみ

```go
// 内部パッケージでは新しいセンチネルエラーを定義しない
// 必要な情報はラップで追加
return fmt.Errorf("parsing timestamp %q: %w", line[:19], err)
```

---

## エラー設計

### 公開エラー (pkg/vrclog/errors.go)

```go
package vrclog

import "errors"

// Sentinel errors for this package.
// Use errors.Is() to check for these errors.
var (
    // ErrLogDirNotFound is returned when the VRChat log directory
    // cannot be found or does not contain log files.
    //
    // This can happen when:
    //   - VRChat is not installed
    //   - The log directory was moved or deleted
    //   - The specified path is invalid
    ErrLogDirNotFound = errors.New("vrclog: log directory not found")

    // ErrNoLogFiles is returned when the log directory exists
    // but contains no output_log files.
    //
    // This can happen when:
    //   - VRChat has never been run
    //   - Log files were manually deleted
    ErrNoLogFiles = errors.New("vrclog: no log files found")
)
```

### エラーの使用例

```go
events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

// エラーチャネルから受信
for err := range errs {
    if errors.Is(err, vrclog.ErrLogDirNotFound) {
        log.Fatal("VRChat log directory not found. Is VRChat installed?")
    }
    if errors.Is(err, vrclog.ErrNoLogFiles) {
        log.Println("No log files found. Has VRChat been run?")
        continue
    }
    // その他のエラー
    log.Printf("error: %v", err)
}
```

---

## エラーラップのパターン

### パターン1: 単純なラップ

```go
func FindLogDir(explicit string) (string, error) {
    if explicit != "" {
        if !isValidLogDir(explicit) {
            return "", fmt.Errorf("%w: %s", ErrLogDirNotFound, explicit)
        }
        return explicit, nil
    }
    // ...
}
```

### パターン2: コンテキスト追加

```go
func FindLatestLogFile(dir string) (string, error) {
    matches, err := filepath.Glob(filepath.Join(dir, "output_log_*.txt"))
    if err != nil {
        return "", fmt.Errorf("globbing log files in %s: %w", dir, err)
    }
    if len(matches) == 0 {
        return "", fmt.Errorf("%w in %s", ErrNoLogFiles, dir)
    }
    // ...
}
```

### パターン3: 操作の説明

```go
func (t *Tail) Stop() error {
    if err := t.t.Stop(); err != nil {
        return fmt.Errorf("stopping tail: %w", err)
    }
    return nil
}
```

---

## エラー vs nil, nil

### ParseLine の戻り値パターン

| 状況 | event | error | 説明 |
|------|-------|-------|------|
| パース成功 | non-nil | nil | 正常なイベント |
| 認識されない行 | nil | nil | スキップすべき行 |
| パースエラー | nil | non-nil | 異常な行 |

```go
func ParseLine(line string) (*Event, error) {
    // 認識されないパターンはエラーではない
    if !containsEventPattern(line) {
        return nil, nil
    }

    // パースを試みる
    event, err := parse(line)
    if err != nil {
        // パース中のエラーは報告
        return nil, fmt.Errorf("parsing line: %w", err)
    }

    return event, nil
}
```

### 呼び出し側での処理

```go
for _, line := range lines {
    event, err := vrclog.ParseLine(line)
    if err != nil {
        log.Printf("parse error: %v", err)
        continue
    }
    if event == nil {
        continue // 認識されない行、スキップ
    }
    // イベントを処理
    processEvent(event)
}
```

---

## Watch API のエラーハンドリング

### 致命的エラー vs 非致命的エラー

```go
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
    eventCh := make(chan Event)
    errCh := make(chan error)

    go func() {
        defer close(eventCh)
        defer close(errCh)

        // 致命的エラー: ログディレクトリが見つからない
        dir, err := logfinder.FindLogDir(opts.LogDir)
        if err != nil {
            errCh <- err
            return // 両チャネルを閉じて終了
        }

        // ファイル監視ループ
        for {
            select {
            case <-ctx.Done():
                return
            case line := <-tailer.Lines():
                event, err := parser.Parse(line)
                if err != nil {
                    // 非致命的エラー: パースエラー
                    select {
                    case errCh <- err:
                    default:
                        // エラーチャネルがブロックされている場合はスキップ
                    }
                    continue
                }
                if event != nil {
                    eventCh <- *event
                }
            }
        }
    }()

    return eventCh, errCh
}
```

### エラーチャネルの設計

```go
// バッファなしチャネル
errCh := make(chan error)

// 送信時のブロック回避
select {
case errCh <- err:
default:
    // 受信者がいない場合はドロップ
}
```

---

## CLI でのエラー表示

### 標準エラー出力

```go
func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

### ユーザーフレンドリーなメッセージ

```go
func runTail(cmd *cobra.Command, args []string) error {
    events, errs := vrclog.Watch(ctx, opts)

    for {
        select {
        case err := <-errs:
            if errors.Is(err, vrclog.ErrLogDirNotFound) {
                return fmt.Errorf("VRChat log directory not found.\n" +
                    "Make sure VRChat is installed and has been run at least once.\n" +
                    "You can also specify the directory with --log-dir flag.")
            }
            if verbose {
                fmt.Fprintf(os.Stderr, "warning: %v\n", err)
            }
        // ...
        }
    }
}
```

---

## panic の使用

### 使用してよい場合

```go
// テストヘルパー内でのみ
func mustParseTime(s string) time.Time {
    t, err := time.ParseInLocation(layout, s, time.Local)
    if err != nil {
        panic(err)
    }
    return t
}
```

### 使用しない場合

- 公開API
- ランタイムで回復可能なエラー
- ユーザー入力の検証

```go
// 悪い例
func ParseLine(line string) *Event {
    if line == "" {
        panic("empty line") // NG: エラーを返すべき
    }
    // ...
}

// 良い例
func ParseLine(line string) (*Event, error) {
    if line == "" {
        return nil, nil // 空行はスキップ
    }
    // ...
}
```

---

## エラーメッセージのスタイル

### 小文字で始める

```go
// Good
fmt.Errorf("finding log directory: %w", err)

// Bad
fmt.Errorf("Finding log directory: %w", err)
```

### 句読点を付けない

```go
// Good
errors.New("vrclog: log directory not found")

// Bad
errors.New("vrclog: log directory not found.")
```

### 具体的な情報を含める

```go
// Good
fmt.Errorf("parsing timestamp %q: %w", line[:19], err)

// Bad
fmt.Errorf("parse error: %w", err)
```

---

## errors.Is と errors.As

### errors.Is の使用

```go
// センチネルエラーのチェック
if errors.Is(err, vrclog.ErrLogDirNotFound) {
    // ログディレクトリが見つからない
}

// ラップされていても検出可能
wrapped := fmt.Errorf("watch failed: %w", vrclog.ErrLogDirNotFound)
errors.Is(wrapped, vrclog.ErrLogDirNotFound) // true
```

### errors.As の使用（カスタムエラー型）

```go
// 将来的に必要になった場合
type ParseError struct {
    Line   string
    Reason string
}

func (e *ParseError) Error() string {
    return fmt.Sprintf("parse error at %q: %s", e.Line, e.Reason)
}

// 使用例
var parseErr *ParseError
if errors.As(err, &parseErr) {
    log.Printf("failed to parse: %s", parseErr.Line)
}
```

---

## まとめ

| 場面 | 方針 |
|------|------|
| 公開API | センチネルエラーを定義 |
| 内部実装 | `fmt.Errorf` でラップ |
| 認識されない入力 | `(nil, nil)` を返す |
| パースエラー | `(nil, error)` を返す |
| 致命的エラー | チャネルを閉じる |
| 非致命的エラー | errチャネルに送信 |
| panic | テストヘルパーのみ |
