package llm

import (
	"context"
	"testing"

	"github.com/mdryaaan/flakehunter/internal/verdict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicClassify(t *testing.T) {
	tests := []struct {
		name    string
		excerpt string
		want    verdict.Category
	}{
		{"network timeout", "Get \"https://proxy.golang.org\": dial tcp 142.250.1.1:443: i/o timeout", verdict.NetworkTimeout},
		{"race condition", "==================\nWARNING: DATA RACE\nWrite at 0x00c0000b4010 by goroutine 8:", verdict.RaceCondition},
		{"infra flake", "The runner has received a shutdown signal. Cancelling the job.", verdict.InfraFlake},
		{"resource exhaustion", "write /home/runner/work/x: no space left on device", verdict.ResourceExhaustion},
		{"test order", "PASS when run in isolation; fails only with -shuffle=on", verdict.TestOrderDependency},
		{"genuine bug", "panic: runtime error: index out of range [5] with length 3", verdict.GenuineBug},
		{"nothing matches", "everything looked fine, exiting", verdict.Unknown},
	}

	d := NewDeterministic()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.Classify(context.Background(), Request{Excerpt: tt.excerpt})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.Category)
			require.NoError(t, got.Validate())
			assert.Contains(t, got.Explanation, BaselineDisclaimer,
				"every baseline verdict must declare that no model was involved")
		})
	}
}

func TestDeterministicIsStable(t *testing.T) {
	d := NewDeterministic()
	excerpt := "dial tcp: i/o timeout\nWARNING: DATA RACE\nno space left on device"

	first, err := d.Classify(context.Background(), Request{Excerpt: excerpt})
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		got, err := d.Classify(context.Background(), Request{Excerpt: excerpt})
		require.NoError(t, err)
		assert.Equal(t, first.Category, got.Category,
			"a tie must not resolve differently between runs")
		assert.InDelta(t, first.Confidence, got.Confidence, 0.0001)
	}
}

func TestDeterministicConfidenceIsBounded(t *testing.T) {
	d := NewDeterministic()
	excerpt := "dial tcp i/o timeout connection refused tls handshake timeout " +
		"context deadline exceeded no such host could not resolve host"

	got, err := d.Classify(context.Background(), Request{Excerpt: excerpt})
	require.NoError(t, err)
	assert.LessOrEqual(t, got.Confidence, 0.92, "a keyword match is never certainty")
}

func TestDeterministicCitesRealLines(t *testing.T) {
	d := NewDeterministic()
	excerpt := "step one\ndial tcp 10.0.0.1:443: i/o timeout\nstep three"

	got, err := d.Classify(context.Background(), Request{Excerpt: excerpt})
	require.NoError(t, err)

	verified := got.VerifyCitations(excerpt)
	assert.Empty(t, verified.Hallucinated, "baseline citations are lifted from the text")
	assert.NotEmpty(t, verified.CitedLines)
}

func TestNewProvider(t *testing.T) {
	for _, name := range []string{ProviderOllama, ProviderDeterministic, ""} {
		p, err := New(Options{Provider: name})
		require.NoError(t, err)
		assert.NotEmpty(t, p.Name())
	}

	_, err := New(Options{Provider: "nope"})
	assert.Error(t, err)
}
