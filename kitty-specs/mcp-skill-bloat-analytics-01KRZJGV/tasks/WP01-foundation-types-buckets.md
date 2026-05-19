---
work_package_id: WP01
title: 'Foundation: Types & Bucketing Helpers'
dependencies: []
requirement_refs:
- FR-008
- FR-010
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T001
- T002
- T003
- T004
agent: claude
history:
- event: generated
  at: '2026-05-19T08:00:33Z'
  by: /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/tooling_buckets.go
execution_mode: code_change
mission_id: 01KRZJGVG3MCCCY9MKB1YRDBQR
mission_slug: mcp-skill-bloat-analytics-01KRZJGV
owned_files:
- internal/analyzer/types.go
- internal/analyzer/tooling_buckets.go
- internal/analyzer/tooling_buckets_test.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load the agent profile that governs this work package by invoking the `/ad-hoc-profile-load` skill (or equivalent in your runtime) with `profile_id: "implementer-ivan"` and `role: "implementer"`. The profile sets your identity, governance scope, and boundaries for this WP. Then return to this file and continue.

## Objective

Lay the type and helper foundations for the rest of the mission. Add the new `ToolingUtilization`/`MCPUtilization`/`SkillUtilization` types to `internal/analyzer/types.go`, append `ToolingUtilization` to `Ecosystem`, create `internal/analyzer/tooling_buckets.go` with the closed-enumeration bucketing helpers, and prove them with table-driven unit tests in `internal/analyzer/tooling_buckets_test.go`.

After this WP merges, every later WP (detection, classification, wiring, fixtures) can import these types and helpers without needing to change them.

## Context

Read first:
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/spec.md` — the **Functional Requirements** table (FR-001, FR-008, FR-010), **Constraints** (especially C-003 bucket enumerations, C-004 backward compat), and **Domain Language** section.
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md` — §D-1 (placement nested in Ecosystem), §"Technical Context" (Go 1.25, stdlib-only).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md` — full type definitions, closed enums, invariants I-1..I-6, bucketing pseudocode.
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json` — the JSON Schema you must satisfy.
- `internal/analyzer/types.go` — the existing `Ecosystem` struct. Note that **every existing field must stay**, and the new field is appended at the end.
- `internal/analyzer/analyzer.go:506` — the existing `bucket()` helper. **Do not modify it.** Its label format (`"0_1024"`, `"1024_plus"`) is different from the spec's mandated format (`"1-3"`, `"1k-5k"`) and changing it would break existing aggregate tests. Add a parallel function instead.

Branch contract:
- Planning base: `main`. Merge target: `main`. Lane worktree is allocated by `/spec-kitty.implement` from `lanes.json`.

## Detailed Guidance

### Subtask T001 — Add new types and Ecosystem field

**Purpose**: Add `ToolingUtilization`, `MCPUtilization`, `SkillUtilization` to `internal/analyzer/types.go`, append a `ToolingUtilization ToolingUtilization \`json:"tooling_utilization"\`` field to `Ecosystem`, and ensure JSON tags exactly match the contract.

**Steps**:
1. Open `internal/analyzer/types.go`. Read the existing `Ecosystem` struct (lines 51-65 in the current file).
2. Append a new field at the end of `Ecosystem`:
   ```go
   ToolingUtilization ToolingUtilization `json:"tooling_utilization"`
   ```
3. Below `Ecosystem`, add three new structs. The exact field order and JSON tags must match `contracts/tooling-utilization.json`:
   ```go
   type ToolingUtilization struct {
       MCP   MCPUtilization   `json:"mcp"`
       Skill SkillUtilization `json:"skill"`
   }

   type MCPUtilization struct {
       KnownServerIDs           []string `json:"known_server_ids"`
       UnknownServerCount       int      `json:"unknown_server_count"`
       ServerCountBucket        string   `json:"server_count_bucket"`
       ExposedToolCountBucket   string   `json:"exposed_tool_count_bucket"`
       ContextTokenBucket       string   `json:"context_token_bucket"`
       ExposureKnown            bool     `json:"exposure_known"`
       InferenceSource          string   `json:"inference_source"`
       CallCount                int      `json:"call_count"`
       KnownCallCount           int      `json:"known_call_count"`
       UnknownCallCount         int      `json:"unknown_call_count"`
       UniqueKnownCalledIDs     []string `json:"unique_known_called_ids"`
       UniqueUnknownCalledCount int      `json:"unique_unknown_called_count"`
       UtilizationRatioPct      int      `json:"utilization_ratio_pct"`
       ContextEfficiencyBucket  string   `json:"context_efficiency_bucket"`
       WarningBand              string   `json:"warning_band"`
   }

   type SkillUtilization struct {
       KnownExposedIDs         []string `json:"known_exposed_ids"`
       UnknownExposedCount     int      `json:"unknown_exposed_count"`
       ExposedCountBucket      string   `json:"exposed_count_bucket"`
       ContextTokenBucket      string   `json:"context_token_bucket"`
       ExposureKnown           bool     `json:"exposure_known"`
       InferenceSource         string   `json:"inference_source"`
       ExecutedCount           int      `json:"executed_count"`
       KnownExecutedIDs        []string `json:"known_executed_ids"`
       UnknownExecutedCount    int      `json:"unknown_executed_count"`
       UtilizationRatioPct     int      `json:"utilization_ratio_pct"`
       ContextEfficiencyBucket string   `json:"context_efficiency_bucket"`
       WarningBand             string   `json:"warning_band"`
   }
   ```
