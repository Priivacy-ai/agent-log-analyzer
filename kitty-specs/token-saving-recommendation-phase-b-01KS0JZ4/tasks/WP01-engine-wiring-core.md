---
work_package_id: WP01
title: Engine wiring core (helper + Report field + Analyze call site)
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-010
- FR-011
- FR-012
- NFR-001
- NFR-004
- C-001
- C-002
- C-005
- C-006
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
created_at: '2026-05-19T20:30:00+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
- T007
agent_profile: implementer-ivan
role: implementer
agent: claude:sonnet:implementer-ivan:implementer
authoritative_surface: internal/analyzer/recommendation_wiring
execution_mode: code_change
owned_files:
- internal/analyzer/types.go
- internal/analyzer/recommendation_wiring.go
- internal/analyzer/recommendation_wiring_test.go
- internal/analyzer/analyzer.go
history:
- '2026-05-19': created from mission token-saving-recommendation-phase-b-01KS0JZ4
tags:
- foundation
- engine-wiring
---

## ⚡ Do This First: Load Agent Profile

Before reading the rest of this prompt, load the assigned agent profile:

```text
/ad-hoc-profile-load implementer-ivan
```

The profile defines your identity, governance scope, allowed file surface, and
initialization declaration for this work package. After it loads, return here
and continue with **Objective** below.

## Objective

Build the **single foundation** of Phase B: the new helper
`analyzer.AttachRecommendation(report *Report)` that derives engine signals
and tool state from a `Report`, calls the frozen Phase A `Recommend` engine,
and assigns the result to `report.Recommendation`. After this WP lands,
single-report analysis (`analyzer.Analyze`) produces a `Report` whose
`Recommendation` field is populated.

This WP introduces:

1. A new optional pointer field `Recommendation *RecommendationSet` on
   `Report` (`internal/analyzer/types.go`).
2. A new file `internal/analyzer/recommendation_wiring.go` containing the
   public helper `AttachRecommendation` plus the two package-private
   helpers `deriveSignals` and `deriveToolStateMap`.
3. A new test file `internal/analyzer/recommendation_wiring_test.go`.
4. A single call to `AttachRecommendation(&report)` at the bottom of
   `analyzer.Analyze` (after the `Report` is fully constructed).

The Phase A engine is **frozen** — do not touch any file under
`internal/analyzer/token_saving_*.go`. Read the engine; do not modify it.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. The implementing
agent operates on the work-branch and execution worktree assigned by
`lanes.json` (resolve via `spec-kitty agent context resolve --mission
token-saving-recommendation-phase-b-01KS0JZ4 --wp WP01 --json`); do not
invent or hop branches manually.

## Context

The contracts that govern this WP live in:

- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/spec.md`
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/plan.md` (Engineering Alignment + Determinism sections)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/data-model.md` (Report shape diff)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/contracts/attach-recommendation-go-api.md` (function signature, invariants)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/contracts/signal-derivation-map.md` (rules S-01..S-07)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/contracts/tool-state-derivation-map.md` (rules T-F-01..T-S-02)

You do not need anything outside this mission directory plus the existing
`internal/analyzer/token_saving_*.go` engine files.

## Owned files

This WP is the only writer of:

- `internal/analyzer/types.go` — **field-add only** (add `Recommendation *RecommendationSet` with `json:"recommendation,omitempty"`). Do not reorder existing fields.
- `internal/analyzer/recommendation_wiring.go` — new file (see Subtasks below).
- `internal/analyzer/recommendation_wiring_test.go` — new file.
- `internal/analyzer/analyzer.go` — **append a single call** at the bottom of `Analyze`. Do not refactor anything else in this file.

Any change outside this list is a scope violation and must be deferred.

## Subtasks

### T001 — Add `Recommendation` field to `Report`

**Purpose**: Expose the engine output as an optional, omitempty field on
`Report`.

**Steps**:

1. Open `internal/analyzer/types.go`.
2. Inside the existing `Report` struct, **append** (after `AggregateEvent`):
   ```go
   Recommendation *RecommendationSet `json:"recommendation,omitempty"`
   ```
3. Do not reorder existing fields. Do not add any other field. Do not touch
   any other struct in this file.

**Validation**: `gofmt -l internal/analyzer/types.go` exits clean; the file
compiles with `go vet ./internal/analyzer/...`.

### T002 — Create `recommendation_wiring.go` skeleton

**Purpose**: Define the public helper and the two private derivation
helpers.

**Steps**:

1. Create `internal/analyzer/recommendation_wiring.go` with this top:
   ```go
   // Package analyzer — Phase B wiring between report data and the
   // frozen Phase A token-saving recommendation engine.
   //
   // AttachRecommendation derives engine signals and tool state from a
   // fully-constructed Report and assigns the engine output to
   // report.Recommendation. The function is deterministic, side-effect-
   // free beyond the report mutation, and never returns an error.
   package analyzer
   ```
2. Declare the public helper:
   ```go
   func AttachRecommendation(report *Report) {
       if report == nil {
           return
       }
       signals := deriveSignals(report)
       state := deriveToolStateMap(report)
       set := Recommend(signals, state)
       report.Recommendation = &set
   }
   ```
3. Declare the two package-private helpers with empty bodies that return
   the zero value of their return type; T003 and T004 fill them in.
   ```go
   func deriveSignals(report *Report) []Signal { return nil }
   func deriveToolStateMap(report *Report) ToolStateMap { return ToolStateMap{} }
   ```

**Validation**: file compiles; `go vet ./internal/analyzer/...` passes.

### T003 — Implement signal derivation (S-01..S-07)

**Purpose**: Replace the skeleton `deriveSignals` with the full mapping
defined in `contracts/signal-derivation-map.md`.

**Steps**:

1. Iterate `report.Findings` in slice order (already deterministic) and
   emit:
   - S-01 `tool_output_bloat` → `SignalToolOutputBloat`
   - S-02 `repeated_file_reads` → `SignalRepeatedFileReads`
   - S-03 `retry_loop` → `SignalRetryLoop`
   - S-04 `context_growth_spikes` → `SignalContextGrowthSpikes`
2. Inspect `report.Ecosystem.ToolingUtilization.MCP.WarningBand` — if it
   equals `"high"` or `"severe"`, append `SignalMCPSkillBloat`.
3. Inspect `report.Ecosystem.ToolingUtilization.Skill.WarningBand` — same
   rule.
4. Determine whether any active usage-tracker fingerprint is present:
   - Walk `report.Ecosystem.WorkflowFingerprints`.
   - For each entry where `Active == true`, resolve registry membership via:
     ```go
     if tool, ok := GetTool(ToolID(entry.ID)); ok && tool.RecommendationClass == ClassUsageVisibility {
         hasActiveUsageTracker = true
         break
     }
     ```
     - `GetTool` returns `(TokenSavingTool, bool)`; the second return is
       false for IDs outside the registry. Unknown IDs are silently
       skipped (count toward `UnknownIDCount` via the engine, not here).
     - The field on `TokenSavingTool` is `RecommendationClass` (not
       `Class`). Using the wrong name will fail to compile.
   - If **no** active usage-visibility fingerprint is found, append
     `SignalNoUsageVisibility`.
5. Dedupe and sort via the existing `sortedSignalIDs` helper in
   `token_saving_types.go` (this is the only allowed cross-file dependency
   for derivation).

**Validation**: the function returns a sorted, deduplicated `[]Signal`.
Unit tests in T006 cover every rule.

### T004 — Implement tool-state derivation (T-F-01..T-S-02)

**Purpose**: Replace the skeleton `deriveToolStateMap` with the full
mapping defined in `contracts/tool-state-derivation-map.md`.

**Steps**:

