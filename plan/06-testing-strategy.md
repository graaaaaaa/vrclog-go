# テスト戦略

## 基本方針

- **標準ライブラリのみ**: testify は使用しない（依存を最小限に）
- **テーブル駆動テスト**: 網羅的かつ保守しやすいテスト
- **サブテスト**: `t.Run()` で個別テストケースを分離
- **並列実行**: 可能な限り `t.Parallel()` を使用
- **race detector**: `go test -race` で競合検出

---

## テストファイル構成

```
pkg/vrclog/
└── vrclog_test.go        # 公開APIのテスト（Watcher統合テスト含む）

internal/
├── parser/
│   └── parser_test.go    # パーサーのテーブル駆動テスト + Fuzzテスト
├── logfinder/
│   └── finder_test.go    # ファイル検出のテスト
└── tailer/
    └── tailer_test.go    # tailerの統合テスト（Codex推奨）

testdata/
└── logs/
    └── sample.txt        # テスト用サンプルログ
```

---

## パーサーテスト (parser_test.go)

### テーブル駆動テストの基本形

```go
package parser

import (
    "testing"
    "time"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestParse(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *vrclog.Event
        wantErr bool
    }{
        // 正常系: プレイヤー参加
        {
            name:  "player join without ID",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
            want: &vrclog.Event{
                Type:       vrclog.EventPlayerJoin,
                Timestamp:  mustParseTime("2024.01.15 23:59:59"),
                PlayerName: "TestUser",
            },
        },
        {
            name:  "player join with ID",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser (usr_12345678-1234-1234-1234-123456789abc)",
            want: &vrclog.Event{
                Type:       vrclog.EventPlayerJoin,
                Timestamp:  mustParseTime("2024.01.15 23:59:59"),
                PlayerName: "TestUser",
                PlayerID:   "usr_12345678-1234-1234-1234-123456789abc",
            },
        },

        // 正常系: プレイヤー退出
        {
            name:  "player left",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser",
            want: &vrclog.Event{
                Type:       vrclog.EventPlayerLeft,
                Timestamp:  mustParseTime("2024.01.15 23:59:59"),
                PlayerName: "TestUser",
            },
        },

        // 正常系: ワールド入室
        {
            name:  "entering room",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World",
            want: &vrclog.Event{
                Type:      vrclog.EventWorldJoin,
                Timestamp: mustParseTime("2024.01.15 23:59:59"),
                WorldName: "Test World",
            },
        },
        {
            name:  "joining world with instance",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] Joining wrld_12345678-1234-1234-1234-123456789abc:12345~region(us)",
            want: &vrclog.Event{
                Type:       vrclog.EventWorldJoin,
                Timestamp:  mustParseTime("2024.01.15 23:59:59"),
                WorldID:    "wrld_12345678-1234-1234-1234-123456789abc",
                InstanceID: "12345~region(us)",
            },
        },

        // スキップ系: 認識されない行
        {
            name:    "unrecognized line",
            input:   "2024.01.15 23:59:59 Log        -  [Network] Connected",
            want:    nil,
            wantErr: false,
        },
        {
            name:    "empty line",
            input:   "",
            want:    nil,
            wantErr: false,
        },
        {
            name:    "exclusion: OnPlayerLeftRoom",
            input:   "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeftRoom",
            want:    nil,
            wantErr: false,
        },
        {
            name:    "exclusion: Joining or Creating",
            input:   "2024.01.15 23:59:59 Log        -  [Behaviour] Joining or Creating Room",
            want:    nil,
            wantErr: false,
        },

        // 境界値
        {
            name:  "player name with spaces",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined Test User Name",
            want: &vrclog.Event{
                Type:       vrclog.EventPlayerJoin,
                Timestamp:  mustParseTime("2024.01.15 23:59:59"),
                PlayerName: "Test User Name",
            },
        },
        {
            name:  "world name with special chars",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test [World] (v1.0)",
            want: &vrclog.Event{
                Type:      vrclog.EventWorldJoin,
                Timestamp: mustParseTime("2024.01.15 23:59:59"),
                WorldName: "Test [World] (v1.0)",
            },
        },
        {
            name:  "japanese player name",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined テストユーザー",
            want: &vrclog.Event{
                Type:       vrclog.EventPlayerJoin,
                Timestamp:  mustParseTime("2024.01.15 23:59:59"),
                PlayerName: "テストユーザー",
            },
        },

        // Windows CRLF対応（Codex推奨）
        {
            name:  "CRLF line ending",
            input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser\r",
            want: &vrclog.Event{
                Type:       vrclog.EventPlayerJoin,
                Timestamp:  mustParseTime("2024.01.15 23:59:59"),
                PlayerName: "TestUser",
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Parse(tt.input)

            // Check error
            if (err != nil) != tt.wantErr {
                t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            // Check result
            if !eventEqual(got, tt.want) {
                t.Errorf("Parse() = %+v, want %+v", got, tt.want)
            }
        })
    }
}

// Helper functions

func mustParseTime(s string) time.Time {
    t, err := time.ParseInLocation("2006.01.02 15:04:05", s, time.Local)
    if err != nil {
        panic(err)
    }
    return t
}

func eventEqual(a, b *vrclog.Event) bool {
    if a == nil && b == nil {
        return true
    }
    if a == nil || b == nil {
        return false
    }
    return a.Type == b.Type &&
        a.Timestamp.Equal(b.Timestamp) &&
        a.PlayerName == b.PlayerName &&
        a.PlayerID == b.PlayerID &&
        a.WorldID == b.WorldID &&
        a.WorldName == b.WorldName &&
        a.InstanceID == b.InstanceID
}
```

