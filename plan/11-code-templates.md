# コードテンプレート

この文書には実装時にそのまま使えるコードテンプレートを記載します。

---

## go.mod

```go
module github.com/vrclog/vrclog-go

go 1.21

require (
	github.com/nxadm/tail v1.4.11
	github.com/spf13/cobra v1.8.0
)
```

---

## .gitignore

```gitignore
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/vrclog

# Test binary
*.test

# Output of go coverage tool
*.out
coverage.html

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Build output
/dist/
```

---

## pkg/vrclog/doc.go

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
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})
//
//	for event := range events {
//	    switch event.Type {
//	    case vrclog.EventPlayerJoin:
//	        fmt.Printf("%s joined\n", event.PlayerName)
//	    case vrclog.EventPlayerLeft:
//	        fmt.Printf("%s left\n", event.PlayerName)
//	    case vrclog.EventWorldJoin:
//	        fmt.Printf("Joined world: %s\n", event.WorldName)
//	    }
//	}
//
// To parse a single log line:
//
//	event, err := vrclog.ParseLine(line)
//	if err != nil {
//	    log.Printf("parse error: %v", err)
//	} else if event != nil {
//	    // process event
//	}
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

---

## pkg/vrclog/event.go

```go
package vrclog

import "time"

// EventType represents the type of VRChat log event.
type EventType string

const (
	// EventWorldJoin indicates the user has joined a world/instance.
	EventWorldJoin EventType = "world_join"

	// EventPlayerJoin indicates another player has joined the instance.
	EventPlayerJoin EventType = "player_join"

	// EventPlayerLeft indicates another player has left the instance.
	EventPlayerLeft EventType = "player_left"
)

// Event represents a parsed VRChat log event.
type Event struct {
	// Type is the event type.
	Type EventType `json:"type"`

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

---

## pkg/vrclog/errors.go

```go
package vrclog

import "errors"

// Sentinel errors for this package.
// Use errors.Is() to check for these errors.
var (
	// ErrLogDirNotFound is returned when the VRChat log directory
	// cannot be found or does not contain log files.
	ErrLogDirNotFound = errors.New("vrclog: log directory not found")

	// ErrNoLogFiles is returned when the log directory exists
	// but contains no output_log files.
	ErrNoLogFiles = errors.New("vrclog: no log files found")
)
```

---

## pkg/vrclog/parse.go

```go
package vrclog

import "github.com/vrclog/vrclog-go/internal/parser"

// ParseLine parses a single VRChat log line into an Event.
//
// Return values:
//   - (*Event, nil): Successfully parsed event
//   - (nil, nil): Line doesn't match any known event pattern (not an error)
//   - (nil, error): Line partially matches but is malformed
func ParseLine(line string) (*Event, error) {
	return parser.Parse(line)
}
```

---

## internal/parser/patterns.go

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
	// Captures: (1) world ID, (2) instance ID
	joiningPattern = regexp.MustCompile(
		`\[Behaviour\] Joining (wrld_[a-f0-9-]+):(.+)$`,
	)
)

// exclusionPatterns are substrings that indicate a line should be skipped
var exclusionPatterns = []string{
	"OnPlayerJoined:",     // Different log format
	"OnPlayerLeftRoom",    // Self leaving
	"Joining or Creating", // Not actual join
	"Joining friend",      // Not actual join
}
```

---

## internal/parser/parser.go

```go
package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// Parse parses a VRChat log line into an Event.
func Parse(line string) (*vrclog.Event, error) {
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
	if event := parsePlayerJoin(line, ts); event != nil {
		return event, nil
	}
	if event := parsePlayerLeft(line, ts); event != nil {
		return event, nil
	}
	if event := parseWorldJoin(line, ts); event != nil {
		return event, nil
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

func parsePlayerJoin(line string, ts time.Time) *vrclog.Event {
	match := playerJoinPattern.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	event := &vrclog.Event{
		Type:       vrclog.EventPlayerJoin,
		Timestamp:  ts,
		PlayerName: strings.TrimSpace(match[1]),
	}

	if len(match) > 2 && match[2] != "" {
		event.PlayerID = match[2]
	}

	return event
}

func parsePlayerLeft(line string, ts time.Time) *vrclog.Event {
	match := playerLeftPattern.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	return &vrclog.Event{
		Type:       vrclog.EventPlayerLeft,
		Timestamp:  ts,
		PlayerName: strings.TrimSpace(match[1]),
	}
}

func parseWorldJoin(line string, ts time.Time) *vrclog.Event {
	// Try "Entering Room" first (has world name)
	if match := enteringRoomPattern.FindStringSubmatch(line); match != nil {
		return &vrclog.Event{
			Type:      vrclog.EventWorldJoin,
			Timestamp: ts,
			WorldName: strings.TrimSpace(match[1]),
		}
	}

	// Try "Joining" (has world ID and instance ID)
	if match := joiningPattern.FindStringSubmatch(line); match != nil {
		return &vrclog.Event{
			Type:       vrclog.EventWorldJoin,
			Timestamp:  ts,
			WorldID:    match[1],
			InstanceID: match[2],
		}
	}

	return nil
}
```

