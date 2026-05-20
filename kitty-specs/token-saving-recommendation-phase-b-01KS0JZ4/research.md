# Research: Token-Saving Recommendation Phase B Wiring

This document captures the Phase 0 research decisions that resolved every
open question before Phase 1 design. Each entry maps to a settled decision
in `decisions/`.

## R-01 — Engine call site: single helper vs inline

- **Decision**: Single helper `analyzer.AttachRecommendation(report *Report)` called from both `Analyze` and `AggregateReports`. (decision `01KS0K7NEM1FNE38KKSRWGVBK5`)
- **Rationale**: One test surface for signal derivation and tool-state derivation. The same code path serves the free single-report flow and the paid aggregate flow, eliminating drift risk between two parallel call sites. Cost is a new file (`internal/analyzer/recommendation_wiring.go`) which already needs to exist to host the derivation logic.
- **Alternatives considered**:
  - **Inline in both `Analyze` and `AggregateReports`**: rejected. Doubles the test surface; signal-derivation tweaks would have to be applied in two places.
  - **Only in `Analyze`; `AggregateReports` calls `Analyze`-internals on the merged report**: rejected. Ties the aggregate path to a `Analyze`-only function and obscures the merge-then-recommend invariant.
- **Implication for design**: `data-model.md` and `contracts/attach-recommendation-go-api.md` describe one function. Tests live in one file (`internal/analyzer/recommendation_wiring_test.go`).

## R-02 — When to emit `no_usage_visibility`

- **Decision**: Always emit `Signal=no_usage_visibility` when no active usage-tracking tool is detected, independent of other waste signals. (decision `01KS0NFDAQQ27YZ7R3JTY13R57`, superseding `01KS0K7GC9GH94QCMZ4SHJEBJP`)
- **Rationale**: The product motivation captured in `start-here.md` is "users want lower spend, fewer degraded sessions, fewer retries, more stable workflows, and confidence they are using Claude Code well". Knowing what tokens cost is the foundational visibility layer; recommending a tracker any time one is absent is the strongest single lever this engine can pull.
- **Alternatives considered**:
  - **Emit only when other waste signals also fire**: rejected. Quieter but defers the strongest lever for the most informed users — exactly the wrong audience to silence.
  - **Defer to Phase C**: rejected. Removes a real source of value from this PR for no compensating gain.
- **Implication for design**: Signal derivation in `recommendation_wiring.go` checks for the absence of an active usage-tracker fingerprint (the engine's `usage_visibility` class tool IDs, exposed via the existing `token_saving_tools` registry) and emits the signal unconditionally when absent.

## R-03 — Where to render the new recommendation panel

- **Decision**: Render the new section **above Workflow Fingerprints** in the report intelligence band. (decision `01KS0K7JR5J25BJ4HERMJ0P913`)
- **Rationale**: Matches the "next-best recommendation" framing in `start-here.md`. The recommendation is the most actionable item on the page; placing it above the inventory section gives the user a direct call-to-action before they scroll through detection data.
- **Alternatives considered**:
  - **Between Workflow Fingerprints and MCP/Skill Utilization**: rejected. Preserves the existing read order but buries the call-to-action.
  - **Below MCP/Skill Utilization**: rejected. Lowest visual prominence; would make Phase B feel like a footnote.
- **Implication for design**: `web/index.html` gets a new `<section id="recommendation-section">` placed before the `workflow-fingerprints` section. `web/styles.css` adds a matching ruleset.

## R-04 — Savings estimate display

- **Decision**: Render a bounded savings bucket (`low` / `medium` / `high`) in the UI, derived inline from `Report.EstimatedWaste.High`. **No engine change.** (decision `01KS0NFFPCDHF5YYVPRXF2QDW2`)
- **Rationale**: Surfaces an impact estimate alongside the next-best recommendation without breaking the C-002 engine-frozen constraint. The threshold values (10 / 30) come from the existing `WasteBucket` derivation in `analyzer.go` (which already uses `bucket(report.EstimatedWaste.High, []int{10, 20, 40, 60})`). Reusing that data source keeps the savings line tied to a number the report already exposes.
- **Alternatives considered**:
  - **Add `estimated_savings_bucket` enum to `RecommendationSet`**: rejected. Breaks C-002. Forces every future engine release to populate the field.
  - **Defer to Phase C**: rejected. The user explicitly asked for a savings line at specify-phase clarification.
- **Implication for design**: Phase B does not introduce any new bounded field on the recommendation surface. The UI layer composes the bucket label from existing numerics.

## R-05 — Engine frozen contract verification

- **Decision**: Phase B does not modify any file under `internal/analyzer/token_saving_*.go` other than (a) reading the registry / engine, (b) adding new tests in companion files.
- **Rationale**: C-002 is structural — the engine signature, registry, and rule precedence are the foundation Phase A laid. Any change to those is a separate mission.
- **Implication for design**: The contracts/ folder explicitly does not include any engine-side artifact. Test fixtures live in `internal/analyzer/testdata/` (existing pattern).

## R-06 — Aggregate merge semantics

- **Decision**: `AggregateReports` merges the underlying inputs (findings, fingerprints, utilization) first, then calls `AttachRecommendation` **once** on the merged `Report`. The aggregate path does not field-merge two `RecommendationSet` values.
- **Rationale**: The engine already implements deterministic dedupe and class-rank precedence on a single input set. Re-running it on merged inputs preserves those invariants without requiring a parallel merge ruleset. A field-level `RecommendationSet` merger would also need its own conflict-resolution rules, which is duplicative.
- **Alternatives considered**:
  - **Per-input recommend, then field-merge two `RecommendationSet` values**: rejected. Adds a new merger surface (with its own tests) for no behavioral gain.
- **Implication for design**: The aggregate path inserts a single `AttachRecommendation(merged)` call after merge logic settles.

## R-07 — Privacy budget extension

- **Decision**: Extend `internal/analyzer/leak_test.go` to assert the same forbidden-pattern invariants over `report.recommendation` JSON and over the merged paid-artifact JSON.
- **Rationale**: The privacy invariants from NFR-002 are structural; the leak test is the existing gate. Phase B adds two assertions to that gate; it does not need a new test file.
- **Implication for design**: `quickstart.md` documents the leak-test extension. The work package(s) for tests cite this gate as their pass condition.

## R-08 — Performance budget

- **Decision**: Phase B adds engine call cost ≪ 1ms per `Report` (deterministic, in-memory). Free-report render p95 budget stays at 500ms on the existing severe-MCP fixture.
- **Rationale**: The engine is a pure function over small enum slices and maps; its cost is negligible compared to JSON marshal/unmarshal already in the path. The web render adds one section with allowlisted text — no template engine, no fetch, no async work.
- **Implication for design**: No special perf measurement is required; existing browser QA harness already times render p95 against the severe-MCP fixture and is the gate.
