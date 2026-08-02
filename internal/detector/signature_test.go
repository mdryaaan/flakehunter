package detector

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConclusionDecisive(t *testing.T) {
	tests := []struct {
		c        Conclusion
		decisive bool
		passed   bool
	}{
		{ConclusionSuccess, true, true},
		{ConclusionFailure, true, false},
		{ConclusionTimedOut, true, false},
		{ConclusionCancelled, false, false},
		{ConclusionSkipped, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.c), func(t *testing.T) {
			assert.Equal(t, tt.decisive, tt.c.Decisive())
			assert.Equal(t, tt.passed, tt.c.Passed())
		})
	}
}

func TestGroupDropsIndecisiveAndIncomplete(t *testing.T) {
	jobs := []JobResult{
		job("aaa", "unit", ConclusionFailure, 1, 0),
		job("aaa", "unit", ConclusionCancelled, 2, 1),
		job("", "unit", ConclusionFailure, 3, 2),
		job("aaa", "", ConclusionFailure, 4, 3),
	}

	got := group(jobs)
	assert.Len(t, got, 1)
	for _, attempts := range got {
		assert.Len(t, attempts, 1)
	}
}

func TestOccurrenceIDIsStableAndDistinct(t *testing.T) {
	a := occurrenceID(signatureKey{SHA: "aaa", WorkflowFile: "ci.yml", JobName: "unit"})
	b := occurrenceID(signatureKey{SHA: "aaa", WorkflowFile: "ci.yml", JobName: "unit"})
	c := occurrenceID(signatureKey{SHA: "aaa", WorkflowFile: "ci.yml", JobName: "lint"})

	assert.Equal(t, a, b, "same signature must produce the same id")
	assert.NotEqual(t, a, c, "different job must produce a different id")
	assert.Len(t, a, 12)
}