### 並列テストの実行

```go
func TestParse_Parallel(t *testing.T) {
    tests := []struct {
        name  string
        input string
        want  *vrclog.Event
    }{
        // ... test cases
    }

    for _, tt := range tests {
        tt := tt // capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // 並列実行を有効化

            got, _ := Parse(tt.input)
            if !eventEqual(got, tt.want) {
                t.Errorf("Parse() = %+v, want %+v", got, tt.want)
            }
        })
    }
}
```

### Fuzzテスト（Codex推奨）

Go 1.18以降のFuzz機能を使用してパーサーの堅牢性をテスト。

```go
func FuzzParse(f *testing.F) {
    // シードコーパス（既知の有効入力）
    f.Add("2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser")
    f.Add("2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser")
    f.Add("2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World")
    f.Add("")
    f.Add("invalid line")
    f.Add("2024.01.15 23:59:59 Log        -  [Network] Connected")

    f.Fuzz(func(t *testing.T, line string) {
        // パニックしないことを確認
        _, _ = Parse(line)
    })
}
```

実行方法:
```bash
# Fuzzテストを10秒間実行
go test -fuzz=FuzzParse -fuzztime=10s ./internal/parser/

# クラッシュを発見した場合、testdata/fuzz/に保存される
```

---

## tailerテスト（tailer_test.go）（Codex推奨）

nxadm/tailのラッパーをテスト。実際のファイル操作を使用。

