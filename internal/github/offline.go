package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mdryaaan/flakehunter/internal/detector"
)

// FixtureSource reads job results and logs from a local directory instead of
// the GitHub API.
//
// This is not a test double bolted on afterwards — it is a supported mode. A
// reviewer with no token, no network and no repository access can run the full
// scan → classify → report pipeline and see real output, which is the
// difference between a tool someone evaluates and a tool someone reads about.
type FixtureSource struct {
	dir string
}

// NewFixtureSource builds an offline source rooted at dir.
func NewFixtureSource(dir string) (*FixtureSource, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("fixture directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("fixture path %q is not a directory", dir)
	}
	return &FixtureSource{dir: dir}, nil
}

// Describe names the source.
func (f *FixtureSource) Describe() string {
	return fmt.Sprintf("offline fixtures (%s)", f.dir)
}

// fixtureFile is the on-disk shape of workflow-runs.json.
type fixtureFile struct {
	Repo string            `json:"repo"`
	Jobs []fixtureJob      `json:"jobs"`
	Logs map[string]string `json:"logs"`
}

type fixtureJob struct {
	RunID        int64  `json:"run_id"`
	RunAttempt   int    `json:"run_attempt"`
	JobID        int64  `json:"job_id"`
	JobName      string `json:"job_name"`
	WorkflowName string `json:"workflow_name"`
	WorkflowFile string `json:"workflow_file"`
	HeadSHA      string `json:"head_sha"`
	Branch       string `json:"branch"`
	Conclusion   string `json:"conclusion"`
	StartedAt    string `json:"started_at"`
	URL          string `json:"url"`
	// LogArchive names a zip in the same directory, for failing jobs.
	LogArchive string `json:"log_archive,omitempty"`
}

const fixtureIndex = "workflow-runs.json"

// ListJobResults reads the fixture index.
func (f *FixtureSource) ListJobResults(_ context.Context, opts ListOptions) ([]detector.JobResult, error) {
	parsed, err := f.load()
	if err != nil {
		return nil, err
	}

	out := make([]detector.JobResult, 0, len(parsed.Jobs))
	for _, job := range parsed.Jobs {
		if opts.WorkflowFile != "" && job.WorkflowFile != opts.WorkflowFile {
			continue
		}

		started, err := parseFixtureTime(job.StartedAt)
		if err != nil {
			return nil, fmt.Errorf("job %d has an unparseable started_at: %w", job.JobID, err)
		}

		out = append(out, detector.JobResult{
			RunID:        job.RunID,
			RunAttempt:   job.RunAttempt,
			JobID:        job.JobID,
			JobName:      job.JobName,
			WorkflowName: job.WorkflowName,
			WorkflowFile: job.WorkflowFile,
			HeadSHA:      job.HeadSHA,
			Branch:       job.Branch,
			Conclusion:   detector.Conclusion(job.Conclusion),
			StartedAt:    started,
			URL:          job.URL,
		})
	}

	return out, nil
}

// DownloadJobLog reads the archive a fixture job points at.
func (f *FixtureSource) DownloadJobLog(_ context.Context, _ int64, jobID int64) ([]byte, error) {
	parsed, err := f.load()
	if err != nil {
		return nil, err
	}

	for _, job := range parsed.Jobs {
		if job.JobID != jobID {
			continue
		}
		if job.LogArchive == "" {
			return nil, fmt.Errorf("fixture job %d has no log archive", jobID)
		}

		path := filepath.Join(f.dir, filepath.Base(job.LogArchive))
		data, err := os.ReadFile(path) //nolint:gosec // path is confined to the fixture dir
		if err != nil {
			return nil, fmt.Errorf("reading fixture archive %q: %w", path, err)
		}
		return data, nil
	}

	return nil, fmt.Errorf("no fixture job with id %d", jobID)
}

func (f *FixtureSource) load() (*fixtureFile, error) {
	path := filepath.Join(f.dir, fixtureIndex)
	data, err := os.ReadFile(path) //nolint:gosec // path is confined to the fixture dir
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var parsed fixtureFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(parsed.Jobs) == 0 {
		return nil, fmt.Errorf("%s contains no jobs", path)
	}

	return &parsed, nil
}
