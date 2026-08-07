package eval

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

func TestScorePerfect(t *testing.T) {
	outcomes := []Outcome{
		{CaseID: "1", Expected: verdict.NetworkTimeout, Predicted: verdict.NetworkTimeout, Correct: true, Confidence: 0.9},
		{CaseID: "2", Expected: verdict.RaceCondition, Predicted: verdict.RaceCondition, Correct: true, Confidence: 0.8},
	}

	s := Score(outcomes)
	assert.Equal(t, 2, s.Total)
	assert.Equal(t, 2, s.Correct)
	assert.InDelta(t, 1.0, s.Accuracy, 0.001)
	assert.InDelta(t, 1.0, s.MacroF1, 0.001)
	assert.InDelta(t, 0.85, s.MeanConfidenceCorrect, 0.001)
}

func TestScoreWithMistakes(t *testing.T) {
	outcomes := []Outcome{
		{Expected: verdict.NetworkTimeout, Predicted: verdict.NetworkTimeout, Correct: true, Confidence: 0.9},
		{Expected: verdict.NetworkTimeout, Predicted: verdict.InfraFlake, Confidence: 0.4},
		{Expected: verdict.InfraFlake, Predicted: verdict.InfraFlake, Correct: true, Confidence: 0.7},
	}

	s := Score(outcomes)
	assert.InDelta(t, 2.0/3.0, s.Accuracy, 0.001)

	byCat := map[verdict.Category]CategoryScore{}
	for _, c := range s.PerCategory {
		byCat[c.Category] = c
	}

	// network: 1 of 1 predicted correct, 1 of 2 actual recalled.
	assert.InDelta(t, 1.0, byCat[verdict.NetworkTimeout].Precision, 0.001)
	assert.InDelta(t, 0.5, byCat[verdict.NetworkTimeout].Recall, 0.001)
	// infra: 1 of 2 predicted correct, 1 of 1 actual recalled.
	assert.InDelta(t, 0.5, byCat[verdict.InfraFlake].Precision, 0.001)
	assert.InDelta(t, 1.0, byCat[verdict.InfraFlake].Recall, 0.001)

	assert.InDelta(t, 0.4, s.MeanConfidenceWrong, 0.001)
}

func TestScoreCountsErrorsSeparately(t *testing.T) {
	outcomes := []Outcome{
		{Expected: verdict.NetworkTimeout, Predicted: verdict.NetworkTimeout, Correct: true, Confidence: 0.9},
		{Expected: verdict.InfraFlake, Error: "provider unreachable"},
	}

	s := Score(outcomes)
	assert.Equal(t, 2, s.Total)
	assert.Equal(t, 1, s.Errored)
	assert.InDelta(t, 1.0, s.Accuracy, 0.001,
		"accuracy is over cases that were actually scored, not over errors")
}

func TestScoreEmpty(t *testing.T) {
	s := Score(nil)
	assert.Zero(t, s.Total)
	assert.Zero(t, s.Accuracy)
	assert.Empty(t, s.PerCategory)
}

func TestConfusionMatrixRenders(t *testing.T) {
	s := Score([]Outcome{
		{Expected: verdict.NetworkTimeout, Predicted: verdict.NetworkTimeout, Correct: true},
		{Expected: verdict.InfraFlake, Predicted: verdict.Unknown},
	})

	out := s.ConfusionMatrix()
	assert.Contains(t, out, "actual \\ predicted")
	assert.Contains(t, out, "net")
	assert.Contains(t, out, "unk")
}
