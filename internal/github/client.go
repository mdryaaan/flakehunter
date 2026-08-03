package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	gh "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"

	"github.com/mdryaaan/flakehunter/internal/detector"
)

// Source supplies job results and logs, from the live API or from fixtures.
//
// The whole ingestion layer sits behind this interface so that --offline is a
// first-class mode rather than a test hack: a reviewer with no credentials runs
// exactly the same pipeline a maintainer does.
type Source interface {
	// ListJobResults returns every decisive job execution in the window.
	ListJobResults(ctx context.Context, opts ListOptions) ([]detector.JobResult, error)
	// DownloadJobLog returns the raw zip archive for one job.
	DownloadJobLog(ctx context.Context, runID, jobID int64) ([]byte, error)
	// Describe names the source for report provenance.
	Describe() string
}

// ListOptions narrows a scan.
type ListOptions struct {
	Owner string
	Repo  string
	// Days is how far back to look.
	Days int
	// WorkflowFile optionally restricts to a single workflow, e.g. "ci.yml".
	WorkflowFile string
	// MaxRuns caps how many runs are fetched, bounding both time and API quota.
	MaxRuns int
}

// DefaultListOptions returns sensible scan defaults.
func DefaultListOptions() ListOptions {
	return ListOptions{Days: 7, MaxRuns: 200}
}

// ParseRepo splits "owner/name" into its parts.
func ParseRepo(s string) (owner, repo string, err error) {
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repository must be in owner/name form, got %q", s)
	}
	return parts[0], parts[1], nil
}

// Client is the live GitHub API source.
type Client struct {
	api   *gh.Client
	owner string
	repo  string
}

// For sets the repository this client operates on. Kept separate from the
// constructor so one authenticated client can scan several repositories.
func (c *Client) For(owner, repo string) *Client {
	c.owner = owner
	c.repo = repo
	return c
}

// NewClient builds an authenticated client. A token is optional for public
// repositories but raises the rate limit from 60 to 5000 requests an hour,
// which is the difference between scanning one repo and scanning a fleet.
func NewClient(ctx context.Context, token string) *Client {
	if token == "" {
		token = firstNonEmpty(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))
	}

	if token == "" {
		return &Client{api: gh.NewClient(nil)}
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return &Client{api: gh.NewClient(oauth2.NewClient(ctx, ts))}
}

// Describe names the source.
func (c *Client) Describe() string { return "github api (live)" }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
