# 公開API設計 (pkg/vrclog)

## 設計原則

### 1. Zero Value を有用に

```go
// これが動作する
events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})
```

- 構造体のゼロ値がそのまま有効なデフォルトとして機能
- ユーザーは必要なオプションのみ指定

### 2. Context First

```go
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error)
```

- 最初の引数は `context.Context`
- キャンセル可能、タイムアウト設定可能
- Google のスタイルガイドに準拠

### 3. 小さなインターフェース

- 公開する型・関数を最小限に
- 内部実装の詳細は隠蔽

### 4. エラーはラップして返す

```go
return nil, fmt.Errorf("finding log directory: %w", err)
```

- `%w` でオリジナルエラーを保持
- `errors.Is()` / `errors.As()` で検査可能

---

## Event型

### 定義 (pkg/vrclog/event/event.go)

**注意**: Import Cycle回避のため、Event型は独立パッケージに配置

```go
package event

import "time"

// Type represents the type of VRChat log event.
type Type string

const (
    // WorldJoin indicates the user has joined a world/instance.
    WorldJoin Type = "world_join"

    // PlayerJoin indicates another player has joined the instance.
    PlayerJoin Type = "player_join"

    // PlayerLeft indicates another player has left the instance.
    PlayerLeft Type = "player_left"
)

// Event represents a parsed VRChat log event.
type Event struct {
    // Type is the event type.
    Type Type `json:"type"`

    // Timestamp is when the event occurred (local time from log).
    Timestamp time.Time `json:"timestamp"`

    // PlayerName is the display name of the player (for player events).
    PlayerName string `json:"player_name,omitempty"`

    // PlayerID is the VRChat user ID (usr_xxx format, if available).
    PlayerID string `json:"player_id,omitempty"`

    // WorldID is the VRChat world ID (wrld_xxx format).
    WorldID string `json:"world_id,omitempty"`

    // WorldName is the display name of the world.
    WorldName string `json:"world_name,omitempty"`

    // InstanceID is the instance identifier (e.g., "12345~region(us)").
    InstanceID string `json:"instance_id,omitempty"`

    // RawLine is the original log line (only included if requested).
    RawLine string `json:"raw_line,omitempty"`
}
```

### Re-export (pkg/vrclog/types.go)

```go
package vrclog

import "github.com/vrclog/vrclog-go/pkg/vrclog/event"

// Re-export event types for convenience
type Event = event.Event
type EventType = event.Type

const (
    EventWorldJoin  = event.WorldJoin
    EventPlayerJoin = event.PlayerJoin
    EventPlayerLeft = event.PlayerLeft
)
```

### 設計決定

| 項目 | 決定 | 理由 |
|------|------|------|
| イベント名 | `world_join`, `player_join`, `player_left` | 直感的で一貫性がある |
| Meta フィールド | 省略 | YAGNI原則、必要になったら追加 |
| JSON タグ | `omitempty` 付き | 空フィールドを出力しない |
| 時刻 | `time.Time` | 標準型、JSONでISO8601形式 |

### イベントタイプの詳細

#### EventWorldJoin

- **発火条件**: `[Behaviour] Entering Room:` または `[Behaviour] Joining wrld_xxx:`
- **含まれるフィールド**: Timestamp, WorldName, WorldID（あれば）, InstanceID（あれば）

#### EventPlayerJoin

- **発火条件**: `[Behaviour] OnPlayerJoined`
- **含まれるフィールド**: Timestamp, PlayerName, PlayerID（あれば）

#### EventPlayerLeft

- **発火条件**: `[Behaviour] OnPlayerLeft`
- **含まれるフィールド**: Timestamp, PlayerName

---

## Watch API（Codex MCPレビュー反映版）

### 設計変更の背景

**問題**: 元の `Watch()` 関数は初期化エラーとランタイムエラーの区別ができない

**解決策**: fsnotifyパターン + `Watch(ctx)` メソッド

### ReplayConfig（リプレイオプション）

