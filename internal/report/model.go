// Package report renders scan and classification results for humans and machines.
package report

import (
	"time"

	"github.com/mdryaaan/flakehunter/internal/detector"
	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// ScanResult is what `flakehunter scan` writes and `classify` consumes.
type ScanResult struct {
	Repo        string                     `json:"repo"`
	Source      string                     `json:"source"`
	GeneratedAt time.Time                  `json:"generated_at"`
	WindowDays  int                        `json:"window_days"`
	Summary     detector.Summary           `json:"summary"`
	Occurrences []detector.FlakyOccurrence `json:"occurrences"`
}

// Classified pairs an occurrence with the verdict returned for it.
type Classified struct {
	Occurrence detector.FlakyOccurrence `json:"occurrence"`
	Verdict    verdict.Verdict          `json:"verdict"`
	StepName   string                   `json:"step_name"`
	ExcerptLen int                      `json:"excerpt_len"`
	// Error records a classification that failed, so a partial run is still
	// reportable rather than being thrown away entirely.
	Error string `json:"error,omitempty"`
}

// ClassifiedResult is what `flakehunter classify` writes and `report` consumes.
type ClassifiedResult struct {
	Repo        string    `json:"repo"`
	Source      string    `json:"source"`
	GeneratedAt time.Time `json:"generated_at"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	// Baseline marks output produced by the rule-based provider, so no report
	// can present it as a language model's work.
	Baseline bool         `json:"baseline"`
	Items    []Classified `json:"items"`
}

// CountsByCategory tallies verdicts for summary tables.
func (r ClassifiedResult) CountsByCategory() map[verdict.Category]int {
	counts := make(map[verdict.Category]int)
	for _, item := range r.Items {
		if item.Error != "" {
			continue
		}
		counts[item.Verdict.Category]++
	}
	return counts
}

// ProviderLabel describes the classifier for report headers, marking the
// baseline explicitly.
func (r ClassifiedResult) ProviderLabel() string {
	if r.Baseline {
		return r.Provider + " / " + r.Model + " — rule-based baseline, not a language model"
	}
	return r.Provider + " / " + r.Model
}
