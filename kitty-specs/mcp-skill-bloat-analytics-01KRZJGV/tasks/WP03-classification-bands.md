---
work_package_id: WP03
title: 'Classification: Warning Bands'
dependencies:
- WP01
requirement_refs:
- FR-005
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T010
- T011
- T012
- T013
agent: claude
history:
- event: generated
  at: '2026-05-19T08:00:33Z'
  by: /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/tooling_classify.go
execution_mode: code_change
mission_id: 01KRZJGVG3MCCCY9MKB1YRDBQR
mission_slug: mcp-skill-bloat-analytics-01KRZJGV
owned_files:
- internal/analyzer/tooling_classify.go
- internal/analyzer/tooling_classify_test.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load the agent profile by invoking `/ad-hoc-profile-load` with `profile_id: "implementer-ivan"` and `role: "implementer"`. Then return here.

## Objective

Implement the deterministic warning-band classifier and efficiency bucket helper. Two pure functions, fully unit-tested, with explicit invariant tests proving that **count alone never triggers anything above `normal`** (FR-005, SC-5) and that `exposure_known=false` always yields `unknown` (I-1).

After this WP merges, WP04 (wiring) imports these functions and feeds them the buckets and utilization ratio computed earlier in the pipeline.

## Context

Read first:
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/spec.md` — FR-005 (deterministic bands), SC-5 (high count alone never produces a warning).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md` — §D-4 with the concrete threshold table for both MCP and skill bands.
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md` — §Invariants I-1, I-2, I-3.
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/research.md` — R-3 for the rationale behind chosen thresholds.
- `internal/analyzer/tooling_buckets.go` (from WP01) — for the `WarningBand*` constants and the bucket label format.
- `internal/analyzer/types.go` — for `Metrics` struct (fields `Rereads`, `RetryDepthMax`, `ContextGrowthEvents` are the degradation signals).

Branch contract:
- Planning base: `main`. Merge target: `main`. Lane worktree allocated by `/spec-kitty.implement`.

## Detailed Guidance

### Subtask T010 — Warning band classifier

**Purpose**: Pure function `(buckets, ratio, exposure_known, degradation_signals) → WarningBand`. Implements the D-4 table.

**Steps**:
1. Create `internal/analyzer/tooling_classify.go` with package `analyzer`.
2. Define two inputs types and two classifiers (MCP and skill bands have slightly different thresholds per D-4):
   ```go
   type mcpBandInput struct {
       ServerCountBucket      string // from countBucket
       ExposedToolCountBucket string
       ContextTokenBucket     string
       UtilizationRatioPct    int    // 0..100
       ExposureKnown          bool
       Rereads                int
       RetryDepthMax          int
       ContextGrowthEvents    int
   }
   type skillBandInput struct {
       ExposedCountBucket  string
       ContextTokenBucket  string
       UtilizationRatioPct int
       ExposureKnown       bool
       Rereads             int
       RetryDepthMax       int
       ContextGrowthEvents int
   }

   func classifyMCPBand(in mcpBandInput) string { ... }
   func classifySkillBand(in skillBandInput) string { ... }
   ```
3. Implementation of `classifyMCPBand` (matches plan §D-4 exactly):
   - If `!in.ExposureKnown`: return `WarningBandUnknown`.
   - Compute `degradation := in.Rereads >= 3 || in.RetryDepthMax >= 3 || in.ContextGrowthEvents >= 2`.
   - Helper: `serverAtLeast(b string, threshold string) bool` returns true iff bucket `b` is at or above `threshold` in the ordering `none < 1-3 < 4-10 < 11-25 < 26-50 < 51-100 < 100+`. Implement as an explicit `map[string]int` of bucket ranks.
   - **`normal` precondition** (check first — any of these → `normal`):
     - `in.ServerCountBucket` ≤ `4-10` (rank ≤ 2)
     - OR `in.UtilizationRatioPct >= 40`
   - **`severe` check** (high conditions + degradation):
     - `serverAtLeast(ServerCountBucket, "11-25")` AND `UtilizationRatioPct < 20` AND (`tokenAtLeast(ContextTokenBucket, "5k-15k")` OR `countAtLeast(ExposedToolCountBucket, "26-50")`) AND `degradation` → `WarningBandSevere`.
   - **`high` check** (same conditions, no degradation requirement):
     - same as severe but no `degradation` requirement → `WarningBandHigh`.
   - **`watch`**:
     - `serverAtLeast(ServerCountBucket, "11-25")` AND `UtilizationRatioPct < 40` AND NOT `degradation` AND `ServerCountBucket` rank ≤ `26-50` rank (i.e., moderate not huge) → `WarningBandWatch`.
   - Default: `WarningBandNormal`.
