package wasm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

const (
	// DefaultTimeout is the default timeout for parse_line execution.
	DefaultTimeout = 50 * time.Millisecond

	// MaxOutputSize is the maximum size of output from parse_line (1MB).
	// This prevents memory exhaustion from malicious plugins.
	MaxOutputSize = 1 * 1024 * 1024
)

// WasmParser implements vrclog.Parser using a WebAssembly plugin.
// It is goroutine-safe: each ParseLine call creates a new module instance.
type WasmParser struct {
	compiled      *CompiledWasm
	timeout       atomic.Int64 // Timeout in nanoseconds (use atomic for thread-safety)
	logger        *slog.Logger
	abiVersion    uint32        // Cached ABI version (validated at load time)
	moduleCounter atomic.Uint64 // Counter for unique module names
}

// Load loads a Wasm plugin from the given file path.
func Load(ctx context.Context, path string, logger *slog.Logger) (*WasmParser, error) {
	compiled, err := LoadWasm(ctx, path, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load wasm: %w", err)
	}

	// Validate ABI version by instantiating once
	modConfig := wazero.NewModuleConfig().WithName("plugin-init")
	mod, err := compiled.runtime.InstantiateModule(ctx, compiled.compiled, modConfig)
	if err != nil {
		cleanupCtx := context.Background()
		compiled.Close(cleanupCtx)
		return nil, &WasmRuntimeError{Operation: "initial module instantiation", Err: err}
	}

	// Call abi_version()
	abiVersionFn := mod.ExportedFunction("abi_version")
	if abiVersionFn == nil {
		cleanupCtx := context.Background()
		mod.Close(cleanupCtx)
		compiled.Close(cleanupCtx)
		return nil, &ABIError{Function: "abi_version", Reason: "not exported"}
	}

	results, err := abiVersionFn.Call(ctx)
	mod.Close(ctx) // Close init instance immediately (use original ctx since this is not error path)
	if err != nil {
		cleanupCtx := context.Background()
		compiled.Close(cleanupCtx)
		return nil, &WasmRuntimeError{Operation: "abi_version call", Err: err}
	}
	if len(results) == 0 {
		cleanupCtx := context.Background()
		compiled.Close(cleanupCtx)
		return nil, &ABIError{Function: "abi_version", Reason: "no return value"}
	}

	abiVersion := uint32(results[0])
	if abiVersion != ExpectedABIVersion {
		cleanupCtx := context.Background()
		compiled.Close(cleanupCtx)
		return nil, ErrABIVersionMismatch
	}

	p := &WasmParser{
		compiled:   compiled,
		logger:     logger,
		abiVersion: abiVersion,
	}
	p.timeout.Store(int64(DefaultTimeout))
	return p, nil
}

