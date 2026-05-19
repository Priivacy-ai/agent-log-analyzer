---
work_package_id: WP04
title: Engine acceptance and privacy tests
dependencies:
- WP01
- WP02
- WP03
requirement_refs:
- FR-008
- FR-009
- FR-010
- FR-011
- FR-012
- FR-013
- FR-014
- FR-015
- FR-016
- FR-017
- FR-018
- FR-020
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T019
- T020
- T021
- T022
- T023
- T024
- T025
agent: claude
history:
- '2026-05-19': created from mission token-saving-recommendation-engine-phase-a-01KRZKCJ
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/token_saving_recommendations_test.go
execution_mode: code_change
owned_files:
- internal/analyzer/token_saving_recommendations_test.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading the rest of this prompt, load the assigned agent profile:

```text
/ad-hoc-profile-load implementer-ivan
```

Then continue with **Objective** below.

## Objective

Lock the engine's behaviour with a complete acceptance + invariant suite.
This WP delivers seven tests covering AS-01 … AS-13, plus determinism,
active-tool skipping, MCP-skill-bloat-never-adds-MCP, ≤ 1 primary + ≤ 1
secondary, and the **positive-list privacy scanner** that pins NFR-002 for
all of Phase A and beyond.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. Rebase onto
merged WP01+WP02+WP03 head per `lanes.json` before starting.

## Context

Primary references:

- `spec.md` §"Acceptance scenarios" (AS-01 … AS-14) and "Edge cases"
- `contracts/token_saving_engine_go_api.md` §"Invariants the engine enforces"
- `research.md` §7 "Privacy test design"
- `quickstart.md` §6 (privacy assertion pattern)

## Owned files

This WP owns and is the only writer of:

- `internal/analyzer/token_saving_recommendations_test.go` (new)

**Do not edit** any non-test file. If a test exposes an engine bug, file
that as a follow-up — do not patch the engine from a test file. If a
genuinely missing helper (e.g. a sort helper) needs to live in
non-test code, **stop and report**; do not cross WP ownership.

## Implementation command

```bash
spec-kitty agent action implement WP04 --agent claude
```

---

### Subtask T019 — Test scaffold + shared helpers + privacy scanner

**Purpose.** Centralize the helpers every test will reuse.

**Steps.**

1. Create `internal/analyzer/token_saving_recommendations_test.go` with
   `package analyzer` and the import block: `bytes`, `encoding/json`,
   `regexp`, `sort`, `strings`, `testing`, `unicode/utf8`.
2. Add the shared helpers:

   ```go
   func buildState(entries ...ToolStateEntry) ToolStateMap { … }
   func marshalSet(t *testing.T, set RecommendationSet) []byte { … }
   ```

3. Add the **positive-list privacy scanner**. Build the allowlist set once
   in an `init()`-style helper (cached in a package-private var if you
   prefer):

   ```go
   func recommendationAllowlist() map[string]bool {
       allow := map[string]bool{}
       // Every ToolID present in AllTools()
       for _, t := range AllTools() { allow[string(t.ID)] = true }
       // Every enum string value
       for _, v := range allToolStates() { allow[string(v)] = true }
       for _, v := range allEvidenceSources() { allow[string(v)] = true }
       for _, v := range allSignals() { allow[string(v)] = true }
       for _, v := range allRecommendationClasses() { allow[string(v)] = true }
       for _, v := range allConfidences() { allow[string(v)] = true }
       for _, v := range allRiskLevels() { allow[string(v)] = true }
       for _, v := range allInstallPolicies() { allow[string(v)] = true }
       for _, v := range allReasons() { allow[string(v)] = true }
       // Field names and structural fixed strings
       for _, v := range []string{
           "recommendation_id","primary_tool_id","skipped_tool_ids",
           "reason","signal_ids","confidence","risk_level","install_policy",
           "evidence_counts","tool_id","for_signal","primary","secondary",
           "skipped","registry_version","engine_version","signals",
           "unknown_id_count","rec","none",
           RegistryVersion(), EngineVersion(),
       } { allow[v] = true }
       return allow
   }

   func findNonAllowlistedSubstrings(jsonBlob []byte) []string {
       allow := recommendationAllowlist()
       // tokenize: split on any character that is NOT [a-z0-9_.-]
       re := regexp.MustCompile(`[a-zA-Z0-9_.-]+`)
       var leaks []string
       for _, tok := range re.FindAllString(string(jsonBlob), -1) {
           if allow[tok] { continue }
           // bare integers (counts) are fine
           if isAsciiDigits(tok) { continue }
           leaks = append(leaks, tok)
       }
       return leaks
   }
   ```

