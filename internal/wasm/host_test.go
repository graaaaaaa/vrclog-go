package wasm

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestNewHostFunctions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	hf := newHostFunctions(logger)

	if hf.cache == nil {
		t.Error("cache should be initialized")
	}
	if hf.logger == nil {
		t.Error("logger should be set")
	}
	if hf.rateLimiter == nil {
		t.Error("rateLimiter should be initialized")
	}
}

func TestHostFunctions_NowMs(t *testing.T) {
	hf := newHostFunctions(nil)

	before := time.Now().UnixMilli()
	result := hf.nowMs()
	after := time.Now().UnixMilli()

	if result < before || result > after {
		t.Errorf("nowMs returned %d, expected between %d and %d", result, before, after)
	}
}

func TestHostFunctions_RateLimiter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Quiet for test
	}))
	hf := newHostFunctions(logger)

	// Should allow first LogRateLimit calls
	for i := 0; i < LogRateLimit; i++ {
		if !hf.rateLimiter.Allow() {
			t.Errorf("call %d should be allowed", i)
		}
	}

	// Next call should be rate limited
	if hf.rateLimiter.Allow() {
		t.Error("expected rate limit to be enforced")
	}

	// Wait for rate limiter to refill
	time.Sleep(time.Second)

	// Should allow calls again
	if !hf.rateLimiter.Allow() {
		t.Error("rate limiter should have refilled")
	}
}

func TestHostFunctions_CacheIntegration(t *testing.T) {
	hf := newHostFunctions(nil)

	// Cache should be accessible and functional
	re, err := hf.cache.Get("test")
	if err != nil {
		t.Fatalf("cache.Get failed: %v", err)
	}
	if !re.MatchString("test") {
		t.Error("regex should match 'test'")
	}

	// Pattern too long should error
	longPattern := make([]byte, MaxPatternLength+1)
	for i := range longPattern {
		longPattern[i] = 'a'
	}
	_, err = hf.cache.Get(string(longPattern))
	if err == nil {
		t.Error("expected error for pattern exceeding max length")
	}
}

// Note: Full integration tests for regexMatch, regexFindSubmatch, and log
// require a Wasm module instance with Memory, which will be tested in
// parser_test.go once the full plugin infrastructure is implemented.
