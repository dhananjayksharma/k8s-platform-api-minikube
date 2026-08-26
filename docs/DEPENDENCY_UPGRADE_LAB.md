# Go OSS Dependency Upgrade Lab

This lab is designed for the `k8s-platform-api-minikube` POC and stays local. It does not commit or push anything.

## What it adds

| Dependency | Baseline | Upgrade exercise | Purpose |
|---|---:|---:|---|
| `github.com/google/uuid` | v1.3.1 | v1.6.0 | direct dependency / safe version bump |
| `github.com/Masterminds/semver/v3` | v3.2.1 | v3.3.1 | semantic-version assessment |
| `github.com/cenkalti/backoff/v4` | v4.3.0 | `/v5` v5.0.3 | intentional breaking major-version API migration |
| `github.com/stretchr/testify` | v1.8.4 | v1.11.1 | test-only dependency and transitive changes |

The project already has `github.com/jackc/pgx/v5`; keep it as the real database dependency and use it when discussing direct/transitive dependencies.

## 1. Bootstrap the local lab

This full local project already contains the dependency-lab files. From the repo root, install the intentionally older baseline dependency versions:

```bash
make -f Makefile.dependencylab deps-bootstrap
```

This updates only your local `go.mod`/`go.sum` as required by Go module resolution and runs the baseline dependency-lab test. It does not run `git commit` or `git push`.

## 2. Inspect the baseline

```bash
make -f Makefile.dependencylab deps-test
make -f Makefile.dependencylab deps-snapshot
make -f Makefile.dependencylab deps-status

go list -m all
go mod graph
go mod why -m github.com/google/uuid
go mod why -m github.com/cenkalti/backoff/v4
git diff -- go.mod go.sum
```

Interview explanation:

```text
Application
  ├─ pgx/v5                    existing DB dependency
  ├─ google/uuid               new direct lab dependency
  ├─ Masterminds/semver/v3     new direct lab dependency
  ├─ cenkalti/backoff/v4       new direct lab dependency
  └─ testify                   imported by tests
       └─ additional transitives
```

## 3. Exercise A — safe dependency upgrades

```bash
make -f Makefile.dependencylab deps-upgrade-safe
```

The script snapshots the module graph before and after, upgrades UUID, semver and testify, runs `go mod tidy`, and executes validation.

Review:

```bash
git diff -- go.mod go.sum internal/dependencylab

diff -u \
  .dependency-snapshots/before-safe-upgrade/modules.txt \
  .dependency-snapshots/after-safe-upgrade/modules.txt
```

Questions to answer:

- Which modules changed directly?
- Which modules changed transitively?
- Did `go.sum` change more than `go.mod`?
- Were source changes required?
- Did existing tests still pass?

## 4. Exercise B — reproduce a breaking v4 -> v5 upgrade

```bash
make -f Makefile.dependencylab deps-break-v5
```

This intentionally changes the import path from `backoff/v4` to `backoff/v5` while leaving the old API calls in place. The compiler failure is expected.

Typical reasoning:

```text
Major module path changed
        ↓
source import changed /v4 -> /v5
        ↓
old caller still uses v4 API
        ↓
compiler identifies incompatible symbols/signatures
        ↓
remediate directly affected caller
```

Do not randomly fix every error. Identify the first root API incompatibility and inspect the target library API.

## 5. Exercise C — remediate the breaking API

```bash
make -f Makefile.dependencylab deps-fix-v5
```

The repaired implementation demonstrates that v5 uses a context, a generic operation and functional retry options rather than the v4 `Retry(operation, BackOff)` call pattern.

Then inspect:

```bash
git diff -- go.mod go.sum internal/dependencylab/retry.go
go mod why -m github.com/cenkalti/backoff/v5
go mod graph | grep -E 'backoff|testify|uuid|semver'
```

## 6. Restore baseline v4

```bash
make -f Makefile.dependencylab deps-restore-v4
```

After `go mod tidy`, unused `/v5` should disappear from the module graph.

## 7. Full validation

```bash
make -f Makefile.dependencylab deps-validate
```

This executes:

```text
go test dependency lab
        ↓
go test -race dependency lab
        ↓
go test ./...
        ↓
go vet ./...
        ↓
go build ./cmd/api
```

This maps closely to an enterprise upgrade path: build + existing tests + broader validation.

## 8. Validate the upgraded code in Minikube

Once the dependency migration is in a PASS state:

```bash
make image
make deploy
kubectl get pods -n platform-demo
kubectl logs -n platform-demo -l app=k8s-platform-api-minikube --tail=100
```

In another terminal:

```bash
make port-forward
```

Then:

```bash
curl localhost:8080/healthz
curl localhost:8080/readyz
curl -H 'X-Client-ID: dependency-lab' \
  localhost:8080/api/v1/metadata/cluster
```

The point is to prove that a dependency change which passes package tests also still builds into the real application image and starts in the target runtime.

## 9. Optional vulnerability tool

Install locally if desired:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

Use this as an assessment signal, not as a reason to blindly jump to the newest major version.

## 10. Migration report practice

After each exercise create evidence like:

```text
Package: github.com/cenkalti/backoff
Old: /v4 v4.3.0
New: /v5 v5.0.3
Reason: major-version migration exercise
Breaking change: Retry API changed
Affected caller: internal/dependencylab/retry.go
Remediation: context + generic operation + functional options
Existing tests: PASS after remediation
Race test: PASS
Full go test: PASS
Go vet: PASS
Application build: PASS
Patch disposition: N/A
Exceptions: none
```

## 11. Useful interview commands

```bash
go list -m all
go list -m -u all
go mod graph
go mod why -m MODULE
go list -deps ./...
go mod tidy
go test ./...
go test -race ./...
go vet ./...
git diff -- go.mod go.sum
```

## 12. Interview story

A concise answer:

> I first snapshot the dependency graph and identify direct callers. I make the smallest version change, inspect `go.mod` and `go.sum`, and use compiler failures to identify API incompatibilities. I remediate only directly affected callers, run existing tests and race checks, then validate the full application build. For a major module migration I separately track the module-path change, source API remediation, transitive dependency movement and final validation evidence.
