package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// ValidFormats lists all valid output formats.
var ValidFormats = map[string]bool{
	"jsonl":  true,
	"pretty": true,
}

// OutputEvent writes an event in the specified format to the writer.
func OutputEvent(format string, event vrclog.Event, out io.Writer) error {
	switch format {
	case "jsonl":
		return OutputJSON(event, out)
	case "pretty":
		return OutputPretty(event, out)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// OutputJSON writes an event as JSON Lines format.
func OutputJSON(event vrclog.Event, out io.Writer) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

// OutputPretty writes an event in human-readable format.
func OutputPretty(event vrclog.Event, out io.Writer) error {
	ts := event.Timestamp.Format("15:04:05")

	var err error
	switch event.Type {
	case vrclog.EventPlayerJoin:
		_, err = fmt.Fprintf(out, "[%s] + %s joined\n", ts, event.PlayerName)
	case vrclog.EventPlayerLeft:
		_, err = fmt.Fprintf(out, "[%s] - %s left\n", ts, event.PlayerName)
	case vrclog.EventWorldJoin:
		if event.WorldName != "" {
			_, err = fmt.Fprintf(out, "[%s] > Joined world: %s\n", ts, event.WorldName)
		} else {
			_, err = fmt.Fprintf(out, "[%s] > Joined instance: %s\n", ts, event.InstanceID)
		}
	default:
		_, err = fmt.Fprintf(out, "[%s] ? %s\n", ts, event.Type)
	}

	return err
}
