# CLI設計 (cmd/vrclog)

## フレームワーク選定

### spf13/cobra を選んだ理由

| 要件 | cobra | flag (標準) |
|------|-------|------------|
| サブコマンド | ○ 組み込み | △ 手動実装 |
| ヘルプ生成 | ○ 自動 | △ 手動 |
| シェル補完 | ○ 組み込み | × |
| フラグ継承 | ○ PersistentFlags | × |
| 学習コスト | 低い | 低い |
| 採用実績 | kubectl, gh, hugo など | - |

**結論**: サブコマンド構造と将来の拡張性を考慮し、cobra を使用

---

## コマンド構造

```
vrclog                    # ルートコマンド
├── tail                  # ログ監視・イベント出力
├── version              # バージョン表示（将来）
└── completion           # シェル補完生成（cobra組み込み）
```

---

## ルートコマンド (main.go)

```go
package main

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var (
    // Version information (set by ldflags)
    version = "dev"
    commit  = "none"
    date    = "unknown"

    // Global flags
    verbose bool
)

func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}

var rootCmd = &cobra.Command{
    Use:   "vrclog",
    Short: "VRChat log parser and monitor",
    Long: `vrclog is a tool for parsing and monitoring VRChat log files.

It can parse VRChat logs to extract events like player joins/leaves,
world changes, and more. Events are output as JSON Lines for easy
processing with other tools.

This is an unofficial tool and is not affiliated with VRChat Inc.`,
    SilenceUsage: true, // Don't show usage on error
}

func init() {
    // Global flags (inherited by all subcommands)
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
        "Enable verbose logging")

    // Add subcommands
    rootCmd.AddCommand(tailCmd)
    // rootCmd.AddCommand(versionCmd) // 将来追加
}
```

---

## tailサブコマンド (tail.go)

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    "github.com/spf13/cobra"
    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

var (
    // tail flags
    logDir      string
    format      string
    eventTypes  []string
    includeRaw  bool
    replayLast  int
    replaySince string
)

var tailCmd = &cobra.Command{
    Use:   "tail",
    Short: "Monitor VRChat logs and output events",
    Long: `Monitor VRChat log files in real-time and output parsed events.

Events are output as JSON Lines by default (one JSON object per line),
which makes it easy to process with tools like jq.

Examples:
  # Monitor with default settings (auto-detect log directory)
  vrclog tail

  # Specify log directory
  vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

  # Output only player join/leave events
  vrclog tail --types player_join,player_left

  # Human-readable output
  vrclog tail --format pretty

  # Pipe to jq for filtering
  vrclog tail | jq 'select(.type == "player_join")'`,
    RunE: runTail,
}

func init() {
    tailCmd.Flags().StringVarP(&logDir, "log-dir", "d", "",
        "VRChat log directory (auto-detected if not specified)")
    tailCmd.Flags().StringVarP(&format, "format", "f", "jsonl",
        "Output format: jsonl, pretty")
    tailCmd.Flags().StringSliceVarP(&eventTypes, "types", "t", nil,
        "Event types to show (comma-separated: world_join,player_join,player_left)")
    tailCmd.Flags().BoolVar(&includeRaw, "raw", false,
        "Include raw log lines in output")

    // Replay options (mutually exclusive)
    tailCmd.Flags().IntVar(&replayLast, "replay-last", 0,
        "Replay last N lines before tailing (0 = disabled)")
    tailCmd.Flags().StringVar(&replaySince, "replay-since", "",
        "Replay events since timestamp (RFC3339 format, e.g., 2024-01-15T12:00:00Z)")
}

func runTail(cmd *cobra.Command, args []string) error {
    // Setup context with signal handling
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    // Build event type filter
    typeFilter := make(map[vrclog.EventType]bool)
    if len(eventTypes) > 0 {
        for _, t := range eventTypes {
            typeFilter[vrclog.EventType(t)] = true
        }
    }

    // Build replay config
    replay := vrclog.ReplayConfig{}
    if replayLast > 0 {
        replay.Mode = vrclog.ReplayLastN
        replay.LastN = replayLast
    } else if replaySince != "" {
        t, err := time.Parse(time.RFC3339, replaySince)
        if err != nil {
            return fmt.Errorf("invalid --replay-since format: %w", err)
        }
        replay.Mode = vrclog.ReplaySinceTime
        replay.Since = t
    }

    // Build options
    opts := vrclog.WatchOptions{
        LogDir:         logDir,
        IncludeRawLine: includeRaw,
        Replay:         replay,
    }

    // Validate options
    if err := opts.Validate(); err != nil {
        return fmt.Errorf("invalid options: %w", err)
    }

    // Create watcher (validates log directory)
    watcher, err := vrclog.NewWatcher(opts)
    if err != nil {
        return err // User-friendly error from NewWatcher
    }
    defer watcher.Close()

    // Start watching
    events, errs := watcher.Watch(ctx)

    // Output loop
    for {
        select {
        case event, ok := <-events:
            if !ok {
                return nil // Channel closed
            }

            // Apply type filter
            if len(typeFilter) > 0 && !typeFilter[event.Type] {
                continue
            }

            // Output event
            if err := outputEvent(event); err != nil {
                return fmt.Errorf("output error: %w", err)
            }

        case err, ok := <-errs:
            if !ok {
                return nil // Channel closed
            }
            if verbose {
                fmt.Fprintf(os.Stderr, "warning: %v\n", err)
            }

        case <-ctx.Done():
            return nil
        }
    }
}

