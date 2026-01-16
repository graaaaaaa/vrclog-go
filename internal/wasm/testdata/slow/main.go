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

//export parse_line
func parseLine(inputPtr, inputLen uint32) uint64 {
	// Infinite loop to test timeout
	for {
		// Busy loop
	}
}

func main() {}
