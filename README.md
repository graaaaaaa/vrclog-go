# vrclog-go

[![Go Reference](https://pkg.go.dev/badge/github.com/vrclog/vrclog-go.svg)](https://pkg.go.dev/github.com/vrclog/vrclog-go)

A Go library and CLI tool for parsing and monitoring VRChat log files.

[日本語版はこちら](README.ja.md)

## Features

- Parse VRChat log files into structured events
- Monitor log files in real-time (like `tail -f`)
- Output events as JSON Lines for easy processing with tools like `jq`
- Human-readable pretty output format
- Replay historical log data
- Designed for Windows where VRChat runs

## Requirements

- Go 1.21+
- Windows (for actual VRChat log monitoring)

## Installation

```bash
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest
```

Or build from source:

```bash
git clone https://github.com/vrclog/vrclog-go.git
cd vrclog-go
go build -o vrclog ./cmd/vrclog/
```

## CLI Usage

### Commands

```bash
vrclog tail      # Monitor VRChat logs
vrclog version   # Print version information
vrclog --help    # Show help
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--verbose`, `-v` | Enable verbose logging |

### Basic Monitoring

```bash
# Monitor with auto-detected log directory
vrclog tail

# Specify log directory
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# Human-readable output
vrclog tail --format pretty

# Include raw log lines in output
vrclog tail --raw
```

### Filtering Events

```bash
# Show only player join events
vrclog tail --types player_join

# Show only world join events
vrclog tail --types world_join

# Show player join and leave events
vrclog tail --types player_join,player_left

# Short form
vrclog tail -t player_join,player_left
```

### Replay Historical Data

```bash
# Replay from the start of the log file
vrclog tail --replay-last 0

# Replay last 100 lines
vrclog tail --replay-last 100

# Replay events since a specific time
vrclog tail --replay-since "2024-01-15T12:00:00Z"
```

Note: `--replay-last` and `--replay-since` cannot be used together.

### tail Command Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--log-dir` | `-d` | auto-detect | VRChat log directory |
| `--format` | `-f` | `jsonl` | Output format: `jsonl`, `pretty` |
| `--types` | `-t` | all | Event types to show (comma-separated) |
| `--raw` | | false | Include raw log lines in output |
| `--replay-last` | | -1 (disabled) | Replay last N lines (0 = from start) |
| `--replay-since` | | | Replay since timestamp (RFC3339) |

### Processing with jq

```bash
# Filter specific player
vrclog tail | jq 'select(.player_name == "FriendName")'

# Count events by type
vrclog tail | jq -s 'group_by(.type) | map({type: .[0].type, count: length})'

# Extract player names from join events
vrclog tail | jq 'select(.type == "player_join") | .player_name'
```

## Library Usage

### Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start watching with default options (auto-detect log directory)
    events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{})
    if err != nil {
        log.Fatal(err)
    }

    for {
        select {
        case event, ok := <-events:
            if !ok {
                return
            }
            switch event.Type {
            case vrclog.EventPlayerJoin:
                fmt.Printf("%s joined\n", event.PlayerName)
            case vrclog.EventPlayerLeft:
                fmt.Printf("%s left\n", event.PlayerName)
            case vrclog.EventWorldJoin:
                fmt.Printf("Joined world: %s\n", event.WorldName)
            }
        case err, ok := <-errs:
            if !ok {
                return
            }
            log.Printf("error: %v", err)
        }
    }
}
```

### Advanced Usage with Watcher

For more control over the watcher lifecycle:

```go
// Create watcher with options
watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
    LogDir:         "", // auto-detect
    PollInterval:   5 * time.Second,
    IncludeRawLine: true,
    Replay: vrclog.ReplayConfig{
        Mode:  vrclog.ReplayLastN,
        LastN: 100,
    },
})
if err != nil {
    log.Fatal(err)
}
defer watcher.Close()

// Start watching
events, errs := watcher.Watch(ctx)
// ... process events
```

### WatchOptions

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `LogDir` | `string` | auto-detect | VRChat log directory |
| `PollInterval` | `time.Duration` | 2s | Log rotation check interval |
| `IncludeRawLine` | `bool` | false | Include raw log line in events |
| `Replay` | `ReplayConfig` | none | Replay configuration |

### ReplayConfig

| Mode | Description |
|------|-------------|
| `ReplayNone` | Only new lines (default) |
| `ReplayFromStart` | Read from file start |
| `ReplayLastN` | Read last N lines |
| `ReplaySinceTime` | Read since timestamp |

### Parse Single Lines

```go
line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
event, err := vrclog.ParseLine(line)
if err != nil {
    log.Printf("parse error: %v", err)
} else if event != nil {
    fmt.Printf("Player joined: %s\n", event.PlayerName)
}
// event == nil && err == nil means line is not a recognized event
```

## Event Types

| Type | Description | Fields |
|------|-------------|--------|
| `world_join` | User joined a world | WorldName, WorldID, InstanceID |
| `player_join` | Player joined the instance | PlayerName, PlayerID |
| `player_left` | Player left the instance | PlayerName |

## Output Format

### JSON Lines (default)

```json
{"type":"player_join","timestamp":"2024-01-15T23:59:59+09:00","player_name":"TestUser"}
{"type":"player_left","timestamp":"2024-01-16T00:00:05+09:00","player_name":"TestUser"}
```

### Pretty

```
[23:59:59] + TestUser joined
[00:00:05] - TestUser left
[00:01:00] > Joined world: Test World
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `VRCLOG_LOGDIR` | Override default log directory |

## Project Structure

```
vrclog-go/
├── cmd/vrclog/        # CLI application
├── pkg/vrclog/        # Public API
│   └── event/         # Event type definitions
└── internal/          # Internal packages
    ├── parser/        # Log line parser
    ├── tailer/        # File tailing
    └── logfinder/     # Log directory detection
```

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Ensure code is formatted (`go fmt ./...`)
4. Run tests (`go test ./...`)
5. Commit your changes
6. Push to the branch
7. Open a Pull Request

## License

MIT License

## Disclaimer

This is an unofficial tool and is not affiliated with VRChat Inc.
