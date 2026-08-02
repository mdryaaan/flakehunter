package extractor

import (
	"strings"
)

// failureMarkers are the phrases that indicate a step actually failed, in
// rough order of specificity. Matching is case-insensitive.
var failureMarkers = []string{
	"##[error]",
	"process completed with exit code",
	"panic:",
	"data race",
	"fatal error:",
	"--- fail:",
	"error:",
	"failed",
	"assertion",
	"timeout",
	"exception",
}

// FailingStep picks the step most likely to contain the real failure.
//
// The last step with an explicit error marker wins. Preferring the last one
// matters: post-job cleanup steps frequently mention "error" while unwinding,
// and a naive first-match lands on a red herring instead of the test output.
func FailingStep(logs []StepLog) (StepLog, bool) {
	best := -1
	bestScore := 0

	for i, log := range logs {
		score := failureScore(log)
		if score == 0 {
			continue
		}
		// Later steps win ties, since the failure is normally near the end.
		if score >= bestScore {
			best = i
			bestScore = score
		}
	}

	if best < 0 {
		return StepLog{}, false
	}
	return logs[best], true
}

// failureScore ranks how strongly a step looks like the failing one.
func failureScore(log StepLog) int {
	lower := strings.ToLower(log.Body)
	nameLower := strings.ToLower(log.Name)

	// Housekeeping steps are noisy and almost never the true cause.
	if strings.HasPrefix(nameLower, "post ") || nameLower == "complete job" {
		return 0
	}

	score := 0
	for i, marker := range failureMarkers {
		if strings.Contains(lower, marker) {
			// Earlier markers in the list are more specific, so worth more.
			score += len(failureMarkers) - i
		}
	}
	return score
}

// ErrorBlock returns the contiguous region around the strongest failure signal,
// with `before` lines of lead-in and `after` lines of trailing context.
//
// Anchoring on the failure rather than on the end of the file matters because
// Go's test runner, and most others, print a summary block after the failure —
// tailing the file blindly captures the summary and drops the stack trace that
// explains it.
func ErrorBlock(body string, before, after int) string {
	lines := strings.Split(body, "\n")
	anchor := -1

	for i := len(lines) - 1; i >= 0; i-- {
		lower := strings.ToLower(lines[i])
		for _, marker := range failureMarkers[:5] { // only the specific markers
			if strings.Contains(lower, marker) {
				anchor = i
				break
			}
		}
		if anchor >= 0 {
			break
		}
	}

	if anchor < 0 {
		return body
	}

	start := anchor - before
	if start < 0 {
		start = 0
	}
	end := anchor + after + 1
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}
