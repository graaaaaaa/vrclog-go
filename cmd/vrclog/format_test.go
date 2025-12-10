package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestOutputJSON(t *testing.T) {
	event := vrclog.Event{
		Type:       vrclog.EventPlayerJoin,
		Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
		PlayerName: "TestUser",
		PlayerID:   "usr_12345",
	}

	var buf bytes.Buffer
	err := OutputJSON(event, &buf)
	if err != nil {
		t.Fatalf("OutputJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var decoded vrclog.Event
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("OutputJSON() produced invalid JSON: %v", err)
	}

	if decoded.PlayerName != "TestUser" {
		t.Errorf("decoded.PlayerName = %q, want %q", decoded.PlayerName, "TestUser")
	}
}

func TestOutputPretty(t *testing.T) {
	tests := []struct {
		name     string
		event    vrclog.Event
		contains string
	}{
		{
			name: "player_join",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerJoin,
				Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				PlayerName: "TestUser",
			},
			contains: "+ TestUser joined",
		},
		{
			name: "player_left",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerLeft,
				Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				PlayerName: "TestUser",
			},
			contains: "- TestUser left",
		},
		{
			name: "world_join_with_name",
			event: vrclog.Event{
				Type:      vrclog.EventWorldJoin,
				Timestamp: time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				WorldName: "Test World",
			},
			contains: "> Joined world: Test World",
		},
		{
			name: "world_join_instance_only",
			event: vrclog.Event{
				Type:       vrclog.EventWorldJoin,
				Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				InstanceID: "12345~private",
			},
			contains: "> Joined instance: 12345~private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := OutputPretty(tt.event, &buf)
			if err != nil {
				t.Fatalf("OutputPretty() error = %v", err)
			}

			if !strings.Contains(buf.String(), tt.contains) {
				t.Errorf("OutputPretty() = %q, want to contain %q", buf.String(), tt.contains)
			}
		})
	}
}

func TestOutputEvent(t *testing.T) {
	event := vrclog.Event{
		Type:       vrclog.EventPlayerJoin,
		Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
		PlayerName: "TestUser",
	}

	tests := []struct {
		format    string
		wantErr   bool
		checkFunc func(string) bool
	}{
		{
			format:  "jsonl",
			wantErr: false,
			checkFunc: func(s string) bool {
				return strings.Contains(s, `"player_name":"TestUser"`)
			},
		},
		{
			format:  "pretty",
			wantErr: false,
			checkFunc: func(s string) bool {
				return strings.Contains(s, "+ TestUser joined")
			},
		},
		{
			format:  "unknown",
			wantErr: true,
			checkFunc: func(s string) bool {
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			var buf bytes.Buffer
			err := OutputEvent(tt.format, event, &buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("OutputEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !tt.checkFunc(buf.String()) {
				t.Errorf("OutputEvent() output check failed: %q", buf.String())
			}
		})
	}
}
