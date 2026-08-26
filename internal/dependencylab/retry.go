package dependencylab

import (
	"errors"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
)

// Retry runs op at most attempts times using backoff/v4.
// This file is the baseline for the deliberate v4 -> v5 breaking-upgrade lab.
func Retry(attempts uint64, op func() error) error {
	if attempts == 0 {
		return errors.New("attempts must be greater than zero")
	}
	if op == nil {
		return errors.New("operation must not be nil")
	}

	policy := backoff.WithMaxRetries(
		backoff.NewConstantBackOff(time.Millisecond),
		attempts-1,
	)
	return backoff.Retry(op, policy)
}
