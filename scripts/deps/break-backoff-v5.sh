#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$ROOT"

./scripts/deps/snapshot.sh before-backoff-v5

go get github.com/cenkalti/backoff/v5@v5.0.3
cp internal/dependencylab/_templates/retry_v5_broken.go.txt internal/dependencylab/retry.go
gofmt -w internal/dependencylab/retry.go
go mod tidy

printf '\nExpected result: compilation should FAIL because v4 call sites were not remediated.\n\n'
set +e
go test ./internal/dependencylab/...
STATUS=$?
set -e

if [[ $STATUS -eq 0 ]]; then
  echo 'ERROR: expected a compiler failure, but tests passed.'
  exit 1
fi

echo
echo 'GOOD: the deliberate breaking-upgrade failure was reproduced.'
echo 'Next: ./scripts/deps/fix-backoff-v5.sh'
