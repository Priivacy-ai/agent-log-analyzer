# Tasks: Token-Saving Recommendation Engine (Phase A)

**Mission**: `token-saving-recommendation-engine-phase-a-01KRZKCJ`
**Mission ID**: `01KRZKCJN2VCSE6M6T29VHZS96`
**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · **Research**: [research.md](research.md) · **Data Model**: [data-model.md](data-model.md) · **Contract**: [contracts/token_saving_engine_go_api.md](contracts/token_saving_engine_go_api.md) · **Quickstart**: [quickstart.md](quickstart.md)
**Planning base branch**: `main` · **Merge target branch**: `main`
**Work branch (planned)**: `codex/token-recommendations-phase-a` (timestamp suffix if collision)

## Planning refinement (logged here, not a contract change)

Plan.md proposed one combined `internal/analyzer/token_saving_recommendations.go`
file holding both the engine types and the engine logic. For clean work-package
ownership we split that into two new files:

- `internal/analyzer/token_saving_types.go` (enums, structs, `ToolStateMap`,
  `EngineVersion()`)
- `internal/analyzer/token_saving_recommendations.go` (rule precedence,
  candidate selection, `Recommend()`)

This is purely a file-layout refinement; the public Go surface in
`contracts/token_saving_engine_go_api.md` is unchanged.

## Subtask Index

| ID   | Description                                                              | WP   | Parallel |
| ---- | ------------------------------------------------------------------------ | ---- | -------- |
| T001 | Create `token_saving_tools.go` with `TokenSavingTool` struct             | WP01 |          |
| T002 | Encode registry Go literal with all allowlist entries (research.md ¶8)   | WP01 |          |
| T003 | Implement `GetTool`, `AllTools`, `RegistryVersion`                       | WP01 |          |
| T004 | Create `token_saving_tools_test.go` with `TestRegistryInvariants`        | WP01 | [P]      |
| T005 | Add `TestRegistryAllowlistCoverage` (every brief-listed ID present)      | WP01 | [P]      |
| T006 | Create `token_saving_types.go`                                           | WP02 |          |
| T007 | Declare all enum types + constants (ToolState/EvidenceSource/Signal/…)   | WP02 |          |
| T008 | Declare `ToolStateEntry`, `ToolStateMap`                                 | WP02 |          |
| T009 | Implement `ToolStateMap.Resolve` (conflict precedence)                   | WP02 |          |
| T010 | Implement `ToolStateMap.SortedTools` and `sortedSignalIDs` helpers       | WP02 |          |
| T011 | Declare output structs (`TokenSavingRecommendation`, `SkipNote`, `RecommendationSet`) + `EngineVersion()` | WP02 |          |
| T012 | Create `token_saving_recommendations.go` skeleton                        | WP03 |          |
| T013 | Encode fixed 8-step rule precedence list                                 | WP03 |          |
| T014 | Implement `signalsForRule` helper (signal → rule mapping)                | WP03 |          |
| T015 | Implement primary-tool candidate selection (registry filter, eligibility)| WP03 |          |
| T016 | Implement per-rule state-machine branches (absent / installed / configured / active / rejected) | WP03 |          |
| T017 | Implement secondary-tool selection (next rule, non-overlapping class)    | WP03 |          |
| T018 | Implement `Recommend` entry point (validate, count unknowns, assemble)   | WP03 |          |
| T019 | Create `token_saving_recommendations_test.go` with shared helpers + privacy scanner | WP04 |          |
| T020 | Add table-driven tests for AS-01 … AS-13                                 | WP04 |          |
| T021 | Add `TestRecommendDeterminism` (byte-equal JSON, NFR-001)                | WP04 |          |
| T022 | Add `TestRecommendSkipsActiveTool` (FR-017, AS-10)                       | WP04 |          |
| T023 | Add `TestRecommendMCPSkillBloatNeverAddsMCP` (FR-013, AS-11)             | WP04 |          |
| T024 | Add `TestRecommendOnePlusOne` (C-006: ≤1 primary + ≤1 secondary, distinct classes) | WP04 |          |
| T025 | Add `TestRecommendPrivacyBudget` (AS-14, NFR-002 positive-list scan)     | WP04 |          |
| T026 | Create `docs/remediation/token-saving-recommendation-engine.md` skeleton | WP05 | [P]      |
| T027 | Document tool classes, allowlist policy, dedupe contract                 | WP05 | [P]      |
| T028 | Document state model + recommendation contract (rule precedence, skip behaviour, ≤1+1) | WP05 | [P]      |
| T029 | Document risk levels, install policies, waiver gate                      | WP05 | [P]      |
| T030 | Document privacy constraints + Phase B integration plan for #38/#39      | WP05 | [P]      |
| T031 | Update `docs/remediation/token-saving-tooling-matrix.md` (additive: registry cross-ref + dedupe note) | WP06 |          |
| T032 | Update `docs/remediation/plugin-artifacts.md` (additive: recommendation-object note) | WP06 |          |
| T033 | Add cross-reference line to engine doc in both updated files             | WP06 |          |