4. The eight `all*()` helpers (`allSignals`, etc.) live in this test file
   too — each returns the static slice of constants from
   `token_saving_types.go`. They give the privacy scanner a single source
   of truth.

**Files.** `token_saving_recommendations_test.go` grows to ~150 lines
after this subtask.

---

### Subtask T020 — Table-driven AS-01 … AS-13 scenarios

**Purpose.** One row per acceptance scenario, each asserting `Primary`,
`Reason`, and (where the brief specifies) `Secondary` and `Skipped`.

**Steps.**

1. Add `TestRecommend_AcceptanceScenarios(t *testing.T)` with a
   `[]struct{ name string; signals []Signal; state ToolStateMap; want acceptanceExpectation }` table.
2. The `acceptanceExpectation` struct captures:
   - `primary ToolID` (or empty for no-op),
   - `primaryReason Reason`,
   - `secondary ToolID` (optional),
   - `skipped []SkipNote` (optional, sorted).
3. Encode all 13 rows from `spec.md` §"Acceptance scenarios". Example:

   ```go
   {
       name: "AS-03 shell bloat, RTK absent",
       signals: []Signal{SignalShellOutputBloat},
       state: buildState(),
       want: acceptanceExpectation{primary: "rtk", primaryReason: ReasonAbsent},
   },
   {
       name: "AS-10 repeated reads, Serena active",
       signals: []Signal{SignalRepeatedFileReads},
       state: buildState(ToolStateEntry{
           Tool: "serena", State: ToolStateActiveHigh,
           Sources: map[EvidenceSource]int{EvidenceLogActiveCommand: 4},
       }),
       want: acceptanceExpectation{
           primary: "", // engine advances to next eligible (research_only entries are skipped)
           skipped: []SkipNote{{ToolID: "serena", Reason: ReasonActivePersistent, ForSignal: SignalRepeatedFileReads}},
       },
   },
   ```

4. For each row, the subtest calls `Recommend`, then asserts the relevant
   fields of `RecommendationSet`. Use `t.Run(tt.name, …)` so failures are
   per-row.

**Files.** `token_saving_recommendations_test.go` grows to ~350 lines.

**Validation.** All 13 sub-tests green.

---

### Subtask T021 — `TestRecommendDeterminism`

**Purpose.** NFR-001 byte-identical JSON.

**Steps.**

1. Build a slice of 50 deterministic inputs from a fixed seed (e.g.
   iterate Cartesian product of `signals × tool states`, take the first
   50, sorted).
2. For each input: call `Recommend` twice, marshal each result with
   `json.Marshal`, assert `bytes.Equal(a, b)`. If not equal, `t.Errorf`
   with both JSON blobs and stop the loop at the first failure (so the
   diff is readable).

**Validation.** `go test -run TestRecommendDeterminism` passes.

---

### Subtask T022 — `TestRecommendSkipsActiveTool`

**Purpose.** FR-017 / AS-10 across every signal whose first-choice
candidate could otherwise be re-recommended.

**Steps.**

1. For each row in `rulePrecedence` (covered by `firingRules`
   indirectly), find the first eligible candidate. If
   `eligibleCandidates(rule.Class)` is empty (all entries
   `research_only`), skip the row.
