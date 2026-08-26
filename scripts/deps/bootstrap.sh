#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$ROOT"

printf 'Installing baseline dependency-lab versions locally...\n'
go get github.com/google/uuid@v1.3.1
go get github.com/Masterminds/semver/v3@v3.2.1
go get github.com/cenkalti/backoff/v4@v4.3.0
go get github.com/stretchr/testify@v1.8.4
go mod tidy

printf '\nRunning dependency-lab baseline tests...\n'
go test -count=1 ./internal/dependencylab/...

printf '\nBootstrap PASS. No git commit or push was performed.\n'
