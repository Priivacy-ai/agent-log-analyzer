---
work_package_id: WP03
title: Paid aggregate merge for ecosystem fields
dependencies:
- WP02
requirement_refs:
- FR-007
- FR-008
- FR-009
- FR-010
- NFR-001
- NFR-002
- NFR-005
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T012
- T013
- T014
- T015
- T016
- T017
- T018
phase: Phase 1 — Launch Correctness
assignee: ''
agent: "claude:opus-4-7:reviewer-renata:reviewer"
shell_pid: "52554"
history:
- at: '2026-05-19T11:55:54Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
- at: '2026-05-19T12:10:00Z'
  actor: system
  action: 'Post-analyze fixes: declare WP02 dependency, correct testdata/golden/ path, add internal/remediation/artifact.go to owned_files.'
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/aggregate
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/aggregate.go
- internal/analyzer/aggregate_test.go
- internal/analyzer/leak_test.go
- internal/analyzer/golden_test.go
- testdata/golden/**
- internal/remediation/artifact.go
- internal/remediation/artifact_test.go
role: implementer
tags: []
---

# Work Package Prompt: WP03 – Paid aggregate merge for ecosystem fields

## ⚡ Do This First: Load Agent Profile

Before reading anything else in this prompt, load the assigned agent profile:

```
/ad-hoc-profile-load implementer-ivan
```

Then return to this file and read top-to-bottom.

## Branch Strategy

- **Planning/base branch at prompt creation**: `main`
- **Final merge target for completed work**: `main`
- **Shared feature branch for all three WPs of this mission**: `codex/launch-correctness` (add timestamp suffix if the name is taken on the remote).
- **Actual execution workspace**: `/spec-kitty.implement` selects the lane worktree and records the lane branch. Trust the printed lane workspace; do not guess.
- **If human instructions contradict these fields**: stop and resolve before coding.

## Objectives & Success Criteria

This WP delivers issue **#72** in full. After this WP:

1. `mergeEcosystems` preserves `Ecosystem.WorkflowFingerprints` across N input reports using the FR-007 rules: merge by `id`, sources unioned, `evidence_count` summed (per C-007), confidence held to max-rank, active/installed OR'd, version_bucket retained-or-emptied (FR-007).
2. `mergeEcosystems` preserves `Ecosystem.ToolingUtilization` (MCP + Skill) across N inputs using the FR-008 rules: known IDs unioned, unknown counts summed, call counts summed, warning bands held to max-rank, utilization ratio recomputed (FR-008).
3. The generated paid plugin artifact (`internal/remediation/artifact.go`'s `Generate()`) consumes the merged values, not pre-merge values from any single input (FR-009).
4. Six aggregate invariants hold: identity, commutativity, associativity, coverage, privacy, bounded-cardinality (all backed by `contracts/aggregate-merge.md`).
5. The privacy canary in `leak_test.go` extends across the merged ecosystem JSON, the generated paid artifact, and the aggregate event payload — zero leak strings (NFR-002).
6. The 100-input merge completes in under 5 seconds (NFR-005).
7. The charter verification baseline still passes (NFR-001).

The implementer must also post a "starting work" comment on issue #72 and a "ready for review" comment on issue #72 when the PR opens — this WP carries one-third of FR-010.

**Independent test:**

```bash
go test ./internal/analyzer/ -run TestAggregateReports -v
go test ./internal/analyzer/ -run TestLeak -v
go test ./internal/remediation/ -run TestArtifact -v
```

## Context & Constraints

- Spec: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/spec.md` — FR-007..FR-009, NFR-002, NFR-005, C-007.
- Plan: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/plan.md`.
- Data model: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/data-model.md` — read the merge semantics tables in full.
- Contract: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/contracts/aggregate-merge.md`. Read this in full — definitive merge rules, invariants, and consumer contract.
- Research: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/research.md` — Bug #72 section.
- Existing code:
  - `internal/analyzer/aggregate.go:8` (`AggregateReports`), `aggregate.go:30` (loop), `aggregate.go:128..143` (`mergeEcosystems` — bug site, skips two fields).
  - `internal/analyzer/types.go:51..67` (`Ecosystem` struct), `types.go:83..119` (`ToolingUtilization`, `MCPUtilization`, `SkillUtilization`).
  - `internal/analyzer/ecosystem.go:218` (`computeToolingUtilization` — already populates per-report values; nothing to change here).
  - `internal/analyzer/golden_test.go:55..59` (currently nils `WorkflowFingerprints` in aggregate compare; this WP fixes that).
  - `internal/analyzer/leak_test.go` (privacy canary; extend coverage).
  - `internal/remediation/artifact.go:502` (`safeKnownEcosystem`), `artifact.go:120` (`toolingRecommendations`); reads the fields but currently sees zero data for aggregate paths.
  - `internal/paidscan/bundle.go:44` (`AnalyzeBundle`) calls `AggregateReports`; unchanged.

**Constraints carried from spec/charter:**
- C-001 (bounded-cardinality schema): every merged value is an allowlisted ID, a closed enum, a bounded bucket, or a numeric count. No new shape.
- C-002 (privacy stance): no private name appears in merged output. Canary asserts this.
- C-006 (no-op stability): doesn't directly apply here (C-006 is WP02's concern), but `testdata/golden/sample-report.json` will shift to include merged fingerprint/utilization data — that is the intended fix, not a regression.
- C-007 (`evidence_count = sum`): locked. Do not switch to `max` in this WP.

**WP02 rebase / hand-off (dependency declared in frontmatter as `dependencies: [WP02]`)**:

- This WP runs after WP02 lands its lane. Before starting code work, rebase
  onto the latest WP02 lane head and re-read the WP02 activity log.
- WP02's T011 may have recorded a hand-off note about a shift in
  `testdata/golden/sample-report.json` (because the per-report MCP detector
  now masks header tokens correctly). If such a note exists:
  - Run `UPDATE_GOLDEN=1 go test ./internal/analyzer -run TestGoldenSampleReport`
    once, inspect the diff carefully, confirm it is downward-only on MCP call
    counts, and commit the refreshed snapshot alongside this WP's aggregate
    work.
  - The same `testdata/golden/sample-report.json` will also receive the
    aggregate-merge updates from T013 / T016. Folding both refreshes into one
    commit is intentional and keeps the file diff coherent.
- If WP02's hand-off note says "no per-report shift", only the aggregate-merge
  changes touch the golden file.
- `internal/analyzer/golden_test.go` lines 55..59 currently nil
  `WorkflowFingerprints` and `AggregateEvent.Ecosystem.WorkflowFingerprints`
  before the golden comparison. T016 removes that nilling (the field becomes
  reproducible now that aggregate merge populates it deterministically). This
  is the load-bearing reason this WP owns `golden_test.go`.

## Subtasks & Detailed Guidance

### Subtask T012 — Add merge helpers

- **Purpose**: Build the small primitives that `mergeEcosystems` will compose.
- **Files**: `internal/analyzer/aggregate.go`.
- **Steps**: Add the following unexported helpers, all in the same file:
  ```go
  // unionSorted returns the deduplicated, ascending-sorted union of a and b.
  // Inputs are not mutated. Output is a fresh slice.
  func unionSorted(a, b []string) []string { ... }

  // maxWarningBand returns the higher band by rank:
  // severe > high > watch > normal > unknown
  func maxWarningBand(a, b string) string { ... }

  // maxConfidence returns the higher confidence by rank:
  // high > medium > low
  func maxConfidence(a, b string) string { ... }

  // mergeMCPUtilization merges two MCPUtilization values per FR-008.
  // - KnownServerIDs / UniqueKnownCalledIDs: unionSorted
  // - UnknownServerCount / CallCount / KnownCallCount: sum
  // - WarningBand: maxWarningBand
  // - UtilizationRatioPct: recompute from summed counts, clamped [0,100]
  // - Buckets: hold max-rank for now (recompute deferred if bucket boundaries
  //   are not exposed at this layer)
  func mergeMCPUtilization(a, b MCPUtilization) MCPUtilization { ... }

  // mergeSkillUtilization mirrors mergeMCPUtilization for skills.
  func mergeSkillUtilization(a, b SkillUtilization) SkillUtilization { ... }

  // mergeWorkflowFingerprints merges by id per FR-007.
  // - sources: unionSorted
  // - evidence_count: SUM (C-007)
  // - confidence: maxConfidence
  // - active / installed: OR
  // - version_bucket: retain when all inputs agree on a non-empty value,
  //   else empty (NO "mixed" value introduced)
  func mergeWorkflowFingerprints(a, b []EcosystemFingerprint) []EcosystemFingerprint { ... }
  ```
- **Parallel?**: No — T013 depends on T012.
- **Notes**:
  - Define the band-rank order as a small `map[string]int` or a switch. Tests will exercise every rank pair.
  - Sort the output of `unionSorted` ascending so merge output is deterministic regardless of input order.

### Subtask T013 — Extend `mergeEcosystems`

- **Purpose**: Hook the new helpers into the existing merge.
- **Files**: `internal/analyzer/aggregate.go` (lines around 128..143).
- **Steps**:
  1. Preserve the existing 13-field merges (Client, OS, Shell, etc.) exactly as today. Do not reorganize.
  2. At the end of the function, add:
     ```go
     out.ToolingUtilization.MCP = mergeMCPUtilization(a.ToolingUtilization.MCP, b.ToolingUtilization.MCP)
     out.ToolingUtilization.Skill = mergeSkillUtilization(a.ToolingUtilization.Skill, b.ToolingUtilization.Skill)
     out.WorkflowFingerprints = mergeWorkflowFingerprints(a.WorkflowFingerprints, b.WorkflowFingerprints)
     ```
     (Field-access syntax may differ slightly depending on how the function is currently structured — read the existing code first; the principle is to add three lines that wire the helpers in.)
- **Parallel?**: No — T014..T018 depend on T013.

### Subtask T014 — Aggregate tests for FR-007 / FR-008 + invariants

- **Purpose**: Lock the merge semantics with row-by-row tests and invariant tests.
- **Files**: `internal/analyzer/aggregate_test.go` (extend or create).
- **Steps**:
  1. Locate or create `aggregate_test.go`. If absent, create it next to `aggregate.go`.
  2. Test FR-007 (fingerprints by id):
     - Build inputs `A` and `B` with the same fingerprint id but different sources, evidence_counts, confidence values.
     - Assert `merge(A,B).WorkflowFingerprints[<id>].sources == sortedUnion(A.sources, B.sources)`.
     - Assert `evidence_count` summed.
     - Assert `confidence` is the max-rank.
     - Assert `active` / `installed` are OR'd.
     - Assert `version_bucket` empty when inputs disagree, retained when they agree.
  3. Test FR-008 MCP (similar row-by-row).
  4. Test FR-008 Skill (similar row-by-row).
  5. Test invariants on synthetic inputs `A`, `B`, `C`:
     - **Identity:** `merge(A, empty) == A`, `merge(empty, A) == A`.
     - **Commutativity:** `merge(A, B) == merge(B, A)` (deep-equal).
     - **Associativity:** `merge(merge(A,B), C) == merge(A, merge(B,C))` (deep-equal).
     - **Coverage:** every fingerprint id from `A ∪ B` is present in the merged output.
     - **Bounded-cardinality:** every closed-enum field in the merged output is in the input enum domain.
- **Parallel?**: [P] after T013.
- **Notes**:
  - Use small synthetic `Ecosystem` values (constructed in the test, not loaded from fixtures) to make assertions trivially readable.
  - `reflect.DeepEqual` is fine for commutativity / associativity comparisons.

### Subtask T015 — Privacy canary across merged output (NFR-002)

- **Purpose**: Prove the merge doesn't accidentally leak private names.
- **Files**: `internal/analyzer/leak_test.go`. Also reference `internal/analyzer/golden_test.go::TestPrivacyLeakCorpus` as the existing template — that test already iterates fixtures `06-private-only.log` and `07-mixed-known-unknown.log` and asserts forbidden substrings are absent from `Report` and `AggregateEvent` serializations of a single report. T015 extends the same idea across `AggregateReports` output.
- **Steps**:
  1. Locate the existing leak-string lists in `internal/analyzer/golden_test.go:286..330` (the `TestPrivacyLeakCorpus` substring lists for `06-private-only` and `07-mixed-known-unknown`).
  2. In `leak_test.go`, build two input reports `A` and `B` by analyzing those two fixtures through the existing `Analyze()` pipeline so the private names appear in unknown counts (never in allowlisted ID lists).
  3. Call `AggregateReports("agg-canary", []Report{A, B}, len(A.Sources)+len(B.Sources))` (or whatever the actual signature is — confirm by reading `aggregate.go:8`).
  4. Marshal the merged `Ecosystem` to JSON (`encoding/json`). Assert none of the union of both fixtures' leak strings appears in the resulting bytes.
  5. Repeat the assertion against the aggregate event payload from the merged report (`merged.AggregateEvent`).
  6. Repeat the assertion against the generated paid plugin artifact bytes produced by `internal/remediation/artifact.go:Generate()` from the merged report.
- **Parallel?**: [P] after T013.
- **Notes**:
  - Reuse the existing leak-string fixtures from `golden_test.go::TestPrivacyLeakCorpus`; do not invent new ones.
  - If `Generate()` requires additional setup (entitlement, session token, etc.), use the same setup as the existing `internal/remediation/artifact_test.go` cases.

### Subtask T016 — Update `golden_test.go`

- **Purpose**: Make the aggregate golden compare assert merged shape instead of nilling fields.
- **Files**: `internal/analyzer/golden_test.go`.
- **Steps**:
  1. Around lines 55..59, remove the lines that nil `WorkflowFingerprints` in the aggregate copies.
  2. Run the test; expect golden snapshots in `testdata/golden/` for aggregate paths to fail (because they no longer match the now-populated fingerprint fields).
  3. Inspect the diff carefully. Confirm the new content is allowlisted IDs / closed enums / counts only — no private names, no raw paths. If anything else appears, STOP and investigate.
  4. Update the golden snapshot files to reflect the merged shape.
- **Parallel?**: [P] after T013.

### Subtask T017 — Artifact test proves merged data flows to paid plugin

- **Purpose**: Lock FR-009.
- **Files**: `internal/remediation/artifact_test.go`, and (if needed) `internal/remediation/artifact.go`.
- **Steps**:
  1. Add a test `TestGenerate_MergedAggregate_FlowsToArtifact`.
  2. Construct two input `Report` values with distinct `Ecosystem.ToolingUtilization` and `Ecosystem.WorkflowFingerprints` values (e.g. report A has MCP `CallCount=5` and fingerprint `spec-kitty:high`, report B has MCP `CallCount=7` and fingerprint `github-spec-kit:medium`).
  3. Call `AggregateReports` to merge them.
  4. Call `Generate()` on the merged report.
  5. Assert the generated artifact bytes contain markers reflecting the merged values (e.g. `CallCount==12`, both fingerprint ids present). Use whatever string-marker patterns the existing artifact tests use.
  6. If the assertion fails because `safeKnownEcosystem` (`artifact.go:502`) or any other helper inside `internal/remediation/artifact.go` strips `ToolingUtilization` and `WorkflowFingerprints` before serialization, **extend that helper** to pass the merged fields through. The current code reads only the simple string fields per research.md; the fix may be as small as adding two field assignments. This is the load-bearing reason this WP owns `internal/remediation/artifact.go` (added to `owned_files` post-analyze).
- **Parallel?**: [P] after T013.

### Subtask T018 — Timing test for NFR-005

- **Purpose**: Lock the 100-input < 5s ceiling.
- **Files**: `internal/analyzer/aggregate_test.go`.
- **Steps**:
  1. Add a test `TestAggregateReports100_PerfCeiling` (use `testing.T`, not `testing.B`, so it runs in CI by default).
  2. Build 100 synthetic input reports based on the largest bundled fixture data (or generated values of comparable size).
  3. Time `AggregateReports` with `time.Now()` bookends.
  4. Assert elapsed < 5 seconds; fail with a clear message naming the actual elapsed time on failure.
- **Parallel?**: [P] after T013.
- **Notes**:
  - If the test is flaky on slow CI, raise the budget to 7s with a comment explaining the headroom. Do NOT skip the test.

## Test Strategy

Tests are required for FR-007, FR-008, FR-009, NFR-002, and NFR-005. Test work is bundled in T014..T018.

Run during development:

```bash
go test ./internal/analyzer/ -v
go test ./internal/remediation/ -v
```

Charter verification baseline at the end:

```bash
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
terraform -chdir=infra/aws fmt -check -recursive
./scripts/smoke-local.sh
./scripts/load-local.sh 25
```

## Risks & Mitigations

- **Risk**: Order-dependent merge output (commutativity violation).
  - **Mitigation**: T014 invariant tests catch this. Helpers sort all union outputs explicitly.
- **Risk**: Naive O(N²) implementations blow the NFR-005 budget for large inputs.
  - **Mitigation**: `unionSorted` is linear-time merge of two sorted slices. `mergeWorkflowFingerprints` groups by id with a map lookup (linear in total fingerprints).
- **Risk**: Private name leakage via a forgotten field.
  - **Mitigation**: T015 canary serializes the entire merged ecosystem AND the paid artifact AND the aggregate event payload, asserting zero leak strings.
- **Risk**: Golden snapshot drift unintended.
  - **Mitigation**: T016 step 3 instructs the implementer to inspect the diff and STOP if anything outside the closed-enum / allowlisted-ID / count space appears.
- **Risk**: Bucket boundary recomputation introduces inconsistencies.
  - **Mitigation**: This WP defaults buckets to "hold max-rank" rather than recompute, when bucket boundaries are not exposed at the merge layer. Document this in code comments and as a follow-up consideration in the PR description.
- **Risk**: `evidence_count = sum` semantics are debated later.
  - **Mitigation**: C-007 locks sum for this mission. Any change is a follow-up that updates spec, plan, and golden in lockstep.

## Review Guidance

Reviewer checkpoints for `/spec-kitty.review`:

1. **FR-007 row-by-row**: every rule in the data-model.md WorkflowFingerprints table is exercised by a test in T014.
2. **FR-008 row-by-row**: every rule in the MCPUtilization and SkillUtilization tables is exercised by a test in T014.
3. **Invariants**: identity, commutativity, associativity, coverage, privacy, bounded-cardinality all asserted in T014/T015.
4. **FR-009**: T017 proves merged values reach the paid artifact.
5. **NFR-002 canary**: T015 covers merged ecosystem JSON, paid artifact, aggregate event — all three.
6. **NFR-005 budget**: T018 asserts < 5s for 100 inputs.
7. **Golden diff**: only allowlisted IDs / closed enums / counts appear in updated snapshots — no private names, no raw paths.
8. **C-007 locked**: `evidence_count` aggregation is `sum` (not `max`). Reviewer can grep for `Sum` / `+=` in `mergeWorkflowFingerprints` to confirm.
9. **Issue #72 comments**: implementer commented at start and ready-for-review.

## Activity Log

> **CRITICAL**: Activity log entries MUST be in chronological order (oldest first, newest last). Append at the END.

Initial entry:

- 2026-05-19T11:55:54Z -- system -- Prompt created.

---

### Status Management

```bash
spec-kitty agent tasks move-task WP03 --to <status> --note "message"
```

### File Structure

All WP files live in a flat `tasks/` directory.
- 2026-05-19T12:30:57Z – claude:opus-4-7:implementer-ivan:implementer – shell_pid=50013 – Started implementation via action command
- 2026-05-19T12:42:02Z – claude:opus-4-7:implementer-ivan:implementer – shell_pid=50013 – Ready for review. mergeEcosystems extended with sorted-union/max-rank helpers; 6 invariants asserted; privacy canary across merged ecosystem + artifact + aggregate event; T017 proves artifact consumes merged data; NFR-005 timing test green. evidence_count=sum locked per C-007.
- 2026-05-19T12:42:40Z – claude:opus-4-7:reviewer-renata:reviewer – shell_pid=52554 – Started review via action command
- 2026-05-19T12:44:50Z – claude:opus-4-7:reviewer-renata:reviewer – shell_pid=52554 – Review passed: FR-007/FR-008/FR-009 covered by row-by-row tests + invariants + paid-artifact test. NFR-002 canary green across all 3 sinks. NFR-005 measured 162.8µs (30000x under ceiling). T016 deviation (kept nilling) justified — single-report path doesn't exercise merge; coverage moved to aggregate_test.go and artifact_test.go. C-007 sum locked. safeKnownEcosystem privacy gate preserved. (reviewer-renata)
