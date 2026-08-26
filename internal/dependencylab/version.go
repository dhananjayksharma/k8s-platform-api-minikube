package dependencylab

import (
	"fmt"

	semver "github.com/Masterminds/semver/v3"
)

// ClassifyUpgrade reports the semantic-version impact of moving current -> target.
// It is intentionally small, but useful for package-assessment interview exercises.
func ClassifyUpgrade(current, target string) (string, error) {
	from, err := semver.NewVersion(current)
	if err != nil {
		return "", fmt.Errorf("parse current version %q: %w", current, err)
	}
	to, err := semver.NewVersion(target)
	if err != nil {
		return "", fmt.Errorf("parse target version %q: %w", target, err)
	}

	switch {
	case to.Equal(from):
		return "same", nil
	case to.LessThan(from):
		return "downgrade", nil
	case to.Major() != from.Major():
		return "major", nil
	case to.Minor() != from.Minor():
		return "minor", nil
	default:
		return "patch", nil
	}
}