```go
package tailer

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func TestTailer_NewLines(t *testing.T) {
    dir := t.TempDir()
    logFile := filepath.Join(dir, "test.log")

    // ファイル作成
    f, err := os.Create(logFile)
    if err != nil {
        t.Fatal(err)
    }
    defer f.Close()

    // tailer開始
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    tailer, err := New(ctx, logFile)
    if err != nil {
        t.Fatal(err)
    }
    defer tailer.Stop()

    // 行を書き込み
    f.WriteString("line1\n")
    f.Sync()

    // 受信確認
    select {
    case line := <-tailer.Lines():
        if line != "line1" {
            t.Errorf("got %q, want %q", line, "line1")
        }
    case <-time.After(2 * time.Second):
        t.Error("timeout waiting for line")
    }
}

func TestTailer_FileNotExist_ThenCreated(t *testing.T) {
    dir := t.TempDir()
    logFile := filepath.Join(dir, "test.log")

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // ファイルが存在しない状態でtailer開始
    tailer, err := New(ctx, logFile)
    if err != nil {
        t.Fatal(err)
    }
    defer tailer.Stop()

    // ファイルを作成して書き込み
    f, err := os.Create(logFile)
    if err != nil {
        t.Fatal(err)
    }
    defer f.Close()

    f.WriteString("line1\n")
    f.Sync()

    // 受信確認
    select {
    case line := <-tailer.Lines():
        if line != "line1" {
            t.Errorf("got %q, want %q", line, "line1")
        }
    case <-time.After(5 * time.Second):
        t.Error("timeout waiting for line after file creation")
    }
}

func TestTailer_ErrorPropagation(t *testing.T) {
    // line.Errがerrorsチャネルに伝播することをテスト
    // （実際のエラーを発生させるのは難しいため、統合テストで確認）
}

func TestTailer_Stop(t *testing.T) {
    dir := t.TempDir()
    logFile := filepath.Join(dir, "test.log")

    f, err := os.Create(logFile)
    if err != nil {
        t.Fatal(err)
    }
    f.Close()

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    tailer, err := New(ctx, logFile)
    if err != nil {
        t.Fatal(err)
    }

    // Stop呼び出し後、チャネルがcloseされることを確認
    tailer.Stop()

    select {
    case _, ok := <-tailer.Lines():
        if ok {
            t.Error("expected Lines channel to be closed")
        }
    case <-time.After(time.Second):
        t.Error("timeout waiting for Lines channel to close")
    }
}
```

---

## logfinder テスト (finder_test.go)

```go
package logfinder

import (
    "os"
    "path/filepath"
    "testing"
)

func TestFindLatestLogFile(t *testing.T) {
    // Create temp directory
    dir := t.TempDir()

    // Create test log files
    files := []string{
        "output_log_2024-01-01_00-00-00.txt",
        "output_log_2024-01-02_00-00-00.txt",
        "output_log_2024-01-03_00-00-00.txt",
    }

    for i, name := range files {
        path := filepath.Join(dir, name)
        if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
            t.Fatal(err)
        }
        // Set modification time (oldest first)
        // Note: actual implementation uses ModTime, not filename
    }

    // Test
    got, err := FindLatestLogFile(dir)
    if err != nil {
        t.Fatalf("FindLatestLogFile() error = %v", err)
    }

    // Should return the most recently modified file
    if filepath.Base(got) != files[len(files)-1] {
        t.Errorf("FindLatestLogFile() = %v, want %v", got, files[len(files)-1])
    }
}

func TestFindLatestLogFile_NoFiles(t *testing.T) {
    dir := t.TempDir()

    _, err := FindLatestLogFile(dir)
    if err == nil {
        t.Error("FindLatestLogFile() expected error for empty directory")
    }
}

func TestFindLogDir_EnvVar(t *testing.T) {
    // Create temp directory with log file
    dir := t.TempDir()
    logFile := filepath.Join(dir, "output_log_test.txt")
    if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
        t.Fatal(err)
    }

    // Set environment variable
    oldVal := os.Getenv(EnvLogDir)
    os.Setenv(EnvLogDir, dir)
    defer os.Setenv(EnvLogDir, oldVal)

    // Test
    got, err := FindLogDir("")
    if err != nil {
        t.Fatalf("FindLogDir() error = %v", err)
    }
    if got != dir {
        t.Errorf("FindLogDir() = %v, want %v", got, dir)
    }
}

func TestFindLogDir_Explicit(t *testing.T) {
    // Create temp directory with log file
    dir := t.TempDir()
    logFile := filepath.Join(dir, "output_log_test.txt")
    if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
        t.Fatal(err)
    }

    // Explicit should take priority over env
    os.Setenv(EnvLogDir, "/some/other/path")
    defer os.Unsetenv(EnvLogDir)

    got, err := FindLogDir(dir)
    if err != nil {
        t.Fatalf("FindLogDir() error = %v", err)
    }
    if got != dir {
        t.Errorf("FindLogDir() = %v, want %v", got, dir)
    }
}
```

---

## 公開APIテスト (vrclog_test.go)

### ParseLineテスト

