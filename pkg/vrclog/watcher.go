package vrclog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/vrclog/vrclog-go/internal/logfinder"
	"github.com/vrclog/vrclog-go/internal/tailer"
)

// ReplayMode specifies how to handle existing log lines.
type ReplayMode int

const (
	// ReplayNone only watches for new lines (default, tail -f behavior).
	ReplayNone ReplayMode = iota
	// ReplayFromStart reads from the beginning of the file.
	ReplayFromStart
	// ReplayLastN reads the last N lines before tailing.
	ReplayLastN
	// ReplaySinceTime reads lines since a specific timestamp.
	ReplaySinceTime
)

// DefaultMaxReplayLastN is the default maximum lines for ReplayLastN mode.
// This limits memory usage to roughly tens of MB for typical VRChat logs.
const DefaultMaxReplayLastN = 10000

// watcherErrBuffer is the buffer size for the error channel.
// A small buffer prevents error loss during brief moments when the consumer
// is busy processing events, while keeping memory usage minimal.
const watcherErrBuffer = 16

// ReplayConfig configures replay behavior.
// Only one mode can be active at a time (mutually exclusive).
type ReplayConfig struct {
	Mode  ReplayMode
	LastN int       // For ReplayLastN
	Since time.Time // For ReplaySinceTime
}

// Watcher monitors VRChat log files.
type Watcher struct {
	cfg    watchConfig // internal configuration (immutable after creation)
	logDir string
	log    *slog.Logger

	mu       sync.Mutex
	closed   bool
	cancel   context.CancelFunc // cancel func to stop the goroutine
	doneCh   chan struct{}      // signals when goroutine has exited
	watching bool               // true if Watch() has been called
}

// discardLogger returns a logger that discards all output.
var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// Watch starts watching and returns channels.
// Starts internal goroutines here.
// When ctx is cancelled, channels are closed automatically.
// Both channels close on ctx.Done() or fatal error.
// Watch can only be called once per Watcher instance.
//
// Returns ErrWatcherClosed if the watcher has been closed.
// Returns ErrAlreadyWatching if Watch() has already been called.
func (w *Watcher) Watch(ctx context.Context) (<-chan Event, <-chan error, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, nil, ErrWatcherClosed
	}
	if w.watching {
		return nil, nil, ErrAlreadyWatching
	}
	w.watching = true

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.doneCh = make(chan struct{})

	eventCh := make(chan Event)
	errCh := make(chan error, watcherErrBuffer)

	go w.run(ctx, eventCh, errCh)

	return eventCh, errCh, nil
}

// Close stops the watcher and releases resources.
// Safe to call multiple times.
// Blocks until the goroutine has exited.
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true

	// Cancel the context to stop the goroutine
	if w.cancel != nil {
		w.cancel()
	}
	doneCh := w.doneCh
	w.mu.Unlock()

	// Wait for goroutine to exit if Watch was called
	if doneCh != nil {
		<-doneCh
	}
	return nil
}

