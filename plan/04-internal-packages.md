# 内部パッケージ設計 (internal/)

## 概要

`internal/` 配下のパッケージは外部から import できない。
これにより実装の詳細を隠蔽し、公開APIを壊さずに内部実装を変更できる。

```
internal/
├── logfinder/    # ログディレクトリ・ファイル検出
├── parser/       # ログ行パース
└── tailer/       # ファイル監視
```

---

## logfinder パッケージ

### 役割

- VRChat ログディレクトリの検出
- 最新ログファイルの特定
- 環境変数・明示指定・自動検出の優先順位管理

### API定義 (finder.go)

```go
package logfinder

import (
    "os"
    "path/filepath"
    "sort"
)

// EnvLogDir is the environment variable name for specifying log directory.
const EnvLogDir = "VRCLOG_LOGDIR"

// DefaultLogDirs returns candidate VRChat log directories in priority order.
// The directories are OS-specific (Windows only for VRChat PC).
func DefaultLogDirs() []string {
    localAppData := os.Getenv("LOCALAPPDATA")
    if localAppData == "" {
        // Fallback: try to construct from USERPROFILE
        userProfile := os.Getenv("USERPROFILE")
        if userProfile != "" {
            localAppData = filepath.Join(userProfile, "AppData", "Local")
        }
    }

    if localAppData == "" {
        return nil
    }

    // LocalLow is one level up from Local
    localLow := filepath.Join(filepath.Dir(localAppData), "LocalLow")

    return []string{
        filepath.Join(localLow, "VRChat", "VRChat"),
        filepath.Join(localLow, "VRChat", "vrchat"),
    }
}

// FindLogDir returns the VRChat log directory.
//
// Priority:
//  1. explicit (if non-empty)
//  2. VRCLOG_LOGDIR environment variable
//  3. Auto-detect from DefaultLogDirs()
//
// Returns ErrLogDirNotFound if no valid directory is found.
func FindLogDir(explicit string) (string, error) {
    // 1. Check explicit
    if explicit != "" {
        if isValidLogDir(explicit) {
            return explicit, nil
        }
        return "", fmt.Errorf("%w: %s", ErrLogDirNotFound, explicit)
    }

    // 2. Check environment variable
    if envDir := os.Getenv(EnvLogDir); envDir != "" {
        if isValidLogDir(envDir) {
            return envDir, nil
        }
        return "", fmt.Errorf("%w: %s (from %s)", ErrLogDirNotFound, envDir, EnvLogDir)
    }

    // 3. Auto-detect
    for _, dir := range DefaultLogDirs() {
        if isValidLogDir(dir) {
            return dir, nil
        }
    }

    return "", ErrLogDirNotFound
}

// FindLatestLogFile returns the path to the most recently modified
// output_log file in the given directory.
//
// Returns ErrNoLogFiles if no log files are found.
func FindLatestLogFile(dir string) (string, error) {
    pattern := filepath.Join(dir, "output_log_*.txt")
    matches, err := filepath.Glob(pattern)
    if err != nil {
        return "", fmt.Errorf("globbing log files: %w", err)
    }

    if len(matches) == 0 {
        return "", ErrNoLogFiles
    }

    // Sort by modification time (newest first)
    sort.Slice(matches, func(i, j int) bool {
        infoI, _ := os.Stat(matches[i])
        infoJ, _ := os.Stat(matches[j])
        if infoI == nil || infoJ == nil {
            return false
        }
        return infoI.ModTime().After(infoJ.ModTime())
    })

    return matches[0], nil
}

// isValidLogDir checks if the directory exists and contains log files.
func isValidLogDir(dir string) bool {
    info, err := os.Stat(dir)
    if err != nil || !info.IsDir() {
        return false
    }

    // Check if there are any output_log files
    pattern := filepath.Join(dir, "output_log_*.txt")
    matches, _ := filepath.Glob(pattern)
    return len(matches) > 0
}
```

### エラー定義

```go
package logfinder

import "errors"

var (
    ErrLogDirNotFound = errors.New("log directory not found")
    ErrNoLogFiles     = errors.New("no log files found")
)
```

### 優先順位

```
1. WatchOptions.LogDir（明示指定）
   ↓ (空の場合)
2. VRCLOG_LOGDIR 環境変数
   ↓ (未設定の場合)
3. 自動検出
   a. %LOCALAPPDATA%\..\LocalLow\VRChat\VRChat
   b. %LOCALAPPDATA%\..\LocalLow\VRChat\vrchat
```

---

## parser パッケージ

### 役割

- ログ行から Event 構造体への変換
- 正規表現パターンの管理
- タイムスタンプのパース

### パターン定義 (patterns.go)

