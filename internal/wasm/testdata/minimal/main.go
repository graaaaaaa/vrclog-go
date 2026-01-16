//go:build tinygo

package main

import (
	"encoding/json"
	"unsafe"
)

const (
	INPUT_REGION      = 0x10000
	INPUT_REGION_SIZE = 8192
)

// Bump allocator for simple memory management
var heapPtr uintptr = 0x20000 // Start after INPUT_REGION

//export abi_version
func abiVersion() uint32 {
	return 1
}

//export alloc
func alloc(size uint32) uint32 {
	ptr := uint32(heapPtr)
	heapPtr += uintptr(size)
	return ptr
}

//export free
func free(ptr, size uint32) {
	// Bump allocator doesn't free individual allocations
}

//export parse_line
func parseLine(inputPtr, inputLen uint32) uint64 {
	// Read input JSON from INPUT_REGION
	inputBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(inputPtr))), inputLen)

	var input struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return encodeError("failed to parse input JSON")
	}

	// Minimal test: always return no match
	output := map[string]interface{}{
		"ok":     true,
		"events": []interface{}{},
	}

	outputJSON, err := json.Marshal(output)
	if err != nil {
		return encodeError("failed to marshal output")
	}

	// Allocate output buffer
	outPtr := alloc(uint32(len(outputJSON)))
	outSlice := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), len(outputJSON))
	copy(outSlice, outputJSON)

	// Pack (out_len << 32) | out_ptr
	return (uint64(len(outputJSON)) << 32) | uint64(outPtr)
}

func encodeError(msg string) uint64 {
	output := map[string]interface{}{
		"ok":    false,
		"error": msg,
	}
	outputJSON, _ := json.Marshal(output)
	outPtr := alloc(uint32(len(outputJSON)))
	outSlice := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), len(outputJSON))
	copy(outSlice, outputJSON)
	return (uint64(len(outputJSON)) << 32) | uint64(outPtr)
}

func main() {}
