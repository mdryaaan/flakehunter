package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mdryaaan/flakehunter/internal/utils"
)

// maxLogBytes bounds a single archive download.
const maxLogBytes = 64 << 20

// DownloadJobLog fetches the zip log archive for one job.
//
// The API answers with a redirect to a short-lived blob URL, which go-github
// surfaces rather than following. The redirect target is fetched with a bare
// client on purpose: forwarding the Authorization header to the storage host
// would leak the token to a third party.
func (c *Client) DownloadJobLog(ctx context.Context, _ int64, jobID int64) ([]byte, error) {
	var data []byte

	err := utils.Do(ctx, utils.DefaultRetry(), func(int) error {
		url, resp, err := c.api.Actions.GetWorkflowJobLogs(ctx, c.owner, c.repo, jobID, 10)
		if apiErr := classifyFromResponse(resp, err); apiErr != nil {
			return apiErr
		}
		if err != nil {
			return fmt.Errorf("requesting log url for job %d: %w", jobID, err)
		}
		if url == nil {
			return fmt.Errorf("github returned no log url for job %d", jobID)
		}

		body, err := fetchBlob(ctx, url.String())
		if err != nil {
			return err
		}
		data = body
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("downloading logs for job %d: %w", jobID, err)
	}

	return data, nil
}

func fetchBlob(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building blob request: %w", err)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &APIError{StatusCode: 0, Message: err.Error(), retryable: true}
	}
	defer func() { _ = resp.Body.Close() }()

	if apiErr := classifyResponse(resp); apiErr != nil {
		return nil, apiErr
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLogBytes))
	if err != nil {
		return nil, fmt.Errorf("reading log archive: %w", err)
	}
	return body, nil
}
