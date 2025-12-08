package vrclog_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// ExampleWatchWithOptions demonstrates basic usage of the WatchWithOptions convenience function.
func ExampleWatchWithOptions() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watching with functional options (auto-detect log directory)
	events, errs, err := vrclog.WatchWithOptions(ctx,
		vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Process events
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			switch event.Type {
			case vrclog.EventPlayerJoin:
				fmt.Printf("%s joined\n", event.PlayerName)
			case vrclog.EventPlayerLeft:
				fmt.Printf("%s left\n", event.PlayerName)
			case vrclog.EventWorldJoin:
				fmt.Printf("Joined world: %s\n", event.WorldName)
			}
		case err, ok := <-errs:
			if !ok {
				return
			}
			log.Printf("error: %v", err)
		case <-ctx.Done():
			return
		}
	}
}

// ExampleNewWatcherWithOptions demonstrates advanced usage with explicit Watcher control.
func ExampleNewWatcherWithOptions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create watcher with functional options
	watcher, err := vrclog.NewWatcherWithOptions(
		// LogDir auto-detected if not specified
		vrclog.WithPollInterval(5*time.Second),
		vrclog.WithIncludeRawLine(true),
		vrclog.WithReplayLastN(100),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Start watching
	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			fmt.Printf("[%s] %s: %s\n",
				event.Timestamp.Format("15:04:05"),
				event.Type,
				event.PlayerName)
		case err, ok := <-errs:
			if !ok {
				return
			}
			log.Printf("error: %v", err)
		case <-ctx.Done():
			return
		}
	}
}

// ExampleParseLine demonstrates parsing a single log line.
func ExampleParseLine() {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser"

	event, err := vrclog.ParseLine(line)
	if err != nil {
		log.Printf("parse error: %v", err)
		return
	}

	if event == nil {
		// Line doesn't match any known event pattern
		fmt.Println("Not a recognized event")
		return
	}

	fmt.Printf("Type: %s\n", event.Type)
	fmt.Printf("Player: %s\n", event.PlayerName)
	// Output:
	// Type: player_join
	// Player: TestUser
}

// ExampleParseLine_worldJoin demonstrates parsing a world join event.
func ExampleParseLine_worldJoin() {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World"

	event, err := vrclog.ParseLine(line)
	if err != nil {
		log.Printf("parse error: %v", err)
		return
	}

	if event != nil {
		fmt.Printf("Type: %s\n", event.Type)
		fmt.Printf("World: %s\n", event.WorldName)
	}
	// Output:
	// Type: world_join
	// World: Test World
}