**Subtask count**: 33 across 6 work packages.

---

## Phase 1 — Foundation (must complete first)

### WP01 — Token-saving tool registry

**Goal**: Ship an additive, code-owned registry of token-saving tools with a
small public lookup API and a test that locks the registry's invariants.

**Priority**: P0 (foundation — every later WP imports `TokenSavingTool` / `ToolID`).
**Independent test**: `go test ./internal/analyzer/ -run TokenSavingTools`
**Estimated prompt size**: ~420 lines (5 subtasks).

**Included subtasks**

- [x] T001 Create `internal/analyzer/token_saving_tools.go` with the `TokenSavingTool` struct (fields per data-model.md). (WP01)
- [x] T002 Encode the registry as a package-level Go literal `var registry = []TokenSavingTool{ … }` containing every entry from research.md §"Per-tool research notes". (WP01)
- [x] T003 Implement `GetTool(id ToolID)`, `AllTools()` (defensive copy, sorted by class+rank), and `RegistryVersion()` returning the constant `"phase-a-2026-05-19"`. (WP01)
- [x] T004 [P] Create `internal/analyzer/token_saving_tools_test.go` with `TestRegistryInvariants` enforcing every invariant from data-model.md ¶"Invariants". (WP01)
- [x] T005 [P] Add `TestRegistryAllowlistCoverage` asserting every brief-listed ID (`ccusage`, `rtk`, `context_mode`, …) appears exactly once in `AllTools()`. (WP01)

**Implementation sketch**: write the type, encode the literal, expose three pure functions, write the invariant tests. No engine logic touches this file.

**Parallel opportunities**: T004/T005 are parallel with each other; they run after T001–T003.

**Dependencies**: none.

**Risks**: forgetting to set `RollbackGuidance` on a `recommend_with_waiver` entry — caught by `TestRegistryInvariants`.

**Prompt file**: `tasks/WP01-token-saving-tool-registry.md`

---

### WP02 — Engine types, enums, and conflict resolution

**Goal**: Declare every enum and struct the engine and tests will consume, plus the `ToolStateMap.Resolve` precedence helper. No business logic here.

**Priority**: P0.
**Independent test**: `go test ./internal/analyzer/ -run TokenSavingTypes` (the conflict-resolution test lives in WP04's test file; this WP's correctness is verified by WP04 running green).
**Estimated prompt size**: ~380 lines (6 subtasks).

**Included subtasks**

- [x] T006 Create `internal/analyzer/token_saving_types.go` with the file header comment naming this as Phase A additive code. (WP02)
- [x] T007 Declare named string types and constants for `ToolState`, `EvidenceSource`, `Signal`, `RecommendationClass`, `Confidence`, `RiskLevel`, `InstallPolicy`, `Reason` (exact values from data-model.md). (WP02)
- [x] T008 Declare `ToolStateEntry` struct and `type ToolStateMap map[ToolID]ToolStateEntry`. (WP02)
- [x] T009 Implement `ToolStateMap.Resolve(state1, state2 ToolState) ToolState` using precedence `rejected_medium > active_high > configured_medium > installed_medium > mentioned_low > unknown`. (WP02)
- [x] T010 Implement deterministic helpers `(ToolStateMap).SortedTools() []ToolID` and `sortedSignalIDs(in []Signal) []Signal`. (WP02)
- [x] T011 Declare output structs (`TokenSavingRecommendation`, `SkipNote`, `RecommendationSet`) with the JSON tags from contracts/, and an `EngineVersion()` returning the constant `"v0.1-phase-a"`. (WP02)

