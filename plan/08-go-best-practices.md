# Go ベストプラクティス

## 参考資料

- [Google Go Style Guide](https://google.github.io/styleguide/go/best-practices.html)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Wiki: CodeReviewComments](https://go.dev/wiki/CodeReviewComments)
- [Dave Cheney's Blog](https://dave.cheney.net/)
- [golang-standards/project-layout](https://github.com/golang-standards/project-layout)

---

## 1. Zero Value を有用に

### 原則

構造体のゼロ値がそのまま有効なデフォルトとして機能するよう設計する。

### 例

```go
// Good: ゼロ値が有用
type WatchOptions struct {
    LogDir       string        // "" = 自動検出
    PollInterval time.Duration // 0 = デフォルト2秒
}

// 使用側はシンプルに
opts := vrclog.WatchOptions{} // すべてデフォルト
opts := vrclog.WatchOptions{LogDir: "/custom/path"} // 一部だけ指定

// Bad: ゼロ値が無効
type BadOptions struct {
    PollInterval time.Duration // 0は無効な値
    Required     string        // 必須フィールド
}
```

### 実装側での処理

```go
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
    // ゼロ値にデフォルトを適用
    if opts.PollInterval == 0 {
        opts.PollInterval = 2 * time.Second
    }
    // ...
}
```

---

## 2. Context First

### 原則

`context.Context` は常に最初の引数。

### 例

```go
// Good
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error)
func ParseLine(line string) (*Event, error) // context不要な関数は除く

// Bad
func Watch(opts WatchOptions, ctx context.Context) (<-chan Event, <-chan error)
```

### Google の推奨

> At Google, we require that Go programmers pass a Context parameter as the
> first argument to every function on the call path between incoming and
> outgoing requests.

---

## 3. インターフェースは使用側で定義

### 原則

インターフェースは実装側ではなく、使用側で定義する。

### 例

```go
// Bad: 実装側でインターフェースを定義
package tailer

type Tailer interface {
    Lines() <-chan string
    Stop() error
}

type fileTailer struct { ... }

func New() Tailer { return &fileTailer{} }

// Good: 使用側で必要なインターフェースを定義
package watch

// 使用する機能だけを要求
type lineSource interface {
    Lines() <-chan string
}

func watchWithSource(src lineSource) { ... }
```

### 理由

- 依存関係が逆転する
- テスト時にモックしやすい
- 不要なメソッドを公開しない

---

## 4. 小さなインターフェース

### 原則

1つまたは少数のメソッドを持つインターフェースを好む。

### 例

```go
// Good: 小さなインターフェース
type Reader interface {
    Read(p []byte) (n int, err error)
}

// Bad: 大きなインターフェース
type AllInOne interface {
    Read(p []byte) (n int, err error)
    Write(p []byte) (n int, err error)
    Close() error
    Seek(offset int64, whence int) (int64, error)
    // ... more methods
}
```

### 標準ライブラリの例

- `io.Reader` - 1メソッド
- `io.Writer` - 1メソッド
- `io.Closer` - 1メソッド
- `io.ReadWriteCloser` - 3メソッド（合成）

---

## 5. パッケージ命名

### 原則

- パッケージ名は短く、意味のある名詞
- 汎用的な名前（`util`, `common`, `misc`）を避ける
- 呼び出し時の読みやすさを考慮

### 例

```go
// Good
package vrclog
package parser
package logfinder

// Bad
package util
package common
package vrclogutils
```

### 呼び出し時の読みやすさ

```go
// Good: パッケージ名.関数名 が自然に読める
vrclog.Watch(ctx, opts)
logfinder.FindLogDir("")

// Bad: 冗長な命名
vrclog.VrclogWatch(ctx, opts)
logfinder.FindLogFinderDir("")
```

---

## 6. エクスポートを最小限に

### 原則

- 必要なものだけをエクスポート
- 内部実装は `internal/` に配置
- 型より関数を優先してエクスポート

### 例

```go
// Good: 必要最小限のエクスポート
package vrclog

// エクスポート: 公開API
func Watch(...) { ... }
func ParseLine(...) { ... }
type Event struct { ... }
type EventType string
var ErrLogDirNotFound = errors.New(...)

// 非エクスポート: 内部実装
func applyDefaults(...) { ... }
type watcher struct { ... }
```

---

## 7. 命名規則

### 変数名

```go
// Good: 短く明確
for i, v := range items { }
ctx, cancel := context.WithCancel(...)
opts := WatchOptions{}

// Bad: 冗長
for index, value := range items { }
context, cancelFunc := context.WithCancel(...)
watchOptions := WatchOptions{}
```

### レシーバー名

```go
// Good: 短い名前（通常1-2文字）
func (t *Tail) Lines() <-chan string { ... }
func (e *Event) String() string { ... }

// Bad: 冗長な名前
func (tail *Tail) Lines() <-chan string { ... }
func (event *Event) String() string { ... }
```

### 定数・変数名

```go
// Good
const maxRetries = 3
var ErrNotFound = errors.New("not found")

// Bad
const MAX_RETRIES = 3  // Goはキャメルケース
var errNotFound = errors.New("not found")  // エクスポートするならErr
```

---

## 8. コメント

### パッケージコメント

```go
// Package vrclog provides parsing and monitoring of VRChat log files.
//
// This package allows you to:
//   - Parse VRChat log lines into structured events
//   - Monitor log files in real-time for new events
package vrclog
```

### 関数コメント

```go
// Watch monitors VRChat log files and returns events through channels.
//
// The function returns two channels:
//   - events: receives parsed Event structs
//   - errors: receives non-fatal errors
//
// Both channels are closed when ctx is cancelled.
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error)
```

### 不要なコメントを避ける

```go
// Bad: コードの繰り返し
// increments i by 1
i++

// Good: コメント不要（自明）
i++

// Good: 「なぜ」を説明
// Skip the first line as it's a header
i++
```

---

## 9. 並行処理

### goroutine のライフサイクル管理

```go
// Good: goroutine の終了を管理
func Watch(ctx context.Context, ...) {
    done := make(chan struct{})

    go func() {
        defer close(done)
        for {
            select {
            case <-ctx.Done():
                return
            case line := <-lines:
                // process
            }
        }
    }()

    return events, errors
}

// Bad: goroutine がリークする可能性
func Watch(...) {
    go func() {
        for {
            line := <-lines  // ブロックし続ける可能性
            // process
        }
    }()
}
```

### チャネルのクローズ

```go
// 送信側がクローズする
func producer() <-chan int {
    ch := make(chan int)
    go func() {
        defer close(ch)  // 送信側でクローズ
        for i := 0; i < 10; i++ {
            ch <- i
        }
    }()
    return ch
}

// 受信側はクローズしない
for v := range producer() {
    // process v
}
```

---

## 10. テスト

### テーブル駆動テスト

```go
func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *Event
        wantErr bool
    }{
        {name: "case1", input: "...", want: &Event{...}},
        {name: "case2", input: "...", wantErr: true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 並列テスト

```go
for _, tt := range tests {
    tt := tt  // capture range variable
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // ...
    })
}
```

### t.Helper()

```go
func assertEqual(t *testing.T, got, want interface{}) {
    t.Helper()  // エラー時に呼び出し元の行番号を表示
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

---

## 11. 依存関係

### 最小限の依存

```go
// このプロジェクトの依存
require (
    github.com/spf13/cobra v1.8.0    // CLI必須
    github.com/nxadm/tail v1.4.11    // tail必須
)

// 追加しない
// github.com/stretchr/testify  // 標準ライブラリで十分
// github.com/pkg/errors        // Go 1.13+ は標準で十分
```

### 理由

- 依存が少ない = 保守が容易
- セキュリティリスクの低減
- ビルド時間の短縮

---

## 12. プロジェクト構造

### cmd/

```
cmd/
└── vrclog/
    ├── main.go   # エントリポイント
    └── tail.go   # サブコマンド
```

- `main.go` は最小限
- ロジックは `pkg/` または `internal/` へ

### pkg/ vs internal/

```
pkg/vrclog/      # 外部に公開するAPI
internal/parser/ # 内部実装
```

- `pkg/`: 安定したAPI、後方互換性を維持
- `internal/`: 自由に変更可能

---

## まとめチェックリスト

- [ ] ゼロ値が有用か？
- [ ] Context は最初の引数か？
- [ ] インターフェースは使用側で定義しているか？
- [ ] パッケージ名は適切か？
- [ ] エクスポートは最小限か？
- [ ] goroutine のライフサイクルは管理されているか？
- [ ] テストはテーブル駆動か？
- [ ] 依存関係は最小限か？
