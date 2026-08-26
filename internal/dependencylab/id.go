package dependencylab

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// NewJobID creates a stable application-facing ID while hiding the OSS UUID API
// behind this package. This gives the dependency-upgrade lab a real direct caller.
func NewJobID(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "job"
	}
	return prefix + "-" + uuid.New().String()
}

// ParseJobID validates and extracts the UUID portion of an ID created by NewJobID.
func ParseJobID(prefix, id string) (uuid.UUID, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "job"
	}

	marker := prefix + "-"
	if !strings.HasPrefix(id, marker) {
		return uuid.Nil, fmt.Errorf("id %q does not have expected prefix %q", id, prefix)
	}

	parsed, err := uuid.Parse(strings.TrimPrefix(id, marker))
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse uuid from %q: %w", id, err)
	}
	return parsed, nil
}