1. Initialize an empty `ToolStateMap`.
2. Walk `report.Ecosystem.WorkflowFingerprints` in slice order. For each
   entry whose `ID` resolves to a known tool via `GetTool`:
   - Compute the rule state:
     - `Active == true` → `ToolStateActiveHigh`
     - `Active == false && Installed == true` → `ToolStateInstalledMedium`
     - `Active == false && Installed == false` → `ToolStateMentionedLow`
   - Compute the increments:
     - Always: `EvidenceReportMention += 1`.
     - If `Sources` slice contains `"cli_probe"`: `EvidenceCLIPresence += 1`.
     - If `VersionBucket != ""`: `EvidenceCLIVersion += 1`.
   - Merge into the map (see merge step below).
3. Walk `report.Ecosystem.ToolingUtilization.MCP.KnownServerIDs` in slice
   order. For each ID:
   - If ID ∈ `UniqueKnownCalledIDs`: state `ToolStateActiveHigh`,
     `EvidenceMCPActive += 1`.
   - Else: state `ToolStateConfiguredMedium`, `EvidenceMCPConfigured += 1`.
4. Walk `report.Ecosystem.ToolingUtilization.Skill.KnownExposedIDs` in
   slice order. For each ID:
   - If ID ∈ `KnownExecutedIDs`: state `ToolStateActiveHigh`,
     `EvidenceSkillConfigured += 1` and `EvidenceReportMention += 1`.
   - Else: state `ToolStateConfiguredMedium`, `EvidenceSkillConfigured += 1`.
5. **Merge step**: for each new contribution to a `ToolID`:
   - If absent, create a fresh `ToolStateEntry{Tool: id, State: rule.state, Sources: map}`.
   - If present, `ToolStateMap.Resolve(existing.State, rule.state)` and add
     the new source counts into the existing `Sources` map. Cap each
     source count at 100 (privacy budget).
6. Use slice membership helpers; do not `range` over any map in this
   function except for incrementing inside a per-entry `Sources` map (the
   per-entry source map is OK to range because writes are keyed, not
   iterated).

**Validation**: returned map is deterministic for identical input;
membership checks use sorted contains helpers or stdlib `slices.Contains`.

### T005 — Call `AttachRecommendation` from `Analyze`

**Purpose**: Wire the helper into the per-report analysis path.

**Steps**:

1. Open `internal/analyzer/analyzer.go`.
2. Find the function `Analyze`. After the `Report` is fully constructed
   (after `AggregateEvent` is populated and immediately before the
   `return report` statement at the bottom of `Analyze`), insert:
   ```go
   AttachRecommendation(&report)
   ```
3. Do not refactor anything else in this file.

**Validation**: `grep -n 'AttachRecommendation' internal/analyzer/analyzer.go`
returns exactly one match.

### T006 — Table-driven derivation tests

**Purpose**: Cover every signal-derivation rule and every tool-state rule
with a table-driven test.

**Steps**:

1. Create `internal/analyzer/recommendation_wiring_test.go`.
2. Test `deriveSignals`:
   - One case per rule (S-01 through S-07).
   - One multi-source dedupe case (MCP band severe AND skill band high
     produces one `SignalMCPSkillBloat`).
   - One empty-input case (no findings, no utilization, no fingerprints)
     produces `[SignalNoUsageVisibility]` (rule S-07 fires).
3. Test `deriveToolStateMap`:
   - One case per rule (T-F-01..T-S-02), 7 cases.
   - One multi-source conflict case (a tool seen in fingerprint with
     `Installed==true` AND in MCP `KnownServerIDs` without execution)
     resolves via precedence.
   - One privacy case (unknown MCP/skill names supplied via
     `UnknownMCPServerCount` and similar fields do not appear in the map).
4. Test `AttachRecommendation`:
   - Empty `Report` produces a non-nil `report.Recommendation` whose
     `EngineVersion == EngineVersion()`.
   - A canonical fixture with `tool_output_bloat` + no usage-tracker
     produces `Primary != nil` (the engine's recommendation for that
     case).