```go
// ReplayMode specifies how to handle existing log lines.
type ReplayMode int

const (
    ReplayNone      ReplayMode = iota // Only new lines (default)
    ReplayFromStart                    // Read from file start
    ReplayLastN                        // Read last N lines
    ReplaySinceTime                    // Read since timestamp
)

// ReplayConfig configures replay behavior.
// Only one mode can be active at a time (mutually exclusive).
type ReplayConfig struct {
    Mode  ReplayMode
    LastN int       // For ReplayLastN
    Since time.Time // For ReplaySinceTime
}
```

### WatchOptions

```go
// WatchOptions configures log watching behavior.
// The zero value is valid and uses sensible defaults.
type WatchOptions struct {
    // LogDir specifies the VRChat log directory.
    // If empty, auto-detects from default Windows locations.
    // Can also be set via VRCLOG_LOGDIR environment variable.
    LogDir string

    // PollInterval is how often to check for new/rotated log files.
    // Default: 2 seconds.
    PollInterval time.Duration

    // IncludeRawLine includes the original log line in Event.RawLine.
    // Default: false.
    IncludeRawLine bool

    // Replay configures replay behavior for existing log lines.
    // Default: ReplayNone (only new lines).
    Replay ReplayConfig
}

// Validate checks for invalid option combinations.
// Returns error if multiple replay modes are specified.
func (o WatchOptions) Validate() error
```

### Watcher型

```go
// Watcher monitors VRChat log files.
type Watcher struct {
    // internal fields
}

// NewWatcher creates a watcher.
// Validates options and checks log directory existence.
// Does NOT start goroutines (cheap to call).
// Returns error for invalid options or missing log directory.
func NewWatcher(opts WatchOptions) (*Watcher, error)

// Watch starts watching and returns channels.
// Starts internal goroutines here.
// When ctx is cancelled, Close() is called automatically.
// Both channels close on ctx.Done() or fatal error.
func (w *Watcher) Watch(ctx context.Context) (<-chan Event, <-chan error)

// Close stops the watcher and releases resources.
// Safe to call multiple times.
func (w *Watcher) Close() error
```

### 便利関数（後方互換）

```go
// Watch is a convenience function that creates a watcher and starts watching.
// Returns error immediately for initialization failures.
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error, error) {
    w, err := NewWatcher(opts)
    if err != nil {
        return nil, nil, err
    }
    events, errs := w.Watch(ctx)
    return events, errs, nil
}
```

### 使用例

```go
// パターン1: 便利関数（シンプル）
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{})
if err != nil {
    log.Fatal(err) // 初期化エラー
}

for {
    select {
    case event, ok := <-events:
        if !ok {
            return // channel closed
        }
        fmt.Printf("%s: %s\n", event.Type, event.PlayerName)
    case err, ok := <-errs:
        if !ok {
            return // channel closed
        }
        log.Printf("error: %v", err)
    }
}

// パターン2: Watcher直接使用（細かい制御）
watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
    Replay: vrclog.ReplayConfig{
        Mode:  vrclog.ReplayLastN,
        LastN: 100,
    },
})
if err != nil {
    log.Fatal(err)
}
defer watcher.Close()

events, errs := watcher.Watch(ctx)
// ...
```

### デフォルト値

| オプション | デフォルト | 説明 |
|-----------|-----------|------|
| LogDir | 自動検出 | Windows標準パス / VRCLOG_LOGDIR |
| PollInterval | 2秒 | 新ファイル検出間隔 |
| IncludeRawLine | false | 元ログ行を含めない |
| Replay.Mode | ReplayNone | 新規行のみ |

### チャネルライフサイクル

**クローズルール**:
- `ctx.Done()` でクローズ
- 致命的エラーでクローズ
- `defer close(eventCh); defer close(errCh)` で確実にクローズ

**バッファリング**:
- イベントチャネル: **無バッファ**（バックプレッシャー適用）
- エラーチャネル: **無バッファ** + `select { default: }` でブロック時ドロップ

