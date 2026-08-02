package verdict

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		verdict Verdict
		wantErr bool
	}{
		{"valid", Verdict{Category: NetworkTimeout, Confidence: 0.9, Explanation: "timeout"}, false},
		{"unknown category", Verdict{Category: "made_up", Confidence: 0.9, Explanation: "x"}, true},
		{"confidence too high", Verdict{Category: InfraFlake, Confidence: 1.4, Explanation: "x"}, true},
		{"confidence negative", Verdict{Category: InfraFlake, Confidence: -0.1, Explanation: "x"}, true},
		{"empty explanation", Verdict{Category: InfraFlake, Confidence: 0.7, Explanation: "   "}, true},
		{"zero confidence is valid", Verdict{Category: Unknown, Confidence: 0, Explanation: "x"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.verdict.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrInvalid)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestApplyConfidenceFloor(t *testing.T) {
	tests := []struct {
		name          string
		in            Verdict
		floor         float64
		wantCategory  Category
		wantDowngrade bool
	}{
		{"above floor kept", Verdict{Category: RaceCondition, Confidence: 0.8}, 0.5, RaceCondition, false},
		{"at floor kept", Verdict{Category: RaceCondition, Confidence: 0.5}, 0.5, RaceCondition, false},
		{"below floor demoted", Verdict{Category: RaceCondition, Confidence: 0.3}, 0.5, Unknown, true},
		{"unknown untouched", Verdict{Category: Unknown, Confidence: 0.1}, 0.5, Unknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.ApplyConfidenceFloor(tt.floor)
			assert.Equal(t, tt.wantCategory, got.Category)
			assert.Equal(t, tt.wantDowngrade, got.Downgraded)
			if tt.wantDowngrade {
				assert.Equal(t, tt.in.Category, got.RawCategory)
			}
		})
	}
}

func TestVerifyCitations(t *testing.T) {
	excerpt := "line one\n  dial tcp 10.0.0.1:443: i/o timeout\nline three"

	tests := []struct {
		name             string
		cited            []string
		wantKept         int
		wantHallucinated int
	}{
		{"exact match kept", []string{"line one"}, 1, 0},
		{"trimmed match kept", []string{"  dial tcp 10.0.0.1:443: i/o timeout  "}, 1, 0},
		{"fabricated dropped", []string{"ERROR: nothing like this exists"}, 0, 1},
		{"mixed", []string{"line three", "invented line"}, 1, 1},
		{"blank ignored", []string{"   "}, 0, 0},
		{"none", nil, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Verdict{CitedLines: tt.cited}.VerifyCitations(excerpt)
			assert.Len(t, got.CitedLines, tt.wantKept)
			assert.Len(t, got.Hallucinated, tt.wantHallucinated)
		})
	}
}

func TestParseCategory(t *testing.T) {
	for _, c := range AllCategories() {
		got, err := ParseCategory(string(c))
		require.NoError(t, err)
		assert.Equal(t, c, got)
		assert.NotEmpty(t, got.Label())
	}

	_, err := ParseCategory("nonsense")
	assert.Error(t, err)
}

func TestMitigationForCoversEveryCategory(t *testing.T) {
	for _, c := range AllCategories() {
		m := MitigationFor(c)
		assert.NotEmpty(t, m.Summary, "category %s has no mitigation summary", c)
		assert.NotEmpty(t, m.Steps, "category %s has no mitigation steps", c)
		assert.NotEmpty(t, m.Owner, "category %s has no owner", c)
	}
}