func outputEvent(event vrclog.Event) error {
    switch format {
    case "jsonl":
        return outputJSON(event)
    case "pretty":
        return outputPretty(event)
    default:
        return fmt.Errorf("unknown format: %s", format)
    }
}

func outputJSON(event vrclog.Event) error {
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    fmt.Println(string(data))
    return nil
}

func outputPretty(event vrclog.Event) error {
    ts := event.Timestamp.Format("15:04:05")

    switch event.Type {
    case vrclog.EventPlayerJoin:
        fmt.Printf("[%s] + %s joined\n", ts, event.PlayerName)
    case vrclog.EventPlayerLeft:
        fmt.Printf("[%s] - %s left\n", ts, event.PlayerName)
    case vrclog.EventWorldJoin:
        if event.WorldName != "" {
            fmt.Printf("[%s] > Joined world: %s\n", ts, event.WorldName)
        } else {
            fmt.Printf("[%s] > Joined instance: %s\n", ts, event.InstanceID)
        }
    default:
        fmt.Printf("[%s] ? %s\n", ts, event.Type)
    }

    return nil
}
```

---

## フラグ設計

### グローバルフラグ (PersistentFlags)

| フラグ | 短縮 | デフォルト | 説明 |
|--------|------|-----------|------|
| `--verbose` | `-v` | false | 詳細ログ出力 |

### tailフラグ (Flags)

| フラグ | 短縮 | デフォルト | 説明 |
|--------|------|-----------|------|
| `--log-dir` | `-d` | 自動検出 | ログディレクトリ |
| `--format` | `-f` | jsonl | 出力形式 |
| `--types` | `-t` | 全て | フィルタするイベント種別 |
| `--raw` | - | false | 元ログ行を含める |

### 出力形式

#### jsonl (JSON Lines)

```json
{"type":"player_join","timestamp":"2024-01-15T23:59:59+09:00","player_name":"TestUser"}
{"type":"player_left","timestamp":"2024-01-16T00:00:05+09:00","player_name":"TestUser"}
```

- 1行1JSON
- jq などでフィルタリング可能
- プログラムからのパースが容易

#### pretty (人間向け)

```
[23:59:59] + TestUser joined
[00:00:05] - TestUser left
[00:01:00] > Joined world: Test World
```

- 人間が読みやすい形式
- 開発・デバッグ時に便利

---

## シグナルハンドリング

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    cancel()
}()
```

- Ctrl+C (SIGINT) でグレースフルシャットダウン
- コンテキストキャンセルで Watch を停止
- チャネルが閉じるまで待機

---

## エラーハンドリング

### cobra の設定

```go
var rootCmd = &cobra.Command{
    // ...
    SilenceUsage: true, // エラー時に使用方法を表示しない
}
```

### エラー出力

```go
func main() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

- エラーは標準エラー出力へ
- 終了コード 1 で異常終了

---

## 将来の拡張

### versionサブコマンド

```go
var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print version information",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("vrclog %s (commit: %s, built: %s)\n",
            version, commit, date)
    },
}
```

### ビルド時のバージョン埋め込み

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/vrclog
```

### シェル補完

cobra が自動生成する:

```bash
# bash
vrclog completion bash > /etc/bash_completion.d/vrclog

# zsh
vrclog completion zsh > "${fpath[1]}/_vrclog"

# fish
vrclog completion fish > ~/.config/fish/completions/vrclog.fish

# PowerShell
vrclog completion powershell > vrclog.ps1
```

---

## 使用例

```bash
# 基本的な使用方法
vrclog tail

# ログディレクトリを指定
vrclog tail -d "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# player_join のみをフィルタ
vrclog tail -t player_join

# 複数のイベントタイプをフィルタ
vrclog tail -t player_join,player_left

# 人間向け出力
vrclog tail -f pretty

# jq と組み合わせ
vrclog tail | jq 'select(.player_name == "FriendName")'

# ファイルに保存
vrclog tail > events.jsonl

# バックグラウンドで実行
vrclog tail &
```
