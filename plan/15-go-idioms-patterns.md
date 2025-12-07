# Go イディオムとパターン

## 参考資料

- [Dave Cheney: Functional options for friendly APIs](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
- [Go Blog: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [Accept Interfaces Return Structs](https://dev.to/shrsv/designing-go-apis-the-standard-library-way-accept-interfaces-return-structs-410k)
- [Three Dots Labs: Increasing Cohesion in Go](https://threedots.tech/post/increasing-cohesion-in-go-with-generic-decorators/)

---

## 1. Accept Interfaces, Return Structs

### 原則

> "Accept interfaces, return structs" - Jack Lindamood

**理由**:
- インターフェースを受け取る → 柔軟性、テスト容易性
- 構造体を返す → 拡張性、具体的な機能へのアクセス

### vrclog-goでの適用

```go
// Good: 構造体を返す
func ParseLine(line string) (*Event, error) {
    return &Event{...}, nil
}

// Good: 内部でインターフェースを受け取る（テスト用）
type lineSource interface {
    Lines() <-chan string
}

func watchWithSource(ctx context.Context, src lineSource) <-chan Event {
    // ...
}
```

### 適用しない場面

```go
// Bad: 不必要にインターフェースを返す
func ParseLine(line string) (EventInterface, error) {
    return &Event{...}, nil
}
```

---

## 2. Functional Options パターン

### 概要

複雑な設定を持つ関数のためのパターン。

```go
type Option func(*config)

func WithTimeout(d time.Duration) Option {
    return func(c *config) {
        c.timeout = d
    }
}

func NewClient(opts ...Option) *Client {
    cfg := defaultConfig()
    for _, opt := range opts {
        opt(cfg)
    }
    return &Client{cfg: cfg}
}
```

### vrclog-goでの判断

**採用しない**

理由:
- `WatchOptions` 構造体で十分
- オプションが少ない（3つ）
- KISS原則に従う
- 将来必要になれば追加

```go
// 現在の設計（シンプル）
type WatchOptions struct {
    LogDir         string
    PollInterval   time.Duration
    IncludeRawLine bool
}

// Functional Optionsは過剰
// func WithLogDir(dir string) WatchOption { ... }
```

### 将来の検討

オプションが5つ以上になった場合に検討:
- 設定の検証が複雑になった場合
- デフォルト値の計算が複雑になった場合

---

## 3. Pipeline パターン

### 概要

チャネルで接続されたステージの連鎖。

```go
func gen(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        for _, n := range nums {
            out <- n
        }
        close(out)
    }()
    return out
}

func sq(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in {
            out <- n * n
        }
        close(out)
    }()
    return out
}

// Usage: sq(sq(gen(1, 2, 3)))
```

### vrclog-goでの適用

```go
// Watch内部の簡略パイプライン
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
    eventCh := make(chan Event)
    errCh := make(chan error)

    go func() {
        defer close(eventCh)
        defer close(errCh)

        // Stage 1: ファイル検出
        logFile, err := findLatestLogFile(opts.LogDir)
        // ...

        // Stage 2: tail
        lines := tailer.Lines()

        // Stage 3: パース → 出力
        for line := range lines {
            event, err := parser.Parse(line)
            if event != nil {
                eventCh <- *event
            }
        }
    }()

    return eventCh, errCh
}
```

---

## 4. Fan-Out / Fan-In パターン

### 概要

- **Fan-Out**: 1つの入力を複数のワーカーに分散
- **Fan-In**: 複数の入力を1つに集約

### vrclog-goでの判断

**採用しない**

理由:
- 単一ログファイルの処理
- 並列化の必要がない
- 複雑さを増すだけ

### 将来の検討

複数ログファイルの同時監視が必要になった場合:

```go
// 将来の拡張例
func WatchMultiple(ctx context.Context, dirs []string) <-chan Event {
    var wg sync.WaitGroup
    out := make(chan Event)

    // Fan-out: 各ディレクトリ用のgoroutine
    for _, dir := range dirs {
        wg.Add(1)
        go func(d string) {
            defer wg.Done()
            events, _ := Watch(ctx, WatchOptions{LogDir: d})
            for e := range events {
                out <- e
            }
        }(dir)
    }

    // Fan-in: すべて完了後にclose
    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

---

## 5. Worker Pool パターン

### 概要

固定数のワーカーでタスクを処理。

```go
func worker(id int, jobs <-chan int, results chan<- int) {
    for j := range jobs {
        results <- j * 2
    }
}

func main() {
    jobs := make(chan int, 100)
    results := make(chan int, 100)

    // Start 3 workers
    for w := 1; w <= 3; w++ {
        go worker(w, jobs, results)
    }

    // Send jobs
    for j := 1; j <= 9; j++ {
        jobs <- j
    }
    close(jobs)
}
```

### vrclog-goでの判断

**採用しない**

理由:
- ログ行の処理は順序が重要
- I/Oバウンド（CPUバウンドではない）
- 並列化の恩恵が少ない

---

## 6. Context によるキャンセレーション

### 原則

- すべての長時間実行関数は `context.Context` を受け取る
- `ctx.Done()` を定期的にチェック
- キャンセル時は速やかに終了

### vrclog-goでの適用

```go
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
    eventCh := make(chan Event)
    errCh := make(chan error)

    go func() {
        defer close(eventCh)
        defer close(errCh)

        for {
            select {
            case <-ctx.Done():
                return // キャンセルされた
            case line, ok := <-tailer.Lines():
                if !ok {
                    return
                }
                // 処理
            }
        }
    }()

    return eventCh, errCh
}
```

### ベストプラクティス

```go
// Good: selectでctx.Done()をチェック
select {
case <-ctx.Done():
    return ctx.Err()
case ch <- value:
    // 送信成功
}

// Bad: ctx.Done()をチェックしない
ch <- value // ブロックする可能性
```

---

## 7. エラーチャネルパターン

### 概要

結果とエラーを別々のチャネルで返す。

### vrclog-goでの適用

```go
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
    eventCh := make(chan Event)
    errCh := make(chan error)

    go func() {
        defer close(eventCh)
        defer close(errCh)

        // 致命的エラー: 両チャネルをクローズ
        if err := fatalOperation(); err != nil {
            errCh <- err
            return
        }

        // 非致命的エラー: errChに送信して継続
        for line := range lines {
            event, err := parse(line)
            if err != nil {
                select {
                case errCh <- err:
                default:
                    // 受信者がいなければドロップ
                }
                continue
            }
            eventCh <- *event
        }
    }()

    return eventCh, errCh
}
```

---

## 8. defer によるリソース管理

### 原則

- リソース取得直後に `defer` でクリーンアップを登録
- 逆順で実行される

### vrclog-goでの適用

```go
func (t *Tail) Stop() error {
    close(t.stopCh)
    <-t.doneCh        // run()の終了を待機
    return t.t.Stop() // 基底のtailを停止
}

func (t *Tail) run() {
    defer close(t.doneCh)   // 最後にdoneCh をクローズ
    defer close(t.lines)    // その前にlines をクローズ

    for {
        select {
        case <-t.stopCh:
            return
        case line := <-t.t.Lines:
            // ...
        }
    }
}
```

---

## 9. ゼロ値の活用

### 原則

構造体のゼロ値を有用なデフォルトにする。

### vrclog-goでの適用

```go
type WatchOptions struct {
    LogDir         string        // "" → 自動検出
    PollInterval   time.Duration // 0 → 2秒
    IncludeRawLine bool          // false → 含めない
}

// 使用側
opts := WatchOptions{} // すべてデフォルト
opts := WatchOptions{LogDir: "/custom"} // 一部指定
```

### 内部での処理

```go
func Watch(ctx context.Context, opts WatchOptions) (...) {
    // ゼロ値にデフォルトを適用
    if opts.PollInterval == 0 {
        opts.PollInterval = 2 * time.Second
    }
    // ...
}
```

---

## 10. 小さなインターフェース

### 原則

> "The bigger the interface, the weaker the abstraction" - Rob Pike

Go標準ライブラリの平均: **2メソッド/インターフェース**

### vrclog-goでの適用

```go
// Good: 1-2メソッド
type lineSource interface {
    Lines() <-chan string
}

// Bad: 多すぎる
type tailerInterface interface {
    Lines() <-chan string
    Errors() <-chan error
    Stop() error
    Tell() (int64, error)
    Cleanup()
    // ...
}
```

---

## パターン適用判断表

| パターン | vrclog-goでの採用 | 理由 |
|---------|------------------|------|
| Accept Interfaces, Return Structs | ○ | 標準的なGoパターン |
| Functional Options | × | オプションが少ない |
| Pipeline | △ (簡略化) | 単純なデータフロー |
| Fan-Out/Fan-In | × | 単一ファイル処理 |
| Worker Pool | × | 順序が重要 |
| Context Cancellation | ○ | 必須 |
| Error Channel | ○ | 非同期エラー通知 |
| defer Resource Management | ○ | 必須 |
| Zero Value | ○ | API設計の基本 |
| Small Interfaces | ○ | Goの哲学 |
