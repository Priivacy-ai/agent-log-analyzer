---
work_package_id: WP03
title: Engine decision logic
dependencies:
- WP01
- WP02
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
agent: claude
history:
- '2026-05-19': created from mission token-saving-recommendation-engine-phase-a-01KRZKCJ
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/token_saving_recommendations.go
execution_mode: code_change
owned_files:
- internal/analyzer/token_saving_recommendations.go
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

Implement the deterministic recommendation engine: a fixed 8-step rule
precedence list, candidate selection over the WP01 registry, per-rule state
machine, secondary selection with non-overlapping class, and the public
`Recommend(signals, state)` entry point. **All map iteration goes through
WP02's sort helpers** — no naked `range` over a `map` anywhere in this file.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. Rebase onto the
merged WP01+WP02 head before starting (per `lanes.json`). Read it via
`spec-kitty agent context resolve --mission
token-saving-recommendation-engine-phase-a-01KRZKCJ --wp WP03 --json`.

## Context

Primary references:

- `research.md` §3 (rule precedence) and §6 (determinism implementation)
- `data-model.md` §"State transitions" (engine pseudocode)
- `contracts/token_saving_engine_go_api.md` (frozen `Recommend` signature)
- `spec.md` FR-008 … FR-018 (rule semantics) and AS-01 … AS-13 (acceptance)

## Owned files

This WP owns and is the only writer of:

- `internal/analyzer/token_saving_recommendations.go` (new)

**Do not edit** `token_saving_tools.go`, `token_saving_types.go`, any test
file, or any existing analyzer file.

## Implementation command

```bash
spec-kitty agent action implement WP03 --agent claude
```

---

### Subtask T012 — File scaffold

**Purpose.** Stand up the file with a clear header and the package import
block.

**Steps.**

1. Create `internal/analyzer/token_saving_recommendations.go` with
   `package analyzer`.
2. Header comment names this as Phase A engine logic and points at
   `data-model.md` §"State transitions" for the high-level flow.
3. Imports: `encoding/json` (only if a helper needs it — usually no),
   `sort`, `strings`. **No new third-party imports**.

**Validation.** `go build ./...` clean after T013.

---

### Subtask T013 — Encode the fixed 8-step rule precedence

**Purpose.** Make precedence a static, auditable table.

**Steps.**

1. Declare:

   ```go
   type ruleSpec struct {
       FiringSignals []Signal             // any of these triggers the rule
       Class         RecommendationClass  // class the rule recommends from
       PrimaryReason Reason               // reason emitted on the "absent" branch
   }

   var rulePrecedence = []ruleSpec{
       { FiringSignals: []Signal{SignalNoUsageVisibility},    Class: ClassUsageVisibility,    PrimaryReason: ReasonAbsent },
       { FiringSignals: []Signal{SignalMCPSkillBloat},        Class: ClassMCPSkillHygiene,    PrimaryReason: ReasonPruneFirst },
       { FiringSignals: []Signal{SignalMCPToolOutputBloat},   Class: ClassMCPOutputReducer,   PrimaryReason: ReasonAbsent },
       { FiringSignals: []Signal{SignalShellOutputBloat,      SignalToolOutputBloat}, Class: ClassShellOutputReducer, PrimaryReason: ReasonAbsent },
       { FiringSignals: []Signal{SignalRepeatedFileReads,     SignalBroadRepoExploration}, Class: ClassRetrieval, PrimaryReason: ReasonAbsent },
       { FiringSignals: []Signal{SignalUnchangedFileRereads}, Class: ClassRereadGuard,       PrimaryReason: ReasonAbsent },
       { FiringSignals: []Signal{SignalRetryLoop,             SignalContextGrowthSpikes}, Class: ClassContextHygiene, PrimaryReason: ReasonAuditConfig },
       { FiringSignals: []Signal{SignalOutputVerbosity},      Class: ClassOutputVerbosity,   PrimaryReason: ReasonAbsent },
   }
   ```

2. Above the `var` block, add a comment that this ordering is the
   contract documented in research.md §3 and that altering it must bump
   `EngineVersion()`.

**Validation.** Sanity-check that every documented `Signal` value appears
in at least one `FiringSignals` (WP04 has an exhaustiveness test).

---

### Subtask T014 — `signalsForRule` and firing-rule selection

**Purpose.** Translate an input signal set into the ordered list of rules
that should be evaluated.

**Steps.**