func (w *Watcher) run(ctx context.Context, eventCh chan<- Event, errCh chan<- error) {
	defer close(w.doneCh) // Signal that goroutine has exited
	defer close(eventCh)
	defer close(errCh)

	// Find latest log file (with optional waiting)
	logFile, err := w.findLogFileWithWait(ctx, errCh)
	if err != nil {
		// Error already sent to errCh by findLogFileWithWait
		return
	}
	w.log.Debug("found latest log file", "path", logFile)

	// Configure tailer
	cfg := tailer.DefaultConfig()
	// For ReplayFromStart and ReplaySinceTime, read from start
	// For ReplayLastN, we handle it specially below
	cfg.FromStart = w.cfg.replay.Mode == ReplayFromStart || w.cfg.replay.Mode == ReplaySinceTime

	// Handle ReplayLastN: read last N lines first, then tail from end
	if w.cfg.replay.Mode == ReplayLastN && w.cfg.replay.LastN > 0 {
		w.log.Debug("replaying last N lines", "n", w.cfg.replay.LastN, "path", logFile)
		if err := w.replayLastN(ctx, logFile, eventCh, errCh); err != nil {
			sendError(ctx, errCh, &WatchError{Op: WatchOpReplay, Path: logFile, Err: err})
		}
		cfg.FromStart = false // Continue from end after replay
	}

	// Start tailer
	t, err := tailer.New(ctx, logFile, cfg)
	if err != nil {
		sendError(ctx, errCh, &WatchError{Op: WatchOpTail, Path: logFile, Err: err})
		return
	}
	w.log.Debug("started tailing", "path", logFile, "from_start", cfg.FromStart)

	// Set poll interval for log rotation check (defaultWatchConfig guarantees valid interval)
	rotationTicker := time.NewTicker(w.cfg.pollInterval)
	defer rotationTicker.Stop()
	defer func() { _ = t.Stop() }()

	currentFile := logFile

	// Process lines
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-t.Lines():
			if !ok {
				return
			}
			w.processLine(ctx, line, eventCh, errCh)
		case err, ok := <-t.Errors():
			if !ok {
				return
			}
			sendError(ctx, errCh, err)
		case <-rotationTicker.C:
			// Check for new log file (log rotation)
			newFile, err := logfinder.FindLatestLogFile(w.logDir)
			if err != nil {
				sendError(ctx, errCh, &WatchError{Op: WatchOpRotation, Err: err})
				continue
			}
			if newFile != currentFile {
				// New log file found, switch to it
				w.log.Debug("log rotation detected", "from", currentFile, "to", newFile)
				_ = t.Stop()
				cfg := tailer.DefaultConfig()
				cfg.FromStart = true // Read new file from start
				newTailer, err := tailer.New(ctx, newFile, cfg)
				if err != nil {
					sendError(ctx, errCh, &WatchError{Op: WatchOpTail, Path: newFile, Err: err})
					continue
				}
				t = newTailer
				currentFile = newFile
			}
		}
	}
}

// findLogFileWithWait finds the latest log file, optionally waiting if none exist yet.
// Returns the log file path or an error (error is also sent to errCh).
func (w *Watcher) findLogFileWithWait(ctx context.Context, errCh chan<- error) (string, error) {
	logFile, err := logfinder.FindLatestLogFile(w.logDir)

	// If we found a file or got an error other than ErrNoLogFiles, return immediately
	if err == nil {
		return logFile, nil
	}
	if err != ErrNoLogFiles {
		sendError(ctx, errCh, &WatchError{Op: WatchOpFindLatest, Err: err})
		return "", err
	}

	// We got ErrNoLogFiles - check if we should wait
	if !w.cfg.waitForLogs {
		sendError(ctx, errCh, &WatchError{Op: WatchOpFindLatest, Err: err})
		return "", err
	}

	// Wait for log files to appear
	w.log.Debug("no log files found, waiting for logs to appear", "poll_interval", w.cfg.pollInterval)
	ticker := time.NewTicker(w.cfg.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Send error directly (not via sendError) since context is already cancelled
			err := ctx.Err()
			select {
			case errCh <- &WatchError{Op: WatchOpFindLatest, Err: err}:
			default:
				// Channel buffer full, which is very unlikely but non-fatal
			}
			return "", err
		case <-ticker.C:
			logFile, err := logfinder.FindLatestLogFile(w.logDir)
			if err == nil {
				w.log.Debug("log file appeared", "path", logFile)
				return logFile, nil
			}
			if err != ErrNoLogFiles {
				// Different error occurred (e.g., permission denied)
				sendError(ctx, errCh, &WatchError{Op: WatchOpFindLatest, Err: err})
				return "", err
			}
			// Still no log files, continue waiting
		}
	}
}

