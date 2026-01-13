# Examples

This directory contains runnable examples demonstrating various features of vrclog-go.

## Prerequisites

- Go 1.25 or later
- For `watch-events`: VRChat installed and running on Windows (or access to VRChat log files)

## Running Examples

```bash
# Run from repository root
go run ./examples/custom-parser
go run ./examples/parser-chain
go run ./examples/watch-events
```

---

## Examples Overview

### 1. custom-parser

**File**: `custom-parser/main.go`

**What it demonstrates**:
- Defining custom event patterns using YAML
- Using `RegexParser` to extract game-specific events
- Named capture groups to populate `Event.Data`

**Use case**: Extracting custom events from VRChat world logs (e.g., poker game events, quest progress, player stats).

**Key concepts**:
- `pattern.LoadBytes()` - Load patterns from YAML bytes
- `pattern.NewRegexParser()` - Create regex-based parser
- Named capture groups `(?P<name>...)` populate `Event.Data`

**Output example**:
```
Event: poker_hole_cards
  Timestamp: 12:00:00
  Data:
    card1: AceSpades
    card2: KingHearts
```

---

### 2. parser-chain

**File**: `parser-chain/main.go`

**What it demonstrates**:
- Combining `DefaultParser` with `RegexParser` using `ParserChain`
- Handling both built-in VRChat events and custom game events
- Three chain modes: `ChainAll`, `ChainFirst`, `ChainContinueOnError`

**Use case**: Processing logs that contain both standard VRChat events (player join/leave, world join) and custom world-specific events.

**Key concepts**:
- `vrclog.DefaultParser{}` - Handles built-in VRChat events
- `vrclog.ParserChain` - Combines multiple parsers
- `ChainAll` mode - Tries all parsers (each line can produce multiple events)

**Output example**:
```
[1] ✓ player_join - Player: Alice
[2] ✓ poker_hand - Cards: AceSpades, KingDiamonds
[3] ✓ player_left - Player: Bob
[4] ✓ poker_winner - Seat 2 won 150 chips
[5] ✓ world_join - World: Poker VIP
```

---

### 3. watch-events

**File**: `watch-events/main.go`

**What it demonstrates**:
- Real-time log monitoring with `WatchWithOptions()`
- Event type filtering with `WithIncludeTypes()`
- Replaying recent events with `WithReplayLastN()`
- Graceful shutdown with context cancellation

**Use case**: Building real-time notifications, presence tracking, or live dashboards for VRChat activity.

**Requirements**:
- VRChat must be running and generating logs
- Windows OS (or manually specify log directory with `WithLogDir()`)

**Key concepts**:
- `vrclog.WatchWithOptions()` - Start watching with functional options
- `WithIncludeTypes()` - Filter specific event types
- `WithReplayLastN(N)` - Replay last N events from current log
- `WithPollInterval()` - Configure polling frequency
- Context cancellation for graceful shutdown

**Output example**:
```
[12:00:05] ✓ Alice joined
[12:00:23] ✗ Bob left
[12:01:15] ✓ Charlie joined
```

---

## Example Patterns

### Custom YAML Pattern File

Create a file `patterns.yaml`:

```yaml
version: 1
patterns:
  # Poker game events
  - id: poker_hole_cards
    event_type: poker_hole_cards
    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'

  - id: poker_winner
    event_type: poker_winner
    regex: 'player (?P<seat_id>\d+) won (?P<amount>\d+)'

  # Quest events
  - id: quest_complete
    event_type: quest_complete
    regex: 'Quest "(?P<quest_name>[^"]+)" completed by (?P<player>\w+)'

  # Custom game score
  - id: player_score
    event_type: player_score
    regex: 'Player (?P<name>\w+) scored (?P<points>\d+) points'
```

Load in your code:

```go
parser, err := pattern.NewRegexParserFromFile("patterns.yaml")
if err != nil {
    log.Fatal(err)
}
```

---

## Tips

### Security Limits

Pattern files have security constraints to prevent DoS attacks:
- **File size**: Max 1MB (`pattern.MaxPatternFileSize`)
- **Pattern length**: Max 512 bytes per regex (`pattern.MaxPatternLength`)
- **File type**: Must be regular file (not FIFO, device, socket)

### Performance

- Use `ChainFirst` mode if order matters and you want to stop at first match
- Use `WithIncludeTypes()` for filtering instead of post-processing
- Increase `WithPollInterval()` if CPU usage is a concern

### Error Handling

```go
// Check for specific error types
var watchErr *vrclog.WatchError
if errors.As(err, &watchErr) {
    log.Printf("Watch error: op=%s, path=%s", watchErr.Op, watchErr.Path)
}

var parseErr *vrclog.ParseError
if errors.As(err, &parseErr) {
    log.Printf("Parse error: line=%q", parseErr.Line)
}

// Check for sentinel errors
if errors.Is(err, vrclog.ErrLogDirNotFound) {
    log.Println("VRChat log directory not found")
}
```

---

## More Information

- [Main README](../README.md) - Full library documentation
- [API Reference](https://pkg.go.dev/github.com/vrclog/vrclog-go) - pkg.go.dev documentation
- [CHANGELOG](../CHANGELOG.md) - Version history and features
