package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

func TestParseVerdict(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantErr  bool
		wantCat  verdict.Category
		wantConf float64
	}{
		{
			name:    "bare object",
			raw:     `{"category":"network_timeout","confidence":0.9,"explanation":"dial timeout","cited_lines":["dial tcp"],"suggested_mitigation":"retry"}`,
			wantCat: verdict.NetworkTimeout, wantConf: 0.9,
		},
		{
			name:    "fenced json block",
			raw:     "Here is my analysis:\n```json\n{\"category\":\"race_condition\",\"confidence\":0.75,\"explanation\":\"data race\",\"cited_lines\":[],\"suggested_mitigation\":\"sync\"}\n```\nHope that helps.",
			wantCat: verdict.RaceCondition, wantConf: 0.75,
		},
		{
			name:    "object with surrounding prose",
			raw:     `Sure! {"category":"infra_flake","confidence":0.6,"explanation":"runner died","cited_lines":[],"suggested_mitigation":"rerun"} Let me know.`,
			wantCat: verdict.InfraFlake, wantConf: 0.6,
		},
		{
			name:    "nested braces inside strings",
			raw:     `{"category":"genuine_bug","confidence":0.8,"explanation":"map {a:1} differed","cited_lines":["got {x:2}"],"suggested_mitigation":"fix"}`,
			wantCat: verdict.GenuineBug, wantConf: 0.8,
		},
		{"empty", "", true, "", 0},
		{"no json at all", "I could not determine the cause.", true, "", 0},
		{"invalid category", `{"category":"cosmic_rays","confidence":0.9,"explanation":"x"}`, true, "", 0},
		{"confidence out of range", `{"category":"infra_flake","confidence":7,"explanation":"x"}`, true, "", 0},
		{"empty explanation", `{"category":"infra_flake","confidence":0.9,"explanation":""}`, true, "", 0},
		{"unbalanced object", `{"category":"infra_flake","confidence":0.9`, true, "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVerdict(tt.raw)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrMalformed)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCat, got.Category)
			assert.InDelta(t, tt.wantConf, got.Confidence, 0.001)
		})
	}
}

func TestParseVerdictNormalisesCategoryCase(t *testing.T) {
	got, err := ParseVerdict(`{"category":"  NETWORK_TIMEOUT ","confidence":0.9,"explanation":"x"}`)
	require.NoError(t, err)
	assert.Equal(t, verdict.NetworkTimeout, got.Category)
}