**Validation**: `go test -run TestDeriveSignals -run TestDeriveToolStateMap
-run TestAttachRecommendation ./internal/analyzer/...` passes.

### T007 — 100-iteration determinism test

**Purpose**: Lock in NFR-001 (byte-identical JSON for identical inputs).

**Steps**:

1. In `recommendation_wiring_test.go`, add `TestAttachRecommendationDeterminism`.
2. Build a canonical fixture `Report` (use the same fixture as T006's
   `Primary != nil` case).
3. Loop 100 times. On each iteration:
   - Construct a fresh copy of the fixture.
   - Call `AttachRecommendation`.
   - Marshal `report.Recommendation` to JSON.
   - If iteration > 0, assert bytes equal the first iteration's bytes.

**Validation**: test passes; 100 iterations all produce identical bytes.

## Test strategy

- Unit tests live in `internal/analyzer/recommendation_wiring_test.go`.
- Use table-driven tests (Go convention; existing pattern in this
  package).
- Use `t.Run("name", func(t *testing.T) { ... })` for each sub-case.
- Run the full package test suite: `go test ./internal/analyzer/...`.
- The existing leak test and golden test will be extended by WP03 — do
  NOT touch them in WP01. WP03 owns those files.

## Definition of Done

- [ ] `go test ./...` passes (full repo).
- [ ] `go vet ./...` passes.
- [ ] `gofmt -l internal/analyzer/` returns empty.
- [ ] `AttachRecommendation` exists, is exported, has the exact signature
      defined in `contracts/attach-recommendation-go-api.md`.
- [ ] `Report.Recommendation` field is present with `omitempty`.
- [ ] `analyzer.Analyze` calls `AttachRecommendation` exactly once.
- [ ] Determinism test passes (100×).
- [ ] No file under `internal/analyzer/token_saving_*.go` is modified.
- [ ] No file outside `owned_files` is modified.
- [ ] **Diff-scoped lint sweep** (`gofmt -l`, `go vet`) clean.

## Risks

- **Reordering fields in `Report` breaks JSON ordering** → only append.
- **Touching engine internals** → C-002 violation; explicit denylist above.
- **Map iteration in derivation** → breaks NFR-001 determinism; tests
  catch but reviewers must also visually check.
- **Recursive imports / cycle in token_saving_types.go** → unlikely
  because everything lives in `package analyzer`; if a cycle appears, the
  import is wrong — investigate before papering over.

## Reviewer Guidance

The reviewer (WP01-cycle agent) must check:

1. The function signature in `recommendation_wiring.go` matches the
   contract verbatim.
2. No `range` over `state` or any `ToolStateMap` in derivation paths.
3. `Report.Recommendation` is the last field of the struct (or at least
   appended after `AggregateEvent`).
4. The `Analyze` call site is **at the bottom** of `Analyze`, after
   `AggregateEvent` is populated.
5. The determinism test actually marshals JSON and compares bytes; it is
   easy to write a determinism test that compares `*RecommendationSet`
   pointers (which would be trivially equal) — that is **not** what we
   want.
6. Token-saving engine files (`token_saving_*.go`) are untouched.

## Working Directory and Hand-off

After implementation, run from the lane worktree:

```bash
gofmt -w internal/analyzer/
go vet ./internal/analyzer/...
go test ./internal/analyzer/...
```

All must exit 0. Then commit:

```bash
git add internal/analyzer/types.go \
        internal/analyzer/recommendation_wiring.go \
        internal/analyzer/recommendation_wiring_test.go \
        internal/analyzer/analyzer.go
git commit -m "feat(WP01): wire AttachRecommendation into Analyze"
```

Mark subtasks done and move to `for_review`:

```bash
spec-kitty agent tasks mark-status T001 T002 T003 T004 T005 T006 T007 --status done --mission token-saving-recommendation-phase-b-01KS0JZ4
spec-kitty agent tasks move-task WP01 --to for_review --note "Ready for review" --mission token-saving-recommendation-phase-b-01KS0JZ4
```
