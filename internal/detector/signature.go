// Package detector implements flakehunter's flaky-test detection.
package detector

import (
	"sort"
	"time"

	"github.com/mdryaaan/flakehunter/internal/utils"
)

// Conclusion is the outcome GitHub reports for a job.
type Conclusion string

// Job conclusions flakehunter cares about. Cancelled and skipped jobs carry no
// signal about the code and are excluded from detection entirely.
const (
	ConclusionSuccess   Conclusion = "success"
	ConclusionFailure   Conclusion = "failure"
	ConclusionCancelled Conclusion = "cancelled"
	ConclusionSkipped   Conclusion = "skipped"
	ConclusionTimedOut  Conclusion = "timed_out"
)

// Decisive reports whether a conclusion says anything about the code under test.
func (c Conclusion) Decisive() bool {
	return c == ConclusionSuccess || c == ConclusionFailure || c == ConclusionTimedOut
}

// Passed reports whether the conclusion counts as green.
func (c Conclusion) Passed() bool { return c == ConclusionSuccess }

// JobResult is one job execution within one workflow run.
type JobResult struct {
	RunID        int64      `json:"run_id"`
	RunAttempt   int        `json:"run_attempt"`
	JobID        int64      `json:"job_id"`
	JobName      string     `json:"job_name"`
	WorkflowName string     `json:"workflow_name"`
	WorkflowFile string     `json:"workflow_file"`
	HeadSHA      string     `json:"head_sha"`
	Branch       string     `json:"branch"`
	Conclusion   Conclusion `json:"conclusion"`
	StartedAt    time.Time  `json:"started_at"`
	URL          string     `json:"url"`
}

// signatureKey is the identity a job must share to be comparable across runs.
//
// The commit SHA is deliberately part of the key. Two runs of the same job on
// *different* commits disagreeing is the system working — someone fixed the
// build. Only the same job, on the same commit, disagreeing with itself is a
// flake.
type signatureKey struct {
	SHA          string
	WorkflowFile string
	JobName      string
}

func keyFor(job JobResult) signatureKey {
	return signatureKey{
		SHA:          job.HeadSHA,
		WorkflowFile: job.WorkflowFile,
		JobName:      job.JobName,
	}
}

// group buckets jobs by their flaky signature key, dropping any job whose
// conclusion carries no signal.
func group(jobs []JobResult) map[signatureKey][]JobResult {
	buckets := make(map[signatureKey][]JobResult)
	for _, job := range jobs {
		if !job.Conclusion.Decisive() {
			continue
		}
		if job.HeadSHA == "" || job.JobName == "" {
			continue
		}
		k := keyFor(job)
		buckets[k] = append(buckets[k], job)
	}

	for k := range buckets {
		sort.SliceStable(buckets[k], func(i, j int) bool {
			return buckets[k][i].StartedAt.Before(buckets[k][j].StartedAt)
		})
	}

	return buckets
}

// occurrenceID gives a flaky occurrence a stable identity across scans, so
// reports can be diffed and issues de-duplicated.
func occurrenceID(k signatureKey) string {
	return utils.ShortHash(k.SHA, k.WorkflowFile, k.JobName)
}
