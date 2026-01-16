package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/api"
	"golang.org/x/time/rate"
)

const (
	// MaxLogSize is the maximum size of a single log message (256 bytes).
	MaxLogSize = 256

	// LogRateLimit is the maximum number of log calls per second (10/sec).
	LogRateLimit = 10

	// RegexTimeout is the maximum time allowed for a regex operation.
	RegexTimeout = 5 * time.Millisecond
)

// hostFunctions provides Host Functions for Wasm plugins.
type hostFunctions struct {
	cache       *regexCache
	logger      *slog.Logger
	rateLimiter *rate.Limiter
}

// newHostFunctions creates a new host functions provider.
func newHostFunctions(logger *slog.Logger) *hostFunctions {
	return &hostFunctions{
		cache:       newRegexCache(DefaultRegexCacheSize),
		logger:      logger,
		rateLimiter: rate.NewLimiter(LogRateLimit, LogRateLimit),
	}
}

// regexMatch implements the regex_match Host Function.
// Signature: (str_ptr, str_len, re_ptr, re_len) -> i32
// Returns 1 if match, 0 if no match or error.
func (h *hostFunctions) regexMatch(ctx context.Context, m api.Module, strPtr, strLen, rePtr, reLen uint32) uint32 {
	// Read input string
	strBytes, ok := m.Memory().Read(strPtr, strLen)
	if !ok {
		return 0
	}
	str := string(strBytes)

	// Read regex pattern
	reBytes, ok := m.Memory().Read(rePtr, reLen)
	if !ok {
		return 0
	}
	pattern := string(reBytes)

	// Get compiled regex from cache
	re, err := h.cache.Get(pattern)
	if err != nil {
		// Log error if logger is available
		if h.logger != nil {
			h.logger.Warn("regex compilation failed",
				"pattern", pattern,
				"error", err)
		}
		return 0
	}

	// Execute regex with timeout
	// Note: Go's regexp package does not support context cancellation. If the regex times out,
	// the goroutine running re.MatchString() will continue executing until it completes.
	// This is an acceptable trade-off because:
	// 1. Go's regexp engine is RE2-based and guarantees linear time (no catastrophic backtracking)
	// 2. MaxPatternLength (512 bytes) limits pattern complexity
	// 3. The 5ms timeout is short enough to limit impact
	// 4. Leaked goroutines will eventually complete and be garbage collected
	ctx, cancel := context.WithTimeout(ctx, RegexTimeout)
	defer cancel()

	resultCh := make(chan bool, 1)
	go func() {
		resultCh <- re.MatchString(str)
	}()

	select {
	case result := <-resultCh:
		if result {
			return 1
		}
		return 0
	case <-ctx.Done():
		// Timeout - goroutine may continue running (see note above)
		if h.logger != nil {
			h.logger.Warn("regex match timeout",
				"pattern", pattern,
				"str_len", len(str))
		}
		return 0
	}
}

// regexFindSubmatch implements the regex_find_submatch Host Function.
// Signature: (str_ptr, str_len, re_ptr, re_len, out_buf_ptr, out_buf_len) -> i32
// Returns number of bytes written, 0 if no match, -1 (0xFFFFFFFF) if buffer too small.
func (h *hostFunctions) regexFindSubmatch(ctx context.Context, m api.Module, strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen uint32) uint32 {
	// Read input string
	strBytes, ok := m.Memory().Read(strPtr, strLen)
	if !ok {
		return 0
	}
	str := string(strBytes)

	// Read regex pattern
	reBytes, ok := m.Memory().Read(rePtr, reLen)
	if !ok {
		return 0
	}
	pattern := string(reBytes)

	// Get compiled regex from cache
	re, err := h.cache.Get(pattern)
	if err != nil {
		if h.logger != nil {
			h.logger.Warn("regex compilation failed",
				"pattern", pattern,
				"error", err)
		}
		return 0
	}

	// Execute regex with timeout
	// Note: Same goroutine leak behavior as regexMatch (see notes there).
	ctx, cancel := context.WithTimeout(ctx, RegexTimeout)
	defer cancel()

	type result struct {
		matches []string
	}

	resultCh := make(chan result, 1)
	go func() {
		matches := re.FindStringSubmatch(str)
		resultCh <- result{matches: matches}
	}()

	var matches []string
	select {
	case res := <-resultCh:
		matches = res.matches
	case <-ctx.Done():
		// Timeout - goroutine may continue running (see regexMatch notes)
		if h.logger != nil {
			h.logger.Warn("regex find submatch timeout",
				"pattern", pattern,
				"str_len", len(str))
		}
		return 0
	}

	// No match
	if matches == nil {
		return 0
	}

	// Encode matches as JSON array
	jsonBytes, err := json.Marshal(matches)
	if err != nil {
		if h.logger != nil {
			h.logger.Error("failed to marshal submatch results", "error", err)
		}
		return 0
	}

	// Check if buffer is large enough
	if uint32(len(jsonBytes)) > outBufLen {
		// Buffer too small
		return 0xFFFFFFFF
	}

	// Write to output buffer
	if !m.Memory().Write(outBufPtr, jsonBytes) {
		return 0
	}

	return uint32(len(jsonBytes))
}

// log implements the log Host Function.
// Signature: (level, ptr, len)
// Levels: 0=debug, 1=info, 2=warn, 3=error
func (h *hostFunctions) log(ctx context.Context, m api.Module, level, ptr, msgLen uint32) {
	// Rate limiting
	if !h.rateLimiter.Allow() {
		// Silently drop log message if rate limit exceeded
		return
	}

	// Limit message size
	truncated := false
	if msgLen > MaxLogSize {
		truncated = true
		msgLen = MaxLogSize
	}

	// Read log message
	msgBytes, ok := m.Memory().Read(ptr, msgLen)
	if !ok {
		return
	}

	// Sanitize UTF-8
	msg := strings.ToValidUTF8(string(msgBytes), "\ufffd")
	if truncated {
		msg += " [truncated]"
	}

	// Output log
	if h.logger == nil {
		return
	}

	switch level {
	case 0: // debug
		h.logger.Debug("[plugin] " + msg)
	case 1: // info
		h.logger.Info("[plugin] " + msg)
	case 2: // warn
		h.logger.Warn("[plugin] " + msg)
	case 3: // error
		h.logger.Error("[plugin] " + msg)
	default:
		h.logger.Info(fmt.Sprintf("[plugin] (level=%d) %s", level, msg))
	}
}

// nowMs implements the now_ms Host Function.
// Signature: () -> i64
// Returns current Unix time in milliseconds.
func (h *hostFunctions) nowMs() int64 {
	return time.Now().UnixMilli()
}
