// Package extractor turns raw GitHub Actions log archives into the small,
// relevant excerpts an LLM can actually reason about.
package extractor

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

// StepLog is one step's output from within a job's log archive.
type StepLog struct {
	// JobDir is the folder inside the archive, which GitHub names after the job.
	JobDir string
	// Order is the numeric prefix GitHub puts on each step file ("3_Run tests.txt").
	Order int
	Name  string
	Body  string
}

// MaxArchiveBytes caps how much a single archive may expand to. Archives are
// fetched from a remote API, so an unbounded read is a decompression-bomb
// vector as well as an out-of-memory risk on a small runner.
const MaxArchiveBytes = 64 << 20 // 64 MiB

// ReadArchive parses a GitHub Actions log archive.
//
// The layout is not documented as a contract, but in practice is either a flat
// set of "<n>_<step name>.txt" files, or one directory per job containing them.
// Both shapes are handled, and anything that is not a .txt file is skipped.
func ReadArchive(data []byte) ([]StepLog, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening log archive: %w", err)
	}

	var logs []StepLog
	var total int64

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := file.Name
		if !strings.HasSuffix(strings.ToLower(name), ".txt") {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %q in archive: %w", name, err)
		}

		limited := io.LimitReader(rc, MaxArchiveBytes-total)
		body, err := io.ReadAll(limited)
		closeErr := rc.Close()
		if err != nil {
			return nil, fmt.Errorf("reading %q from archive: %w", name, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("closing %q in archive: %w", name, closeErr)
		}

		total += int64(len(body))
		if total >= MaxArchiveBytes {
			return nil, fmt.Errorf("log archive exceeds %d bytes; refusing to expand further", MaxArchiveBytes)
		}

		order, stepName := parseStepFileName(path.Base(name))
		logs = append(logs, StepLog{
			JobDir: path.Dir(name),
			Order:  order,
			Name:   stepName,
			Body:   normaliseLog(string(body)),
		})
	}

	if len(logs) == 0 {
		return nil, fmt.Errorf("log archive contained no .txt step files")
	}

	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].JobDir != logs[j].JobDir {
			return logs[i].JobDir < logs[j].JobDir
		}
		return logs[i].Order < logs[j].Order
	})

	return logs, nil
}

// parseStepFileName splits "3_Run tests.txt" into (3, "Run tests"). Files that
// do not carry a numeric prefix keep order 0 and their whole stem as the name.
func parseStepFileName(base string) (int, string) {
	stem := strings.TrimSuffix(base, path.Ext(base))
	idx := strings.Index(stem, "_")
	if idx <= 0 {
		return 0, stem
	}
	order, err := strconv.Atoi(stem[:idx])
	if err != nil {
		return 0, stem
	}
	return order, stem[idx+1:]
}

// timestampPrefix matches the RFC3339-ish stamp GitHub prepends to every line.
// Stripping it buys back a meaningful share of a small context window.
func normaliseLog(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")

	for i, line := range lines {
		if len(line) > 28 && line[4] == '-' && line[7] == '-' && line[10] == 'T' {
			if sp := strings.IndexByte(line, ' '); sp > 0 && sp < 34 {
				lines[i] = line[sp+1:]
			}
		}
	}

	return strings.Join(lines, "\n")
}
