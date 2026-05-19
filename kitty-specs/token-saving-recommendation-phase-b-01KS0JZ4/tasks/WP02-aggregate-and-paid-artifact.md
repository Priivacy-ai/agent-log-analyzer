---
work_package_id: WP02
title: Aggregate merge + paid artifact passthrough
dependencies:
- WP01
requirement_refs:
- FR-004
- FR-007
- FR-008
- C-004
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
created_at: '2026-05-19T20:30:00+00:00'
subtasks:
- T008
- T009
- T010
- T011
- T012
agent_profile: implementer-ivan
role: implementer
agent: claude:sonnet:implementer-ivan:implementer
authoritative_surface: internal/remediation/artifact
execution_mode: code_change
owned_files:
- internal/analyzer/aggregate.go
- internal/analyzer/aggregate_test.go
- internal/remediation/artifact.go
- internal/remediation/artifact_test.go
history:
- '2026-05-19': created from mission token-saving-recommendation-phase-b-01KS0JZ4
tags:
- aggregate
- paid-artifact
---

## ⚡ Do This First: Load Agent Profile

```text
/ad-hoc-profile-load implementer-ivan
```

Return here and continue with **Objective** below.

## Objective

Wire `analyzer.AttachRecommendation` into the multi-report aggregate path,
then pass the resulting `RecommendationSet` through the paid plugin
artifact as a new `omitempty` field. Existing `VettedRecommendations`
behavior is **preserved untouched** (C-004).

This WP must NOT touch the engine (`internal/analyzer/token_saving_*.go`),
the helper file (`internal/analyzer/recommendation_wiring.go`), or the
single-report path (`internal/analyzer/analyzer.go`). Those belong to
WP01, which is your dependency.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. Execution
worktree is computed by `lanes.json`; resolve via
`spec-kitty agent context resolve --mission
token-saving-recommendation-phase-b-01KS0JZ4 --wp WP02 --json`.

## Context

Relevant docs:

- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/spec.md` (FR-007, FR-008, AS-06)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/plan.md` (Aggregate merge + Paid artifact sections)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/data-model.md` (PluginArtifact diff)

Read the WP01 implementation to learn the helper signature (it will be on
disk by the time you start this WP because WP01 is your declared
dependency).

## Owned files

- `internal/analyzer/aggregate.go` — append a single call to
  `AttachRecommendation(&merged)` at the bottom of `AggregateReports`.
  Do not refactor existing merge logic.
- `internal/analyzer/aggregate_test.go` — add the aggregate re-run test
  (T009). Existing tests stay green.
- `internal/remediation/artifact.go` — append a `Recommendation` field
  to `PluginArtifact`; populate it from `report.Recommendation` in
  `Generate`. Do not reorder existing fields.
- `internal/remediation/artifact_test.go` — add the passthrough test
  (T012).

## Subtasks

### T008 — Call `AttachRecommendation` on merged Report

**Purpose**: Re-run the engine on merged inputs per FR-008.

**Steps**:

1. Open `internal/analyzer/aggregate.go`.
2. Find the function `AggregateReports`.
3. Locate the point after the merged `Report` is fully constructed
   (after `AggregateEvent` is populated, after all `Ecosystem` fields are
   merged, immediately before `return merged` or equivalent).
4. Insert:
   ```go
   AttachRecommendation(&merged)
   ```
   (Adjust the variable name to whatever the merged report is called in
   that function; do not rename it.)
5. Do not refactor any merge logic. Do not touch any helper used by
   `AggregateReports`.

**Validation**: `grep -n 'AttachRecommendation' internal/analyzer/aggregate.go`
returns exactly one match.

### T009 — Aggregate recommendation re-run test

**Purpose**: Prove the merged Report carries an engine-output
`RecommendationSet` that reflects the union of signals + resolved tool
state.

**Steps**:

1. Open `internal/analyzer/aggregate_test.go`.
2. Add a new test `TestAggregateReportsAttachesRecommendation`.
3. Build 3 synthetic `Report` values with different findings and
   utilization bands. For example:
   - Report A: finding `tool_output_bloat`; MCP band `normal`.
   - Report B: MCP band `severe`; no findings.
   - Report C: WorkflowFingerprint for `ccusage` active.
4. Aggregate them with `AggregateReports`. Capture the merged result.
5. Assert:
   - `merged.Recommendation != nil`.
   - `merged.Recommendation.EngineVersion == EngineVersion()`.
   - `merged.Recommendation.Signals` contains
     `SignalToolOutputBloat` AND `SignalMCPSkillBloat` (deduped, sorted).
   - `merged.Recommendation.Signals` does NOT contain
     `SignalNoUsageVisibility` (because Report C provides active
     `ccusage`).

**Validation**: `go test -run TestAggregateReportsAttachesRecommendation
./internal/analyzer/...` passes.

### T010 [P] — Add `Recommendation` field to `PluginArtifact`

**Purpose**: Extend the paid artifact JSON envelope.

**Steps**:

1. Open `internal/remediation/artifact.go`.
2. Find the `PluginArtifact` struct definition.
3. **Append** (after `VettedRecommendations`):
   ```go
   Recommendation *analyzer.RecommendationSet `json:"recommendation,omitempty"`
   ```
4. Do not reorder existing fields. Do not modify `VettedRecommendations`
   or any other field.

**Validation**: `go vet ./internal/remediation/...` passes.

This subtask is `[P]` because it can run in parallel with T008/T009 from
a code-edit standpoint (different file, different package). The
dependency graph still requires WP01 to be done first because
`*analyzer.RecommendationSet` must exist.

### T011 — Populate `PluginArtifact.Recommendation`

**Purpose**: Wire the carry-through.

**Steps**:

1. In `internal/remediation/artifact.go`, find the `Generate` function
   (or whatever entry point produces a `PluginArtifact`).
2. After the existing `PluginArtifact` is constructed, set:
   ```go
   artifact.Recommendation = report.Recommendation
   ```
   (Do **not** copy the struct. The pointer assignment is the contract.)
3. The `VettedRecommendations` slice continues to be populated by
   `toolingRecommendations(report)` exactly as before.

**Validation**: `go build ./internal/remediation/...` succeeds.

### T012 — Paid-artifact passthrough test

**Purpose**: Lock in the passthrough so a future refactor cannot silently
drop it.

**Steps**:

1. Open `internal/remediation/artifact_test.go`.
2. Add `TestGenerateAttachesRecommendation`.
3. Build a synthetic `Report` whose `Recommendation` is a non-nil
   `RecommendationSet` with a fabricated Primary value.
4. Call `Generate` (or whatever the paid artifact factory is named).
5. Marshal the returned `PluginArtifact` to JSON.
6. Assert:
   - The JSON contains the key `"recommendation"`.
   - Unmarshalling the JSON back to `PluginArtifact` yields a value
     whose `Recommendation.Primary != nil` and whose `Primary.PrimaryToolID`
     matches the input.
   - The existing `VettedRecommendations` slice in the JSON is non-empty
     iff the input report had vetted recommendations (regression-guard
     for C-004).
7. Add a second test case where `report.Recommendation == nil`: the
   resulting `PluginArtifact` JSON does NOT contain the `recommendation`
   key (due to `omitempty`).

**Validation**: `go test -run TestGenerateAttachesRecommendation
./internal/remediation/...` passes.

## Test strategy

- Use table-driven cases where possible (existing pattern in
  `aggregate_test.go` and `artifact_test.go`).
- Verify both branches of `omitempty` (field present vs absent).
- Do not mutate engine internals or the WP01 helper from these tests.

## Definition of Done

- [ ] `AggregateReports` calls `AttachRecommendation` exactly once on the
      merged Report.
- [ ] `PluginArtifact` has a new `Recommendation *analyzer.RecommendationSet`
      field with `omitempty`.
- [ ] `Generate` passes `report.Recommendation` through verbatim.
- [ ] Existing `VettedRecommendations` behavior is unchanged (regression
      tests still pass).
- [ ] `go test ./...` passes (full repo).
- [ ] `go vet ./...` passes.
- [ ] `gofmt -l internal/analyzer internal/remediation` returns empty.
- [ ] No file outside `owned_files` is modified.
- [ ] No code under `internal/analyzer/token_saving_*.go` or
      `internal/analyzer/recommendation_wiring.go` or
      `internal/analyzer/analyzer.go` is modified by this WP.

## Risks

- **Reordering `PluginArtifact` fields** breaks paid-artifact JSON
  ordering for downstream consumers. Append only.
- **Pointer aliasing** between `Report.Recommendation` and
  `PluginArtifact.Recommendation` — this is intentional (no copy), but
  reviewers must check that downstream paths do not mutate
  `*RecommendationSet` after artifact generation.
- **Test interdependencies** — the aggregate test consumes the WP01
  helper; if WP01 isn't merged, the test will fail to compile. This is
  the dependency contract.

## Reviewer Guidance

1. Confirm exactly one call to `AttachRecommendation` in
   `aggregate.go`, at the bottom of `AggregateReports`.
2. Confirm `PluginArtifact.Recommendation` is the **last** field of the
   struct (or at least appended; not reordered).
3. Confirm the passthrough is a pointer assignment, not a deep copy.
4. Confirm `omitempty` is present on the new JSON tag.
5. Confirm existing `VettedRecommendations` behavior is preserved.

## Working Directory and Hand-off

```bash
gofmt -w internal/analyzer internal/remediation
go vet ./internal/analyzer/... ./internal/remediation/...
go test ./...
```

All must exit 0. Then commit:

```bash
git add internal/analyzer/aggregate.go \
        internal/analyzer/aggregate_test.go \
        internal/remediation/artifact.go \
        internal/remediation/artifact_test.go
git commit -m "feat(WP02): wire AttachRecommendation through aggregate and paid artifact"
```

Mark subtasks done and move to `for_review`:

```bash
spec-kitty agent tasks mark-status T008 T009 T010 T011 T012 --status done --mission token-saving-recommendation-phase-b-01KS0JZ4
spec-kitty agent tasks move-task WP02 --to for_review --note "Ready for review" --mission token-saving-recommendation-phase-b-01KS0JZ4
```