```go
package vrclog_test

import (
    "testing"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestParseLine(t *testing.T) {
    tests := []struct {
        name       string
        input      string
        wantType   vrclog.EventType
        wantNil    bool
    }{
        {
            name:     "player join",
            input:    "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
            wantType: vrclog.EventPlayerJoin,
        },
        {
            name:    "unrecognized line returns nil",
            input:   "some random text",
            wantNil: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := vrclog.ParseLine(tt.input)
            if err != nil {
                t.Fatalf("ParseLine() error = %v", err)
            }

            if tt.wantNil {
                if got != nil {
                    t.Errorf("ParseLine() = %+v, want nil", got)
                }
                return
            }

            if got == nil {
                t.Fatal("ParseLine() = nil, want non-nil")
            }
            if got.Type != tt.wantType {
                t.Errorf("ParseLine().Type = %v, want %v", got.Type, tt.wantType)
            }
        })
    }
}

// Example as documentation
func ExampleParseLine() {
    line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser"
    event, err := vrclog.ParseLine(line)
    if err != nil {
        panic(err)
    }
    if event != nil {
        // event.Type == vrclog.EventPlayerJoin
        // event.PlayerName == "TestUser"
    }
}
```

### Watcher統合テスト（Codex推奨）

```go
package vrclog_test

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestNewWatcher_InvalidLogDir(t *testing.T) {
    _, err := vrclog.NewWatcher(vrclog.WatchOptions{
        LogDir: "/nonexistent/path",
    })
    if err == nil {
        t.Error("NewWatcher() expected error for invalid log dir")
    }
}

func TestNewWatcher_ValidLogDir(t *testing.T) {
    dir := t.TempDir()
    // ログファイルを作成
    logFile := filepath.Join(dir, "output_log_test.txt")
    if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
        t.Fatal(err)
    }

    watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
        LogDir: dir,
    })
    if err != nil {
        t.Fatalf("NewWatcher() error = %v", err)
    }
    defer watcher.Close()
}

func TestWatcher_ReceivesEvents(t *testing.T) {
    dir := t.TempDir()
    logFile := filepath.Join(dir, "output_log_test.txt")

    f, err := os.Create(logFile)
    if err != nil {
        t.Fatal(err)
    }
    defer f.Close()

    watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
        LogDir: dir,
    })
    if err != nil {
        t.Fatal(err)
    }
    defer watcher.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    events, errs := watcher.Watch(ctx)

    // ログ行を書き込み
    f.WriteString("2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser\n")
    f.Sync()

    // イベント受信確認
    select {
    case event := <-events:
        if event.Type != vrclog.EventPlayerJoin {
            t.Errorf("got type %v, want %v", event.Type, vrclog.EventPlayerJoin)
        }
        if event.PlayerName != "TestUser" {
            t.Errorf("got player %q, want %q", event.PlayerName, "TestUser")
        }
    case err := <-errs:
        t.Fatalf("unexpected error: %v", err)
    case <-ctx.Done():
        t.Fatal("timeout waiting for event")
    }
}

func TestWatcher_ContextCancel(t *testing.T) {
    dir := t.TempDir()
    logFile := filepath.Join(dir, "output_log_test.txt")

    if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
        t.Fatal(err)
    }

    watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
        LogDir: dir,
    })
    if err != nil {
        t.Fatal(err)
    }
    defer watcher.Close()

    ctx, cancel := context.WithCancel(context.Background())
    events, _ := watcher.Watch(ctx)

    // コンテキストをキャンセル
    cancel()

    // チャネルがcloseされることを確認
    select {
    case _, ok := <-events:
        if ok {
            t.Error("expected events channel to be closed")
        }
    case <-time.After(2 * time.Second):
        t.Error("timeout waiting for events channel to close")
    }
}

func TestWatcher_Close(t *testing.T) {
    dir := t.TempDir()
    logFile := filepath.Join(dir, "output_log_test.txt")

    if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
        t.Fatal(err)
    }

    watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
        LogDir: dir,
    })
    if err != nil {
        t.Fatal(err)
    }

    // Close()を複数回呼んでも安全であることを確認
    if err := watcher.Close(); err != nil {
        t.Errorf("first Close() error = %v", err)
    }
    if err := watcher.Close(); err != nil {
        t.Errorf("second Close() error = %v", err)
    }
}

func TestWatchOptions_Validate(t *testing.T) {
    tests := []struct {
        name    string
        opts    vrclog.WatchOptions
        wantErr bool
    }{
        {
            name:    "zero value is valid",
            opts:    vrclog.WatchOptions{},
            wantErr: false,
        },
        {
            name: "ReplayLastN is valid",
            opts: vrclog.WatchOptions{
                Replay: vrclog.ReplayConfig{
                    Mode:  vrclog.ReplayLastN,
                    LastN: 100,
                },
            },
            wantErr: false,
        },
        {
            name: "ReplaySinceTime is valid",
            opts: vrclog.WatchOptions{
                Replay: vrclog.ReplayConfig{
                    Mode:  vrclog.ReplaySinceTime,
                    Since: time.Now().Add(-time.Hour),
                },
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.opts.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestWatch_ConvenienceFunction(t *testing.T) {
    dir := t.TempDir()
    logFile := filepath.Join(dir, "output_log_test.txt")

    if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
        t.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{
        LogDir: dir,
    })
    if err != nil {
        t.Fatalf("Watch() error = %v", err)
    }

    // チャネルが返されることを確認
    if events == nil {
        t.Error("Watch() events channel is nil")
    }
    if errs == nil {
        t.Error("Watch() errs channel is nil")
    }

    cancel() // クリーンアップ
}
```