func (w *Watcher) processLine(ctx context.Context, line string, eventCh chan<- Event, errCh chan<- error) {
	result, err := w.cfg.parser.ParseLine(ctx, line)

	// Process events even if there's an error (e.g., ChainContinueOnError mode)
	// This ensures partial success from multi-parser chains is not lost
	hasEvents := len(result.Events) > 0

	if err != nil {
		// Send error but still process any events we got
		if hasEvents {
			// Process events first, then send the error
			// This allows ChainContinueOnError to emit events + errors
			for _, ev := range result.Events {
				// Apply same filters as below
				if w.cfg.replay.Mode == ReplaySinceTime && ev.Timestamp.Before(w.cfg.replay.Since) {
					continue
				}
				if w.cfg.filter != nil && !w.cfg.filter.Allows(EventType(ev.Type)) {
					continue
				}
				if w.cfg.includeRawLine {
					ev.RawLine = line
				}
				select {
				case eventCh <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
		sendError(ctx, errCh, &ParseError{Line: line, Err: err})
		return
	}

	if !result.Matched {
		return // Not a recognized event
	}

	// Process all events from the result
	for _, ev := range result.Events {
		// Filter by replay time if needed (do this early before other processing)
		if w.cfg.replay.Mode == ReplaySinceTime && ev.Timestamp.Before(w.cfg.replay.Since) {
			continue
		}

		// Apply event type filter (do this before copying RawLine for efficiency)
		if w.cfg.filter != nil && !w.cfg.filter.Allows(EventType(ev.Type)) {
			continue
		}

		// Include raw line if requested
		if w.cfg.includeRawLine {
			ev.RawLine = line
		}

		// Send event
		select {
		case eventCh <- ev:
		case <-ctx.Done():
			return
		}
	}
}

// replayLastN reads and processes the last N lines from the log file.
func (w *Watcher) replayLastN(ctx context.Context, logFile string, eventCh chan<- Event, errCh chan<- error) error {
	lines, err := readLastNLines(logFile, w.cfg.replay.LastN, w.cfg.maxReplayBytes, w.cfg.maxReplayLineBytes)
	if err != nil {
		return err
	}

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			w.processLine(ctx, line, eventCh, errCh)
		}
	}
	return nil
}

// readLastNLines reads the last N non-empty lines from a file using backward chunk scanning.
// Returns lines in order (oldest first).
//
// Memory limits:
//   - maxBytes: Maximum total bytes to read (0 = unlimited)
//   - maxLineBytes: Maximum bytes per single line (0 = unlimited)
//
// Returns ErrReplayLimitExceeded if limits are exceeded.
func readLastNLines(filepath string, n int, maxBytes int, maxLineBytes int) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	if fileSize == 0 {
		return nil, nil
	}

	// Ring buffer to store last N lines
	lines := make([]string, 0, n)

	// Read from end in chunks
	const chunkSize = 4096
	offset := fileSize
	carry := []byte{} // Incomplete line from previous chunk
	totalBytes := 0   // Total bytes read

	for len(lines) < n && offset > 0 {
		// Calculate read position
		readSize := int64(chunkSize)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		// Check total bytes limit
		if maxBytes > 0 && totalBytes+int(readSize)+len(carry) > maxBytes {
			return nil, ErrReplayLimitExceeded
		}

		// Read chunk
		chunk := make([]byte, readSize)
		_, err := file.ReadAt(chunk, offset)
		if err != nil {
			return nil, err
		}
		totalBytes += int(readSize)

		// Append carry to chunk (carry comes after chunk in file order)
		chunk = append(chunk, carry...)

		// Scan backwards for newlines
		newLines, newCarry := extractLinesBackward(chunk, n-len(lines), maxLineBytes)
		if newCarry == nil && maxLineBytes > 0 && len(chunk) > maxLineBytes {
			// No newline found and chunk exceeds line limit
			return nil, ErrReplayLimitExceeded
		}

		// Prepend new lines to result (they come before existing lines)
		if len(newLines) > 0 {
			lines = append(newLines, lines...)
		}
		carry = newCarry
	}

	// Handle final carry (line at beginning of file without leading newline)
	if offset == 0 && len(carry) > 0 {
		if maxLineBytes > 0 && len(carry) > maxLineBytes {
			return nil, ErrReplayLimitExceeded
		}
		line := string(carry)
		// Remove trailing \r for CRLF
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if line != "" {
			// Prepend the first line
			lines = append([]string{line}, lines...)
			// Keep only last n lines if we have too many
			if len(lines) > n {
				lines = lines[len(lines)-n:]
			}
		}
	}

	return lines, nil
}

