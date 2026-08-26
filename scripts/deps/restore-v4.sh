#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$ROOT"

cp internal/dependencylab/_templates/retry_v4.go.txt internal/dependencylab/retry.go
gofmt -w internal/dependencylab/retry.go
go get github.com/cenkalti/backoff/v4@v4.3.0
go mod tidy

./scripts/deps/validate.sh
printf '\nRestored backoff/v4 baseline. Unused v5 should disappear after tidy.\n'
