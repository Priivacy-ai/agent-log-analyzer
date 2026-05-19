# Implementation Plan: Launch Correctness Fixes

**Branch**: `codex/launch-correctness` (created later, during implement)
**Date**: 2026-05-19
**Spec**: [/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/spec.md](./spec.md)
**Input**: Phase 1 of the launch-completion handoff (`../start-here.md`) — GitHub issues [#74](https://github.com/robertDouglass/claude-log-analyzer/issues/74), [#70](https://github.com/robertDouglass/claude-log-analyzer/issues/70), [#72](https://github.com/robertDouglass/claude-log-analyzer/issues/72).

## Summary

Three bug fixes in existing modules with no new external surface:

1. `cmd/claude-analyzer/main.go` — accept exactly one positional log path as an alias for `--log`; refuse on conflict or multiplicity.
2. `internal/analyzer/tooling_detect.go` — record header byte ranges in `mcpExposure` (and `skillExposure`) and mask them from the raw-byte rescan inside `detectMCPCallsFromToolUse` so exposure-header tokens never count as calls.
3. `internal/analyzer/aggregate.go` (`mergeEcosystems`) — merge `Ecosystem.ToolingUtilization` and `Ecosystem.WorkflowFingerprints` across input reports using the deterministic semantics defined in FR-007 and FR-008.

Privacy semantics, golden fixtures, and the verification baseline already exist; this mission extends them rather than introducing new infrastructure.

## Technical Context

**Language/Version**: Go 1.22+ (see `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/go.mod`).
**Primary Dependencies**: Go stdlib only on the hot path (`flag`, `regexp`, `encoding/json`, `os`, `bytes`). No new third-party deps introduced by this mission.
**Storage**: None added. Reads Claude Code JSONL logs from local disk; writes sanitized report JSON to local disk. The paid scan path persists to existing AWS S3 / DynamoDB via `internal/awsstore` — unchanged by this mission.
**Testing**:
  - `go test ./...` runs the full Go suite (analyzer, remediation, backend, awsstore, paidscan, CLI integration).
  - Golden fixtures: `internal/analyzer/testdata/tooling/{00..07}-*.log`; `testdata/golden/sample-report.json`.
  - Privacy canary: `internal/analyzer/leak_test.go`.
  - Detector tests: `internal/analyzer/tooling_detect_test.go`.
  - Aggregate tests: extend whatever exists alongside `aggregate.go` (see research.md).
  - CLI tests: **none exist yet** for `cmd/claude-analyzer/main.go`; this mission adds `cmd/claude-analyzer/main_test.go` for FR-001..FR-004.
  - End-to-end smoke: `./scripts/smoke-local.sh` (Docker compose), `./scripts/load-local.sh 25` (load gate).
**Target Platform**: Linux/macOS developer machines (CLI), Linux containers on AWS Fargate (API/worker). No platform-specific code added.
**Project Type**: Single Go project with multiple `cmd/` binaries (`claude-analyzer`, `api`, `worker`, `local-log-smoke`) and shared `internal/` packages; static `web/` frontend; Terraform `infra/aws/`. No web/mobile split.
**Performance Goals**:
  - Analyze single log: per NFR-004, ≤ 5% wall-clock increase versus current CLI integration test baseline.
  - Aggregate merge: per NFR-005, < 5 seconds for a 100-input paid scan composed of the largest bundled fixtures, on a developer laptop equivalent to the GitHub Actions runner.
**Constraints**:
  - Privacy stance (charter): no private name, raw path, raw URL, transcript fragment, or stable hash of a private string in any new field, log line, test output, or merged artifact.
  - Bounded-cardinality (charter, C-001): every new field must fit allowlisted IDs / closed enums / bounded buckets / numeric counts.
  - Single PR on branch `codex/launch-correctness` merging to `main` (C-003).
  - No `terraform apply` (C-005).
  - C-006 no-op stability: fixtures with zero exposure-header tokens must produce byte-identical `Ecosystem.ToolingUtilization` after the #70 fix.
**Scale/Scope**:
  - Code change footprint: ~5 Go files modified, ~3 new test files added, ~2 new golden fixtures, ~3 doc lines updated.
  - LOC delta estimate: 300–600 lines added, ~50 modified.
  - Test count delta: ~15–25 new test cases across CLI, detector, and aggregate.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Source: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/.kittify/charter/charter.md` (governance.yaml, interview/answers.yaml).

| Gate | Verdict | Notes |
|------|---------|-------|
| **Verification baseline (NFR-001)** | PASS — will run before PR. | `gofmt -w`, `go test ./...`, `go vet ./...`, `terraform -chdir=infra/aws fmt -check -recursive`, `./scripts/smoke-local.sh`. The mission introduces no new gates; it must keep all five passing. |
| **Privacy stance (charter risk_boundaries; NFR-002, C-002)** | PASS by construction. | No new field added by this mission stores or exposes a private MCP/skill/plugin name, raw path, raw URL, hash of a private string, command argument, or transcript fragment. Header byte ranges are integer offsets only. Aggregate merge unions allowlisted IDs and sums counts; unknown names remain counts only. |
| **Bounded-cardinality upload schema (charter quality_gates; C-001)** | PASS. | New fields proposed (`HeaderRanges []byteRange` on `mcpExposure`/`skillExposure`) are **in-memory only**, never serialized to the sanitized report, aggregate event, or paid artifact. Aggregate-merge additions reuse existing serialized field shapes. |
| **PR sequencing (charter review_policy)** | PASS. | This mission delivers exactly the PR1 of the charter-recommended sequence (`codex/launch-correctness`). |
| **GitHub issue hygiene (FR-010, C-004)** | PASS (deferred to implement). | Plan documents the comment-on-start and comment-on-ready obligations for issues #74, #70, #72; agent profile during `/spec-kitty.implement` will execute them. |
| **No casual `terraform apply` (C-005)** | PASS. | Mission touches no Terraform; if a follow-up change is needed, only `plan` output is reviewed in the PR. |
| **UI verification rule (charter exception_policy)** | N/A. | This mission has no UI surface. The web report UX work lives in Phase 2 / a different mission. |
| **Amendment / exception policy (strict)** | PASS. | No charter rule is being relaxed. C-007 records the `evidence_count = sum` choice that the charter brief explicitly authorized either way. |

**Re-check after Phase 1**: see end of `data-model.md` and `contracts/`.

## Project Structure

### Documentation (this feature)

```
/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/
├── plan.md                       # This file
├── spec.md                       # Approved spec (FRs/NFRs/Constraints)
├── meta.json                     # Mission identity
├── research.md                   # Phase 0 output — grounded findings + decisions
├── data-model.md                 # Phase 1 output — struct shape changes
├── quickstart.md                 # Phase 1 output — developer verification path
├── contracts/                    # Phase 1 output
│   ├── cli-analyze.md            # CLI argument contract (FR-001..FR-004)
│   ├── mcp-call-counting.md      # MCP/skill call counting invariant (FR-005, FR-006)
│   └── aggregate-merge.md        # Paid aggregate merge contract (FR-007..FR-009)
├── checklists/
│   └── requirements.md           # Spec quality checklist (PASS on first iteration)
└── tasks/                        # Filled by /spec-kitty.tasks — NOT this command
```

### Source Code (repository root)

```
/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/
├── cmd/
│   └── claude-analyzer/
│       ├── main.go                            # MODIFIED — positional arg handling (#74)
│       └── main_test.go                       # NEW — FR-001..FR-004 CLI integration tests
├── internal/
│   ├── analyzer/
│   │   ├── tooling_detect.go                  # MODIFIED — header byte ranges + masked rescan (#70)
│   │   ├── tooling_detect_test.go             # MODIFIED — header-only-no-calls case
│   │   ├── aggregate.go                       # MODIFIED — mergeEcosystems covers ToolingUtilization + WorkflowFingerprints (#72)
│   │   ├── aggregate_test.go                  # MODIFIED — multi-report merge cases
│   │   ├── types.go                           # UNCHANGED — existing struct shape is sufficient
│   │   ├── leak_test.go                       # MODIFIED — extend canary across aggregate output
│   │   ├── golden_test.go                     # MODIFIED — preserve fingerprints/utilization in merged golden
│   │   └── testdata/
│   │       └── tooling/
│   │           ├── 08-header-only-zero-calls.log    # NEW — fixture for FR-006
│   │           └── (existing 00..07 fixtures)       # UNCHANGED (C-006)
│   └── remediation/
│       ├── artifact.go                        # UNCHANGED OR MINOR — readers consume merged data
│       └── artifact_test.go                   # MODIFIED — aggregate-aware artifact case
├── README.md                                   # MODIFIED — document positional CLI form
├── docs/testing-plan.md                        # MODIFIED — document new fixture + test additions
└── web/
    └── app.js                                  # MODIFIED if command-generator copy advertises --log only
```

**Structure Decision**: single-project Go layout. No new packages introduced. All three fixes land in their existing modules. No reorganization of package boundaries — that would violate `locality-of-change` (tactic).

## Complexity Tracking

No Charter Check violations. Section intentionally empty.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| _none_ | — | — |
