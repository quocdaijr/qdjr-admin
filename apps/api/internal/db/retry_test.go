package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry_SucceedsAfterTransientFailures(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), 5, time.Millisecond, func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("compute waking up")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetry_GivesUpWithLastError(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), 3, time.Millisecond, func(context.Context) error {
		calls++
		return errors.New("still down")
	})
	assert.EqualError(t, err, "after 3 attempts: still down")
	assert.Equal(t, 3, calls)
}

func TestRetry_StopsWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := Retry(ctx, 10, 50*time.Millisecond, func(context.Context) error {
		calls++
		cancel()
		return errors.New("down")
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls)
}
