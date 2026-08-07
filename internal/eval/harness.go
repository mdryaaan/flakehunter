package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mdryaaan/flakehunter/internal/llm"
	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// Case is one hand-labelled excerpt.
type Case struct {
	ID       string `json:"id"`
	Log      string `json:"log"`
	Expected string `json:"expected_category"`
	Note     string `json:"note"`
}

// Corpus is the labelled dataset.
type Corpus struct {
	Description string `json:"description"`
	LabelPolicy string `json:"label_policy"`
	Cases       []Case `json:"cases"`
	// dir is where relative log paths resolve from.
	dir string
}

// LoadCorpus reads a labelled case file and validates every label.
func LoadCorpus(path string) (*Corpus, error) {
	data, err := os.ReadFile(path) //nolint:gosec // operator-supplied path
	if err != nil {
		return nil, fmt.Errorf("reading corpus %q: %w", path, err)
	}

	var c Corpus
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing corpus %q: %w", path, err)
	}
	if len(c.Cases) == 0 {
		return nil, fmt.Errorf("corpus %q contains no cases", path)
	}

	c.dir = filepath.Dir(path)

	// Fail loudly on a bad label: an unnoticed typo in ground truth silently
	// caps the measured accuracy and makes every later comparison wrong.
	for _, kase := range c.Cases {
		if _, err := verdict.ParseCategory(kase.Expected); err != nil {
			return nil, fmt.Errorf("case %s has an invalid label: %w", kase.ID, err)
		}
	}

	return &c, nil
}

// Excerpt reads the log body for a case.
func (c *Corpus) Excerpt(kase Case) (string, error) {
	path := filepath.Join(c.dir, filepath.FromSlash(kase.Log))
	data, err := os.ReadFile(path) //nolint:gosec // path confined to the corpus dir
	if err != nil {
		return "", fmt.Errorf("reading log for case %s: %w", kase.ID, err)
	}
	return string(data), nil
}

// Run classifies every case and scores the results.
//
// Each case is treated as an independent classification with no shared state,
// so ordering cannot leak information between cases and the score is a fair
// measure of single-shot accuracy.
func Run(ctx context.Context, corpus *Corpus, provider llm.Provider, floor float64, onCase func(Outcome)) (Scores, error) {
	outcomes := make([]Outcome, 0, len(corpus.Cases))

	for _, kase := range corpus.Cases {
		expected, err := verdict.ParseCategory(kase.Expected)
		if err != nil {
			return Scores{}, fmt.Errorf("case %s: %w", kase.ID, err)
		}

		excerpt, err := corpus.Excerpt(kase)
		if err != nil {
			return Scores{}, err
		}

		outcome := Outcome{CaseID: kase.ID, Expected: expected}

		v, err := provider.Classify(ctx, llm.Request{
			JobName:  "eval",
			StepName: "Run tests",
			Excerpt:  excerpt,
		})
		if err != nil {
			outcome.Error = err.Error()
		} else {
			v = v.VerifyCitations(excerpt).ApplyConfidenceFloor(floor)
			outcome.Predicted = v.Category
			outcome.Confidence = v.Confidence
			outcome.Correct = v.Category == expected
			outcome.Hallucinated = len(v.Hallucinated)
		}

		outcomes = append(outcomes, outcome)
		if onCase != nil {
			onCase(outcome)
		}
	}

	scores := Score(outcomes)
	scores.Provider = provider.Name()
	scores.Model = provider.Model()
	scores.Baseline = provider.Name() == llm.ProviderDeterministic

	return scores, nil
}
