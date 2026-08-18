package logfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

var ErrNoLogFiles = errors.New("no log files found")

var ErrTooManyLogFiles = errors.New("too many log files")

var ErrNoLogDirectory = errors.New("no log directory available")

const MaxFollowCandidateFiles = 4096

const filenameTimestampLayout = "2006-01-02_15-04-05"

type LogFileInfo struct {
	Path    string
	ModTime time.Time
	Name    string
}

func DefaultLogDirectory() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("%w: auto-detection is only supported on Windows", ErrNoLogDirectory)
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			localAppData = filepath.Join(userProfile, "AppData", "Local")
		}
	}

	if localAppData == "" {
		return "", fmt.Errorf("%w: cannot determine LocalAppData path", ErrNoLogDirectory)
	}

	localLow := filepath.Join(filepath.Dir(localAppData), "LocalLow")

	candidates := []string{
		filepath.Join(localLow, "VRChat", "VRChat"),
		filepath.Join(localLow, "VRChat", "vrchat"),
	}

	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue
		}
		return resolved, nil
	}

	return "", fmt.Errorf("%w: VRChat log directory not found", ErrNoLogDirectory)
}

func ListLogFiles(dir string) ([]LogFileInfo, error) {
	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	if len(matches) == 0 {
		return nil, ErrNoLogFiles
	}

	if len(matches) > MaxFollowCandidateFiles {
		return nil, fmt.Errorf("%w: found %d files (limit %d)", ErrTooManyLogFiles, len(matches), MaxFollowCandidateFiles)
	}

	var files []LogFileInfo
	for _, m := range matches {
		f, info, err := OpenRegular(m)
		if err != nil {
			continue
		}
		f.Close()

		files = append(files, LogFileInfo{
			Path:    m,
			ModTime: info.ModTime(),
			Name:    filepath.Base(m),
		})
	}

	if len(files) == 0 {
		return nil, ErrNoLogFiles
	}

	sort.SliceStable(files, func(i, j int) bool {
		ti, oki := parseFilenameTimestamp(files[i].Name)
		tj, okj := parseFilenameTimestamp(files[j].Name)

		switch {
		case oki && okj:
			return ti.Before(tj)
		case oki && !okj:
			return true
		case !oki && okj:
			return false
		default:
			if !files[i].ModTime.Equal(files[j].ModTime) {
				return files[i].ModTime.Before(files[j].ModTime)
			}
			return files[i].Name < files[j].Name
		}
	})

	return files, nil
}

func FindLatestLogFile(dir string) (string, error) {
	files, err := ListLogFiles(dir)
	if err != nil {
		return "", err
	}
	return files[len(files)-1].Path, nil
}

func parseFilenameTimestamp(name string) (time.Time, bool) {
	const prefix = "output_log_"
	const suffix = ".txt"

	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return time.Time{}, false
	}

	tsStr := name[len(prefix) : len(name)-len(suffix)]
	t, err := time.Parse(filenameTimestampLayout, tsStr)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
