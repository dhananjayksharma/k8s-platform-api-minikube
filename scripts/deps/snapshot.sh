#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$ROOT"

NAME=${1:-$(date +%Y%m%d-%H%M%S)}
OUT=".dependency-snapshots/$NAME"
mkdir -p "$OUT"

printf 'Capturing dependency evidence in %s\n' "$OUT"
go version > "$OUT/go-version.txt"
go env GOOS GOARCH GOPATH GOPROXY GOMOD > "$OUT/go-env.txt"
go list -m all > "$OUT/modules.txt"
go mod graph > "$OUT/module-graph.txt"
go list -deps ./... > "$OUT/package-deps.txt"
cp go.mod "$OUT/go.mod"
cp go.sum "$OUT/go.sum"

printf '\nDirect module requirements:\n'
go list -m -f '{{if not .Indirect}}{{.Path}} {{.Version}}{{end}}' all | sed '/^ $/d'
printf '\nSnapshot complete: %s\n' "$OUT"