---

## テストデータ (testdata/logs/sample.txt)

```
2024.01.15 12:00:00 Log        -  [Behaviour] Entering Room: Test World Name
2024.01.15 12:00:01 Log        -  [Behaviour] Joining wrld_12345678-1234-1234-1234-123456789abc:12345~region(us)
2024.01.15 12:00:05 Log        -  [Behaviour] OnPlayerJoined TestUser1
2024.01.15 12:00:06 Log        -  [Behaviour] OnPlayerJoined TestUser2 (usr_aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee)
2024.01.15 12:05:00 Log        -  [Behaviour] OnPlayerLeft TestUser1
2024.01.15 12:10:00 Log        -  [Network] Some network message
2024.01.15 12:15:00 Warning    -  [Avatar] Avatar warning message
2024.01.15 12:20:00 Log        -  [Behaviour] OnPlayerLeftRoom
2024.01.15 12:25:00 Log        -  [Behaviour] Joining or Creating Room
```

---

## race detector テスト

```bash
# 全テストを race detector 付きで実行
go test -race ./...

# 特定パッケージのみ
go test -race ./internal/parser/...
```

### Watch API の並行処理テスト

```go
func TestWatch_Concurrent(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping in short mode")
    }

    // この種のテストは実際のファイルが必要なため
    // 統合テスト または モックを使用
}
```

---

## テスト実行コマンド

```bash
# 全テスト実行
go test ./...

# 詳細出力
go test -v ./...

# race detector 付き
go test -race ./...

# カバレッジ
go test -cover ./...

# カバレッジレポート生成
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 特定のテストのみ実行
go test -run TestParse ./internal/parser/

# ベンチマーク（必要に応じて）
go test -bench=. ./internal/parser/
```

---

## CI での実行

```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      - name: Test
        run: go test -race -v ./...
      - name: Coverage
        run: go test -coverprofile=coverage.out ./...
```

---

## テストのベストプラクティス

### 1. 変数キャプチャ

```go
for _, tt := range tests {
    tt := tt // ループ変数をキャプチャ
    t.Run(tt.name, func(t *testing.T) {
        t.Parallel()
        // tt を使用
    })
}
```

### 2. t.Helper() の使用

```go
func assertEqual(t *testing.T, got, want interface{}) {
    t.Helper() // エラー時に呼び出し元の行番号を表示
    if got != want {
        t.Errorf("got %v, want %v", got, want)
    }
}
```

### 3. 一時ディレクトリ

```go
func TestSomething(t *testing.T) {
    dir := t.TempDir() // テスト終了時に自動削除
    // dir を使用
}
```

### 4. 環境変数の復元

```go
func TestWithEnv(t *testing.T) {
    oldVal := os.Getenv("KEY")
    os.Setenv("KEY", "test")
    defer os.Setenv("KEY", oldVal)
    // テスト
}
```