---

## internal/logfinder/finder.go

```go
package logfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// EnvLogDir is the environment variable name for specifying log directory.
const EnvLogDir = "VRCLOG_LOGDIR"

// DefaultLogDirs returns candidate VRChat log directories in priority order.
func DefaultLogDirs() []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			localAppData = filepath.Join(userProfile, "AppData", "Local")
		}
	}

	if localAppData == "" {
		return nil
	}

	localLow := filepath.Join(filepath.Dir(localAppData), "LocalLow")

	return []string{
		filepath.Join(localLow, "VRChat", "VRChat"),
		filepath.Join(localLow, "VRChat", "vrchat"),
	}
}

// FindLogDir returns the VRChat log directory.
func FindLogDir(explicit string) (string, error) {
	if explicit != "" {
		if isValidLogDir(explicit) {
			return explicit, nil
		}
		return "", fmt.Errorf("%w: %s", vrclog.ErrLogDirNotFound, explicit)
	}

	if envDir := os.Getenv(EnvLogDir); envDir != "" {
		if isValidLogDir(envDir) {
			return envDir, nil
		}
		return "", fmt.Errorf("%w: %s (from %s)", vrclog.ErrLogDirNotFound, envDir, EnvLogDir)
	}

	for _, dir := range DefaultLogDirs() {
		if isValidLogDir(dir) {
			return dir, nil
		}
	}

	return "", vrclog.ErrLogDirNotFound
}

// FindLatestLogFile returns the path to the most recently modified output_log file.
func FindLatestLogFile(dir string) (string, error) {
	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing log files: %w", err)
	}

	if len(matches) == 0 {
		return "", vrclog.ErrNoLogFiles
	}

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

func isValidLogDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, _ := filepath.Glob(pattern)
	return len(matches) > 0
}
```

---

## internal/tailer/tailer.go

```go
package tailer

import (
	"fmt"

	"github.com/nxadm/tail"
)

// Tail wraps nxadm/tail for VRChat log file tailing.
type Tail struct {
	t      *tail.Tail
	lines  chan string
	stopCh chan struct{}
	doneCh chan struct{}
}

// Config holds configuration for tailing.
type Config struct {
	Follow bool
	ReOpen bool
	Poll   bool
}

// DefaultConfig returns the default configuration for VRChat logs.
func DefaultConfig() Config {
	return Config{
		Follow: true,
		ReOpen: true,
		Poll:   false,
	}
}

// New creates a new Tail for the specified file.
func New(filepath string, cfg Config) (*Tail, error) {
	t, err := tail.TailFile(filepath, tail.Config{
		Follow:    cfg.Follow,
		ReOpen:    cfg.ReOpen,
		Poll:      cfg.Poll,
		MustExist: true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2},
	})
	if err != nil {
		return nil, fmt.Errorf("opening tail: %w", err)
	}

	tailer := &Tail{
		t:      t,
		lines:  make(chan string),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	go tailer.run()

	return tailer, nil
}

// Lines returns a channel that receives log lines.
func (t *Tail) Lines() <-chan string {
	return t.lines
}

// Stop stops tailing and closes all channels.
func (t *Tail) Stop() error {
	close(t.stopCh)
	<-t.doneCh
	return t.t.Stop()
}

func (t *Tail) run() {
	defer close(t.doneCh)
	defer close(t.lines)

	for {
		select {
		case <-t.stopCh:
			return
		case line, ok := <-t.t.Lines:
			if !ok {
				return
			}
			if line.Err != nil {
				continue
			}
			select {
			case t.lines <- line.Text:
			case <-t.stopCh:
				return
			}
		}
	}
}
```

---

## testdata/logs/sample.txt

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
2024.01.15 12:30:00 Log        -  [Behaviour] OnPlayerJoined テストユーザー
2024.01.15 12:35:00 Log        -  [Behaviour] Entering Room: テスト [ワールド] (v1.0)
```
