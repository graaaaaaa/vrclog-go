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

var heapPtr uintptr = 0x20000

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
	// Bump allocator doesn't free
}

//export parse_line
func parseLine(inputPtr, inputLen uint32) uint64 {
	inputBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(inputPtr))), inputLen)

	var input struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return encodeError("failed to parse input JSON")
	}

	// Echo test: return a single event with the line as data
	event := map[string]interface{}{
		"timestamp": "2024-01-01T00:00:00Z",
		"type":      "test_echo",
		"data": map[string]interface{}{
			"line": input.Line,
		},
	}

	output := map[string]interface{}{
		"ok":     true,
		"events": []interface{}{event},
	}

	outputJSON, err := json.Marshal(output)
	if err != nil {
		return encodeError("failed to marshal output")
	}

	outPtr := alloc(uint32(len(outputJSON)))
	outSlice := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), len(outputJSON))
	copy(outSlice, outputJSON)

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
