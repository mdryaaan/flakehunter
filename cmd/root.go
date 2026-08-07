// Package cmd wires up flakehunter's CLI.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/mdryaaan/flakehunter/internal/config"
)

var cfg = config.Default()

var rootCmd = &cobra.Command{
	Use:   "flakehunter",
	Short: "Find flaky tests in GitHub Actions CI and explain why they are flaky",
	Long: `flakehunter finds flaky tests in GitHub Actions CI, classifies the root cause of
each one with a language model, and turns the result into an actionable report.

A test is treated as flaky only when the same job, on the same commit, produced
both a pass and a failure. A red run followed by a green one on a later commit
is someone fixing the build, not a flake.

Classification runs against a local Ollama model by default, so the tool needs
no API key and sends no CI logs to a third party.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI.
func Execute() error { return rootCmd.Execute() }

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&cfg.Provider, "provider", cfg.Provider,
		"classification provider: ollama, claude, or deterministic")
	pf.StringVar(&cfg.Model, "model", "",
		"model name (defaults per provider)")
	pf.StringVar(&cfg.BaseURL, "base-url", "",
		"override the provider endpoint")
	pf.Float64Var(&cfg.Temperature, "temperature", cfg.Temperature,
		"sampling temperature; 0 keeps classification repeatable")
	pf.Float64Var(&cfg.MinConfidence, "min-confidence", cfg.MinConfidence,
		"verdicts below this confidence are reported as unknown")
}