// extractLinesBackward extracts complete lines from a buffer by scanning backwards.
// Returns lines in order (oldest first) and the carry (incomplete line at buffer start).
// If maxLineBytes > 0, checks that no single line exceeds the limit.
//
// This function scans the ENTIRE buffer to find all complete lines, then returns
// only the last maxLines. The carry is the incomplete line at the start of the buffer.
func extractLinesBackward(buffer []byte, maxLines int, maxLineBytes int) ([]string, []byte) {
	var lines []string
	end := len(buffer)

	// Scan backwards for ALL newlines in the buffer
	for i := len(buffer) - 1; i >= 0; i-- {
		if buffer[i] == '\n' {
			// Found a complete line
			lineBytes := buffer[i+1 : end]

			// Check line length limit
			if maxLineBytes > 0 && len(lineBytes) > maxLineBytes {
				// Line too long, but we need to continue to find valid lines
				// Return what we have and signal error through nil carry
				return lines, nil
			}

			line := string(lineBytes)
			// Remove trailing \r for CRLF
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			// Skip empty lines
			if line != "" {
				// Prepend to lines (we're scanning backwards)
				lines = append([]string{line}, lines...)
			}
			end = i
		}
	}

	// Keep only the last maxLines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	// Carry is the incomplete line at the start (before the first newline)
	carry := buffer[:end]
	return lines, carry
}

// sendError sends an error to the error channel.
// With a buffered channel, errors are only dropped if the buffer is full.
// The context case ensures we don't block during shutdown.
func sendError(ctx context.Context, errCh chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case errCh <- err:
	case <-ctx.Done():
		// Don't block during shutdown
	default:
		// Drop error only if buffer is full (rare with buffer size 16)
	}
}

// WatchWithOptions creates a watcher using functional options and starts watching.
// This is the preferred way to create and start a watcher.
//
// Note: This function does not return the underlying Watcher, so callers cannot
// call Close() to perform synchronous shutdown. The watcher will stop when the
// context is cancelled. For more control over shutdown, use NewWatcherWithOptions
// and Watcher.Watch() directly.
//
// Example:
//
//	events, errs, err := vrclog.WatchWithOptions(ctx,
//	    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
//	    vrclog.WithLogger(logger),
//	)
func WatchWithOptions(ctx context.Context, opts ...WatchOption) (<-chan Event, <-chan error, error) {
	w, err := NewWatcherWithOptions(opts...)
	if err != nil {
		return nil, nil, err
	}
	return w.Watch(ctx)
}

// NewWatcherWithOptions creates a watcher using functional options.
// Validates options and checks log directory existence.
// Does NOT start goroutines (cheap to call).
// Returns error for invalid options or missing log directory.
//
// Example:
//
//	watcher, err := vrclog.NewWatcherWithOptions(
//	    vrclog.WithLogDir("/custom/path"),
//	    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	events, errs, err := watcher.Watch(ctx)
func NewWatcherWithOptions(opts ...WatchOption) (*Watcher, error) {
	cfg := applyWatchOptions(opts)

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Find log directory
	logDir, err := logfinder.FindLogDir(cfg.logDir)
	if err != nil {
		return nil, fmt.Errorf("finding log directory: %w", err)
	}

	// Initialize logger (use discard logger if not provided)
	log := cfg.logger
	if log == nil {
		log = discardLogger
	}

	return &Watcher{
		cfg:    *cfg, // copy to ensure immutability
		logDir: logDir,
		log:    log,
	}, nil
}
