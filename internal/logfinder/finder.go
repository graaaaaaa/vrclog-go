// Package logfinder provides VRChat log directory and file detection.
package logfinder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// EnvLogDir is the environment variable name for specifying log directory.
const EnvLogDir = "VRCLOG_LOGDIR"

// Sentinel errors.
var (
	ErrLogDirNotFound = errors.New("log directory not found")
	ErrNoLogFiles     = errors.New("no log files found")
)

// DefaultLogDirs returns candidate VRChat log directories in priority order.
// The directories are OS-specific (Windows only for VRChat PC).
func DefaultLogDirs() []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		// Fallback: try to construct from USERPROFILE
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			localAppData = filepath.Join(userProfile, "AppData", "Local")
		}
	}

	if localAppData == "" {
		return nil
	}

	// LocalLow is one level up from Local
	localLow := filepath.Join(filepath.Dir(localAppData), "LocalLow")

	return []string{
		filepath.Join(localLow, "VRChat", "VRChat"),
		filepath.Join(localLow, "VRChat", "vrchat"),
	}
}

// FindLogDir returns the VRChat log directory.
//
// Priority:
//  1. explicit (if non-empty)
//  2. VRCLOG_LOGDIR environment variable
//  3. Auto-detect from DefaultLogDirs()
//
// Returns ErrLogDirNotFound if no valid directory is found.
func FindLogDir(explicit string) (string, error) {
	// 1. Check explicit
	if explicit != "" {
		if isValidLogDir(explicit) {
			return explicit, nil
		}
		return "", fmt.Errorf("%w: %s", ErrLogDirNotFound, explicit)
	}

	// 2. Check environment variable
	if envDir := os.Getenv(EnvLogDir); envDir != "" {
		if isValidLogDir(envDir) {
			return envDir, nil
		}
		return "", fmt.Errorf("%w: %s (from %s)", ErrLogDirNotFound, envDir, EnvLogDir)
	}

	// 3. Auto-detect
	for _, dir := range DefaultLogDirs() {
		if isValidLogDir(dir) {
			return dir, nil
		}
	}

	return "", ErrLogDirNotFound
}

// FindLatestLogFile returns the path to the most recently modified
// output_log file in the given directory.
//
// Returns ErrNoLogFiles if no log files are found.
func FindLatestLogFile(dir string) (string, error) {
	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing log files: %w", err)
	}

	if len(matches) == 0 {
		return "", ErrNoLogFiles
	}

	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		infoI, errI := os.Stat(matches[i])
		infoJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return matches[0], nil
}

// isValidLogDir checks if the directory exists and contains log files.
func isValidLogDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check if there are any output_log files
	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, _ := filepath.Glob(pattern)
	return len(matches) > 0
}