4. Implementation of `classifySkillBand` is analogous with skill thresholds from §D-4 (`utilization < 30` for normal cutoff, `< 15` for high, only `ContextTokenBucket` gate for high).
5. Add helper `tokenAtLeast` with the rank table `none(0) < <1k(1) < 1k-5k(2) < 5k-15k(3) < 15k-50k(4) < 50k+(5)`.
6. Add helper `countAtLeast` with the rank table `none(0) < 1-3(1) < 4-10(2) < 11-25(3) < 26-50(4) < 51-100(5) < 100+(6)`. The `unknown` label has no rank — when comparing `unknown` to any threshold, return `false` (treat unknown as "below all thresholds").

**Files**:
- `internal/analyzer/tooling_classify.go` (new file, ~150 lines).

**Validation**:
- [ ] Returns a value from `{normal, watch, high, severe, unknown}` for every input.
- [ ] When `ExposureKnown=false`, always returns `unknown`.
- [ ] When `UtilizationRatioPct>=40` (MCP) or `>=30` (skill), returns `normal` regardless of count.

### Subtask T011 — Efficiency bucket classifier

**Purpose**: WP01 added `efficiencyBucket(ratioPct, tokenBucketLabel, known)` — verify the WP01 implementation is sufficient or extend it here if not. Most likely no new function is needed; this subtask is about wiring the existing helper in test scenarios.

**Steps**:
1. Re-read `tooling_buckets.go` from WP01. If `efficiencyBucket` already satisfies the data-model §efficiencyBucket spec, this subtask reduces to **adding tests** that exercise it under realistic classifier inputs.
2. If WP01 left it as a stub, fully implement here (same signature, same behavior as data-model.md §efficiencyBucket).
3. No new file. Tests go in `tooling_classify_test.go`.

**Files**:
- (possibly) `internal/analyzer/tooling_buckets.go` — only if extending the WP01 stub.

**Validation**:
- [ ] `efficiencyBucket(2, "5k-15k", true) == "unused"`.
- [ ] `efficiencyBucket(2, "<1k", true) == "underutilized"` (low footprint downgrades unused → underutilized).
- [ ] `efficiencyBucket(50, "1k-5k", true) == "moderate"`.
- [ ] `efficiencyBucket(85, "15k-50k", true) == "well-utilized"`.
- [ ] `efficiencyBucket(any, any, false) == "unknown"`.

### Subtask T012 — Classifier unit tests for every D-4 transition

**Purpose**: Lock every band transition with explicit table-driven cases.

**Steps**:
1. Create `internal/analyzer/tooling_classify_test.go` with package `analyzer`.
2. Write `TestClassifyMCPBand` with a table that exercises at least:
   - `exposure_known=false` → `unknown` (multiple input shapes).
   - Small count (`1-3`, `4-10`) + low utilization → `normal`.
   - Moderate count (`11-25`, `26-50`) + low utilization + no degradation → `watch`.
   - Moderate-or-larger count + very low utilization (<20%) + high footprint, no degradation → `high`.
   - Same as `high` + degradation → `severe`.
   - Large count + meaningful utilization (≥40%) → `normal` (the key SC-5 case).
   - Boundary on `ServerCountBucket` transition `"4-10"` → `"11-25"`.
   - Boundary on `UtilizationRatioPct` transition 39 → 40 (normal threshold).
   - Boundary 19 → 20 (high threshold).
3. Analogous `TestClassifySkillBand` with skill thresholds (`<30` normal cutoff, `<15` high cutoff).
4. Use a helper to assert membership in the closed enum:
   ```go
   allowedBands := map[string]bool{"normal": true, "watch": true, "high": true, "severe": true, "unknown": true}
   ```
5. Aim for 20+ MCP cases and 15+ skill cases. Each case names what it's checking via a `name` field.

**Files**:
- `internal/analyzer/tooling_classify_test.go` (new file, ~250 lines).

**Validation**:
- [ ] `go test ./internal/analyzer/ -run TestClassify` passes.
- [ ] Every D-4 row has at least one test case.

