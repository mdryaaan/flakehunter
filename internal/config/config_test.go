package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*Config)
		needRepo bool
		wantErr  string
	}{
		{"defaults with repo", func(c *Config) { c.Repo = "acme/api" }, true, ""},
		{"missing repo", func(c *Config) {}, true, "--repo is required"},
		{"bad repo form", func(c *Config) { c.Repo = "acme" }, true, "owner/name"},
		{"offline without fixtures", func(c *Config) { c.Offline = true }, true, "--fixtures"},
		{"offline with missing dir", func(c *Config) { c.Offline = true; c.FixturesDir = "/nope/nowhere" }, true, "fixtures directory"},
		{"zero days", func(c *Config) { c.Repo = "a/b"; c.Days = 0 }, true, "--days"},
		{"confidence too high", func(c *Config) { c.Repo = "a/b"; c.MinConfidence = 1.5 }, true, "--min-confidence"},
		{"unknown provider", func(c *Config) { c.Repo = "a/b"; c.Provider = "gpt" }, true, "--provider"},
		{"no repo needed", func(c *Config) {}, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.mutate(&cfg)

			err := cfg.Validate(tt.needRepo)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestValidateSplitsRepo(t *testing.T) {
	cfg := Default()
	cfg.Repo = "acme/orders-api"
	require.NoError(t, cfg.Validate(true))
	assert.Equal(t, "acme", cfg.Owner)
	assert.Equal(t, "orders-api", cfg.RepoName)
}

func TestLLMOptions(t *testing.T) {
	cfg := Default()
	cfg.Model = "llama3"
	opts := cfg.LLMOptions()
	assert.Equal(t, "ollama", opts.Provider)
	assert.Equal(t, "llama3", opts.Model)
}