### エラーハンドリング

**初期化エラー**（`NewWatcher()` が error を返す）:
- オプションのバリデーションエラー
- ログディレクトリが見つからない

**ランタイム致命的エラー**（errorsチャネルに送信後、両チャネルをclose）:
- ログファイルが見つからない
- ファイル監視の致命的エラー

**非致命的エラー**（errorsチャネルに送信、継続）:
- パースエラー
- 一時的なファイルアクセスエラー
- tailerのline.Err

---

## ParseLine API

### 定義 (parse.go)

```go
package vrclog

// ParseLine parses a single VRChat log line into an Event.
//
// Return values:
//   - (*Event, nil): Successfully parsed event
//   - (nil, nil): Line doesn't match any known event pattern (not an error)
//   - (nil, error): Line partially matches but is malformed
//
// Example:
//
//     line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
//     event, err := vrclog.ParseLine(line)
//     if err != nil {
//         log.Printf("parse error: %v", err)
//     } else if event != nil {
//         fmt.Printf("Player joined: %s\n", event.PlayerName)
//     }
//     // event == nil && err == nil means line is not a recognized event
func ParseLine(line string) (*Event, error)
```

### 戻り値のパターン

| event | error | 意味 |
|-------|-------|------|
| non-nil | nil | パース成功 |
| nil | nil | 認識されないパターン（スキップ） |
| nil | non-nil | パースエラー |

### 使用例

```go
// ファイルを1行ずつ処理
scanner := bufio.NewScanner(file)
for scanner.Scan() {
    event, err := vrclog.ParseLine(scanner.Text())
    if err != nil {
        log.Printf("parse error: %v", err)
        continue
    }
    if event == nil {
        continue // 認識されない行
    }
    // イベントを処理
    processEvent(event)
}
```

---

## エラー定義

### 定義 (errors.go)

```go
package vrclog

import "errors"

// Sentinel errors returned by this package.
var (
    // ErrLogDirNotFound is returned when the VRChat log directory
    // cannot be found or accessed.
    ErrLogDirNotFound = errors.New("vrclog: log directory not found")

    // ErrNoLogFiles is returned when no log files are found
    // in the specified directory.
    ErrNoLogFiles = errors.New("vrclog: no log files found")
)
```

### エラーの使い方

```go
events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

// チャネルが閉じた後にエラーをチェック
// （実際にはerrorsチャネル経由で通知される）

// エラー判定
if errors.Is(err, vrclog.ErrLogDirNotFound) {
    log.Fatal("VRChat log directory not found. Is VRChat installed?")
}
```

---

## パッケージドキュメント

### 定義 (doc.go)

```go
// Package vrclog provides parsing and monitoring of VRChat log files.
//
// This package allows you to:
//   - Parse VRChat log lines into structured events
//   - Monitor log files in real-time for new events
//   - Build tools like join notifications, history viewers, etc.
//
// # Basic Usage
//
// To monitor VRChat logs in real-time:
//
//     ctx, cancel := context.WithCancel(context.Background())
//     defer cancel()
//
//     events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})
//
//     for event := range events {
//         switch event.Type {
//         case vrclog.EventPlayerJoin:
//             fmt.Printf("%s joined\n", event.PlayerName)
//         case vrclog.EventPlayerLeft:
//             fmt.Printf("%s left\n", event.PlayerName)
//         case vrclog.EventWorldJoin:
//             fmt.Printf("Joined world: %s\n", event.WorldName)
//         }
//     }
//
// To parse a single log line:
//
//     event, err := vrclog.ParseLine(line)
//     if err != nil {
//         log.Printf("parse error: %v", err)
//     } else if event != nil {
//         // process event
//     }
//
// # Platform Support
//
// This package is designed for Windows where VRChat runs.
// Log file paths are auto-detected from standard Windows locations.
//
// # Disclaimer
//
// This is an unofficial tool and is not affiliated with VRChat Inc.
package vrclog
```