4. Do **not** rename any existing field. Do **not** reorder existing fields.

**Files**:
- `internal/analyzer/types.go` (modify)

**Validation**:
- [ ] `go build ./...` succeeds.
- [ ] `go vet ./internal/analyzer/...` passes.
- [ ] Field order and JSON tags exactly match `contracts/tooling-utilization.json`.
- [ ] All existing `Ecosystem` field names and JSON tags are unchanged.

### Subtask T002 — Bucketing helpers

**Purpose**: Implement the closed-enumeration bucketing helpers required by C-003. They must be pure functions, deterministic, and never produce a value outside the documented enums.

**Steps**:
1. Create `internal/analyzer/tooling_buckets.go` with package `analyzer`.
2. Implement three functions:
   ```go
   // countBucket maps a non-negative count to one of:
   // "none", "1-3", "4-10", "11-25", "26-50", "51-100", "100+", "unknown".
   // When known is false, returns "unknown" regardless of n.
   func countBucket(n int, known bool) string { ... }

   // tokenBucket maps a non-negative token estimate to one of:
   // "none", "<1k", "1k-5k", "5k-15k", "15k-50k", "50k+", "unknown".
   // When known is false, returns "unknown" regardless of tokens.
   func tokenBucket(tokens int, known bool) string { ... }

   // efficiencyBucket combines a utilization ratio (0..100) and a context-token bucket
   // into one of: "unused", "underutilized", "moderate", "well-utilized", "unknown".
   // When known is false, returns "unknown" regardless of inputs.
   func efficiencyBucket(ratioPct int, tokenBucketLabel string, known bool) string { ... }
   ```
3. Implementation rules (from data-model.md §Bucketing function and §efficiencyBucket):
   - `countBucket`: 0 → `"none"`, 1-3 → `"1-3"`, 4-10 → `"4-10"`, 11-25 → `"11-25"`, 26-50 → `"26-50"`, 51-100 → `"51-100"`, >100 → `"100+"`.
   - `tokenBucket`: 0 → `"none"`, 1-999 → `"<1k"`, 1000-4999 → `"1k-5k"`, 5000-14999 → `"5k-15k"`, 15000-49999 → `"15k-50k"`, ≥50000 → `"50k+"`.
   - `efficiencyBucket`: `ratioPct < 5 && tokenBucketLabel ∉ {"none", "<1k"}` → `"unused"`; `ratioPct < 30` → `"underutilized"`; `ratioPct < 70` → `"moderate"`; otherwise → `"well-utilized"`.
4. No goroutines, no shared mutable state, no time-based values, no I/O.

**Files**:
- `internal/analyzer/tooling_buckets.go` (new file, ~80 lines).

**Validation**:
- [ ] Every helper returns a value from its closed enumeration for every possible input.
- [ ] Boundary values (0, 1, 3, 4, 10, 11, 25, 26, 50, 51, 100, 101) all return the correct bucket.

### Subtask T003 — Closed-enum constants

**Purpose**: Add named string constants for warning bands and inference sources so the rest of the codebase doesn't sprinkle bare string literals. The constants are referenced in WP03 and WP04.

**Steps**:
1. In `internal/analyzer/tooling_buckets.go`, add:
   ```go
   const (
       WarningBandNormal  = "normal"
       WarningBandWatch   = "watch"
       WarningBandHigh    = "high"
       WarningBandSevere  = "severe"
       WarningBandUnknown = "unknown"

       InferenceSourceHeader = "header"
       InferenceSourceCalls  = "calls"
       InferenceSourceNone   = "none"
   )
   ```
2. These names are package-internal — they are exported (capital letter) so tests in other files within `package analyzer` can reference them, but they live in `internal/analyzer/` so they are not part of the public API surface.

