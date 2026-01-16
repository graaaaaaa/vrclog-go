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
	// Bump allocator
}

// Host functions (imported from "env" module)
//
//go:wasm-module env
//export regex_match
func regexMatch(strPtr, strLen, rePtr, reLen uint32) uint32

//go:wasm-module env
//export regex_find_submatch
func regexFindSubmatch(strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen uint32) uint32

//go:wasm-module env
//export log
func hostLog(level, ptr, msgLen uint32)

//export parse_line
func parseLine(inputPtr, inputLen uint32) uint64 {
	inputBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(inputPtr))), inputLen)

	var input struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return encodeError("failed to parse input JSON")
	}

	// Test regex_match host function
	pattern := `test_(\w+)`
	linePtr := (*byte)(unsafe.Pointer(unsafe.StringData(input.Line)))
	patternPtr := (*byte)(unsafe.Pointer(unsafe.StringData(pattern)))

	matched := regexMatch(
		uint32(uintptr(unsafe.Pointer(linePtr))),
		uint32(len(input.Line)),
		uint32(uintptr(unsafe.Pointer(patternPtr))),
		uint32(len(pattern)),
	)

	if matched == 0 {
		// No match
		output := map[string]interface{}{
			"ok":     true,
			"events": []interface{}{},
		}
		outputJSON, _ := json.Marshal(output)
		outPtr := alloc(uint32(len(outputJSON)))
		outSlice := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(outPtr))), len(outputJSON))
		copy(outSlice, outputJSON)
		return (uint64(len(outputJSON)) << 32) | uint64(outPtr)
	}

	// Test regex_find_submatch
	var submatchBuf [4096]byte
	submatchLen := regexFindSubmatch(
		uint32(uintptr(unsafe.Pointer(linePtr))),
		uint32(len(input.Line)),
		uint32(uintptr(unsafe.Pointer(patternPtr))),
		uint32(len(pattern)),
		uint32(uintptr(unsafe.Pointer(&submatchBuf[0]))),
		uint32(len(submatchBuf)),
	)

	var captures []string
	if submatchLen > 0 && submatchLen != 0xFFFFFFFF {
		json.Unmarshal(submatchBuf[:submatchLen], &captures)
	}

	// Log via host function
	logMsg := "regex test matched"
	logMsgPtr := (*byte)(unsafe.Pointer(unsafe.StringData(logMsg)))
	hostLog(1, uint32(uintptr(unsafe.Pointer(logMsgPtr))), uint32(len(logMsg)))

	// Return event with captures
	event := map[string]interface{}{
		"timestamp": "2024-01-01T00:00:00Z",
		"type":      "test_regex",
		"data": map[string]interface{}{
			"captures": captures,
		},
	}

	output := map[string]interface{}{
		"ok":     true,
		"events": []interface{}{event},
	}

	outputJSON, _ := json.Marshal(output)
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
