package extractor

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildZip creates an in-memory log archive shaped like GitHub's.
func buildZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		require.NoError(t, err)
		_, err = f.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestReadArchiveFlatLayout(t *testing.T) {
	data := buildZip(t, map[string]string{
		"1_Set up job.txt": "setting up",
		"2_Run tests.txt":  "--- FAIL: TestA",
		"3_Post job.txt":   "cleaning up",
	})

	logs, err := ReadArchive(data)
	require.NoError(t, err)
	require.Len(t, logs, 3)

	assert.Equal(t, 1, logs[0].Order)
	assert.Equal(t, "Set up job", logs[0].Name)
	assert.Equal(t, "Run tests", logs[1].Name)
}

func TestReadArchiveNestedLayout(t *testing.T) {
	data := buildZip(t, map[string]string{
		"build (ubuntu-latest)/1_Set up job.txt": "setup",
		"build (ubuntu-latest)/2_Run tests.txt":  "boom",
	})

	logs, err := ReadArchive(data)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "build (ubuntu-latest)", logs[0].JobDir)
}

func TestReadArchiveSkipsNonText(t *testing.T) {
	data := buildZip(t, map[string]string{
		"1_Run tests.txt": "ok",
		"artifact.bin":    "\x00\x01binary",
	})

	logs, err := ReadArchive(data)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
}

func TestReadArchiveErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"not a zip", []byte("plain text, definitely not a zip")},
		{"empty", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadArchive(tt.data)
			assert.Error(t, err)
		})
	}
}

func TestReadArchiveNoTextFiles(t *testing.T) {
	data := buildZip(t, map[string]string{"thing.bin": "x"})
	_, err := ReadArchive(data)
	assert.ErrorContains(t, err, "no .txt step files")
}

func TestNormaliseLogStripsTimestamps(t *testing.T) {
	in := "2026-08-18T10:11:12.3456789Z go test ./...\n2026-08-18T10:11:13.0000000Z --- FAIL: TestA\n"
	got := normaliseLog(in)

	assert.NotContains(t, got, "2026-08-18T",
		"per-line timestamps waste scarce context and carry no signal")
	assert.Contains(t, got, "go test ./...")
	assert.Contains(t, got, "--- FAIL: TestA")
}

func TestParseStepFileName(t *testing.T) {
	tests := []struct {
		in        string
		wantOrder int
		wantName  string
	}{
		{"3_Run tests.txt", 3, "Run tests"},
		{"12_Post Run actions_checkout.txt", 12, "Post Run actions_checkout"},
		{"no-prefix.txt", 0, "no-prefix"},
		{"x_Weird.txt", 0, "x_Weird"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			order, name := parseStepFileName(tt.in)
			assert.Equal(t, tt.wantOrder, order)
			assert.Equal(t, tt.wantName, name)
		})
	}
}
