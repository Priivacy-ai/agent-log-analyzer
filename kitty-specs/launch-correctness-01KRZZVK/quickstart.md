# Quickstart: Verify Launch Correctness Fixes Locally

This guide walks a developer through manually verifying each of the three fixes shipped by mission `launch-correctness-01KRZZVK`. It mirrors the per-FR acceptance criteria in `spec.md` and the contracts in `contracts/`.

Run all commands from the repository root: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer`.

## Prerequisites

- Go 1.22+ (`go version` should report ≥ 1.22).
- Docker Desktop running (for the smoke test).
- Clean working tree (`git status` clean — apart from the mission branch).
- The implementation branch checked out (`codex/launch-correctness` after `/spec-kitty.implement` lands).

## Step 1 — Verification baseline (NFR-001)

This is the same baseline the charter requires before any PR:

```bash
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
terraform -chdir=infra/aws fmt -check -recursive
./scripts/smoke-local.sh
```

Expected:
- `go test ./...` reports zero failures across all packages (analyzer, remediation, backend, awsstore, paidscan, the new `cmd/claude-analyzer` test file).
- `go vet ./...` clean.
- `terraform fmt -check` exits 0.
- `./scripts/smoke-local.sh` exits 0 (Docker compose smoke completes successfully).

If any command fails, do **not** proceed to the per-bug verification below.

## Step 2 — Verify Bug #74 (CLI positional log path)

### 2.1 Positive case (FR-001)

Prepare a known log fixture (any small JSONL file works; the bundled `internal/analyzer/testdata/sample.jsonl` will do):

```bash
SAMPLE_LOG="$(pwd)/internal/analyzer/testdata/sample.jsonl"
go run ./cmd/claude-analyzer analyze "$SAMPLE_LOG" --out /tmp/positional-report.json
```

Expected:
- Exit code `0`.
- `/tmp/positional-report.json` exists.
- `jq '.input.file' /tmp/positional-report.json` returns the value of `$SAMPLE_LOG`.

### 2.2 Conflict case (FR-002)

```bash
go run ./cmd/claude-analyzer analyze "$SAMPLE_LOG" --log "$SAMPLE_LOG" --out /tmp/should-not-exist.json
echo "exit=$?"
```

Expected:
- Exit code non-zero.
- Stderr contains the substring `cannot combine positional log path with --log`.
- `/tmp/should-not-exist.json` is NOT created.

### 2.3 Multiplicity case (FR-003)

```bash
go run ./cmd/claude-analyzer analyze "$SAMPLE_LOG" "$SAMPLE_LOG" --out /tmp/should-not-exist.json
echo "exit=$?"
```

Expected:
- Exit code non-zero.
- Stderr contains the substring `unexpected extra argument`.
- `/tmp/should-not-exist.json` is NOT created.

### 2.4 Docs (FR-004)

Grep that the positional form is documented:

```bash
grep -n "claude-analyzer analyze" README.md docs/testing-plan.md
grep -n "analyze" web/app.js
```

Expected: at least one match in each location includes the positional form (or an inline comment explaining it).

## Step 3 — Verify Bug #70 (MCP exposure-header double-counting)

### 3.1 Header-only fixture (FR-006)

The new fixture `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log` contains many `mcp__server__tool` tokens inside an exposure header and zero actual tool-use records.

```bash
go test ./internal/analyzer/ -run TestDetectMCPExposure -v
```

Expected output includes the new fixture case `08-header-only-zero-calls` and reports `CallCount == 0` and `KnownCallCount == 0`.

### 3.2 No-op stability on header-free fixtures (C-006, NFR-003)

```bash
go test ./internal/analyzer/ -run TestGolden -v
```

Expected:
- Fixtures `00..06` produce byte-identical `ToolingUtilization` to the snapshot in `testdata/golden/`.
- Fixture `07-mixed-known-unknown.log` may show a **decrease** in `CallCount` (the fix removing prior double-counts) — accompanied by an updated golden snapshot. The decrease is the fix; an increase would be a regression.

## Step 4 — Verify Bug #72 (paid aggregate merge)

### 4.1 Aggregate merge unit tests (FR-007, FR-008)

```bash
go test ./internal/analyzer/ -run TestAggregateReports -v
```

Expected cases pass:
- Multi-report merge preserves `WorkflowFingerprints` by ID with summed evidence_count, max-rank confidence, OR'd active/installed.
- Multi-report merge preserves `MCPUtilization` with summed counts, unioned known IDs, max-rank warning band.
- Multi-report merge preserves `SkillUtilization` analogously.
- Associativity invariant: `merge(merge(A,B), C) == merge(A, merge(B,C))`.
- Commutativity invariant: `merge(A,B) == merge(B,A)`.

### 4.2 Privacy canary across merge (NFR-002)

```bash
go test ./internal/analyzer/ -run TestLeak -v
```

Expected:
- The canary asserts that after merging two reports containing synthetic private names in their raw inputs, the serialized merged Ecosystem JSON contains zero leak strings.
- The same assertion applies to the generated paid artifact and to the aggregate event payload.

### 4.3 Generated paid artifact consumes merged data (FR-009)

```bash
go test ./internal/remediation/ -run TestArtifact -v
```

Expected:
- A test case constructs a multi-input paid scan, runs `Generate()`, and asserts the artifact reflects the merged `ToolingUtilization` (not the pre-merge values from any single input).

### 4.4 Performance ceiling (NFR-005)

```bash
go test ./internal/analyzer/ -run TestAggregateReports100 -v -timeout 30s
```

Expected:
- The test composes 100 input reports from the largest bundled fixtures and asserts `mergeEcosystems` completes in under 5 seconds wall time on the runner.

## Step 5 — End-to-end smoke

Re-run the full verification baseline once more after the per-bug steps:

```bash
go test ./...
./scripts/smoke-local.sh
./scripts/load-local.sh 25
```

Expected: all three pass.

## Step 6 — Document the verification result

The PR description must list:

- Which steps above the developer ran locally.
- The commit SHAs verified against.
- Any deviations (none expected; if any exist, the PR must explain and the user must explicitly approve per the charter strict exception policy).

GitHub issue comments (FR-010, C-002):

- On PR open: post a "starting work" comment on issues #74, #70, #72.
- On PR ready-for-review: post a "ready for review" comment on the same issues, naming files changed and the steps above that passed.
- On merge: comment with the merge SHA and close the issues only if their per-issue acceptance criteria are satisfied (C-004).