**Implementation sketch**: pure type declarations + two small helpers. No imports beyond `sort` and `strings` for the helpers.

**Parallel opportunities**: none (single file, sequential within WP).

**Dependencies**: WP01 (uses `ToolID`).

**Risks**: drift between data-model.md enum values and the constants here — mitigated by WP04's `TestEnumsAreClosed`.

**Prompt file**: `tasks/WP02-engine-types-and-enums.md`

---

### WP03 — Engine decision logic

**Goal**: Implement the deterministic `Recommend(signals, state) RecommendationSet` function, the fixed 8-step rule precedence, candidate selection, and the per-rule state machine.

**Priority**: P0.
**Independent test**: covered by WP04's engine tests. After WP03 lands, `go vet` and `go build` on the package must pass even before WP04's tests are written.
**Estimated prompt size**: ~480 lines (7 subtasks).

**Included subtasks**

- [x] T012 Create `internal/analyzer/token_saving_recommendations.go` with `package analyzer` and the file header comment. (WP03)
- [x] T013 Encode the fixed 8-step rule precedence list (research.md §3) as a package-level `var rulePrecedence = []ruleSpec{…}` slice. (WP03)
- [x] T014 Implement `signalsForRule` lookup and a helper that returns the firing rules for a given input signal set, in precedence order. (WP03)
- [x] T015 Implement candidate selection: filter registry by `RecommendationClass`, sort by `ClassRank`, exclude `research_only` / `reference_only` entries from default emission, return the first eligible tool plus a fallback chain. (WP03)
- [x] T016 Implement the per-rule state machine: for the selected candidate, produce `(Reason, SkipNote?)` for each `ToolState` branch per FR-009 … FR-015. (WP03)
- [x] T017 Implement secondary-tool selection: scan the remaining firing rules, skip any whose class matches the primary's class, repeat candidate selection. Enforce the ≤ 1 primary + ≤ 1 secondary invariant. (WP03)
- [x] T018 Implement the `Recommend` entry point: dedupe and sort signals, validate against the registered `Signal` enum (unknown signals counted into `UnknownIDCount`), run conflict resolution per tool, drive the rule loop, assemble `RecommendationSet{Primary, Secondary, Skipped, …}` with sorted slice fields. (WP03)

**Implementation sketch**: a small typed state machine driven by a static rule table. All map iteration goes through the deterministic helpers from WP02.

**Parallel opportunities**: none (single file, sequential within WP).

**Dependencies**: WP01, WP02.

**Risks**: accidentally returning `Primary` and `Secondary` with the same `RecommendationClass` — caught by WP04's `TestRecommendOnePlusOne`.

**Prompt file**: `tasks/WP03-engine-decision-logic.md`

---

## Phase 2 — Verification

### WP04 — Engine acceptance and privacy tests

**Goal**: Cover every acceptance scenario AS-01…AS-14 from spec.md, plus the
determinism, no-MCP-stacking, one-primary-one-secondary, and privacy-budget
invariants. Ship the positive-list privacy scanner that pins NFR-002.

**Priority**: P0 (mission DoD requires `go test ./...` green).
**Independent test**: `go test ./internal/analyzer/ -run TokenSavingRecommend`
**Estimated prompt size**: ~520 lines (7 subtasks).

**Included subtasks**

