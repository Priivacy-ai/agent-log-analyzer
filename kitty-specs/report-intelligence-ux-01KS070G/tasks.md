# Tasks: Report Intelligence UX

**Mission**: `report-intelligence-ux-01KS070G`
**Mission ID**: `01KS070GDSG3W56YBCS2C8SHVY`
**Target Branch**: `main`
**Generated**: 2026-05-19T13:47:54Z

## Subtask Index

| ID | Description | WP | Parallel |
|---|---|---|---|
| T001 | Add `<section id="workflow-fingerprints">` and `<section id="tooling-utilization">` blocks to `web/index.html` above the existing Ecosystem block (both initially `hidden`). | WP01 | |
| T002 | Implement `renderWorkflowFingerprints(report)` in `web/app.js` per `contracts/render-workflow-fingerprints.md`. | WP01 | [P] |
| T003 | Implement `renderToolingUtilization(report)` in `web/app.js` per `contracts/render-tooling-utilization.md` (band→finding-ID lookup, `exposure_known` ratio gating). | WP01 | [P] |
| T004 | Wire both renderers into `applyReport(report)` in `web/app.js`. | WP01 | |
| T005 | Add scoped CSS to `web/styles.css` for fingerprint rows, utilization rows, band-chip variants, and advice block. | WP01 | [P] |
| T006 | Add Go-side renderer-input leak test `internal/analyzer/view_render_inputs_test.go` covering hostile fingerprint/utilization input + 5 band permutations. | WP01 | [P] |

The `[P]` markers indicate concerns that touch different surfaces (separate JS function, separate CSS rules, separate Go test file) and could be implemented in parallel within a single WP session if the implementer wishes — they are not separate work-package lanes.

## Work Packages

### WP01 — Report Intelligence UX sections (issues #62 + #63)

- **Goal**: Ship two compact, profiler-style report sections (Workflow Fingerprints and MCP & Skill Utilization with deterministic pruning advice) over already-bounded ecosystem data, with zero private-name leakage and no new serialized fields.
- **Priority**: P0 (Phase 2 launch-completion sprint).
- **Independent test**: Run `docker compose up --build`, open `http://localhost:8080`, exercise the analyze→upload→report flow with a fixture surfacing fingerprints and `high`/`severe` MCP or Skill bands; verify per `quickstart.md` checklist; confirm `grep` over rendered HTML returns zero unknown MCP/skill/plugin names.
- **Estimated prompt size**: ~450 lines (6 subtasks).

#### Included subtasks

- [x] T001 Add `<section id="workflow-fingerprints">` and `<section id="tooling-utilization">` markup to `web/index.html` (WP01)
- [x] T002 Implement `renderWorkflowFingerprints(report)` per contracts/render-workflow-fingerprints.md (WP01)
- [x] T003 Implement `renderToolingUtilization(report)` per contracts/render-tooling-utilization.md (WP01)
- [x] T004 Wire both renderers into `applyReport(report)` (WP01)
- [x] T005 Add scoped CSS for new sections to `web/styles.css` (WP01)
- [x] T006 Add `internal/analyzer/view_render_inputs_test.go` hostile-fixture leak test (WP01)

#### Implementation sketch

1. **Markup first (T001)** — extends `web/index.html` with two new `<section>` blocks both `hidden` by default, placed above the existing `<div><h2>Ecosystem</h2>…</div>` block. The new sections preserve the existing block (per FR-009).
2. **Renderers (T002, T003)** — pure functions over `report.ecosystem.workflow_fingerprints` (T002) and `report.ecosystem.tooling_utilization` + `report.findings` (T003). DOM creation uses `textContent` only — no `innerHTML` interpolation of any field value.
3. **Wiring (T004)** — inside `applyReport(report)`, call both new render functions before the existing `summarizeEcosystem` flat-text dump (so the structured sections render first in the DOM). Hidden-state of each section is owned by its renderer.
4. **Styles (T005)** — additive, scoped to the new IDs and their descendants. Band chips use a small enum of CSS classes (`.band-severe`, `.band-high`, `.band-watch`, `.band-normal`, `.band-unknown`).
5. **Leak test (T006)** — Go-side test that constructs a hostile `Report` value (or analyzes a hostile input through `analyzer.Analyze`) and asserts that none of the canary strings — extending the existing `leak_test.go` canary list — appear in the JSON serialization of the fields the new renderers read. This is the structural backstop for NFR-001.

#### Parallel opportunities

The three `[P]` subtasks (T002, T003, T005) touch different functions/CSS rules and can be drafted in parallel within the same WP session before being reconciled by T004 (wiring).

#### Dependencies

- None outside the mission. Phase 1 (PR #75) is already merged on `main`.

#### Risks

- Existing `web/app.js` uses `innerHTML` interpolation in `findings` rendering (`item.innerHTML = ...`). Do **not** copy that pattern for the new renderers — use DOM API + `textContent`.
- A test fixture that does not surface a `*_bloat_*` band slows manual browser QA. Use `testdata/golden/sample-report.json` or hand-craft a small fixture in `testdata/fixtures/` that produces a `high`/`severe` band.
- Future schema change that removes one of the four `*_bloat_*` findings would silently kill the advice block. Mitigation: leave a code comment in `app.js` pointing to `internal/analyzer/analyzer.go:368-394`.

## Notes

- WP prompt file: `tasks/WP01-report-intelligence-sections.md` (created in this phase).
- Browser QA is the user-visible final gate; see `quickstart.md`.
- This mission is **not** Phase 3 (`/codex/recommendation-phase-b` / #73) — do not add a next-best-recommendation card here.
- This mission is **not** Phase 4+ — no paid pack, Stripe, trust/distribution, cloud changes.
