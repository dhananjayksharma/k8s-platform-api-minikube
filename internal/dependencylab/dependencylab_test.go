package dependencylab

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAndParseJobID(t *testing.T) {
	t.Parallel()

	id := NewJobID("reconcile")
	parsed, err := ParseJobID("reconcile", id)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, parsed)
	assert.Contains(t, id, "reconcile-")
}

func TestParseJobIDRejectsWrongPrefix(t *testing.T) {
	t.Parallel()

	_, err := ParseJobID("payment", NewJobID("order"))
	require.Error(t, err)
}

func TestClassifyUpgrade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		target  string
		want    string
	}{
		{name: "patch", current: "1.2.3", target: "1.2.4", want: "patch"},
		{name: "minor", current: "1.2.3", target: "1.3.0", want: "minor"},
		{name: "major", current: "1.2.3", target: "2.0.0", want: "major"},
		{name: "same", current: "1.2.3", target: "1.2.3", want: "same"},
		{name: "downgrade", current: "2.0.0", target: "1.9.9", want: "downgrade"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ClassifyUpgrade(tc.current, tc.target)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRetryEventuallySucceeds(t *testing.T) {
	t.Parallel()

	calls := 0
	err := Retry(3, func() error {
		calls++
		if calls < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetryStopsAtConfiguredAttempts(t *testing.T) {
	t.Parallel()

	calls := 0
	err := Retry(2, func() error {
		calls++
		return errors.New("still failing")
	})

	require.Error(t, err)
	assert.Equal(t, 2, calls)
}

func TestRetryRejectsZeroAttempts(t *testing.T) {
	t.Parallel()

	err := Retry(0, func() error { return nil })
	require.Error(t, err)
}