### Subtask T013 — Invariant tests

**Purpose**: Lock the two most important invariants with explicit, named tests separate from the transition table.

**Steps**:
1. In `tooling_classify_test.go`, add two more tests:
   ```go
   func TestClassifyInvariantCountAloneNeverWarns(t *testing.T) {
       // Property-style: for every (countBucket × tokenBucket) pair, when
       // UtilizationRatioPct >= 40 (MCP) or >= 30 (skill), band must be "normal".
       counts := []string{"none", "1-3", "4-10", "11-25", "26-50", "51-100", "100+"}
       tokens := []string{"none", "<1k", "1k-5k", "5k-15k", "15k-50k", "50k+"}
       for _, c := range counts {
           for _, tk := range tokens {
               got := classifyMCPBand(mcpBandInput{
                   ServerCountBucket: c, ExposedToolCountBucket: c, ContextTokenBucket: tk,
                   UtilizationRatioPct: 50, ExposureKnown: true,
                   Rereads: 10, RetryDepthMax: 10, ContextGrowthEvents: 10,
               })
               if got != WarningBandNormal {
                   t.Errorf("expected normal for count=%s token=%s util=50%% (even with degradation), got %s", c, tk, got)
               }
           }
       }
       // Mirror for skill at 50% utilization.
   }

   func TestClassifyInvariantExposureUnknownAlwaysUnknown(t *testing.T) {
       // For every plausible non-exposure-known input, band must be "unknown".
       for _, ratio := range []int{0, 50, 100} {
           for _, count := range []string{"unknown", "100+"} {
               got := classifyMCPBand(mcpBandInput{
                   ServerCountBucket: count, ContextTokenBucket: "unknown",
                   UtilizationRatioPct: ratio, ExposureKnown: false,
                   Rereads: 5, RetryDepthMax: 5, ContextGrowthEvents: 5,
               })
               if got != WarningBandUnknown {
                   t.Errorf("expected unknown for exposure_known=false, got %s", got)
               }
           }
       }
   }
   ```
2. These tests deliberately use degradation signals at their maximum to prove that high utilization overrides degradation in the count-alone case, and that `exposure_known=false` overrides every other signal.

**Files**:
- `internal/analyzer/tooling_classify_test.go` (extend from T012).

**Validation**:
- [ ] Both invariant tests pass.
- [ ] If anyone in the future relaxes the band logic, at least one of these tests fails loudly.

## Test Strategy

Tests are required (NFR-002). All tests are pure-function table tests. No mocks. Aim for >40 total cases across `TestClassifyMCPBand`, `TestClassifySkillBand`, and the two invariant tests.

## Definition of Done

- [ ] `internal/analyzer/tooling_classify.go` contains `classifyMCPBand`, `classifySkillBand`, and the rank-helper functions.
- [ ] `internal/analyzer/tooling_classify_test.go` covers every D-4 row plus the two invariants.
- [ ] `go test ./internal/analyzer/ -run TestClassify` passes.
- [ ] `gofmt -l` on both files returns empty.
- [ ] No existing test regresses.

## Risks

- **Risk**: Subtle drift between D-4 spec and the implementation (e.g., off-by-one in a rank comparison). **Mitigation**: write a comment block at the top of each classifier mirroring the D-4 table verbatim, then implement directly from the comment.
- **Risk**: The `unknown` bucket label clashes with rank comparisons. **Mitigation**: explicit `if bucket == "unknown" { return false }` in the `*AtLeast` helpers.
- **Risk**: Skill thresholds copied accidentally from MCP. **Mitigation**: each classifier is a separate function with its own constants; do not share threshold literals.

## Reviewer Guidance

When reviewing:
- Open plan.md §D-4 side-by-side with `tooling_classify.go`. Verify each row of D-4 maps to a specific code path.
- Verify the two invariant tests exist and use degradation signals at their maximum (to prove utilization wins over degradation in the high-util case).
- Spot-check the rank helpers: `countAtLeast("11-25", "11-25")` should be `true`; `countAtLeast("4-10", "11-25")` should be `false`; `countAtLeast("unknown", "1-3")` should be `false`.
- No band string returned from the classifier should appear that isn't in `{normal, watch, high, severe, unknown}`.

## Implementation Command

```bash
spec-kitty agent action implement WP03 --agent claude
```

## Activity Log

- 2026-05-19T09:34:43Z – claude – Moved to done
