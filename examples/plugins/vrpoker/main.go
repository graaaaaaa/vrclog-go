//go:build tinygo

// Package main implements a VRChat log parser plugin for VRPoker game events.
// This plugin demonstrates how to create custom parsers using WebAssembly.
//
// VRPoker is a poker game world in VRChat. This plugin extracts game events
// like game start, hands dealt, winners, etc. from custom log messages that
// VRPoker outputs to the VRChat log.
//
// Example log lines this plugin parses:
//   - "[VRPoker] Game started with 4 players"
//   - "[VRPoker] Player Alice wins with Royal Flush"
//   - "[VRPoker] Round 3 begins"
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

// Host functions from "env" module
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
	// Read input JSON from INPUT_REGION
	inputBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(inputPtr))), inputLen)

	var input struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal(inputBytes, &input); err != nil {
		return encodeError("failed to parse input JSON")
	}

	// Check if line contains VRPoker prefix
	prefix := "[VRPoker] "
	if !contains(input.Line, prefix) {
		// Not a VRPoker event - return no match
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

	// Parse specific VRPoker events using regex
	var eventType string
	var eventData map[string]interface{}

	// Try matching different event patterns
	if matched, data := parseGameStart(input.Line); matched {
		eventType = "vrpoker_game_start"
		eventData = data
	} else if matched, data := parseWinner(input.Line); matched {
		eventType = "vrpoker_winner"
		eventData = data
	} else if matched, data := parseRound(input.Line); matched {
		eventType = "vrpoker_round"
		eventData = data
	} else {
		// Generic VRPoker event
		eventType = "vrpoker_event"
		eventData = map[string]interface{}{
			"message": input.Line,
		}
	}

	// Create event
	event := map[string]interface{}{
		"timestamp": "2024-01-01T00:00:00Z", // Would be extracted from log line in real impl
		"type":      eventType,
		"data":      eventData,
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

// parseGameStart matches "[VRPoker] Game started with N players"
func parseGameStart(line string) (bool, map[string]interface{}) {
	pattern := `\[VRPoker\] Game started with (\d+) players`
	return matchAndExtract(line, pattern, func(captures []string) map[string]interface{} {
		return map[string]interface{}{
			"player_count": captures[1],
		}
	})
}

// parseWinner matches "[VRPoker] Player NAME wins with HAND"
func parseWinner(line string) (bool, map[string]interface{}) {
	pattern := `\[VRPoker\] Player (.+) wins with (.+)`
	return matchAndExtract(line, pattern, func(captures []string) map[string]interface{} {
		return map[string]interface{}{
			"player": captures[1],
			"hand":   captures[2],
		}
	})
}

// parseRound matches "[VRPoker] Round N begins"
func parseRound(line string) (bool, map[string]interface{}) {
	pattern := `\[VRPoker\] Round (\d+) begins`
	return matchAndExtract(line, pattern, func(captures []string) map[string]interface{} {
		return map[string]interface{}{
			"round": captures[1],
		}
	})
}

// matchAndExtract is a helper to call regex host function and extract data
func matchAndExtract(line, pattern string, extractor func([]string) map[string]interface{}) (bool, map[string]interface{}) {
	linePtr := (*byte)(unsafe.Pointer(unsafe.StringData(line)))
	patternPtr := (*byte)(unsafe.Pointer(unsafe.StringData(pattern)))

	// Check if pattern matches
	matched := regexMatch(
		uint32(uintptr(unsafe.Pointer(linePtr))),
		uint32(len(line)),
		uint32(uintptr(unsafe.Pointer(patternPtr))),
		uint32(len(pattern)),
	)

	if matched == 0 {
		return false, nil
	}

	// Extract captures
	var submatchBuf [4096]byte
	submatchLen := regexFindSubmatch(
		uint32(uintptr(unsafe.Pointer(linePtr))),
		uint32(len(line)),
		uint32(uintptr(unsafe.Pointer(patternPtr))),
		uint32(len(pattern)),
		uint32(uintptr(unsafe.Pointer(&submatchBuf[0]))),
		uint32(len(submatchBuf)),
	)

	if submatchLen == 0 || submatchLen == 0xFFFFFFFF {
		return true, nil
	}

	var captures []string
	json.Unmarshal(submatchBuf[:submatchLen], &captures)

	return true, extractor(captures)
}

// contains checks if s contains substr (simple implementation without strings package)
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
