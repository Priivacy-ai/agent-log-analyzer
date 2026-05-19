# Tasks: Token-Saving Recommendation Phase B Wiring

| Field | Value |
| --- | --- |
| Mission slug | `token-saving-recommendation-phase-b-01KS0JZ4` |
| Mission ID | `01KS0JZ495XV0PCKSVBNDVAY16` |
| Target branch | `main` |
| Planning base branch | `main` |
| Merge target | `main` |
| Spec | [spec.md](./spec.md) |
| Plan | [plan.md](./plan.md) |

## Subtask Index

| ID | Description | WP | Parallel |
| --- | --- | --- | --- |
| T001 | Add `Recommendation *RecommendationSet` field to `Report` in `internal/analyzer/types.go` (omitempty) | WP01 |  |
| T002 | Create `internal/analyzer/recommendation_wiring.go` with `AttachRecommendation`, `deriveSignals`, `deriveToolStateMap` skeletons | WP01 |  |
| T003 | Implement signal derivation rules S-01..S-07 per `contracts/signal-derivation-map.md` | WP01 |  |
| T004 | Implement tool-state derivation rules T-F-01..T-S-02 per `contracts/tool-state-derivation-map.md` | WP01 |  |
| T005 | Call `AttachRecommendation` at end of `Analyze` in `internal/analyzer/analyzer.go` | WP01 |  |
| T006 | Add table-driven derivation tests in `internal/analyzer/recommendation_wiring_test.go` | WP01 |  |
| T007 | Add 100-iteration determinism test for `AttachRecommendation` JSON | WP01 |  |
| T008 | Call `AttachRecommendation` on merged Report at end of `AggregateReports` in `internal/analyzer/aggregate.go` | WP02 |  |
| T009 | Add aggregate recommendation re-run test in `internal/analyzer/aggregate_test.go` | WP02 |  |
| T010 | Add `Recommendation *analyzer.RecommendationSet` field to `PluginArtifact` in `internal/remediation/artifact.go` (omitempty) | WP02 | [P] |
| T011 | Populate `PluginArtifact.Recommendation` from `report.Recommendation` in `Generate` | WP02 |  |
| T012 | Add paid-artifact passthrough test in `internal/remediation/artifact_test.go` | WP02 |  |
| T013 | Extend `internal/analyzer/leak_test.go` to assert no `mcp__*` / `skill__*` / `plugin__*` substring in marshaled `Report.Recommendation` JSON | WP03 |  |
| T014 | Extend `internal/analyzer/leak_test.go` to assert zero raw private MCP/skill names appear in recommendation JSON given a Report whose Ecosystem carries unknown private names | WP03 |  |
| T015 | Add `internal/analyzer/golden_test.go` case for `Report.Recommendation` on the existing severe-MCP fixture | WP03 | [P] |
| T016 | Add or extend a leak-test fixture with private tool names that must not leak | WP03 |  |
| T017 | Add `<section id="recommendation-section">` to `web/index.html` immediately above `#workflow-fingerprints` (or current Workflow Fingerprints section) | WP04 |  |
| T018 | Implement `renderRecommendation(report)` in `web/app.js` using `textContent`-only composition (Primary + Secondary cards) | WP04 |  |
| T019 | Implement no-op note rendering when `Primary == null && Secondary == null` (count-only, no IDs) | WP04 |  |
| T020 | Implement deterministic savings-bucket helper in `web/app.js` from `report.estimated_waste_pct.high` (`<10 → low`, `10–29 → medium`, `≥30 → high`) and render only on Primary card | WP04 |  |
| T021 | Add CSS for `#recommendation-section` in `web/styles.css` matching existing intelligence-section styles | WP04 |  |
| T022 | DOM privacy verification: rendered DOM contains no `mcp__*` / `skill__*` / `plugin__*` substrings on the severe-MCP fixture | WP04 |  |

---

## WP01 — Engine wiring core

- **Goal**: Add the `Recommendation` field to `Report`, build the new helper `analyzer.AttachRecommendation`, implement deterministic signal and tool-state derivation per the contracts, and wire the helper into `Analyze`. Single-report path is fully working at the end of this WP.
- **Priority**: P0 (foundation; everything else depends on it).
- **Independent test**: A `Report` built from a synthetic fixture with `tool_output_bloat` + no usage-tracker fingerprint round-trips with `Report.Recommendation.Primary != nil` and `RegistryVersion`/`EngineVersion` populated. Determinism test passes (100×).
- **Subtasks included**:
  - [ ] T001 Add `Recommendation` field to Report (WP01)
  - [ ] T002 Create `recommendation_wiring.go` skeleton (WP01)
  - [ ] T003 Implement signal derivation S-01..S-07 (WP01)
  - [ ] T004 Implement tool-state derivation T-F-01..T-S-02 (WP01)
  - [ ] T005 Call `AttachRecommendation` from `Analyze` (WP01)
  - [ ] T006 Table-driven derivation tests (WP01)
  - [ ] T007 100-iteration determinism test (WP01)
- **Owned files**: `internal/analyzer/types.go`, `internal/analyzer/recommendation_wiring.go`, `internal/analyzer/recommendation_wiring_test.go`, `internal/analyzer/analyzer.go`.
- **Authoritative surface**: `internal/analyzer/recommendation_wiring`.
- **Dependencies**: none.
- **Risks**: types.go modification is field-add only; do not reorder existing fields. The new field must be `*RecommendationSet` (pointer) with `omitempty` so legacy reports round-trip cleanly (NFR-004).
- **Estimated prompt size**: ~350 lines.

