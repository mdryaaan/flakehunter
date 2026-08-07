package github

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"time"

	gh "github.com/google/go-github/v66/github"

	"github.com/mdryaaan/flakehunter/internal/detector"
	"github.com/mdryaaan/flakehunter/internal/utils"
)

const perPage = 100

// ListJobResults walks workflow runs in the window and flattens them to jobs.
//
// Every attempt of every run is needed, not just the latest: a rerun is exactly
// where the flaky signature lives, and the runs endpoint returns only the most
// recent attempt of each run by default.
func (c *Client) ListJobResults(ctx context.Context, opts ListOptions) ([]detector.JobResult, error) {
	if opts.Days <= 0 {
		opts.Days = DefaultListOptions().Days
	}
	if opts.MaxRuns <= 0 {
		opts.MaxRuns = DefaultListOptions().MaxRuns
	}

	since := time.Now().AddDate(0, 0, -opts.Days)
	runs, err := c.listRuns(ctx, opts, since)
	if err != nil {
		return nil, err
	}

	var out []detector.JobResult
	for _, run := range runs {
		attempts := run.GetRunAttempt()
		if attempts < 1 {
			attempts = 1
		}
		for attempt := 1; attempt <= attempts; attempt++ {
			jobs, err := c.listJobsForAttempt(ctx, opts, run.GetID(), attempt)
			if err != nil {
				return nil, err
			}
			out = append(out, jobs...)
		}
	}

	return out, nil
}

func (c *Client) listRuns(ctx context.Context, opts ListOptions, since time.Time) ([]*gh.WorkflowRun, error) {
	var all []*gh.WorkflowRun
	page := 1

	for len(all) < opts.MaxRuns {
		var batch *gh.WorkflowRuns

		err := utils.Do(ctx, utils.DefaultRetry(), func(int) error {
			listOpts := &gh.ListWorkflowRunsOptions{
				Created:     fmt.Sprintf(">=%s", since.Format("2006-01-02")),
				ListOptions: gh.ListOptions{Page: page, PerPage: perPage},
			}

			var resp *gh.Response
			var callErr error
			if opts.WorkflowFile != "" {
				batch, resp, callErr = c.api.Actions.ListWorkflowRunsByFileName(
					ctx, opts.Owner, opts.Repo, opts.WorkflowFile, listOpts)
			} else {
				batch, resp, callErr = c.api.Actions.ListRepositoryWorkflowRuns(
					ctx, opts.Owner, opts.Repo, listOpts)
			}

			if apiErr := classifyFromResponse(resp, callErr); apiErr != nil {
				return apiErr
			}
			return callErr
		})
		if err != nil {
			return nil, fmt.Errorf("listing workflow runs for %s/%s: %w", opts.Owner, opts.Repo, err)
		}
		if batch == nil || len(batch.WorkflowRuns) == 0 {
			break
		}

		all = append(all, batch.WorkflowRuns...)
		if len(batch.WorkflowRuns) < perPage {
			break
		}
		page++
	}

	if len(all) > opts.MaxRuns {
		all = all[:opts.MaxRuns]
	}
	return all, nil
}

func (c *Client) listJobsForAttempt(ctx context.Context, opts ListOptions, runID int64, attempt int) ([]detector.JobResult, error) {
	var out []detector.JobResult
	page := 1

	for {
		var batch *gh.Jobs

		err := utils.Do(ctx, utils.DefaultRetry(), func(int) error {
			var resp *gh.Response
			var callErr error
			batch, resp, callErr = c.api.Actions.ListWorkflowJobsAttempt(
				ctx, opts.Owner, opts.Repo, runID, int64(attempt),
				&gh.ListOptions{Page: page, PerPage: perPage})

			if apiErr := classifyFromResponse(resp, callErr); apiErr != nil {
				return apiErr
			}
			return callErr
		})
		if err != nil {
			return nil, fmt.Errorf("listing jobs for run %d attempt %d: %w", runID, attempt, err)
		}
		if batch == nil || len(batch.Jobs) == 0 {
			break
		}

		for _, job := range batch.Jobs {
			out = append(out, detector.JobResult{
				RunID:        runID,
				RunAttempt:   attempt,
				JobID:        job.GetID(),
				JobName:      job.GetName(),
				WorkflowName: job.GetWorkflowName(),
				WorkflowFile: path.Base(job.GetWorkflowName()),
				HeadSHA:      job.GetHeadSHA(),
				Conclusion:   detector.Conclusion(job.GetConclusion()),
				StartedAt:    job.GetStartedAt().Time,
				URL:          job.GetHTMLURL(),
			})
		}

		if len(batch.Jobs) < perPage {
			break
		}
		page++
	}

	return out, nil
}

// classifyFromResponse converts a go-github response into a retryable error.
func classifyFromResponse(resp *gh.Response, callErr error) error {
	if resp == nil || resp.Response == nil {
		if callErr != nil {
			// No response at all usually means a transport failure, worth a retry.
			return &APIError{StatusCode: 0, Message: callErr.Error(), retryable: true}
		}
		return nil
	}
	if apiErr := classifyResponse(resp.Response); apiErr != nil {
		return apiErr
	}
	return nil
}

var _ = http.StatusOK
