package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdryaaan/flakehunter/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCorpus(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "logs"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "logs", "a.log"),
		[]byte("dial tcp 10.0.0.1:443: i/o timeout"), 0o600))
	path := filepath.Join(dir, "cases.json")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadCorpus(t *testing.T) {
	good := `{"cases":[{"id":"c1","log":"logs/a.log","expected_category":"network_timeout"}]}`
	c, err := LoadCorpus(writeCorpus(t, good))
	require.NoError(t, err)
	assert.Len(t, c.Cases, 1)

	excerpt, err := c.Excerpt(c.Cases[0])
	require.NoError(t, err)
	assert.Contains(t, excerpt, "i/o timeout")
}

func TestLoadCorpusRejectsBadLabel(t *testing.T) {
	bad := `{"cases":[{"id":"c1","log":"logs/a.log","expected_category":"typo_here"}]}`
	_, err := LoadCorpus(writeCorpus(t, bad))
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid label",
		"a typo in ground truth silently caps measured accuracy, so it must fail loudly")
}

func TestLoadCorpusErrors(t *testing.T) {
	_, err := LoadCorpus("/definitely/not/here.json")
	assert.Error(t, err)

	_, err = LoadCorpus(writeCorpus(t, `{"cases":[]}`))
	assert.ErrorContains(t, err, "no cases")

	_, err = LoadCorpus(writeCorpus(t, `not json`))
	assert.Error(t, err)
}

func TestRunScoresCorpus(t *testing.T) {
	path := writeCorpus(t, `{"cases":[{"id":"c1","log":"logs/a.log","expected_category":"network_timeout"}]}`)
	corpus, err := LoadCorpus(path)
	require.NoError(t, err)

	scores, err := Run(context.Background(), corpus, llm.NewDeterministic(), 0.5, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, scores.Total)
	assert.Equal(t, 1, scores.Correct)
	assert.InDelta(t, 1.0, scores.Accuracy, 0.001)
	assert.True(t, scores.Baseline, "deterministic results must be flagged as baseline")
}

func TestRunAgainstRealCorpus(t *testing.T) {
	corpus, err := LoadCorpus(filepath.Join("..", "..", "testdata", "eval", "labeled-cases.json"))
	require.NoError(t, err)
	require.Len(t, corpus.Cases, 40, "the shipped corpus should hold 40 labelled cases")

	scores, err := Run(context.Background(), corpus, llm.NewDeterministic(), 0.5, nil)
	require.NoError(t, err)

	assert.Equal(t, 40, scores.Total)
	assert.Zero(t, scores.Errored)
	assert.Zero(t, scores.HallucinatedCitations,
		"the baseline lifts citations from the text, so none can be fabricated")
	// A guard against silent regression, not a claim about a specific score.
	assert.Greater(t, scores.Accuracy, 0.5,
		"the baseline should comfortably beat chance across 7 categories")
}
