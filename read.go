package vrclog

import (
	"context"
	"errors"
	"io"
	"iter"
	"strings"

	"github.com/vrclog/vrclog-go/internal/logfile"
)

type ReadFileConfig struct {
	Path   string
	Offset int64
	Line   uint64
}

func ReadFile(ctx context.Context, cfg ReadFileConfig) iter.Seq2[Record, error] {
	return func(yield func(Record, error) bool) {
		if cfg.Path == "" {
			yield(Record{}, errors.New("path is required"))
			return
		}
		if cfg.Offset < 0 {
			yield(Record{}, errors.New("offset must not be negative"))
			return
		}
		if cfg.Offset > 0 && cfg.Line == 0 {
			yield(Record{}, errors.New("line is required when offset > 0"))
			return
		}

		startLine := cfg.Line
		if startLine == 0 && cfg.Offset == 0 {
			startLine = 1
		}

		f, _, err := logfile.OpenRegular(cfg.Path)
		if err != nil {
			yield(Record{}, err)
			return
		}
		defer f.Close()

		srcIDStr, err := logfile.SourceID(cfg.Path)
		if err != nil {
			yield(Record{}, err)
			return
		}
		srcID := SourceID(srcIDStr)

		if cfg.Offset > 0 {
			if _, err := f.Seek(cfg.Offset, io.SeekStart); err != nil {
				yield(Record{}, err)
				return
			}
		}

		lr := newLineReader(f, cfg.Offset, startLine)

		for {
			if ctx.Err() != nil {
				return
			}

			rawBytes, rawHash, offset, nextOffset, lineNum, issue, readErr := lr.next()
			if readErr == io.EOF {
				return
			}
			if readErr != nil {
				yield(Record{}, readErr)
				return
			}

			rawStr := strings.ToValidUTF8(string(rawBytes), "�")

			t, level, message, ok := decodeHeader(rawStr, nil)
			if !ok {
				message = rawStr
				level = LevelUnknown
			}

			rec := Record{
				ID:         computeRecordID(srcID, offset, rawHash),
				Time:       t,
				Level:      level,
				Message:    message,
				Raw:        rawStr,
				SourceID:   srcID,
				Path:       cfg.Path,
				Offset:     offset,
				NextOffset: nextOffset,
				Line:       lineNum,
				Issue:      issue,
			}

			if !yield(rec, nil) {
				return
			}
		}
	}
}
