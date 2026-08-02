// Package llm wraps the classification model behind a narrow interface so the
// rest of flakehunter never depends on a specific vendor.
package llm

import (
	"context"
	"fmt"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// Provider classifies a single flaky-failure excerpt.
//
// The interface is deliberately one method: everything flakehunter needs from a
// model is "read this log, return this schema". Keeping it that narrow is what
// makes the deterministic provider a drop-in for evaluation, and what lets a
// new vendor be added without touching the pipeline.
type Provider interface {
	// Name identifies the provider in reports and eval output.
	Name() string
	// Model identifies the specific model in use.
	Model() string
	// Classify returns a validated verdict for the excerpt.
	Classify(ctx context.Context, req Request) (verdict.Verdict, error)
}

// Request is everything a provider needs to classify one occurrence.
type Request struct {
	JobName  string
	StepName string
	Excerpt  string
}

// Options configures provider construction.
type Options struct {
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
	// Temperature is pinned low by callers: classification wants repeatability,
	// not creativity.
	Temperature float64
}

// Known provider identifiers.
const (
	ProviderOllama        = "ollama"
	ProviderClaude        = "claude"
	ProviderDeterministic = "deterministic"
)

// New builds a provider from options.
func New(opts Options) (Provider, error) {
	switch opts.Provider {
	case ProviderOllama, "":
		return NewOllama(opts), nil
	case ProviderClaude:
		return NewClaude(opts)
	case ProviderDeterministic:
		return NewDeterministic(), nil
	default:
		return nil, fmt.Errorf("unknown provider %q (want one of: %s, %s, %s)",
			opts.Provider, ProviderOllama, ProviderClaude, ProviderDeterministic)
	}
}
