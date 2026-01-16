# VRPoker Plugin Example

This is an example WebAssembly plugin for parsing custom VRPoker game events from VRChat logs.

## What is VRPoker?

VRPoker is a poker game world in VRChat. This plugin demonstrates how to create custom log parsers that extract game-specific events (game start, winners, rounds, etc.) from VRChat log files.

## Building

Requires [TinyGo](https://tinygo.org/getting-started/install/) 0.35.0 or later:

```bash
make
```

This produces `vrpoker.wasm`.

## Usage

Use the plugin with the vrclog CLI:

```bash
# Monitor VRChat logs and extract VRPoker events
vrclog tail --plugin ./vrpoker.wasm

# Parse historical logs
vrclog parse --plugin ./vrpoker.wasm

# Combine with pattern files
vrclog tail --patterns ./my-patterns.yaml --plugin ./vrpoker.wasm
```

## Event Types

The plugin recognizes these event types:

- `vrpoker_game_start` - A new poker game started
  - Data: `player_count` - Number of players
- `vrpoker_winner` - A player won a hand
  - Data: `player` - Player name, `hand` - Winning hand (e.g., "Royal Flush")
- `vrpoker_round` - A new round began
  - Data: `round` - Round number
- `vrpoker_event` - Generic VRPoker event (fallback)
  - Data: `message` - The original log line

## Example Log Lines

The plugin parses log lines like:

```
[VRPoker] Game started with 4 players
[VRPoker] Player Alice wins with Royal Flush
[VRPoker] Round 3 begins
```

## Implementation Details

This plugin demonstrates:

- **WebAssembly Plugin ABI**: Implements `abi_version()`, `alloc()`, `free()`, and `parse_line()` functions
- **Host Functions**: Uses `regex_match()` and `regex_find_submatch()` provided by the host (vrclog) for pattern matching
- **JSON Communication**: Input/output via JSON (required by ABI v1)
- **Memory Management**: Simple bump allocator (suitable for short-lived plugins)
- **TinyGo Compatibility**: Built with TinyGo for small Wasm binary size

## Customization

To create your own plugin:

1. Copy this example as a starting point
2. Modify the event patterns in `parseGameStart()`, `parseWinner()`, `parseRound()`
3. Add new parsing functions for your custom events
4. Update the `parse_line()` function to call your parsers
5. Build with `make`

## License

This example is part of vrclog-go and uses the same license (MIT).