1. Implement `func firingRules(signals []Signal) []ruleSpec` that:
   - builds a `map[Signal]bool` from `signals`,
   - iterates `rulePrecedence` in order (deterministic by construction),
   - keeps every rule whose `FiringSignals` intersects the input set,
   - returns the filtered slice.
2. Add a small helper `func ruleFires(r ruleSpec, present map[Signal]bool) bool`
   to keep `firingRules` readable.

**Validation.** Covered by WP04's `TestRecommendOnePlusOne` precedence
loop.

---

### Subtask T015 — Candidate selection over the registry

**Purpose.** Given a `RecommendationClass`, return the eligible tool to
recommend (or nothing).

**Steps.**

1. Implement `func eligibleCandidates(class RecommendationClass) []TokenSavingTool`:
   - filter `AllTools()` by `RecommendationClass == class`,
   - drop entries with `InstallPolicy == "research_only"` or
     `InstallPolicy == "reference_only"` (these never appear as
     recommendations by default),
   - return the slice already sorted by `ClassRank` (`AllTools()` returns
     class+rank order).

2. Implement `func pickPrimary(rule ruleSpec, state ToolStateMap) (toolID ToolID, reason Reason, skipped []SkipNote, ok bool)`:
   - for the rule's relevant firing signal (the first one present in
     `state`'s caller-side input; for primary selection we use the rule's
     class regardless of which firing signal triggered),
   - walk `eligibleCandidates(rule.Class)` in order,
   - inspect `state[candidate.ID].State` (defaulting to `unknown`):
     - `ToolStateActiveHigh` → append `SkipNote{candidate.ID, ReasonActivePersistent, rule.FiringSignals[0]}` and continue.
     - `ToolStateRejectedMedium` → append `SkipNote{candidate.ID, ReasonRejectedAlternative, rule.FiringSignals[0]}` and continue.
     - `ToolStateConfiguredMedium` → return `(candidate.ID, ReasonConfiguredInactive, skipped, true)`.
     - `ToolStateInstalledMedium` → return `(candidate.ID, ReasonInstalledInactive, skipped, true)`.
     - `ToolStateMentionedLow` / `ToolStateUnknown` → return `(candidate.ID, rule.PrimaryReason, skipped, true)`.
   - if no candidate qualifies, return `("", "", skipped, false)`.

**Validation.** WP04's table-driven AS-01 … AS-13 tests cover every branch.

---

### Subtask T016 — Per-rule special cases

**Purpose.** A few rules need overrides on top of the generic state machine.

**Steps.**

1. **`no_usage_visibility` + `ccusage` already `active_high`** (AS-02): if
   the candidate selected for the `usage_visibility` class is `ccusage`
   and `state[ccusage].State == ToolStateActiveHigh`, additionally check
   whether the input `state` carries `EvidenceFailureOrRejection` for
   `ccusage` (the agreed proxy for "server quota mismatch evidence"). If
   yes, emit a recommendation with `Reason = ReasonServerQuotaCheck` and
   `PrimaryToolID = "ccusage"` (the operational guidance is in the doc;
   the engine just emits the reason enum).

2. **`mcp_skill_bloat`** (FR-013, AS-11): when this rule fires, the
   primary's class **must** be `ClassMCPSkillHygiene`. Because the
   precedence list places `mcp_skill_bloat` ahead of `mcp_tool_output_bloat`,
   `repeated_file_reads`, etc., the engine naturally picks the prune-first
   class. Add an assertion-style guard at the end of `Recommend` that
   panics in test builds (use `// +build` is overkill — instead, a small
   internal sanity check that returns a synthetic recommendation with
   `Reason = ReasonNoOp` if the invariant is violated; WP04's
   `TestRecommendMCPSkillBloatNeverAddsMCP` will fail loudly).

