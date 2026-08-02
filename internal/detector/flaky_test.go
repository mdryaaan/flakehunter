package detector

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func job(sha, name string, c Conclusion, runID int64, minute int) JobResult {
	return JobResult{
		RunID:        runID,
		RunAttempt:   1,
		JobName:      name,
		WorkflowFile: "ci.yml",
		WorkflowName: "CI",
		HeadSHA:      sha,
		Branch:       "main",
		Conclusion:   c,
		StartedAt:    time.Date(2026, 8, 18, 10, minute, 0, 0, time.UTC),
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name       string
		jobs       []JobResult
		wantCount  int
		wantJob    string
		wantRate   float64
		wantReason string
	}{
		{
			name: "same sha, fail then pass, is flaky",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
				job("aaa", "unit", ConclusionSuccess, 2, 5),
			},
			wantCount: 1, wantJob: "unit", wantRate: 0.5,
		},
		{
			name: "same sha, pass then fail, is flaky",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionSuccess, 1, 0),
				job("aaa", "unit", ConclusionFailure, 2, 5),
			},
			wantCount: 1, wantJob: "unit", wantRate: 0.5,
		},
		{
			name: "different sha is a fix, not a flake",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
				job("bbb", "unit", ConclusionSuccess, 2, 5),
			},
			wantCount:  0,
			wantReason: "a red commit followed by a green one is someone fixing the build",
		},
		{
			name: "all failures is a broken build",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
				job("aaa", "unit", ConclusionFailure, 2, 5),
			},
			wantCount: 0,
		},
		{
			name: "all passes is a healthy build",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionSuccess, 1, 0),
				job("aaa", "unit", ConclusionSuccess, 2, 5),
			},
			wantCount: 0,
		},
		{
			name: "different job names do not pair up",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
				job("aaa", "lint", ConclusionSuccess, 2, 5),
			},
			wantCount: 0,
		},
		{
			name: "cancelled runs are ignored",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
				job("aaa", "unit", ConclusionCancelled, 2, 5),
			},
			wantCount:  0,
			wantReason: "a cancelled run says nothing about the code",
		},
		{
			name: "timed out counts as a failure",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionTimedOut, 1, 0),
				job("aaa", "unit", ConclusionSuccess, 2, 5),
			},
			wantCount: 1, wantJob: "unit", wantRate: 0.5,
		},
		{
			name: "single attempt cannot be flaky",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
			},
			wantCount: 0,
		},
		{
			name: "same job in a different workflow is separate",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
				func() JobResult {
					j := job("aaa", "unit", ConclusionSuccess, 2, 5)
					j.WorkflowFile = "nightly.yml"
					return j
				}(),
			},
			wantCount: 0,
		},
		{
			name: "two failures and one pass gives a two thirds rate",
			jobs: []JobResult{
				job("aaa", "unit", ConclusionFailure, 1, 0),
				job("aaa", "unit", ConclusionFailure, 2, 3),
				job("aaa", "unit", ConclusionSuccess, 3, 6),
			},
			wantCount: 1, wantJob: "unit", wantRate: 2.0 / 3.0,
		},
		{
			name:      "empty input",
			jobs:      nil,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.jobs, DefaultOptions())
			require.Len(t, got, tt.wantCount, tt.wantReason)
			if tt.wantCount == 0 {
				return
			}
			assert.Equal(t, tt.wantJob, got[0].JobName)
			assert.InDelta(t, tt.wantRate, got[0].FailureRate, 0.001)
			assert.NotEmpty(t, got[0].ID)
			assert.NotEmpty(t, got[0].FailedRuns)
			assert.NotEmpty(t, got[0].PassedRuns)
		})
	}
}

func TestDetectIsDeterministic(t *testing.T) {
	jobs := []JobResult{
		job("aaa", "unit", ConclusionFailure, 1, 0),
		job("aaa", "unit", ConclusionSuccess, 2, 5),
		job("bbb", "e2e", ConclusionFailure, 3, 0),
		job("bbb", "e2e", ConclusionSuccess, 4, 5),
		job("ccc", "lint", ConclusionFailure, 5, 0),
		job("ccc", "lint", ConclusionFailure, 6, 5),
	}

	first := Detect(jobs, DefaultOptions())
	for i := 0; i < 8; i++ {
		assert.Equal(t, first, Detect(jobs, DefaultOptions()),
			"detection must not depend on map iteration order")
	}
}

func TestPrimaryFailurePicksMostRecent(t *testing.T) {
	occ := FlakyOccurrence{FailedRuns: []JobResult{
		job("aaa", "unit", ConclusionFailure, 1, 0),
		job("aaa", "unit", ConclusionFailure, 9, 30),
	}}

	got, ok := occ.PrimaryFailure()
	require.True(t, ok)
	assert.Equal(t, int64(9), got.RunID)

	_, ok = FlakyOccurrence{}.PrimaryFailure()
	assert.False(t, ok)
}

func TestSummarise(t *testing.T) {
	jobs := []JobResult{
		job("aaa", "unit", ConclusionFailure, 1, 0),
		job("aaa", "unit", ConclusionSuccess, 2, 5),
		job("bbb", "lint", ConclusionSuccess, 3, 0),
	}
	occ := Detect(jobs, DefaultOptions())

	s := Summarise(jobs, occ)
	assert.Equal(t, 3, s.TotalJobs)
	assert.Equal(t, 2, s.DistinctCommits)
	assert.Equal(t, 1, s.FlakyOccurrences)
	assert.Equal(t, 1, s.AffectedJobs)
	assert.InDelta(t, 0.5, s.MeanFailureRate, 0.001)
}
