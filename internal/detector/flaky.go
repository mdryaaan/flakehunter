package detector

import "sort"

// FlakyOccurrence is one job that produced both a pass and a fail on a single
// commit — the statistical signature of a flaky test.
type FlakyOccurrence struct {
	ID           string      `json:"id"`
	HeadSHA      string      `json:"head_sha"`
	Branch       string      `json:"branch"`
	WorkflowName string      `json:"workflow_name"`
	WorkflowFile string      `json:"workflow_file"`
	JobName      string      `json:"job_name"`
	FailedRuns   []JobResult `json:"failed_runs"`
	PassedRuns   []JobResult `json:"passed_runs"`
	// TotalAttempts counts every decisive attempt observed for this signature.
	TotalAttempts int `json:"total_attempts"`
	// FailureRate is failures over decisive attempts, in [0,1].
	FailureRate float64 `json:"failure_rate"`
}

// PrimaryFailure returns the failing run best suited to log analysis: the most
// recent one, whose logs are least likely to have been evicted by retention.
func (o FlakyOccurrence) PrimaryFailure() (JobResult, bool) {
	if len(o.FailedRuns) == 0 {
		return JobResult{}, false
	}
	return o.FailedRuns[len(o.FailedRuns)-1], true
}

// Options tunes detection.
type Options struct {
	// MinAttempts is how many decisive attempts a signature needs before it can
	// be called flaky. Two is the minimum that can possibly disagree.
	MinAttempts int
}

// DefaultOptions returns the standard detection settings.
func DefaultOptions() Options { return Options{MinAttempts: 2} }

// Detect finds flaky occurrences in a set of job results.
//
// The rule is deliberately strict: a job is flaky only when the *same job name*,
// in the *same workflow*, on the *same commit SHA*, produced both a pass and a
// failure. Anything looser produces false positives that destroy trust in the
// tool faster than the flakes it is meant to find:
//
//   - "the job failed then passed on a later commit" is a fix, not a flake
//   - "a different job failed" is unrelated
//   - "the run was cancelled" says nothing about the code
func Detect(jobs []JobResult, opts Options) []FlakyOccurrence {
	if opts.MinAttempts < 2 {
		opts.MinAttempts = 2
	}

	var out []FlakyOccurrence

	for key, attempts := range group(jobs) {
		if len(attempts) < opts.MinAttempts {
			continue
		}

		var failed, passed []JobResult
		for _, attempt := range attempts {
			if attempt.Conclusion.Passed() {
				passed = append(passed, attempt)
			} else {
				failed = append(failed, attempt)
			}
		}

		// Both outcomes must be present. All-red is a broken build; all-green is
		// a healthy one. Only disagreement with itself is a flake.
		if len(failed) == 0 || len(passed) == 0 {
			continue
		}

		first := attempts[0]
		out = append(out, FlakyOccurrence{
			ID:            occurrenceID(key),
			HeadSHA:       key.SHA,
			Branch:        first.Branch,
			WorkflowName:  first.WorkflowName,
			WorkflowFile:  key.WorkflowFile,
			JobName:       key.JobName,
			FailedRuns:    failed,
			PassedRuns:    passed,
			TotalAttempts: len(attempts),
			FailureRate:   float64(len(failed)) / float64(len(attempts)),
		})
	}

	// Deterministic order: flakiest first, then by identity so repeat scans of
	// unchanged data produce byte-identical reports.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].FailureRate != out[j].FailureRate {
			return out[i].FailureRate > out[j].FailureRate
		}
		if out[i].JobName != out[j].JobName {
			return out[i].JobName < out[j].JobName
		}
		return out[i].ID < out[j].ID
	})

	return out
}

// Summary aggregates a scan for reporting.
type Summary struct {
	TotalJobs        int     `json:"total_jobs"`
	DistinctCommits  int     `json:"distinct_commits"`
	FlakyOccurrences int     `json:"flaky_occurrences"`
	AffectedJobs     int     `json:"affected_jobs"`
	MeanFailureRate  float64 `json:"mean_failure_rate"`
}

// Summarise builds aggregate statistics for a scan result.
func Summarise(jobs []JobResult, occurrences []FlakyOccurrence) Summary {
	commits := make(map[string]struct{})
	for _, job := range jobs {
		if job.HeadSHA != "" {
			commits[job.HeadSHA] = struct{}{}
		}
	}

	names := make(map[string]struct{})
	var rateSum float64
	for _, occ := range occurrences {
		names[occ.JobName] = struct{}{}
		rateSum += occ.FailureRate
	}

	mean := 0.0
	if len(occurrences) > 0 {
		mean = rateSum / float64(len(occurrences))
	}

	return Summary{
		TotalJobs:        len(jobs),
		DistinctCommits:  len(commits),
		FlakyOccurrences: len(occurrences),
		AffectedJobs:     len(names),
		MeanFailureRate:  mean,
	}
}
