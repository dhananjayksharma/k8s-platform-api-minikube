#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$ROOT"

cp internal/dependencylab/_templates/retry_v5_fixed.go.txt internal/dependencylab/retry.go
gofmt -w internal/dependencylab/retry.go
go get github.com/cenkalti/backoff/v5@v5.0.3
go mod tidy

./scripts/deps/validate.sh
./scripts/deps/snapshot.sh after-backoff-v5-fixed

printf '\nMigration fixed. Review with:\n'
printf '  git diff -- go.mod go.sum internal/dependencylab\n'
