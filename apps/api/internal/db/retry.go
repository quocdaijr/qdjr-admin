package db

import (
	"context"
	"fmt"
	"time"
)

// Retry calls fn up to attempts times, doubling the wait from base between
// tries. Neon suspends idle compute and wakes it on the first connection, so
// the first ping or migration after a quiet period can fail transiently.
func Retry(ctx context.Context, attempts int, base time.Duration, fn func(context.Context) error) error {
	var err error
	wait := base
	for i := 1; i <= attempts; i++ {
		if err = fn(ctx); err == nil {
			return nil
		}
		if i == attempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		wait *= 2
	}
	return fmt.Errorf("after %d attempts: %w", attempts, err)
}
