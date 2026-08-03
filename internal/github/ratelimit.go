// Package github fetches workflow runs and logs, from the live API or from
// local fixtures.
package github

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// APIError carries an HTTP status and whether the call is worth repeating.
type APIError struct {
	StatusCode int
	Message    string
	// RetryAfter is populated from rate-limit headers when the server tells us
	// exactly how long to wait.
	RetryAfter time.Duration
	retryable  bool
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github api: HTTP %d: %s", e.StatusCode, e.Message)
}

// Retryable satisfies utils.Retryable so the backoff helper knows what to do.
func (e *APIError) Retryable() bool { return e.retryable }

// classifyResponse turns an HTTP response into an error, deciding retryability.
//
// The distinction matters: retrying a 404 four times just wastes a maintainer's
// time, while giving up immediately on a 403 secondary rate limit throws away a
// scan that would have succeeded seconds later.
func classifyResponse(resp *http.Response) *APIError {
	if resp == nil {
		return &APIError{StatusCode: 0, Message: "no response", retryable: true}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	err := &APIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}

	switch {
	case resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusTooManyRequests:
		// Primary rate limit exhausts the hourly quota and sets Remaining to 0;
		// a secondary limit is transient. Both are worth waiting on.
		err.retryable = true
		err.RetryAfter = retryAfter(resp)
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining == "0" {
			err.Message = "rate limit exhausted"
		} else {
			err.Message = "secondary rate limit or forbidden"
		}
	case resp.StatusCode >= 500:
		err.retryable = true
	default:
		err.retryable = false
	}

	return err
}

// retryAfter reads the server's own guidance on how long to wait.
func retryAfter(resp *http.Response) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
		if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
			if wait := time.Until(time.Unix(unix, 0)); wait > 0 {
				return wait
			}
		}
	}
	return 0
}
