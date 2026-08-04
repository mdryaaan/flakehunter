package github

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdryaaan/flakehunter/internal/detector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "fixtures")
}

func TestFixtureSourceListsJobs(t *testing.T) {
	src, err := NewFixtureSource(fixtureDir(t))
	require.NoError(t, err)
	assert.Contains(t, src.Describe(), "offline")

	jobs, err := src.ListJobResults(context.Background(), ListOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, jobs)

	for _, job := range jobs {
		assert.NotEmpty(t, job.HeadSHA)
		assert.NotEmpty(t, job.JobName)
		assert.False(t, job.StartedAt.IsZero())
	}
}

func TestFixtureSourceDetectsKnownFlakes(t *testing.T) {
	src, err := NewFixtureSource(fixtureDir(t))
	require.NoError(t, err)

	jobs, err := src.ListJobResults(context.Background(), ListOptions{})
	require.NoError(t, err)

	occurrences := detector.Detect(jobs, detector.DefaultOptions())
	assert.Len(t, occurrences, 4,
		"the fixture set contains exactly four flaky signatures plus negative controls")

	names := map[string]bool{}
	for _, occ := range occurrences {
		names[occ.JobName] = true
	}
	assert.False(t, names["lint"], "a red commit followed by a green one is a fix, not a flake")
	assert.False(t, names["build (windows)"], "an always-red job is a broken build")
	assert.False(t, names["e2e"], "a cancelled rerun carries no signal")
}

func TestFixtureSourceFiltersByWorkflow(t *testing.T) {
	src, err := NewFixtureSource(fixtureDir(t))
	require.NoError(t, err)

	jobs, err := src.ListJobResults(context.Background(), ListOptions{WorkflowFile: "integration.yml"})
	require.NoError(t, err)
	require.NotEmpty(t, jobs)

	for _, job := range jobs {
		assert.Equal(t, "integration.yml", job.WorkflowFile)
	}
}

func TestFixtureSourceDownloadsArchive(t *testing.T) {
	src, err := NewFixtureSource(fixtureDir(t))
	require.NoError(t, err)

	jobs, err := src.ListJobResults(context.Background(), ListOptions{})
	require.NoError(t, err)

	occurrences := detector.Detect(jobs, detector.DefaultOptions())
	require.NotEmpty(t, occurrences)

	failure, ok := occurrences[0].PrimaryFailure()
	require.True(t, ok)

	data, err := src.DownloadJobLog(context.Background(), failure.RunID, failure.JobID)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Equal(t, byte('P'), data[0], "GitHub log archives are zip files")
}

func TestFixtureSourceErrors(t *testing.T) {
	_, err := NewFixtureSource("/definitely/not/a/directory")
	assert.Error(t, err)

	file := filepath.Join(t.TempDir(), "a-file")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	_, err = NewFixtureSource(file)
	assert.ErrorContains(t, err, "not a directory")

	empty, err := NewFixtureSource(t.TempDir())
	require.NoError(t, err)
	_, err = empty.ListJobResults(context.Background(), ListOptions{})
	assert.Error(t, err)
}

func TestParseFixtureTime(t *testing.T) {
	for _, in := range []string{"2026-08-18T09:14:00Z", "2026-08-18T09:14:00", "2026-08-18 09:14:00"} {
		got, err := parseFixtureTime(in)
		require.NoError(t, err, in)
		assert.Equal(t, 2026, got.Year())
	}

	_, err := parseFixtureTime("not a time")
	assert.Error(t, err)
}
