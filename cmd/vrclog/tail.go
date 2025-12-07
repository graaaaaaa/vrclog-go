package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
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

  # Replay from start of log file
  vrclog tail --replay-last 0  # 0 means from start

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

	// Replay options
	tailCmd.Flags().IntVar(&replayLast, "replay-last", -1,
		"Replay last N lines before tailing (-1 = disabled, 0 = from start)")
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
	if replayLast >= 0 {
		if replayLast == 0 {
			replay.Mode = vrclog.ReplayFromStart
		} else {
			replay.Mode = vrclog.ReplayLastN
			replay.LastN = replayLast
		}
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
		return err
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
