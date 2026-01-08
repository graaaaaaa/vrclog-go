package pattern

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	// MaxPatternFileSize is the maximum allowed size for a pattern file (1MB).
	// This limit prevents denial-of-service attacks via extremely large files.
	MaxPatternFileSize = 1 * 1024 * 1024 // 1 MB

	// MaxPatternLength is the maximum allowed length for a regex pattern (512 bytes).
	// This limit helps mitigate ReDoS (Regular Expression Denial of Service) attacks
	// by preventing excessively complex patterns.
	MaxPatternLength = 512

	// SupportedVersion is the currently supported pattern file format version.
	SupportedVersion = 1
)

// Load reads and parses a pattern file from the given path.
// Returns an error if the file cannot be read, is too large, or fails validation.
//
// Example:
//
//	pf, err := pattern.Load("patterns.yaml")
//	if err != nil {
//	    log.Fatalf("failed to load pattern file: %v", err)
//	}
func Load(path string) (*PatternFile, error) {
	// Check file size before reading
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat pattern file: %w", err)
	}
	if info.Size() > MaxPatternFileSize {
		return nil, fmt.Errorf("pattern file too large: %d bytes (max %d)", info.Size(), MaxPatternFileSize)
	}
	if info.Size() == 0 {
		return nil, errors.New("pattern file is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read pattern file: %w", err)
	}

	return LoadBytes(data)
}

// LoadBytes parses a pattern file from a byte slice.
// Returns an error if the data cannot be parsed or fails validation.
//
// Example:
//
//	data := []byte("version: 1\npatterns:\n  - id: test\n    ...")
//	pf, err := pattern.LoadBytes(data)
func LoadBytes(data []byte) (*PatternFile, error) {
	if len(data) == 0 {
		return nil, errors.New("pattern file is empty")
	}
	if len(data) > MaxPatternFileSize {
		return nil, fmt.Errorf("pattern file too large: %d bytes (max %d)", len(data), MaxPatternFileSize)
	}

	var pf PatternFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := pf.Validate(); err != nil {
		return nil, err
	}

	return &pf, nil
}

// Validate performs schema-level validation on the pattern file.
// It checks for:
//   - Supported version number
//   - At least one pattern
//   - Required fields (id, event_type, regex)
//   - Unique pattern IDs
//   - Pattern length limits (ReDoS protection)
//
// Note: This function does NOT compile regular expressions. Regex compilation
// and validation happens in NewRegexParser() to avoid duplicating work.
func (pf *PatternFile) Validate() error {
	// Check version
	if pf.Version != SupportedVersion {
		return &ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version %d (only version %d is supported)", pf.Version, SupportedVersion),
		}
	}

	// Check for at least one pattern
	if len(pf.Patterns) == 0 {
		return &ValidationError{
			Field:   "patterns",
			Message: "at least one pattern is required",
		}
	}

	// Track IDs for uniqueness check
	seenIDs := make(map[string]int, len(pf.Patterns))

	// Validate each pattern
	for i, p := range pf.Patterns {
		// Check required fields
		if p.ID == "" {
			return &PatternError{
				Index:   i,
				Field:   "id",
				Message: "id is required",
			}
		}
		if p.EventType == "" {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "event_type",
				Message: "event_type is required",
			}
		}
		if p.Regex == "" {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "regex",
				Message: "regex is required",
			}
		}

		// Check ID uniqueness
		if prevIndex, exists := seenIDs[p.ID]; exists {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "id",
				Message: fmt.Sprintf("duplicate id (previously defined at pattern[%d])", prevIndex),
			}
		}
		seenIDs[p.ID] = i

		// Check pattern length for ReDoS protection
		if len(p.Regex) > MaxPatternLength {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "regex",
				Message: fmt.Sprintf("pattern too long: %d bytes (max %d)", len(p.Regex), MaxPatternLength),
			}
		}
	}

	return nil
}
