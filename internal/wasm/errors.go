// Package wasm provides WebAssembly plugin support for vrclog-go.
package wasm

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidWasm indicates the Wasm file is invalid or corrupted.
	ErrInvalidWasm = errors.New("invalid wasm file")

	// ErrABIVersionMismatch indicates the plugin's ABI version is incompatible.
	ErrABIVersionMismatch = errors.New("abi version mismatch")

	// ErrMissingExport indicates a required export function is missing.
	ErrMissingExport = errors.New("missing required export function")

	// ErrPluginPanic indicates the plugin panicked during execution.
	ErrPluginPanic = errors.New("plugin panicked")

	// ErrTimeout indicates the plugin exceeded the execution timeout.
	ErrTimeout = errors.New("plugin timeout")

	// ErrFileTooLarge indicates the Wasm file exceeds the size limit.
	ErrFileTooLarge = errors.New("wasm file too large")
)

// ABIError represents an error related to ABI validation.
type ABIError struct {
	Function string
	Reason   string
}

func (e *ABIError) Error() string {
	return fmt.Sprintf("abi error in %s: %s", e.Function, e.Reason)
}

// PluginError represents an error returned by the plugin.
type PluginError struct {
	Code    string
	Message string
}

func (e *PluginError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("plugin error %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("plugin error: %s", e.Message)
}

// WasmRuntimeError represents a wazero runtime error.
type WasmRuntimeError struct {
	Operation string
	Err       error
}

func (e *WasmRuntimeError) Error() string {
	return fmt.Sprintf("wasm runtime error during %s: %v", e.Operation, e.Err)
}

func (e *WasmRuntimeError) Unwrap() error {
	return e.Err
}
