package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/vrclog/vrclog-go/internal/wasm"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

// buildParser builds a Parser from pattern file paths and plugin file paths.
// Returns nil parser if no patterns/plugins are specified (use default parser).
// Returns a cleanup function that must be called to release resources (use defer).
// The cleanup function is always non-nil, even on error.
// If pluginTimeout is > 0, it will be applied to all loaded plugins.
func buildParser(ctx context.Context, patternFiles, pluginFiles []string, pluginTimeout time.Duration, logger *slog.Logger) (vrclog.Parser, func(), error) {
	noop := func() {} // No-op cleanup for error cases

	if len(patternFiles) == 0 && len(pluginFiles) == 0 {
		return nil, noop, nil
	}

	parsers := []vrclog.Parser{vrclog.DefaultParser{}}
	var cleanups []func()

	// Load YAML pattern files
	for i, path := range patternFiles {
		rp, err := pattern.NewRegexParserFromFile(path)
		if err != nil {
			// Error from pattern package is already sanitized (no path)
			return nil, noop, fmt.Errorf("pattern file %d: %w", i+1, err)
		}
		parsers = append(parsers, rp)
	}

	// Load Wasm plugins
	for i, path := range pluginFiles {
		wp, err := wasm.Load(ctx, path, logger)
		if err != nil {
			// Cleanup already-loaded plugins before returning error
			for _, cleanup := range cleanups {
				cleanup()
			}
			return nil, noop, fmt.Errorf("plugin file %d: %w", i+1, err)
		}
		// Apply timeout if specified
		if pluginTimeout > 0 {
			wp.SetTimeout(pluginTimeout)
		}
		parsers = append(parsers, wp)
		cleanups = append(cleanups, func() { wp.Close() })
	}

	cleanup := func() {
		for _, c := range cleanups {
			c()
		}
	}

	return &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: parsers,
	}, cleanup, nil
}
