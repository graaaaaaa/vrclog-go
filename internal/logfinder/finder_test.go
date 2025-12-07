package logfinder

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindLatestLogFile(t *testing.T) {
	// Create temp directory
	dir := t.TempDir()

	// Create test log files with different modification times
	files := []string{
		"output_log_2024-01-01_00-00-00.txt",
		"output_log_2024-01-02_00-00-00.txt",
		"output_log_2024-01-03_00-00-00.txt",
	}

	for i, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		// Set modification time (oldest first)
		modTime := time.Now().Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	// Test
	got, err := FindLatestLogFile(dir)
	if err != nil {
		t.Fatalf("FindLatestLogFile() error = %v", err)
	}

	// Should return the most recently modified file (last one)
	want := files[len(files)-1]
	if filepath.Base(got) != want {
		t.Errorf("FindLatestLogFile() = %v, want %v", filepath.Base(got), want)
	}
}

func TestFindLatestLogFile_NoFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := FindLatestLogFile(dir)
	if err == nil {
		t.Error("FindLatestLogFile() expected error for empty directory")
	}
	if !errors.Is(err, ErrNoLogFiles) {
		t.Errorf("FindLatestLogFile() error = %v, want %v", err, ErrNoLogFiles)
	}
}

func TestFindLogDir_EnvVar(t *testing.T) {
	// Create temp directory with log file
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	oldVal := os.Getenv(EnvLogDir)
	os.Setenv(EnvLogDir, dir)
	defer os.Setenv(EnvLogDir, oldVal)

	// Test
	got, err := FindLogDir("")
	if err != nil {
		t.Fatalf("FindLogDir() error = %v", err)
	}
	if got != dir {
		t.Errorf("FindLogDir() = %v, want %v", got, dir)
	}
}

func TestFindLogDir_Explicit(t *testing.T) {
	// Create temp directory with log file
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Explicit should take priority over env
	oldVal := os.Getenv(EnvLogDir)
	os.Setenv(EnvLogDir, "/some/other/path")
	defer os.Setenv(EnvLogDir, oldVal)

	got, err := FindLogDir(dir)
	if err != nil {
		t.Fatalf("FindLogDir() error = %v", err)
	}
	if got != dir {
		t.Errorf("FindLogDir() = %v, want %v", got, dir)
	}
}

func TestFindLogDir_ExplicitInvalid(t *testing.T) {
	_, err := FindLogDir("/nonexistent/path")
	if err == nil {
		t.Error("FindLogDir() expected error for invalid explicit path")
	}
	if !errors.Is(err, ErrLogDirNotFound) {
		t.Errorf("FindLogDir() error = %v, want %v", err, ErrLogDirNotFound)
	}
}

func TestFindLogDir_EnvVarInvalid(t *testing.T) {
	// Set environment variable to invalid path
	oldVal := os.Getenv(EnvLogDir)
	os.Setenv(EnvLogDir, "/nonexistent/path")
	defer os.Setenv(EnvLogDir, oldVal)

	_, err := FindLogDir("")
	if err == nil {
		t.Error("FindLogDir() expected error for invalid env var path")
	}
	if !errors.Is(err, ErrLogDirNotFound) {
		t.Errorf("FindLogDir() error = %v, want %v", err, ErrLogDirNotFound)
	}
}

func TestIsValidLogDir(t *testing.T) {
	// Create temp directory with log file
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	if !isValidLogDir(dir) {
		t.Error("isValidLogDir() = false, want true for valid dir")
	}
}

func TestIsValidLogDir_Empty(t *testing.T) {
	dir := t.TempDir()

	if isValidLogDir(dir) {
		t.Error("isValidLogDir() = true, want false for empty dir")
	}
}

func TestIsValidLogDir_NotExists(t *testing.T) {
	if isValidLogDir("/nonexistent/path") {
		t.Error("isValidLogDir() = true, want false for nonexistent path")
	}
}
