// Package eval measures classifier accuracy against a hand-labelled corpus.
package eval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// CategoryScore is precision, recall and F1 for one category.
type CategoryScore struct {
	Category  verdict.Category `json:"category"`
	Support   int              `json:"support"`   // labelled instances of this category
	Predicted int              `json:"predicted"` // times the classifier chose it
	TruePos   int              `json:"true_positives"`
	Precision float64          `json:"precision"`
	Recall    float64          `json:"recall"`
	F1        float64          `json:"f1"`
}

// Scores is the full evaluation result.
type Scores struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// Baseline marks results from the rule-based provider so no reader can
	// mistake them for a language model's accuracy.
	Baseline bool `json:"baseline"`

	Total    int     `json:"total"`
	Correct  int     `json:"correct"`
	Errored  int     `json:"errored"`
	Accuracy float64 `json:"accuracy"`
	// MacroF1 averages per-category F1 without weighting by support, so a large
	// easy category cannot hide poor performance on a small hard one.
	MacroF1 float64 `json:"macro_f1"`

	PerCategory []CategoryScore `json:"per_category"`
	// Confusion[actual][predicted] counts.
	Confusion map[verdict.Category]map[verdict.Category]int `json:"confusion"`

	MeanConfidenceCorrect float64 `json:"mean_confidence_correct"`
	MeanConfidenceWrong   float64 `json:"mean_confidence_wrong"`
	HallucinatedCitations int     `json:"hallucinated_citations"`
}

// Outcome is one case's result.
type Outcome struct {
	CaseID       string           `json:"case_id"`
	Expected     verdict.Category `json:"expected"`
	Predicted    verdict.Category `json:"predicted"`
	Confidence   float64          `json:"confidence"`
	Correct      bool             `json:"correct"`
	Hallucinated int              `json:"hallucinated"`
	Error        string           `json:"error,omitempty"`
}

// Score turns a set of outcomes into aggregate metrics.
func Score(outcomes []Outcome) Scores {
	s := Scores{
		Confusion: make(map[verdict.Category]map[verdict.Category]int),
	}

	tp := make(map[verdict.Category]int)
	predicted := make(map[verdict.Category]int)
	support := make(map[verdict.Category]int)

	var confCorrect, confWrong float64
	var nCorrect, nWrong int

	for _, o := range outcomes {
		s.Total++
		if o.Error != "" {
			s.Errored++
			continue
		}

		support[o.Expected]++
		predicted[o.Predicted]++
		s.HallucinatedCitations += o.Hallucinated

		if s.Confusion[o.Expected] == nil {
			s.Confusion[o.Expected] = make(map[verdict.Category]int)
		}
		s.Confusion[o.Expected][o.Predicted]++

		if o.Correct {
			s.Correct++
			tp[o.Expected]++
			confCorrect += o.Confidence
			nCorrect++
		} else {
			confWrong += o.Confidence
			nWrong++
		}
	}

	scored := s.Total - s.Errored
	if scored > 0 {
		s.Accuracy = float64(s.Correct) / float64(scored)
	}
	if nCorrect > 0 {
		s.MeanConfidenceCorrect = confCorrect / float64(nCorrect)
	}
	if nWrong > 0 {
		s.MeanConfidenceWrong = confWrong / float64(nWrong)
	}

	var f1Sum float64
	var f1Count int

	for _, category := range verdict.AllCategories() {
		if support[category] == 0 && predicted[category] == 0 {
			continue
		}

		cs := CategoryScore{
			Category:  category,
			Support:   support[category],
			Predicted: predicted[category],
			TruePos:   tp[category],
		}
		if cs.Predicted > 0 {
			cs.Precision = float64(cs.TruePos) / float64(cs.Predicted)
		}
		if cs.Support > 0 {
			cs.Recall = float64(cs.TruePos) / float64(cs.Support)
		}
		if cs.Precision+cs.Recall > 0 {
			cs.F1 = 2 * cs.Precision * cs.Recall / (cs.Precision + cs.Recall)
		}

		// Macro-F1 averages over categories that were actually labelled.
		if cs.Support > 0 {
			f1Sum += cs.F1
			f1Count++
		}

		s.PerCategory = append(s.PerCategory, cs)
	}

	if f1Count > 0 {
		s.MacroF1 = f1Sum / float64(f1Count)
	}

	sort.SliceStable(s.PerCategory, func(i, j int) bool {
		return s.PerCategory[i].Category < s.PerCategory[j].Category
	})

	return s
}

// ConfusionMatrix renders the matrix as a fixed-width text table.
func (s Scores) ConfusionMatrix() string {
	cats := verdict.AllCategories()

	// Short headers keep the table inside a terminal.
	short := map[verdict.Category]string{
		verdict.NetworkTimeout:      "net",
		verdict.RaceCondition:       "race",
		verdict.InfraFlake:          "infra",
		verdict.ResourceExhaustion:  "res",
		verdict.TestOrderDependency: "order",
		verdict.GenuineBug:          "bug",
		verdict.Unknown:             "unk",
	}

	var b strings.Builder
	b.WriteString("actual \\ predicted  ")
	for _, c := range cats {
		fmt.Fprintf(&b, "%6s", short[c])
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", 20+6*len(cats)) + "\n")

	for _, actual := range cats {
		row, seen := s.Confusion[actual]
		if !seen {
			continue
		}
		fmt.Fprintf(&b, "%-20s", short[actual])
		for _, pred := range cats {
			n := row[pred]
			if n == 0 {
				fmt.Fprintf(&b, "%6s", ".")
				continue
			}
			fmt.Fprintf(&b, "%6d", n)
		}
		b.WriteString("\n")
	}

	return b.String()
}
