package main

import (
	"testing"
)

func TestValidFormats(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"jsonl", true},
		{"pretty", true},
		{"json", false},
		{"xml", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := validFormats[tt.format]
			if got != tt.valid {
				t.Errorf("validFormats[%q] = %v, want %v", tt.format, got, tt.valid)
			}
		})
	}
}

func TestValidEventTypes(t *testing.T) {
	tests := []struct {
		eventType string
		valid     bool
	}{
		{"player_join", true},
		{"player_left", true},
		{"world_join", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			got := validEventTypes[tt.eventType]
			if got != tt.valid {
				t.Errorf("validEventTypes[%q] = %v, want %v", tt.eventType, got, tt.valid)
			}
		})
	}
}
