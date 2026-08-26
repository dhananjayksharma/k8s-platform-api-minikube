package dependencylab

import (
	"context"
	"errors"
	"time"

	backoff "github.com/cenkalti/backoff/v5"
)

// Retry is the remediated implementation for backoff/v5. v5 changed Retry to
// accept context, a generic operation, and functional retry options.
func Retry(attempts uint64, op func() error) error {
	if attempts == 0 {
		return errors.New("attempts must be greater than zero")
	}
	if op == nil {
		return errors.New("operation must not be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := backoff.Retry(
		ctx,
		func() (struct{}, error) {
			return struct{}{}, op()
		},
		backoff.WithBackOff(backoff.NewConstantBackOff(time.Millisecond)),
		backoff.WithMaxTries(uint(attempts)),
	)
	return err
}
