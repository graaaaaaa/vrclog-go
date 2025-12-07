# CLI コードテンプレート

## cmd/vrclog/main.go

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
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose logging")

	rootCmd.AddCommand(tailCmd)
}
```

---

## cmd/vrclog/tail.go

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

var (
	logDir     string
	format     string
	eventTypes []string
	includeRaw bool
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
}

func runTail(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	typeFilter := make(map[vrclog.EventType]bool)
	if len(eventTypes) > 0 {
		for _, t := range eventTypes {
			typeFilter[vrclog.EventType(t)] = true
		}
	}

	opts := vrclog.WatchOptions{
		LogDir:         logDir,
		IncludeRawLine: includeRaw,
	}

	events, errs := vrclog.Watch(ctx, opts)

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil
			}

			if len(typeFilter) > 0 && !typeFilter[event.Type] {
				continue
			}

			if err := outputEvent(event); err != nil {
				return fmt.Errorf("output error: %w", err)
			}

		case err, ok := <-errs:
			if !ok {
				return nil
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

## pkg/vrclog/watch.go

```go
package vrclog

import (
	"context"
	"fmt"
	"time"

	"github.com/vrclog/vrclog-go/internal/logfinder"
	"github.com/vrclog/vrclog-go/internal/parser"
	"github.com/vrclog/vrclog-go/internal/tailer"
)

// WatchOptions configures log watching behavior.
// The zero value is valid and uses sensible defaults.
type WatchOptions struct {
	// LogDir specifies the VRChat log directory.
	// If empty, auto-detects from default Windows locations.
	LogDir string

	// PollInterval is how often to check for new/rotated log files.
	// Default: 2 seconds.
	PollInterval time.Duration

	// IncludeRawLine includes the original log line in Event.RawLine.
	// Default: false.
	IncludeRawLine bool
}

// Watch monitors VRChat log files and returns events through channels.
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error) {
	eventCh := make(chan Event)
	errCh := make(chan error)

	// Apply defaults
	if opts.PollInterval == 0 {
		opts.PollInterval = 2 * time.Second
	}

	go func() {
		defer close(eventCh)
		defer close(errCh)

		// Find log directory
		dir, err := logfinder.FindLogDir(opts.LogDir)
		if err != nil {
			sendError(errCh, err)
			return
		}

		// Find latest log file
		logFile, err := logfinder.FindLatestLogFile(dir)
		if err != nil {
			sendError(errCh, err)
			return
		}

		// Start tailing
		t, err := tailer.New(logFile, tailer.DefaultConfig())
		if err != nil {
			sendError(errCh, fmt.Errorf("starting tailer: %w", err))
			return
		}
		defer t.Stop()

		// Poll timer for log rotation
		pollTicker := time.NewTicker(opts.PollInterval)
		defer pollTicker.Stop()

		currentFile := logFile

		for {
			select {
			case <-ctx.Done():
				return

			case line, ok := <-t.Lines():
				if !ok {
					return
				}

				event, err := parser.Parse(line)
				if err != nil {
					sendError(errCh, err)
					continue
				}
				if event == nil {
					continue
				}

				if opts.IncludeRawLine {
					event.RawLine = line
				}

				select {
				case eventCh <- *event:
				case <-ctx.Done():
					return
				}

			case <-pollTicker.C:
				// Check for log rotation
				newFile, err := logfinder.FindLatestLogFile(dir)
				if err != nil {
					continue
				}
				if newFile != currentFile {
					// Log file rotated, restart tailer
					t.Stop()
					t, err = tailer.New(newFile, tailer.DefaultConfig())
					if err != nil {
						sendError(errCh, fmt.Errorf("restarting tailer: %w", err))
						return
					}
					currentFile = newFile
				}
			}
		}
	}()

	return eventCh, errCh
}

func sendError(ch chan<- error, err error) {
	select {
	case ch <- err:
	default:
	}
}
```

---

## internal/parser/parser_test.go

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
		{
			name:  "player left",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser",
			want: &vrclog.Event{
				Type:       vrclog.EventPlayerLeft,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "TestUser",
			},
		},
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
			name:  "japanese player name",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined テストユーザー",
			want: &vrclog.Event{
				Type:       vrclog.EventPlayerJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "テストユーザー",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !eventEqual(got, tt.want) {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

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

---

## internal/logfinder/finder_test.go

```go
package logfinder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindLatestLogFile(t *testing.T) {
	dir := t.TempDir()

	files := []string{
		"output_log_2024-01-01_00-00-00.txt",
		"output_log_2024-01-02_00-00-00.txt",
		"output_log_2024-01-03_00-00-00.txt",
	}

	for _, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := FindLatestLogFile(dir)
	if err != nil {
		t.Fatalf("FindLatestLogFile() error = %v", err)
	}

	// Most recently created file (last in the loop)
	if filepath.Base(got) != files[len(files)-1] {
		t.Errorf("FindLatestLogFile() = %v, want %v", filepath.Base(got), files[len(files)-1])
	}
}

func TestFindLatestLogFile_NoFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := FindLatestLogFile(dir)
	if err == nil {
		t.Error("FindLatestLogFile() expected error for empty directory")
	}
}

func TestFindLogDir_Explicit(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := FindLogDir(dir)
	if err != nil {
		t.Fatalf("FindLogDir() error = %v", err)
	}
	if got != dir {
		t.Errorf("FindLogDir() = %v, want %v", got, dir)
	}
}

func TestFindLogDir_EnvVar(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	oldVal := os.Getenv(EnvLogDir)
	os.Setenv(EnvLogDir, dir)
	defer os.Setenv(EnvLogDir, oldVal)

	got, err := FindLogDir("")
	if err != nil {
		t.Fatalf("FindLogDir() error = %v", err)
	}
	if got != dir {
		t.Errorf("FindLogDir() = %v, want %v", got, dir)
	}
}
```

---

## pkg/vrclog/vrclog_test.go

```go
package vrclog_test

import (
	"testing"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType vrclog.EventType
		wantNil  bool
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

func ExampleParseLine() {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser"
	event, err := vrclog.ParseLine(line)
	if err != nil {
		panic(err)
	}
	if event != nil {
		// event.Type == vrclog.EventPlayerJoin
		// event.PlayerName == "TestUser"
		_ = event
	}
}
```
