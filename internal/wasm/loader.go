package wasm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/vrclog/vrclog-go/internal/safefile"
)

const (
	// MaxWasmFileSize is the maximum size of a Wasm file (10MB).
	MaxWasmFileSize = 10 * 1024 * 1024

	// ExpectedABIVersion is the ABI version this implementation supports.
	ExpectedABIVersion = 1

	// INPUT_REGION is the fixed memory region where the host writes input data.
	// This is 64KB offset (0x10000), which doesn't conflict with TinyGo's heap.
	INPUT_REGION = 0x10000

	// INPUT_REGION_SIZE is the size of the input region (8KB).
	INPUT_REGION_SIZE = 8192
)

// CompiledWasm represents a compiled Wasm module ready for instantiation.
type CompiledWasm struct {
	runtime        wazero.Runtime
	compiled       wazero.CompiledModule
	cache          wazero.CompilationCache
	hostFunctions  *hostFunctions
}

// Close releases resources held by the compiled Wasm.
// Resources are closed in reverse order of creation: cache, compiled module, runtime.
// Safe to call multiple times.
func (c *CompiledWasm) Close(ctx context.Context) error {
	var firstErr error

	// Close cache first (if present)
	if c.cache != nil {
		if err := c.cache.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		c.cache = nil // Prevent double-close
	}

	// Close compiled module before runtime
	if c.compiled != nil {
		if err := c.compiled.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		c.compiled = nil // Prevent double-close
	}

	// Close runtime last
	if c.runtime != nil {
		if err := c.runtime.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		c.runtime = nil // Prevent double-close
	}

	return firstErr
}

// LoadWasm loads and compiles a Wasm file.
func LoadWasm(ctx context.Context, path string, logger *slog.Logger) (*CompiledWasm, error) {
	// Open file with TOCTOU and symlink protection
	f, info, err := safefile.OpenRegular(path)
	if err != nil {
		if errors.Is(err, safefile.ErrNotRegularFile) {
			return nil, fmt.Errorf("wasm path is not a regular file: %w", err)
		}
		return nil, fmt.Errorf("failed to open wasm file: %w", err)
	}
	defer f.Close()

	// Validate file size
	if info.Size() > MaxWasmFileSize {
		return nil, ErrFileTooLarge
	}

	// Read Wasm file with size limit (prevent TOCTOU where file grows after stat)
	wasmBytes, err := io.ReadAll(io.LimitReader(f, MaxWasmFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read wasm file: %w", err)
	}
	if int64(len(wasmBytes)) > MaxWasmFileSize {
		return nil, ErrFileTooLarge
	}

	// Create runtime with configuration
	rtConfig := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true) // Enable Context-based timeout

	// Setup disk cache
	cacheDir, err := getCacheDir()
	var cache wazero.CompilationCache
	if err == nil {
		cache, err = wazero.NewCompilationCacheWithDir(cacheDir)
		if err == nil {
			rtConfig = rtConfig.WithCompilationCache(cache)
			if logger != nil {
				logger.Debug("using wasm compilation cache", "dir", cacheDir)
			}
		} else if logger != nil {
			logger.Warn("failed to create compilation cache, continuing without cache", "error", err)
		}
	}

	rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)

	// Instantiate WASI
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		// Use background context for cleanup to avoid cancelled context issues
		cleanupCtx := context.Background()
		rt.Close(cleanupCtx)
		if cache != nil {
			cache.Close(cleanupCtx)
		}
		return nil, &WasmRuntimeError{Operation: "wasi instantiation", Err: err}
	}

	// Create host functions
	hf := newHostFunctions(logger)

	// Register host functions in the "env" module
	envBuilder := rt.NewHostModuleBuilder("env")

	// regex_match: (str_ptr, str_len, re_ptr, re_len) -> i32
	envBuilder = envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, strPtr, strLen, rePtr, reLen uint32) uint32 {
			return hf.regexMatch(ctx, m, strPtr, strLen, rePtr, reLen)
		}).
		Export("regex_match")

	// regex_find_submatch: (str_ptr, str_len, re_ptr, re_len, out_buf_ptr, out_buf_len) -> i32
	envBuilder = envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen uint32) uint32 {
			return hf.regexFindSubmatch(ctx, m, strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen)
		}).
		Export("regex_find_submatch")

	// log: (level, ptr, len) -> void
	envBuilder = envBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, level, ptr, msgLen uint32) {
			hf.log(ctx, m, level, ptr, msgLen)
		}).
		Export("log")

	// now_ms: () -> i64
	envBuilder = envBuilder.NewFunctionBuilder().
		WithFunc(func() int64 {
			return hf.nowMs()
		}).
		Export("now_ms")

	if _, err := envBuilder.Instantiate(ctx); err != nil {
		cleanupCtx := context.Background()
		rt.Close(cleanupCtx)
		if cache != nil {
			cache.Close(cleanupCtx)
		}
		return nil, &WasmRuntimeError{Operation: "host functions registration", Err: err}
	}

	// Compile Wasm module (AOT)
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		cleanupCtx := context.Background()
		rt.Close(cleanupCtx)
		if cache != nil {
			cache.Close(cleanupCtx)
		}
		return nil, &WasmRuntimeError{Operation: "wasm compilation", Err: err}
	}

	// Validate ABI
	if err := validateABI(compiled); err != nil {
		cleanupCtx := context.Background()
		compiled.Close(cleanupCtx)
		rt.Close(cleanupCtx)
		if cache != nil {
			cache.Close(cleanupCtx)
		}
		return nil, err
	}

	return &CompiledWasm{
		runtime:       rt,
		compiled:      compiled,
		cache:         cache,
		hostFunctions: hf,
	}, nil
}

// validateABI checks that the Wasm module exports required functions.
// Note: This only validates function existence, not their behavior.
// Actual ABI version validation is done in Load() by calling abi_version().
func validateABI(compiled wazero.CompiledModule) error {
	// Required exports
	requiredExports := []string{"abi_version", "alloc", "free", "parse_line"}

	exportedFunctions := compiled.ExportedFunctions()
	exportMap := make(map[string]bool)
	for name := range exportedFunctions {
		exportMap[name] = true
	}

	for _, name := range requiredExports {
		if !exportMap[name] {
			return &ABIError{
				Function: name,
				Reason:   "missing required export",
			}
		}
	}

	return nil
}

// getCacheDir returns the wazero compilation cache directory.
// It follows the XDG Base Directory specification.
func getCacheDir() (string, error) {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cacheHome = filepath.Join(home, ".cache")
	}
	dir := filepath.Join(cacheHome, "vrclog", "wasm")

	// Create directory with 0700 permissions (user-only access)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	return dir, nil
}