3. **No-op** (empty signals or no firing rule produces a candidate):
   `Primary = nil`, `Secondary = nil`, `Skipped = nil`, `Reason` is not
   emitted (there's no recommendation to attach it to).

**Validation.** AS-02 and AS-11 are dedicated WP04 scenarios.

---

### Subtask T017 — Secondary selection

**Purpose.** At most one secondary, must be a different class from the
primary, sourced from the next firing rule.

**Steps.**

1. After `pickPrimary` returns successfully for the first firing rule,
   walk the remaining firing rules. Skip any rule whose `Class` matches
   the primary's `Class`.
2. For the first remaining rule whose class differs, call `pickPrimary`
   on it (yes, reuse the same function — primary-selection logic applies
   identically). If it succeeds, that's the secondary.
3. If no remaining rule yields a successful candidate, leave
   `Secondary = nil`.

**Validation.** WP04's `TestRecommendOnePlusOne`.

---

### Subtask T018 — Public `Recommend` entry point

**Purpose.** Assemble the `RecommendationSet`.

**Steps.**

1. Implement:

   ```go
   func Recommend(signals []Signal, state ToolStateMap) RecommendationSet {
       sortedSignals := sortedSignalIDs(signals)
       knownSet := registeredSignals()
       var validSignals []Signal
       unknownCount := 0
       for _, s := range sortedSignals {
           if knownSet[s] {
               validSignals = append(validSignals, s)
           } else {
               unknownCount++
           }
       }
       // ... continue with conflict-resolve per tool, drive rule loop ...
   }
   ```

2. Build `validState ToolStateMap` from `state` by:
   - dropping entries whose `Tool` is not in `AllTools()` (and incrementing
     `UnknownIDCount`),
   - collapsing any per-tool duplicates via `ToolStateMap.Resolve` (the
     caller is unlikely to supply duplicates, but be defensive: if the
     same tool has two `Sources` entries with conflicting `State`, resolve
     them).
3. Call `firingRules(validSignals)`. If empty: return the no-op
   `RecommendationSet` with `RegistryVersion: RegistryVersion()`,
   `EngineVersion: EngineVersion()`, `Signals: validSignals`,
   `UnknownIDCount: unknownCount`.
4. Otherwise, pick the primary from the first firing rule. Apply secondary
   selection (T017). Sort `skipped` by `(ToolID, ForSignal)` ascending.
5. Compose `recommendation_id` for each emitted recommendation using the
   scheme from research.md §2:
   `"rec." + class + "." + toolID + "." + strings.Join(stringSlice(signalIDs), "_")`.
   When `signalIDs` is empty (no-op), use the literal `"none"` suffix.
6. Set `EvidenceCounts` on each emitted recommendation to a copy of
   `state[primary].Sources`, dropping any unknown evidence-source keys
   defensively (`registeredEvidenceSources()` returns the allowlist).
7. Compute `Confidence` deterministically:
   - if the candidate's underlying `ToolState` is `ToolStateActiveHigh`
     → `ConfidenceHigh`
   - else if `ToolStateConfiguredMedium`, `ToolStateInstalledMedium`, or
     `ToolStateRejectedMedium` → `ConfidenceMedium`
   - else → `ConfidenceLow`
8. Set `RiskLevel = candidate.InstallRisk` and
   `InstallPolicy = candidate.InstallPolicy`.

**Validation.** `go vet ./internal/analyzer/` clean; `go build ./...`
succeeds. The acceptance suite is in WP04.

---

## Definition of Done

- [ ] `internal/analyzer/token_saving_recommendations.go` exists and
      compiles against WP01+WP02.
- [ ] `Recommend`, `firingRules`, `eligibleCandidates`, `pickPrimary` are
      implemented per the steps above.
- [ ] Every map iteration in this file routes through a sort helper from
      WP02. (Reviewer: `grep -nE 'for[[:space:]]+[a-zA-Z_]+,?[[:space:]]*[a-zA-Z_]*[[:space:]]+:=[[:space:]]+range[[:space:]]+[a-zA-Z_]+$'` should not find a naked map-range.)
- [ ] `go vet ./internal/analyzer/` and `go build ./...` succeed.
- [ ] No file outside `owned_files` is modified.
- [ ] `gofmt -w internal/analyzer/token_saving_recommendations.go` is clean.
- [ ] The file is < 500 lines.

## Risks & reviewer guidance

- **Map iteration leaking into output** is the single biggest determinism
  risk. Reviewer should grep for `range ` over any `map[…]…` and confirm
  every match has a paired sort helper.
- **`Confidence` derivation** is policy, not contract — if reviewers
  disagree, document the change in `research.md` §6 and bump
  `EngineVersion()`.
- **Reason enum coverage** — every branch in `pickPrimary` returns a
  `Reason` from the documented enum; WP04's `TestEnumsAreClosed` (a
  helper test included in WP04's suite) will catch any new value.

## Out of scope for WP03

- All tests live in WP04.
- All types and helpers live in WP02.
- The registry literal lives in WP01.
- Docs live in WP05/WP06.
