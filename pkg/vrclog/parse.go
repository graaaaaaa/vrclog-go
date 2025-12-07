package vrclog

import "github.com/vrclog/vrclog-go/internal/parser"

// ParseLine parses a single VRChat log line into an Event.
//
// Return values:
//   - (*Event, nil): Successfully parsed event
//   - (nil, nil): Line doesn't match any known event pattern (not an error)
//   - (nil, error): Line partially matches but is malformed
//
// Example:
//
//	line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
//	event, err := vrclog.ParseLine(line)
//	if err != nil {
//	    log.Printf("parse error: %v", err)
//	} else if event != nil {
//	    fmt.Printf("Player joined: %s\n", event.PlayerName)
//	}
//	// event == nil && err == nil means line is not a recognized event
func ParseLine(line string) (*Event, error) {
	return parser.Parse(line)
}