- [x] T019 Create `internal/analyzer/token_saving_recommendations_test.go` with the table-driven test scaffold, shared helpers (`buildState`, `marshalSet`), and the package-private allowlist string set used by the privacy scanner. (WP04)
- [x] T020 Add 13 table-driven scenarios for AS-01 … AS-13 with explicit expected `Primary.PrimaryToolID`, `Reason`, `Skipped[*].ToolID`, and (where the brief specifies) `Secondary.PrimaryToolID`. (WP04)
- [x] T021 Add `TestRecommendDeterminism` that calls `Recommend` twice with the same input and asserts `bytes.Equal(json.Marshal(a), json.Marshal(b))` over 50 randomized-input pairs (random seed is fixed). (WP04)
- [x] T022 Add `TestRecommendSkipsActiveTool` covering FR-017 / AS-10 across every `Signal` whose first-choice candidate is `active_high`. (WP04)
- [x] T023 Add `TestRecommendMCPSkillBloatNeverAddsMCP` covering FR-013 / AS-11 across all combinations of `mcp_skill_bloat` + any other signal. (WP04)
- [x] T024 Add `TestRecommendOnePlusOne` (C-006) sweeping every pair of firing signals and asserting (a) at most one Primary, (b) at most one Secondary, (c) when both exist their `RecommendationClass` differ. (WP04)
- [x] T025 Add `TestRecommendPrivacyBudget` (AS-14 / NFR-002): build inputs containing deliberately private-looking decoy `ToolID`s, marshal the result, and assert via `findNonAllowlistedSubstrings` that every byte is either an allowlisted enum string, a structural JSON character, an ASCII digit, period, underscore, or whitespace. (WP04)

**Implementation sketch**: one test file, ~10 tests, all using table-driven patterns and shared helpers. The privacy scanner is the centerpiece — it's the single source of truth for NFR-002.

**Parallel opportunities**: T021/T022/T023/T024/T025 can be filled out in any order once the scaffold in T019 lands.

**Dependencies**: WP01, WP02, WP03.

**Risks**: brittle expected-output comparisons if researchers later promote a `research_only` tool — mitigated by referring to tools by their `RecommendationClass` rank rather than hard-coded IDs in the assertions where feasible.

**Prompt file**: `tasks/WP04-engine-acceptance-privacy-tests.md`

---

## Phase 3 — Documentation

### WP05 — Engine documentation (new doc)

**Goal**: Write the new `docs/remediation/token-saving-recommendation-engine.md`
covering classes, allowlist policy, dedupe contract, state model, risk and
install policy, waiver gate, privacy constraints, and Phase B integration
plan.

**Priority**: P1 (required by DoD; can start in parallel with WP01).
**Independent test**: human review against the FR-022 checklist + `markdownlint` clean (if configured).
**Estimated prompt size**: ~340 lines (5 subtasks).

**Included subtasks**

- [x] T026 [P] Create `docs/remediation/token-saving-recommendation-engine.md` with the document header, audience, and table of contents. (WP05)
- [x] T027 [P] Document the eight `RecommendationClass` values + the full allowlist with one-line rationales (cross-reference the registry source file, do not duplicate field-by-field tables). (WP05)
- [x] T028 [P] Document the state model (six `ToolState` values, conflict precedence) and the recommendation contract (fixed rule precedence, skip behaviour, ≤ 1 + ≤ 1 invariant, secondary-class rule). (WP05)
- [x] T029 [P] Document risk levels (`low`/`medium`/`high`), install policies (five values), and the waiver gate (when `recommend_with_waiver` is required and how callers must enforce it). (WP05)
- [x] T030 [P] Document privacy constraints (allowlisted enum strings only, positive-list scan) and the Phase B integration plan for issues #38 and #39 (fingerprint + utilization → `ToolStateMap`). (WP05)

**Implementation sketch**: single new Markdown file, narrative + small tables, no code-block heroics.

**Parallel opportunities**: every subtask is `[P]` — write sections in any order.

**Dependencies**: none (can run alongside WP01).

**Risks**: drift between the doc and the registry — mitigated by linking to source files rather than copying values.

**Prompt file**: `tasks/WP05-engine-documentation.md`

---

### WP06 — Update existing remediation docs

**Goal**: Additively update `docs/remediation/token-saving-tooling-matrix.md` and `docs/remediation/plugin-artifacts.md` so they cross-reference the new registry and dedupe-aware recommendation contract without breaking existing content or links.

**Priority**: P1.
**Independent test**: docs review against FR-019 plus link check (existing internal links still resolve).
**Estimated prompt size**: ~220 lines (3 subtasks).

