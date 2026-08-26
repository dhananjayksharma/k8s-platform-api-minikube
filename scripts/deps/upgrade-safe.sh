#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$ROOT"

./scripts/deps/snapshot.sh before-safe-upgrade

printf 'Upgrading non-breaking lab dependencies...\n'
go get github.com/google/uuid@v1.6.0
go get github.com/Masterminds/semver/v3@v3.3.1
go get github.com/stretchr/testify@v1.11.1
go mod tidy

./scripts/deps/snapshot.sh after-safe-upgrade

printf '\nModule version diff:\n'
diff -u .dependency-snapshots/before-safe-upgrade/modules.txt \
        .dependency-snapshots/after-safe-upgrade/modules.txt || true

./scripts/deps/validate.sh