// ParseLine parses a single log line using the Wasm plugin.
// This method is goroutine-safe.
func (p *WasmParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
	// Check if parser has been closed
	if p.compiled == nil {
		return vrclog.ParseResult{}, errors.New("parser is closed")
	}

	// Apply timeout (load atomically for thread-safety)
	timeout := time.Duration(p.timeout.Load())
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Instantiate module (creates new instance for goroutine safety)
	// ABI version was already validated during Load()
	// Use unique module name to avoid conflicts in concurrent calls
	name := fmt.Sprintf("plugin-%d", p.moduleCounter.Add(1))
	modConfig := wazero.NewModuleConfig().WithName(name)
	mod, err := p.compiled.runtime.InstantiateModule(ctx, p.compiled.compiled, modConfig)
	if err != nil {
		return vrclog.ParseResult{}, &WasmRuntimeError{Operation: "module instantiation", Err: err}
	}
	defer mod.Close(context.Background())

	// Prepare input JSON
	type inputData struct {
		Line string `json:"line"`
	}
	input := inputData{Line: line}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return vrclog.ParseResult{}, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Check input size
	if len(inputJSON) > INPUT_REGION_SIZE {
		return vrclog.ParseResult{}, fmt.Errorf("input too large: %d bytes (max %d)", len(inputJSON), INPUT_REGION_SIZE)
	}

	// Verify INPUT_REGION is within memory bounds
	memSize := mod.Memory().Size()
	requiredSize := INPUT_REGION + uint32(len(inputJSON))
	if requiredSize > memSize {
		return vrclog.ParseResult{}, fmt.Errorf("INPUT_REGION (0x%x) + input size (%d) exceeds wasm memory size (%d bytes). Plugin may need larger initial memory", INPUT_REGION, len(inputJSON), memSize)
	}

	// Write input to INPUT_REGION
	if !mod.Memory().Write(INPUT_REGION, inputJSON) {
		return vrclog.ParseResult{}, fmt.Errorf("failed to write input to wasm memory")
	}

	// Call parse_line
	parseLineFn := mod.ExportedFunction("parse_line")
	if parseLineFn == nil {
		return vrclog.ParseResult{}, &ABIError{Function: "parse_line", Reason: "not exported"}
	}

	results, err := parseLineFn.Call(ctx, uint64(INPUT_REGION), uint64(len(inputJSON)))
	if err != nil {
		// Check for context errors (timeout or cancellation)
		if ctx.Err() != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return vrclog.ParseResult{}, ErrTimeout
			}
			// Return context.Canceled directly
			return vrclog.ParseResult{}, ctx.Err()
		}
		return vrclog.ParseResult{}, &WasmRuntimeError{Operation: "parse_line call", Err: err}
	}

	if len(results) == 0 {
		return vrclog.ParseResult{}, &ABIError{Function: "parse_line", Reason: "no return value"}
	}

	// Decode return value: (out_len << 32) | out_ptr
	packed := results[0]
	outPtr := uint32(packed & 0xFFFFFFFF)
	outLen := uint32(packed >> 32)

	// Validate output size before reading (prevent memory exhaustion)
	if outLen > MaxOutputSize {
		return vrclog.ParseResult{}, fmt.Errorf("plugin output too large: %d bytes (max %d)", outLen, MaxOutputSize)
	}

	// Read output from wasm memory
	outBytes, ok := mod.Memory().Read(outPtr, outLen)
	if !ok {
		return vrclog.ParseResult{}, fmt.Errorf("failed to read output from wasm memory")
	}

	// CRITICAL: Copy memory before free() - wazero's Memory().Read() returns a view, not a copy.
	// After free(), the plugin may overwrite this memory region, corrupting our data.
	outBytesCopy := make([]byte, len(outBytes))
	copy(outBytesCopy, outBytes)

	// Free output buffer (safe now that we have our own copy)
	freeFn := mod.ExportedFunction("free")
	if freeFn != nil {
		// Call free(out_ptr, out_len)
		// Ignoring error as this is cleanup and we already have the data
		_, _ = freeFn.Call(ctx, uint64(outPtr), uint64(outLen))
	}

	// Parse output JSON (using our safe copy)
	type outputData struct {
		Ok     bool          `json:"ok"`
		Events []event.Event `json:"events"`
		Error  *string       `json:"error,omitempty"`
		Code   *string       `json:"code,omitempty"`
	}

	var output outputData
	if err := json.Unmarshal(outBytesCopy, &output); err != nil {
		return vrclog.ParseResult{}, fmt.Errorf("failed to unmarshal output: %w", err)
	}

	// Handle error response
	if !output.Ok {
		errMsg := "unknown error"
		if output.Error != nil {
			errMsg = *output.Error
		}
		code := ""
		if output.Code != nil {
			code = *output.Code
		}
		return vrclog.ParseResult{}, &PluginError{Code: code, Message: errMsg}
	}

	// Success
	if len(output.Events) == 0 {
		return vrclog.ParseResult{Matched: false}, nil
	}

	return vrclog.ParseResult{
		Matched: true,
		Events:  output.Events,
	}, nil
}

// Close releases resources held by the parser.
// Implements io.Closer.
// Safe to call multiple times.
func (p *WasmParser) Close() error {
	if p.compiled == nil {
		return nil // Already closed
	}
	err := p.compiled.Close(context.Background())
	p.compiled = nil // Prevent double-close and enable nil check in ParseLine
	return err
}

// SetTimeout sets the parse_line execution timeout.
// This method is goroutine-safe.
func (p *WasmParser) SetTimeout(timeout time.Duration) {
	p.timeout.Store(int64(timeout))
}
