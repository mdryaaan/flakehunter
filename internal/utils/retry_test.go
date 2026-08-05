package utils

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transient struct{ retry bool }

func (t transient) Error() string   { return fmt.Sprintf("transient(retry=%v)", t.retry) }
func (t transient) Retryable() bool { return t.retry }

func fastConfig(attempts int) RetryConfig {
	return RetryConfig{Attempts: attempts, BaseWait: time.Millisecond, MaxWait: 2 * time.Millisecond}
}

func TestDo(t *testing.T) {
	tests := []struct {
		name      string
		attempts  int
		failFor   int
		err       error
		wantCalls int
		wantErr   bool
	}{
		{"succeeds first time", 4, 0, nil, 1, false},
		{"retryable then succeeds", 4, 2, transient{true}, 3, false},
		{"retryable exhausts attempts", 3, 99, transient{true}, 3, true},
		{"permanent error stops immediately", 4, 99, transient{false}, 1, true},
		{"plain error is not retried", 4, 99, errors.New("nope"), 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := Do(context.Background(), fastConfig(tt.attempts), func(int) error {
				calls++
				if calls <= tt.failFor {
					return tt.err
				}
				return nil
			})

			assert.Equal(t, tt.wantCalls, calls)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestDoRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	err := Do(ctx, fastConfig(4), func(int) error {
		calls++
		return transient{true}
	})

	require.Error(t, err)
	assert.Zero(t, calls, "a cancelled context should not run the function at all")
}

func TestDoWrapsFinalError(t *testing.T) {
	err := Do(context.Background(), fastConfig(2), func(int) error {
		return transient{true}
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "gave up after 2 attempts")
}

func TestIsRetryableUnwrapsWrappedErrors(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", transient{true})
	assert.True(t, IsRetryable(wrapped))

	assert.False(t, IsRetryable(fmt.Errorf("outer: %w", errors.New("plain"))))
	assert.False(t, IsRetryable(nil))
}
