package extractor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailingStepPrefersRealFailureOverCleanup(t *testing.T) {
	logs := []StepLog{
		{Order: 1, Name: "Set up job", Body: "starting"},
		{Order: 2, Name: "Run tests", Body: "--- FAIL: TestA\npanic: boom"},
		{Order: 3, Name: "Post Run actions/checkout", Body: "error: cleanup had an error"},
	}

	got, ok := FailingStep(logs)
	require.True(t, ok)
	assert.Equal(t, "Run tests", got.Name,
		"post-job cleanup mentioning 'error' must not outrank the real failure")
}

func TestFailingStepNoneFound(t *testing.T) {
	logs := []StepLog{{Order: 1, Name: "Set up job", Body: "all fine"}}
	_, ok := FailingStep(logs)
	assert.False(t, ok)
}

func TestFailingStepPicksLastOnTie(t *testing.T) {
	logs := []StepLog{
		{Order: 1, Name: "First", Body: "error: something"},
		{Order: 2, Name: "Second", Body: "error: something"},
	}

	got, ok := FailingStep(logs)
	require.True(t, ok)
	assert.Equal(t, "Second", got.Name)
}

func TestChunkArchiveEndToEnd(t *testing.T) {
	data := buildZip(t, map[string]string{
		"1_Set up job.txt": "setting up the job",
		"2_Run tests.txt":  "$ go test ./...\n--- FAIL: TestFlaky (0.10s)\n    flaky_test.go:12: dial tcp: i/o timeout\nFAIL",
		"3_Post job.txt":   "cleaning up",
	})

	got, err := ChunkArchive(data, DefaultChunkOptions())
	require.NoError(t, err)

	assert.Equal(t, "Run tests", got.StepName)
	assert.Contains(t, got.Text, "--- FAIL: TestFlaky")
	assert.Contains(t, got.Text, "dial tcp: i/o timeout")
	assert.NotContains(t, got.Text, "cleaning up",
		"only the failing step should reach the model")
}

func TestChunkArchiveFallsBackToLastStep(t *testing.T) {
	data := buildZip(t, map[string]string{
		"1_Set up job.txt": "setup",
		"2_Run tests.txt":  "everything looked fine but exit was nonzero",
	})

	got, err := ChunkArchive(data, DefaultChunkOptions())
	require.NoError(t, err)
	assert.Equal(t, "Run tests", got.StepName)
}

func TestChunkArchiveBadInput(t *testing.T) {
	_, err := ChunkArchive([]byte("not a zip"), DefaultChunkOptions())
	assert.Error(t, err)
}