## WP02 — Aggregate and paid artifact wiring

- **Goal**: Wire `AttachRecommendation` into `AggregateReports` (re-run on merged inputs), and pass the resulting `RecommendationSet` through to the paid `PluginArtifact` as a new `omitempty` field. Existing `VettedRecommendations` slice is preserved (C-004).
- **Priority**: P1.
- **Independent test**: Aggregating 3 synthetic reports produces a merged Report whose `Recommendation` is the engine output over union signals + resolved tool state. Generated paid artifact JSON has `recommendation` field that equals `Report.Recommendation` byte-for-byte. Existing `VettedRecommendations` entries are unchanged.
- **Subtasks included**:
  - [ ] T008 Call `AttachRecommendation` on merged Report (WP02)
  - [ ] T009 Aggregate recommendation re-run test (WP02)
  - [ ] T010 [P] Add `Recommendation` field to `PluginArtifact` (WP02)
  - [ ] T011 Populate `PluginArtifact.Recommendation` in `Generate` (WP02)
  - [ ] T012 Paid-artifact passthrough test (WP02)
- **Owned files**: `internal/analyzer/aggregate.go`, `internal/analyzer/aggregate_test.go`, `internal/remediation/artifact.go`, `internal/remediation/artifact_test.go`.
- **Authoritative surface**: `internal/remediation/artifact`.
- **Dependencies**: `WP01` (needs `Report.Recommendation` field + `AttachRecommendation` helper).
- **Risks**: Do not reorder fields in `PluginArtifact`; only append the new field (paid artifact JSON ordering matters for downstream consumers).
- **Estimated prompt size**: ~280 lines.

## WP03 — Privacy and determinism gates

- **Goal**: Extend the existing `leak_test.go` privacy gate and the existing `golden_test.go` snapshot to cover the new `Report.Recommendation` field. Verify zero forbidden substrings and add a Report fixture that contains private MCP/skill names to prove they never reach the recommendation JSON.
- **Priority**: P1.
- **Independent test**: `go test -run TestLeak ./internal/analyzer/...` asserts no `mcp__*` / `skill__*` / `plugin__*` substrings appear in `Report.Recommendation` JSON for a fixture whose `Ecosystem.UnknownMCPServerCount > 0`. `go test -run TestGolden ./internal/analyzer/...` passes against the updated golden file.
- **Subtasks included**:
  - [ ] T013 Extend leak_test.go for recommendation-JSON forbidden substrings (WP03)
  - [ ] T014 Extend leak_test.go for private-name probe (WP03)
  - [ ] T015 [P] Add golden test for recommendation JSON (WP03)
  - [ ] T016 Add leak-test fixture with private tool names (WP03)
- **Owned files**: `internal/analyzer/leak_test.go`, `internal/analyzer/golden_test.go`, `internal/analyzer/testdata/recommendation/**`.
- **Authoritative surface**: `internal/analyzer/testdata/recommendation/`.
- **Dependencies**: `WP01` (needs the new field populated).
- **Risks**: Do not destabilize existing leak/golden assertions; add new tests rather than rewriting existing ones. If existing golden fixtures need their JSON updated because the new field is now present, that is expected and is part of WP03.
- **Estimated prompt size**: ~260 lines.

## WP04 — Web UI rendering and savings bucket

- **Goal**: Render the new recommendation panel **above** the Workflow Fingerprints section on the free report page. Compose all text from enum values via `textContent`; never use `innerHTML`. Compute and render a bounded savings bucket from `report.estimated_waste_pct.high`. Render a "no action needed" note when both Primary and Secondary are absent.
- **Priority**: P1.
- **Independent test**: Loading the existing severe-MCP fixture in the web UI renders a recommendation panel above Workflow Fingerprints. Browser DOM grep returns zero `mcp__*` / `skill__*` / `plugin__*` substrings. Render p95 < 500ms (existing baseline).
- **Subtasks included**:
  - [ ] T017 Add `#recommendation-section` HTML (WP04)
  - [ ] T018 Implement `renderRecommendation` with `textContent` only (WP04)
  - [ ] T019 Implement no-op note rendering (WP04)
  - [ ] T020 Implement bounded savings-bucket helper (WP04)
  - [ ] T021 Add CSS for the new section (WP04)
  - [ ] T022 DOM privacy verification (WP04)
- **Owned files**: `web/index.html`, `web/app.js`, `web/styles.css`.
- **Authoritative surface**: `web/`.
- **Dependencies**: `WP01` (needs `Report.Recommendation` field populated).
- **Risks**: PR #76 DOM-privacy invariant must be preserved; use `textContent` for all string nodes. Do not introduce any template engine or framework.
- **Estimated prompt size**: ~330 lines.

---

## Dependency Graph

```
WP01 ──┬── WP02
       ├── WP03
       └── WP04
```

WP02, WP03, WP04 can execute in parallel after WP01 reaches `approved`.

## MVP Scope

WP01 alone is **not** an MVP — the recommendation is only computed but never surfaced to the user (no UI, no paid artifact passthrough). The smallest user-visible value comes from WP01 + WP04 (free report shows recommendations). Paid artifact (WP02) and privacy gates (WP03) are required for launch-ready.

For an aggressive single-PR ship, all four WPs land together; for an incremental landing strategy, WP01+WP04 first, then WP02+WP03 in a follow-up.
