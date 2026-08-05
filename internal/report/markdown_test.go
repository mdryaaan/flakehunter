package report

import (
	"bytes"
	"testing"
	"time"

	"github.com/mdryaaan/flakehunter/internal/detector"
	"github.com/mdryaaan/flakehunter/internal/verdict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleResult() ClassifiedResult {
	return ClassifiedResult{
		Repo:        "acme/orders-api",
		Source:      "offline fixtures",
		GeneratedAt: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		Provider:    "ollama",
		Model:       "llama3",
		Items: []Classified{
			{
				Occurrence: detector.FlakyOccurrence{
					JobName: "unit-tests", HeadSHA: "4f2a9c1abc", Branch: "main",
					WorkflowName: "CI", WorkflowFile: "ci.yml",
					FailureRate: 0.5, TotalAttempts: 2,
					FailedRuns: []detector.JobResult{{URL: "https://example.test/fail"}},
					PassedRuns: []detector.JobResult{{URL: "https://example.test/pass"}},
				},
				Verdict: verdict.Verdict{
					Category: verdict.NetworkTimeout, Confidence: 0.88,
					Explanation: "The module proxy timed out.",
					CitedLines:  []string{"dial tcp 1.2.3.4:443: i/o timeout"},
				},
			},
			{
				Occurrence: detector.FlakyOccurrence{JobName: "pricing", HeadSHA: "d7a3f10"},
				Verdict: verdict.Verdict{
					Category: verdict.GenuineBug, Confidence: 0.71,
					Explanation: "A rounding assertion failed deterministically.",
				},
			},
		},
	}
}

func TestWriteMarkdown(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteMarkdown(&buf, sampleResult()))
	out := buf.String()

	assert.Contains(t, out, "# Flake report")
	assert.Contains(t, out, "acme/orders-api")
	assert.Contains(t, out, "Network timeout")
	assert.Contains(t, out, "dial tcp 1.2.3.4:443: i/o timeout")
	assert.Contains(t, out, "https://example.test/fail")

	// A genuine bug outranks a network timeout, so it must appear first.
	assert.Less(t, indexOf(out, "— Genuine bug"), indexOf(out, "— Network timeout"),
		"the most actionable category should lead the detail section")
}

func TestWriteMarkdownFlagsBaseline(t *testing.T) {
	res := sampleResult()
	res.Baseline = true
	res.Provider = "deterministic"

	var buf bytes.Buffer
	require.NoError(t, WriteMarkdown(&buf, res))
	assert.Contains(t, buf.String(), "not a language model",
		"baseline output must never be mistakable for model output")
}

func TestWriteMarkdownEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteMarkdown(&buf, ClassifiedResult{Repo: "acme/x"}))
	assert.Contains(t, buf.String(), "No flaky occurrences")
}

func TestWriteMarkdownSurfacesErrors(t *testing.T) {
	res := ClassifiedResult{Items: []Classified{{
		Occurrence: detector.FlakyOccurrence{JobName: "broken"},
		Error:      "downloading log: 404",
	}}}

	var buf bytes.Buffer
	require.NoError(t, WriteMarkdown(&buf, res))
	assert.Contains(t, buf.String(), "Classification failed: downloading log: 404")
}

func TestWriteIssue(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteIssue(&buf, sampleResult()))
	out := buf.String()

	assert.Contains(t, out, "## Flaky tests detected")
	assert.Contains(t, out, "- [ ] `unit-tests`")
	assert.Contains(t, out, "<details><summary>Suggested steps</summary>")
	assert.Less(t, indexOf(out, "### Genuine bug"), indexOf(out, "### Network timeout"))
}

func TestWriteDigest(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteDigest(&buf, sampleResult(), 7))
	out := buf.String()

	assert.Contains(t, out, "Flake digest — last 7 days")
	assert.Contains(t, out, "Noisiest jobs")
	assert.Contains(t, out, "genuine bug",
		"a bug masked by a rerun deserves an explicit warning in the digest")
}

func TestWriteDigestEmpty(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteDigest(&buf, ClassifiedResult{Repo: "acme/x"}, 7))
	assert.Contains(t, buf.String(), "No flaky occurrences detected")
}

func TestJSONRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteJSON(&buf, sampleResult()))

	var back ClassifiedResult
	require.NoError(t, ReadJSON(&buf, &back))
	assert.Equal(t, "acme/orders-api", back.Repo)
	require.Len(t, back.Items, 2)
	assert.Equal(t, verdict.NetworkTimeout, back.Items[0].Verdict.Category)
}

func TestCountsByCategoryIgnoresErrors(t *testing.T) {
	res := sampleResult()
	res.Items = append(res.Items, Classified{Error: "boom"})

	counts := res.CountsByCategory()
	assert.Equal(t, 1, counts[verdict.NetworkTimeout])
	assert.Equal(t, 1, counts[verdict.GenuineBug])
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