```go
package parser

import "regexp"

// Timestamp format in VRChat logs: "2024.01.15 23:59:59"
const timestampLayout = "2006.01.02 15:04:05"

// Compiled regex patterns for event detection
var (
    // Matches: "2024.01.15 23:59:59"
    timestampPattern = regexp.MustCompile(`^(\d{4}\.\d{2}\.\d{2} \d{2}:\d{2}:\d{2})`)

    // Matches: "[Behaviour] OnPlayerJoined DisplayName"
    // Matches: "[Behaviour] OnPlayerJoined DisplayName (usr_xxx)"
    // Captures: (1) display name, (2) user ID (optional)
    playerJoinPattern = regexp.MustCompile(
        `\[Behaviour\] OnPlayerJoined (.+?)(?:\s+\((usr_[a-f0-9-]+)\))?$`,
    )

    // Matches: "[Behaviour] OnPlayerLeft DisplayName"
    // Excludes: "OnPlayerLeftRoom"
    // Captures: (1) display name
    playerLeftPattern = regexp.MustCompile(
        `\[Behaviour\] OnPlayerLeft ([^(].*)$`,
    )

    // Matches: "[Behaviour] Entering Room: World Name"
    // Captures: (1) world name
    enteringRoomPattern = regexp.MustCompile(
        `\[Behaviour\] Entering Room: (.+)$`,
    )

    // Matches: "[Behaviour] Joining wrld_xxx:instance_id"
    // Excludes: "Joining or Creating", "Joining friend"
    // Captures: (1) world ID, (2) instance ID
    joiningPattern = regexp.MustCompile(
        `\[Behaviour\] Joining (wrld_[a-f0-9-]+):(.+)$`,
    )
)

// exclusionPatterns are patterns that look like events but should be ignored
var exclusionPatterns = []string{
    "OnPlayerJoined:",     // Different log format
    "OnPlayerLeftRoom",    // Self leaving
    "Joining or Creating", // Not actual join
    "Joining friend",      // Not actual join
}
```

### パーサー実装 (parser.go)

**注意**: Import Cycle回避のため、`pkg/vrclog/event`パッケージをimportする。

```go
package parser

import (
    "strings"
    "time"

    "github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// Parse parses a VRChat log line into an Event.
//
// Returns:
//   - (*Event, nil): Successfully parsed
//   - (nil, nil): Not a recognized event pattern
//   - (nil, error): Malformed line
func Parse(line string) (*event.Event, error) {
    // Quick exclusion check
    for _, pattern := range exclusionPatterns {
        if strings.Contains(line, pattern) {
            return nil, nil
        }
    }

    // Extract timestamp
    ts, err := parseTimestamp(line)
    if err != nil {
        // No timestamp means not a standard log line
        return nil, nil
    }

    // Try each event pattern
    if ev := parsePlayerJoin(line, ts); ev != nil {
        return ev, nil
    }
    if ev := parsePlayerLeft(line, ts); ev != nil {
        return ev, nil
    }
    if ev := parseWorldJoin(line, ts); ev != nil {
        return ev, nil
    }

    // Not a recognized event
    return nil, nil
}

func parseTimestamp(line string) (time.Time, error) {
    match := timestampPattern.FindStringSubmatch(line)
    if match == nil {
        return time.Time{}, fmt.Errorf("no timestamp found")
    }
    return time.ParseInLocation(timestampLayout, match[1], time.Local)
}

func parsePlayerJoin(line string, ts time.Time) *event.Event {
    match := playerJoinPattern.FindStringSubmatch(line)
    if match == nil {
        return nil
    }

    ev := &event.Event{
        Type:       event.PlayerJoin,
        Timestamp:  ts,
        PlayerName: strings.TrimSpace(match[1]),
    }

    if len(match) > 2 && match[2] != "" {
        ev.PlayerID = match[2]
    }

    return ev
}

func parsePlayerLeft(line string, ts time.Time) *event.Event {
    match := playerLeftPattern.FindStringSubmatch(line)
    if match == nil {
        return nil
    }

    return &event.Event{
        Type:       event.PlayerLeft,
        Timestamp:  ts,
        PlayerName: strings.TrimSpace(match[1]),
    }
}

func parseWorldJoin(line string, ts time.Time) *event.Event {
    // Try "Entering Room" first (has world name)
    if match := enteringRoomPattern.FindStringSubmatch(line); match != nil {
        return &event.Event{
            Type:      event.WorldJoin,
            Timestamp: ts,
            WorldName: strings.TrimSpace(match[1]),
        }
    }

    // Try "Joining" (has world ID and instance ID)
    if match := joiningPattern.FindStringSubmatch(line); match != nil {
        return &event.Event{
            Type:       event.WorldJoin,
            Timestamp:  ts,
            WorldID:    match[1],
            InstanceID: match[2],
        }
    }

    return nil
}
```

---

## tailer パッケージ

### 役割

- ファイルの末尾監視（tail -f 相当）
- ファイルローテーション対応
- nxadm/tail ライブラリのラッパー
- **エラー伝播**（Codex推奨: line.Errをerrorsチャネルに送信）

### 実装 (tailer.go)

```go
package tailer

import (
    "context"
    "fmt"

    "github.com/nxadm/tail"
)

// Tailer wraps nxadm/tail for VRChat log file tailing.
type Tailer struct {
    t      *tail.Tail
    ctx    context.Context
    cancel context.CancelFunc
    lines  chan string
    errors chan error
    doneCh chan struct{}
}

// Config holds configuration for tailing.
type Config struct {
    // Follow continues reading as the file grows (tail -f).
    Follow bool

    // ReOpen reopens the file when it's truncated or recreated (tail -F).
    ReOpen bool

    // Poll uses polling instead of inotify (more compatible but less efficient).
    Poll bool

    // MustExist requires the file to exist before starting (false = wait for creation).
    MustExist bool
}

// DefaultConfig returns the default configuration for VRChat logs.
func DefaultConfig() Config {
    return Config{
        Follow:    true,
        ReOpen:    true,
        Poll:      false, // Use inotify/ReadDirectoryChangesW when available
        MustExist: false, // Wait for file creation
    }
}

// New creates a new Tailer for the specified file.
// The provided context controls the tailer's lifecycle.
func New(ctx context.Context, filepath string, cfg Config) (*Tailer, error) {
    t, err := tail.TailFile(filepath, tail.Config{
        Follow:    cfg.Follow,
        ReOpen:    cfg.ReOpen,
        Poll:      cfg.Poll,
        MustExist: cfg.MustExist,
        Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // Start from end
    })
    if err != nil {
        return nil, fmt.Errorf("opening tail: %w", err)
    }

    ctx, cancel := context.WithCancel(ctx)

    tailer := &Tailer{
        t:      t,
        ctx:    ctx,
        cancel: cancel,
        lines:  make(chan string),
        errors: make(chan error), // Unbuffered, non-blocking send
        doneCh: make(chan struct{}),
    }

    go tailer.run()

    return tailer, nil
}

// Lines returns a channel that receives log lines.
func (t *Tailer) Lines() <-chan string {
    return t.lines
}

// Errors returns a channel that receives errors from tailing.
// Errors are sent non-blocking; if the channel is not read, errors are dropped.
func (t *Tailer) Errors() <-chan error {
    return t.errors
}

// Stop stops tailing and closes all channels.
// Safe to call multiple times.
func (t *Tailer) Stop() error {
    t.cancel()
    <-t.doneCh // Wait for run() to finish
    return t.t.Stop()
}

func (t *Tailer) run() {
    defer close(t.doneCh)
    defer close(t.lines)
    defer close(t.errors)

    for {
        select {
        case <-t.ctx.Done():
            return
        case line, ok := <-t.t.Lines:
            if !ok {
                return
            }
            if line.Err != nil {
                // Non-blocking error send (Codex推奨)
                select {
                case t.errors <- fmt.Errorf("tail: %w", line.Err):
                default:
                    // Drop error if channel is full/not read
                }
                continue
            }
            select {
            case t.lines <- line.Text:
            case <-t.ctx.Done():
                return
            }
        }
    }
}
```

### nxadm/tail を選んだ理由

| 要件 | nxadm/tail | 自前実装 |
|------|-----------|---------|
| Windows対応 | ○ | 要実装 |
| ログローテーション | ○ (ReOpen) | 要実装 |
| ファイル位置追跡 | ○ (Tell) | 要実装 |
| 実績 | 広く使用 | なし |
| 依存追加 | 1ライブラリ | なし |

**結論**: 実装コストとリスクを考慮し、nxadm/tail を使用

### 設定詳細

```go
tail.Config{
    Follow:    true,  // tail -f: ファイル成長を追跡
    ReOpen:    true,  // tail -F: ファイル再作成を追跡
    Poll:      false, // inotify/ReadDirectoryChangesW を使用
    MustExist: true,  // ファイルが存在しない場合はエラー
    Location:  &tail.SeekInfo{
        Offset: 0,
        Whence: 2,    // io.SeekEnd: ファイル末尾から開始
    },
}
```

---

## パッケージ間の依存関係

```
pkg/vrclog
    ├── pkg/vrclog/event        # Event型の定義
    ├── internal/logfinder
    ├── internal/parser
    └── internal/tailer
            └── github.com/nxadm/tail

pkg/vrclog/event
    └── (標準ライブラリのみ)

internal/parser
    └── pkg/vrclog/event        # Import Cycle回避

internal/logfinder
    └── (標準ライブラリのみ)

internal/tailer
    └── github.com/nxadm/tail
```

### Import Cycle回避（Codex推奨）

**問題**: `internal/parser`が`pkg/vrclog`をimportし、`pkg/vrclog`が`internal/parser`をimportするとimport cycleが発生

**解決策**: Event型を`pkg/vrclog/event`サブパッケージに分離

```go
// internal/parser/parser.go
import "github.com/vrclog/vrclog-go/pkg/vrclog/event"

func Parse(line string) (*event.Event, error) {
    // ...
}

// pkg/vrclog/types.go - re-export for convenience
type Event = event.Event
type EventType = event.Type
const EventPlayerJoin = event.PlayerJoin
// ...
```

これにより:
- `internal/parser` → `pkg/vrclog/event` (OK)
- `pkg/vrclog` → `internal/parser` (OK)
- `pkg/vrclog` → `pkg/vrclog/event` (OK, re-export)

循環依存なし。
