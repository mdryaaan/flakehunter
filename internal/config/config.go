// Package config holds flakehunter's runtime settings.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/mdryaaan/flakehunter/internal/llm"
	"github.com/mdryaaan/flakehunter/internal/verdict"
)

// Config is the resolved configuration for a command invocation.
type Config struct {
	Repo         string
	Owner        string
	RepoName     string
	Days         int
	WorkflowFile string
	MaxRuns      int

	Offline     bool
	FixturesDir string

	Provider    string
	Model       string
	BaseURL     string
	Temperature float64

	MinConfidence float64
	Token         string
}

// Default returns the baseline settings before flags are applied.
func Default() Config {
	return Config{
		Days:          7,
		MaxRuns:       200,
		Provider:      llm.ProviderOllama,
		Temperature:   0.0,
		MinConfidence: verdict.DefaultConfidenceFloor,
	}
}

// Validate checks the configuration and normalises derived fields.
func (c *Config) Validate(needRepo bool) error {
	if needRepo {
		if c.Offline {
			// Offline runs read a fixture set, so a repo is decoration; accept
			// anything the user passed and carry on.
			if c.FixturesDir == "" {
				return fmt.Errorf("--offline needs --fixtures pointing at a fixture directory")
			}
			if _, err := os.Stat(c.FixturesDir); err != nil {
				return fmt.Errorf("fixtures directory: %w", err)
			}
		} else {
			if strings.TrimSpace(c.Repo) == "" {
				return fmt.Errorf("--repo is required (owner/name), or use --offline --fixtures")
			}
			parts := strings.Split(c.Repo, "/")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return fmt.Errorf("--repo must be owner/name, got %q", c.Repo)
			}
			c.Owner, c.RepoName = parts[0], parts[1]
		}
	}

	if c.Days < 1 {
		return fmt.Errorf("--days must be at least 1, got %d", c.Days)
	}
	if c.MinConfidence < 0 || c.MinConfidence > 1 {
		return fmt.Errorf("--min-confidence must be in [0,1], got %.2f", c.MinConfidence)
	}
	switch c.Provider {
	case llm.ProviderOllama, llm.ProviderClaude, llm.ProviderDeterministic:
	default:
		return fmt.Errorf("--provider must be one of ollama, claude, deterministic; got %q", c.Provider)
	}

	return nil
}

// LLMOptions projects the config onto the llm package's options.
func (c Config) LLMOptions() llm.Options {
	return llm.Options{
		Provider:    c.Provider,
		Model:       c.Model,
		BaseURL:     c.BaseURL,
		Temperature: c.Temperature,
	}
}