**Validation**:
- [ ] All five band constants and three inference-source constants exist.
- [ ] String values exactly match the enumerations in `contracts/tooling-utilization.json`.

### Subtask T004 — Bucketing unit tests

**Purpose**: Prove the helpers behave correctly across every boundary and that the closed enumerations are never violated.

**Steps**:
1. Create `internal/analyzer/tooling_buckets_test.go` with package `analyzer`.
2. Write three table-driven tests. The boundary table for `countBucket`:
   ```go
   func TestCountBucket(t *testing.T) {
       allowed := map[string]bool{"none": true, "1-3": true, "4-10": true,
           "11-25": true, "26-50": true, "51-100": true, "100+": true, "unknown": true}
       cases := []struct {
           n     int
           known bool
           want  string
       }{
           {0, true, "none"},
           {1, true, "1-3"}, {3, true, "1-3"},
           {4, true, "4-10"}, {10, true, "4-10"},
           {11, true, "11-25"}, {25, true, "11-25"},
           {26, true, "26-50"}, {50, true, "26-50"},
           {51, true, "51-100"}, {100, true, "51-100"},
           {101, true, "100+"}, {10000, true, "100+"},
           {0, false, "unknown"}, {500, false, "unknown"},
       }
       for _, tc := range cases {
           got := countBucket(tc.n, tc.known)
           if got != tc.want {
               t.Errorf("countBucket(%d, %v) = %q, want %q", tc.n, tc.known, got, tc.want)
           }
           if !allowed[got] {
               t.Errorf("countBucket(%d, %v) returned non-enum value %q", tc.n, tc.known, got)
           }
       }
   }
   ```
3. Analogous tables for `tokenBucket` (boundaries: 0, 1, 999, 1000, 4999, 5000, 14999, 15000, 49999, 50000, 100000) and `efficiencyBucket` (cross product of ratios {0, 4, 5, 29, 30, 69, 70, 100} × token buckets {"none", "<1k", "1k-5k", "50k+"}, plus known=false).
4. For each helper, include an `allowed` map check so any future regression that emits a non-enum value fails loudly.

**Files**:
- `internal/analyzer/tooling_buckets_test.go` (new file, ~120 lines).

**Validation**:
- [ ] `go test ./internal/analyzer/ -run TestBucket` passes.
- [ ] Every test case asserts both the expected bucket and the closed-enum constraint.

## Test Strategy

Tests are required for this WP (the spec requires them via NFR-002/NFR-004). Use table-driven Go tests as above. No mocks needed — these are pure functions.

## Definition of Done

- [ ] `internal/analyzer/types.go` contains the three new types and `Ecosystem.ToolingUtilization` field with exact JSON tags.
- [ ] `internal/analyzer/tooling_buckets.go` exists with `countBucket`, `tokenBucket`, `efficiencyBucket`, and the band/inference-source constants.
- [ ] `internal/analyzer/tooling_buckets_test.go` exercises every boundary plus the closed-enum invariant.
- [ ] `go build ./...` succeeds.
- [ ] `go test ./internal/analyzer/ -run TestBucket` passes.
- [ ] `gofmt -l internal/analyzer/types.go internal/analyzer/tooling_buckets.go internal/analyzer/tooling_buckets_test.go` returns empty.
- [ ] No existing test in `internal/analyzer/` regresses.

## Risks

- **Risk**: Adding a new field to `Ecosystem` changes its zero value. Existing tests that compare `Ecosystem` literals may need a small update — but only the literal equality path. **Mitigation**: keep the field at the end of the struct and rely on Go's zero-value defaults (`ToolingUtilization` zero value is fine and is later normalized by WP04). If a literal-equality test fails, defer the fix to WP04 (which owns analyzer_test.go).
- **Risk**: Confusion with existing `bucket()` helper. **Mitigation**: explicit comment at the top of `tooling_buckets.go` saying "the existing `bucket()` in analyzer.go uses a different label format; do not call it from this file".

## Reviewer Guidance

When reviewing:
- Verify field order and JSON tags exactly match `contracts/tooling-utilization.json` (every key listed in the `required` array must be present with the same name).
- Verify no existing `Ecosystem` field was renamed, removed, or reordered.
- Verify the bucket helpers never return a value outside their closed enumeration — the test should enforce this via the `allowed` map check.
- Verify there are no goroutines, no maps in serialization paths, no random values.

## Implementation Command

```bash
spec-kitty agent action implement WP01 --agent claude
```

## Activity Log

- 2026-05-19T09:34:39Z – claude – Moved to done