2. Build a `state` with that candidate marked `ToolStateActiveHigh`.
3. Call `Recommend` with the rule's first firing signal. Assert:
   - the candidate's ID appears in `Skipped` with
     `Reason == ReasonActivePersistent`,
   - the candidate's ID does **not** appear as
     `set.Primary.PrimaryToolID`.

**Validation.** Passes for every covered rule.

---

### Subtask T023 — `TestRecommendMCPSkillBloatNeverAddsMCP`

**Purpose.** FR-013 / AS-11 across `mcp_skill_bloat` plus every other
signal.

**Steps.**

1. For each `Signal s` in `allSignals()`:
   - call `Recommend([]Signal{SignalMCPSkillBloat, s}, ToolStateMap{})`,
   - if `set.Primary != nil`, assert
     `set.Primary.PrimaryToolID == "" || tool.RecommendationClass == ClassMCPSkillHygiene`
     (the primary must be in the hygiene class).
2. Also assert that no emitted recommendation's class is
   `ClassMCPOutputReducer` or `ClassRetrieval` for this signal-combo.

**Validation.** Passes.

---

### Subtask T024 — `TestRecommendOnePlusOne`

**Purpose.** C-006 — at most one primary, at most one secondary, distinct
classes when both present.

**Steps.**

1. Sweep every pair `(s1, s2)` from `allSignals()` (including
   `s1 == s2`).
2. Call `Recommend([]Signal{s1, s2}, ToolStateMap{})`.
3. Assert: `set.Primary` and `set.Secondary` are either both nil, one
   nil + one non-nil, or both non-nil. **Never both pointing at the same
   tool**. When both non-nil, look up each tool in `AllTools()` and
   assert their `RecommendationClass` differ.

**Validation.** Passes.

---

### Subtask T025 — `TestRecommendPrivacyBudget`

**Purpose.** AS-14 / NFR-002 — positive-list scan of marshalled JSON.

**Steps.**

1. Build a `state` that includes deliberately private-looking decoy
   entries with **unknown** `ToolID`s (e.g.
   `"private_company_secret_tool"`). The engine should drop these and
   bump `UnknownIDCount`.
2. Run the engine for every signal in `allSignals()` plus a handful of
   pair-signal cases.
3. For every produced `RecommendationSet`:
   - marshal with `json.Marshal`,
   - call `findNonAllowlistedSubstrings(jsonBlob)`,
   - assert it returns an empty slice; on failure include both the leak
     list and the blob in `t.Errorf`.
4. Additionally assert `set.UnknownIDCount > 0` for the decoy-bearing
   case — proving the decoys were observed and counted, then discarded.

**Validation.** Passes; this is the single test the reviewer will scrutinise.

---

## Definition of Done

- [ ] `internal/analyzer/token_saving_recommendations_test.go` exists with
      every test listed above plus the shared helpers and `all*()`
      enumerators.
- [ ] `go test ./internal/analyzer/ -run TokenSavingRecommend` is fully
      green.
- [ ] `go test ./...` is fully green.
- [ ] `gofmt -w` and `go vet` clean on the new test file.
- [ ] No non-test file is modified.
- [ ] The test file is < 700 lines.

## Risks & reviewer guidance

- **Privacy scanner false negatives.** Run with a deliberately failing
  input (e.g. inject a literal `"sk-ant-FAKE"` into a `Sources` map key)
  to confirm the scanner catches it. Then revert.
- **Coverage of `research_only` tools.** Many retrieval-tier tools in
  the registry ship `research_only` and are not eligible for default
  emission. Some AS-* assertions should assert `Primary == nil` rather
  than naming a tool — verify against research.md §"Per-tool research
  notes" which IDs are recommend-eligible.
- **Test runtime.** All seven tests together should finish in under a
  second on a developer laptop; if any single test takes > 200 ms, look
  for accidental loops over the entire registry on every assertion.

## Out of scope for WP04

- Engine changes — file ownership forbids this. If a test catches a bug
  in WP03's engine, file a fix-up WP rather than reaching across
  boundaries.
- Doc updates (WP05/WP06).
- Registry changes (WP01).
