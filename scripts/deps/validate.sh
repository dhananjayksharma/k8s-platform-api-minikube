#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$ROOT"

printf '== go test: dependency lab ==\n'
go test -count=1 ./internal/dependencylab/...

printf '\n== race test: dependency lab ==\n'
go test -race -count=1 ./internal/dependencylab/...

printf '\n== full existing tests ==\n'
go test -count=1 ./...

printf '\n== go vet ==\n'
go vet ./...

printf '\n== application build ==\n'
go build ./cmd/api

printf '\nValidation PASS\n'
