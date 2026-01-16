package wasm

import (
	"container/list"
	"regexp"
	"sync"
)

const (
	// DefaultRegexCacheSize is the default maximum number of cached regex patterns.
	DefaultRegexCacheSize = 100

	// MaxPatternLength is the maximum length of a regex pattern (ReDoS protection).
	MaxPatternLength = 512
)

// regexCache is an LRU cache for compiled regular expressions.
// It is thread-safe and can be accessed concurrently.
type regexCache struct {
	mu      sync.RWMutex
	cache   map[string]*list.Element
	lruList *list.List
	maxSize int
}

// cacheEntry represents a single cache entry.
type cacheEntry struct {
	pattern string
	re      *regexp.Regexp
}

// newRegexCache creates a new LRU regex cache with the given maximum size.
func newRegexCache(maxSize int) *regexCache {
	return &regexCache{
		cache:   make(map[string]*list.Element),
		lruList: list.New(),
		maxSize: maxSize,
	}
}

// Get retrieves a compiled regex from the cache, or compiles it if not cached.
// Returns an error if the pattern is invalid or exceeds the maximum length.
func (c *regexCache) Get(pattern string) (*regexp.Regexp, error) {
	// Validate pattern length (ReDoS protection)
	if len(pattern) > MaxPatternLength {
		return nil, &ABIError{
			Function: "regex_match",
			Reason:   "pattern exceeds maximum length",
		}
	}

	// Try to get from cache (read lock)
	c.mu.RLock()
	if elem, ok := c.cache[pattern]; ok {
		c.mu.RUnlock()
		// Move to front (write lock)
		c.mu.Lock()
		c.lruList.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		c.mu.Unlock()
		return entry.re, nil
	}
	c.mu.RUnlock()

	// Not in cache - compile it
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	// Add to cache (write lock)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine might have added it while we were compiling
	if elem, ok := c.cache[pattern]; ok {
		c.lruList.MoveToFront(elem)
		return elem.Value.(*cacheEntry).re, nil
	}

	// Evict oldest entry if cache is full
	if c.lruList.Len() >= c.maxSize {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			oldEntry := oldest.Value.(*cacheEntry)
			delete(c.cache, oldEntry.pattern)
		}
	}

	// Add new entry
	entry := &cacheEntry{
		pattern: pattern,
		re:      re,
	}
	elem := c.lruList.PushFront(entry)
	c.cache[pattern] = elem

	return re, nil
}

// Len returns the current number of cached patterns.
func (c *regexCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}
