package github

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		in        string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"acme/orders-api", "acme", "orders-api", false},
		{"  acme/orders-api  ", "acme", "orders-api", false},
		{"acme", "", "", true},
		{"acme/", "", "", true},
		{"/orders", "", "", true},
		{"a/b/c", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			owner, repo, err := ParseRepo(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestClassifyResponseRetryability(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		headers       map[string]string
		wantErr       bool
		wantRetryable bool
	}{
		{"200 is not an error", 200, nil, false, false},
		{"404 is permanent", 404, nil, true, false},
		{"422 is permanent", 422, nil, true, false},
		{"500 is retryable", 500, nil, true, true},
		{"502 is retryable", 502, nil, true, true},
		{"429 is retryable", 429, nil, true, true},
		{"403 rate limit is retryable", 403, map[string]string{"X-RateLimit-Remaining": "0"}, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tt.status, Header: http.Header{}}
			for k, v := range tt.headers {
				resp.Header.Set(k, v)
			}

			err := classifyResponse(resp)
			if !tt.wantErr {
				assert.Nil(t, err)
				return
			}
			require.NotNil(t, err)
			assert.Equal(t, tt.wantRetryable, err.Retryable())
			assert.Contains(t, err.Error(), "github api")
		})
	}
}

func TestRetryAfterHeaders(t *testing.T) {
	resp := &http.Response{StatusCode: 429, Header: http.Header{}}
	resp.Header.Set("Retry-After", "42")
	assert.Equal(t, 42*time.Second, retryAfter(resp))

	resp2 := &http.Response{StatusCode: 403, Header: http.Header{}}
	assert.Zero(t, retryAfter(resp2))
}

func TestNewClientWithoutTokenStillWorks(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	c := NewClient(context.Background(), "").For("acme", "orders")
	assert.Equal(t, "acme", c.owner)
	assert.Equal(t, "orders", c.repo)
	assert.Contains(t, c.Describe(), "live")
}
