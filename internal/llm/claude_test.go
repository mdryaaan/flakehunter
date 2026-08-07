package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mdryaaan/flakehunter/internal/verdict"
)

func TestNewClaudeRequiresKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := NewClaude(Options{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "ollama", "the error should point at the keyless alternative")

	p, err := NewClaude(Options{APIKey: "sk-test"})
	require.NoError(t, err)
	assert.Equal(t, ProviderClaude, p.Name())
	assert.Equal(t, DefaultClaudeModel, p.Model())
}

func TestNewClaudeReadsEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-from-env")
	p, err := NewClaude(Options{})
	require.NoError(t, err)
	assert.Equal(t, "sk-from-env", p.apiKey)
}

func TestClaudeClassify(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "sk-test", r.Header.Get("x-api-key"))
		assert.Equal(t, anthropicVersion, r.Header.Get("anthropic-version"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"category\":\"race_condition\",\"confidence\":0.81,\"explanation\":\"data race reported\",\"cited_lines\":[],\"suggested_mitigation\":\"add sync\"}"}]}`))
	}))
	defer srv.Close()

	p, err := NewClaude(Options{APIKey: "sk-test", BaseURL: srv.URL})
	require.NoError(t, err)

	got, err := p.Classify(context.Background(), Request{JobName: "unit", Excerpt: "WARNING: DATA RACE"})
	require.NoError(t, err)
	assert.Equal(t, verdict.RaceCondition, got.Category)
	assert.InDelta(t, 0.81, got.Confidence, 0.001)
}

func TestClaudeSurfacesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	}))
	defer srv.Close()

	p, err := NewClaude(Options{APIKey: "sk-test", BaseURL: srv.URL})
	require.NoError(t, err)

	_, err = p.Classify(context.Background(), Request{Excerpt: "x"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "rate_limit_error")
}
