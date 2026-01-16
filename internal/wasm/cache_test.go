package wasm

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRegexCache_Get(t *testing.T) {
	cache := newRegexCache(3)

	// First access - should compile
	re1, err := cache.Get("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !re1.MatchString("test") {
		t.Error("regex should match 'test'")
	}

	// Second access - should return cached
	re2, err := cache.Get("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if re1 != re2 {
		t.Error("expected same regex instance from cache")
	}

	if cache.Len() != 1 {
		t.Errorf("expected cache len 1, got %d", cache.Len())
	}
}

func TestRegexCache_LRU_Eviction(t *testing.T) {
	cache := newRegexCache(3)

	// Add 3 patterns (fill cache)
	patterns := []string{"a", "b", "c"}
	for _, p := range patterns {
		if _, err := cache.Get(p); err != nil {
			t.Fatalf("unexpected error for pattern %q: %v", p, err)
		}
	}

	if cache.Len() != 3 {
		t.Errorf("expected cache len 3, got %d", cache.Len())
	}

	// Access "a" to move it to front
	if _, err := cache.Get("a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Add new pattern "d" - should evict "b" (oldest)
	if _, err := cache.Get("d"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cache.Len() != 3 {
		t.Errorf("expected cache len 3 after eviction, got %d", cache.Len())
	}

	// "b" should have been evicted
	// Check by accessing all patterns and verifying "a", "c", "d" are cached
	// but "b" requires recompilation (we can't directly test this without exposing internals,
	// but we can verify cache size is still 3)
	for _, p := range []string{"a", "c", "d"} {
		if _, err := cache.Get(p); err != nil {
			t.Errorf("pattern %q should be in cache: %v", p, err)
		}
	}
}

func TestRegexCache_ConcurrentAccess(t *testing.T) {
	cache := newRegexCache(10)

	var wg sync.WaitGroup
	numGoroutines := 50
	numIterations := 100

	patterns := []string{"test", "hello", "world", "foo", "bar"}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				pattern := patterns[j%len(patterns)]
				if _, err := cache.Get(pattern); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		}()
	}

	wg.Wait()

	if cache.Len() > 10 {
		t.Errorf("cache exceeded max size: %d", cache.Len())
	}
}

func TestRegexCache_PatternTooLong(t *testing.T) {
	cache := newRegexCache(10)

	// Pattern exceeds MaxPatternLength (512 bytes)
	longPattern := strings.Repeat("a", MaxPatternLength+1)

	_, err := cache.Get(longPattern)
	if err == nil {
		t.Fatal("expected error for pattern exceeding max length")
	}

	var abiErr *ABIError
	if !isType(err, &abiErr) {
		t.Errorf("expected ABIError, got %T", err)
	}
}

func TestRegexCache_InvalidPattern(t *testing.T) {
	cache := newRegexCache(10)

	// Invalid regex pattern
	_, err := cache.Get("[")
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestRegexCache_DoubleCheckLocking(t *testing.T) {
	cache := newRegexCache(10)

	// This test verifies that the double-check locking pattern works correctly
	// by having multiple goroutines try to compile the same pattern concurrently.
	var wg sync.WaitGroup
	pattern := "concurrent-test"
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.Get(pattern); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	// Should have exactly 1 entry for the pattern
	if cache.Len() != 1 {
		t.Errorf("expected cache len 1, got %d", cache.Len())
	}
}

// isType checks if err is of type *ABIError.
// This is a simplified helper for testing.
func isType(err error, target **ABIError) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ABIError)
	return ok
}

func BenchmarkRegexCache_Get(b *testing.B) {
	cache := newRegexCache(100)
	patterns := make([]string, 50)
	for i := range patterns {
		patterns[i] = fmt.Sprintf("pattern-%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		if _, err := cache.Get(pattern); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkRegexCache_GetParallel(b *testing.B) {
	cache := newRegexCache(100)
	patterns := make([]string, 50)
	for i := range patterns {
		patterns[i] = fmt.Sprintf("pattern-%d", i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			pattern := patterns[i%len(patterns)]
			if _, err := cache.Get(pattern); err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			i++
		}
	})
}
