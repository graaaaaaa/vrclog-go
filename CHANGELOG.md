# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial implementation of VRChat log parser and watcher
- `vrclog.Watch()` function for real-time log monitoring
- `vrclog.NewWatcher()` for advanced watcher configuration
- `vrclog.ParseLine()` for parsing individual log lines
- Event types: `world_join`, `player_join`, `player_left`
- Replay functionality with `ReplayConfig` options
- CLI tool with `tail` command
- JSON Lines and pretty output formats
- Event type filtering
- Log directory auto-detection
- Log file rotation handling

### Documentation

- README.md with usage examples
- README.ja.md (Japanese translation)
- Package documentation in doc.go