**Included subtasks**

- [x] T031 Update `docs/remediation/token-saving-tooling-matrix.md`: add a "Registry cross-reference" section pointing at `internal/analyzer/token_saving_tools.go` and the new engine doc; add a short paragraph explaining the dedupe-aware recommendation contract (≤ 1 + ≤ 1, active-tool skip). Do not rewrite existing tier tables. (WP06)
- [x] T032 Update `docs/remediation/plugin-artifacts.md`: add an additive paragraph explaining that paid plugin artifacts may now optionally embed a `TokenSavingRecommendation` object (sourced from the new engine) without breaking current artifact tests. (WP06)
- [x] T033 Add a "See also" line in both updated files pointing at `docs/remediation/token-saving-recommendation-engine.md`. (WP06)

**Implementation sketch**: small additive edits, no deletions. Run `git diff` self-review before committing.

**Parallel opportunities**: T031 and T032 touch distinct files and can be edited in parallel within the WP.

**Dependencies**: WP05 (the engine doc must exist so we can link to it).

**Risks**: accidentally rewriting the existing tier tables — mitigated by an explicit "additive only, no deletions" callout in the prompt.

**Prompt file**: `tasks/WP06-update-remediation-docs.md`

---

## Cross-WP operational notes

These are not work-package subtasks because they are external/operational actions that must be performed by the human operator, not by an autonomous implementation agent.

- **C-007 / start comment on #68** — post when the work branch is created (between WP01 kickoff and merge). The implementing agent must NOT post comments to GitHub autonomously.
- **C-007 / completion comment on #68** — post after all 6 WPs are merged and the branch is pushed. Include files changed, tests run, and what remains for Phase B.
- **C-008 / `gofmt -w` + `go test ./...`** — run on the work branch before push. Smoke (`./scripts/smoke-local.sh`) is documented as N/A for Phase A (no changes to report generation or paid artifacts).

## Requirement coverage

| Requirement | Covered by WPs |
| --- | --- |
| FR-001 (versioned registry) | WP01 |
| FR-002 (entry fields) | WP01 |
| FR-003 (public lookup API) | WP01 |
| FR-004 (ToolState enum) | WP02 |
| FR-005 (EvidenceSource enum) | WP02 |
| FR-006 (Signal enum) | WP02 |
| FR-007 (recommendation struct) | WP02 |
| FR-008 (deterministic engine fn) | WP03, WP04 |
| FR-009 (no-usage-visibility rule) | WP03, WP04 |
| FR-010 (shell-bloat / RTK state machine) | WP03, WP04 |
| FR-011 (mcp-output / context_mode state machine) | WP03, WP04 |
| FR-012 (retrieval rule) | WP03, WP04 |
| FR-013 (unchanged-reread rule) | WP03, WP04 |
| FR-014 (mcp-skill-bloat → prune) | WP03, WP04 |
| FR-015 (output verbosity rule) | WP03, WP04 |
| FR-016 (multi-signal arbitration) | WP03, WP04 |
| FR-017 (skip active_high) | WP03, WP04 |
| FR-018 (conflict-state precedence) | WP02, WP04 |
| FR-019 (matrix + plugin-artifacts doc updates) | WP06 |
| FR-020 (privacy test) | WP04 |
| FR-021 (additive doc edits) | WP06 |
| FR-022 (new engine doc) | WP05 |
| NFR-001 (determinism) | WP04 |
| NFR-002 (privacy budget) | WP04 |
| NFR-003 (additivity) | All WPs (no `types.go` / `ecosystem.go` edits) |
| NFR-004 (hermetic tests) | WP04 |
| NFR-005 (registry version) | WP01 |
| NFR-006 (< 1 ms / call) | WP04 (perf assertion via b.N micro-bench is optional) |
| C-001 … C-010 | All WPs (operational + per-WP guardrails) |

## MVP recommendation

WP01 + WP02 + WP03 + WP04 is the testable MVP. WP05 + WP06 (docs) can land in
the same PR or a follow-up; both are required for the spec's DoD but neither
gates the engine's correctness.
