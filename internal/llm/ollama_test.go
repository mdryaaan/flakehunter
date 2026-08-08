package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mdryaaan/flakehunter/internal/verdict"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClassify(t *testing.T) {
	tests := []struct {
		name      string
		responses []string
		status    int
		wantErr   bool
		wantCat   verdict.Category
		wantCalls int
	}{
		{
			name:      "clean response",
			responses: []string{`{"category":"network_timeout","confidence":0.88,"explanation":"dial timeout","cited_lines":[],"suggested_mitigation":"retry"}`},
			wantCat:   verdict.NetworkTimeout, wantCalls: 1,
		},
		{
			name: "malformed then repaired",
			responses: []string{
				`I think this is a network problem.`,
				`{"category":"network_timeout","confidence":0.7,"explanation":"timeout","cited_lines":[],"suggested_mitigation":"retry"}`,
			},
			wantCat: verdict.NetworkTimeout, wantCalls: 2,
		},
		{
			name:      "malformed twice fails",
			responses: []string{`nope`, `still nope`},
			wantErr:   true, wantCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				idx := calls
				calls++
				if idx >= len(tt.responses) {
					idx = len(tt.responses) - 1
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(ollamaResponse{Response: tt.responses[idx], Done: true})
			}))
			defer srv.Close()

			p := NewOllama(Options{BaseURL: srv.URL, Model: "test-model"})
			got, err := p.Classify(context.Background(), Request{JobName: "unit", Excerpt: "x"})

			assert.Equal(t, tt.wantCalls, calls, "retry should happen exactly once")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCat, got.Category)
		})
	}
}

func TestOllamaTransportErrorIsNotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := NewOllama(Options{BaseURL: srv.URL})
	_, err := p.Classify(context.Background(), Request{Excerpt: "x"})

	require.Error(t, err)
	assert.Equal(t, 1, calls, "a transport failure is not fixed by re-prompting")
}

func TestOllamaDefaults(t *testing.T) {
	p := NewOllama(Options{})
	assert.Equal(t, DefaultOllamaURL, p.baseURL)
	assert.Equal(t, DefaultOllamaModel, p.Model())
	assert.Equal(t, ProviderOllama, p.Name())
}
