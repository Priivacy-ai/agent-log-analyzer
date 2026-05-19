---
work_package_id: WP04
title: Web UI rendering and savings bucket
dependencies:
- WP01
requirement_refs:
- FR-006
- FR-009
- FR-012
- NFR-002
- NFR-003
- C-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
created_at: '2026-05-19T20:30:00+00:00'
subtasks:
- T017
- T018
- T019
- T020
- T021
- T022
agent_profile: implementer-ivan
role: implementer
agent: claude:sonnet:implementer-ivan:implementer
authoritative_surface: web/
execution_mode: code_change
owned_files:
- web/index.html
- web/app.js
- web/styles.css
history:
- '2026-05-19': created from mission token-saving-recommendation-phase-b-01KS0JZ4
tags:
- ui
- report-ux
---

## ⚡ Do This First: Load Agent Profile

```text
/ad-hoc-profile-load implementer-ivan
```

Return here and continue with **Objective** below.

## Objective

Render the new recommendation panel **above** the Workflow Fingerprints
section on the free report page. Compose all text from enum values via
`textContent`; never use `innerHTML`. Compute and render a bounded
savings bucket (`low` / `medium` / `high`) from
`report.estimated_waste_pct.high`. When both Primary and Secondary are
absent, render a short "no action needed" note with the candidate count
only (no IDs).

This WP must not modify any Go file, any test, or any file outside `web/`.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. Execution
worktree per `lanes.json`; resolve via context-resolve.

## Context

- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/spec.md` (FR-006, FR-009, FR-012, NFR-002, NFR-003)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/plan.md` (Web UI section, savings bucket thresholds)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/contracts/report-recommendation-json-envelope.md` (the JSON your renderer consumes)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/research.md` §R-03 (placement) and §R-04 (savings bucket)
- Existing `web/index.html`, `web/app.js`, `web/styles.css` — read them in
  full to learn the existing intelligence-section patterns and the
  PR #76 DOM-privacy invariant.

## Owned files

- `web/index.html` — add `<section id="recommendation-section">` above
  the Workflow Fingerprints section.
- `web/app.js` — add `renderRecommendation`, `savingsBucket`, and the
  small allowlist maps. Wire into the existing render pipeline.
- `web/styles.css` — add a ruleset matching existing intelligence
  sections.

## Subtasks

### T017 — Add `#recommendation-section` HTML

**Purpose**: Reserve the layout slot above the existing Workflow
Fingerprints section.

**Steps**:

1. Open `web/index.html`. Locate the Workflow Fingerprints section
   (search for `workflow-fingerprints` or "Workflow Fingerprints").
2. **Immediately above** that section, insert:
   ```html
   <section id="recommendation-section" class="intel-section" hidden>
     <h2>Next-best recommendation</h2>
     <div id="recommendation-primary"></div>
     <div id="recommendation-secondary"></div>
     <p id="recommendation-empty" hidden>No action needed — your tooling is already in shape.</p>
   </section>
   ```
3. The existing intelligence-section convention (`web/index.html` line ~77
   for Workflow Fingerprints) uses `class="intel-section"` and the HTML5
   boolean `hidden` attribute (NOT a `hidden` CSS class). Match this
   exactly: `<section ... hidden>` to hide, set `section.hidden = false`
   in JS to show.

**Validation**: open the page; the section is hidden until the
renderer populates it.

### T018 — Implement `renderRecommendation(report)` in `web/app.js`

**Purpose**: Render Primary and Secondary cards using `textContent`
only. No `innerHTML`. No template engine.

**Steps**:

1. Open `web/app.js`. Read the existing `render*` functions to learn
   the pattern (e.g. `renderWorkflowFingerprints`,
   `renderToolingUtilization`).
2. Implement `renderRecommendation(report)`:
   - Early return if `report.recommendation == null` (legacy report
     compatibility — FR-012). Section stays hidden.
   - Show `#recommendation-section` by setting `section.hidden = false`
     (HTML5 boolean attribute, **not** a CSS class toggle).
   - Render Primary into `#recommendation-primary` by composing a card
     of bounded text from enum values:
     - Tool ID: human-readable label via an allowlist map
       (`{ rtk: "RTK", leanctx: "LeanCtx", ccusage: "ccusage", ... }`).
       Unknown IDs (not in the map) render as the raw ID with no further
       processing. (Note: unknown IDs from the engine cannot appear here
       because the engine emits only allowlisted IDs; this is a defense
       in depth, not a feature.)
     - Reason: allowlist map from `Reason` enum to short prose
       (`absent → "Not detected yet"`,
       `installed_inactive → "Installed but not active"`, etc.).
     - Confidence, RiskLevel, InstallPolicy: same allowlist pattern.
     - SignalIDs: short prose labels via an allowlist map.
   - Render Secondary into `#recommendation-secondary` using the same
     code path (parametrize the renderer over `which slot`).
   - Hide each `#recommendation-{primary,secondary}` slot if the
     corresponding pointer is null.
3. **Every text node** is created by:
   ```js
   const el = document.createElement('span'); // or appropriate tag
   el.textContent = labelMap[enumValue] ?? enumValue;
   parent.appendChild(el);
   ```
   No string concatenation into `innerHTML`. No `dangerouslySetInnerHTML`-
   analog patterns.
4. Wire the call into the existing render pipeline:
   - The entry point is `function renderReport(report)` in `web/app.js`
     (around line 173).
   - Insert `renderRecommendation(report);` **before** the existing
     `renderWorkflowFingerprints(report)` call (around line 199), so the
     recommendation section's `hidden = false` toggle precedes the
     Workflow Fingerprints render in DOM order.

**Validation**: Open the page on the existing severe-MCP fixture; the
Primary card appears above Workflow Fingerprints.

### T019 — No-op note rendering

**Purpose**: Implement FR-006 / FR-009: when both Primary and Secondary
are absent, render the "no action needed" note containing only counts.

**Steps**:

1. Inside `renderRecommendation`, after the Primary/Secondary slot
   handling, check:
   ```js
   if (!report.recommendation.primary && !report.recommendation.secondary) {
     // show the empty note
   }
   ```
2. The note text composition uses:
   - Count of skipped candidates: `report.recommendation.skipped?.length ?? 0`.
   - `unknown_id_count`: `report.recommendation.unknown_id_count ?? 0`.
   - Compose a sentence via `textContent` only. Do not include any tool
     ID. Example: "Engine evaluated N candidates; none warranted a
     recommendation. (M unknown identifiers were counted only.)"
3. Toggle `#recommendation-empty` visibility by setting its `hidden`
   attribute (`document.querySelector('#recommendation-empty').hidden = !shouldShow`).

**Validation**: a fixture with `primary == null && secondary == null`
renders the note. A fixture with Primary or Secondary populated does
NOT render the note.

### T020 — Bounded savings-bucket helper

**Purpose**: Implement R-04 — derive the savings bucket from existing
`report.estimated_waste_pct.high` inline in JS.

**Steps**:

1. Implement `savingsBucket(report)`:
   ```js
   function savingsBucket(report) {
     const high = report?.estimated_waste_pct?.high ?? 0;
     if (high < 10) return 'low';
     if (high < 30) return 'medium';
     return 'high';
   }
   ```
2. Inside the Primary-card renderer (T018), render the savings bucket
   only when Primary is present. Use the allowlist map
   `{ low: "Low estimated savings", medium: "Medium estimated savings",
   high: "High estimated savings" }` and compose via `textContent`.
3. Do not render the savings line on the Secondary card or on the no-op
   note.

**Validation**: across fixtures with high=5/15/45 the savings bucket
labels are low/medium/high respectively.

### T021 — CSS for `#recommendation-section`

**Purpose**: Match the existing intelligence-section style.

**Steps**:

1. Open `web/styles.css`. Read the existing rules for
   `.report-section`, `.full-width`, and any Workflow Fingerprints
   ruleset.
2. Add minimal new rules:
   - `#recommendation-section` layout (full-width container, the same
     padding/margin as adjacent sections).
   - `.recommendation-card` (Primary/Secondary card style — match
     existing card patterns).
   - `.recommendation-savings-bucket` (small badge under the tool name).
3. Reuse existing color/typography tokens; do not introduce new font
   families or palettes.

**Validation**: open in a browser; the new section visually matches the
existing Workflow Fingerprints and MCP/Skill Utilization sections.

### T022 — DOM privacy verification

**Purpose**: Lock in C-003 (no `mcp__*` / `skill__*` / `plugin__*` token
strings in rendered DOM).

**Steps**:

1. Load the existing severe-MCP fixture page in a browser.
2. In DevTools console, run:
   ```js
   const html = document.documentElement.outerHTML;
   const forbidden = /mcp__|skill__|plugin__/g;
   console.log('matches:', html.match(forbidden) ?? []);
   ```
3. Output must be `[]` (zero matches).
4. Capture a screenshot of the rendered recommendation section and
   attach to the WP review comment.

**Validation**: zero matches in the DOM grep.

## Test strategy

- The web codebase has no Go-side unit tests for `app.js` (current
  pattern). DOM privacy is verified manually per T022 and via the
  existing browser QA harness used in PR #76 review.
- Render performance: use DevTools Performance to confirm < 500ms p95.

## Definition of Done

- [ ] `#recommendation-section` renders above Workflow Fingerprints on
      the existing severe-MCP fixture.
- [ ] Primary and Secondary cards compose all text via `textContent`.
- [ ] No-op note renders when both Primary and Secondary are absent.
- [ ] Savings bucket renders only on the Primary card and uses the
      threshold 10 / 30.
- [ ] DOM grep returns zero `mcp__*` / `skill__*` / `plugin__*` matches.
- [ ] Render p95 < 500ms on the severe-MCP fixture (no regression).
- [ ] `./scripts/smoke-local.sh` succeeds (the existing smoke flow).
- [ ] No file outside `web/` is modified.

## Risks

- **`innerHTML` use creeps in** — the spec invariant is explicit; the
  reviewer must grep for `innerHTML`, `outerHTML`, and `insertAdjacentHTML`
  in the diff and reject any introduction.
- **Tool ID label leak** — never render a tool ID without checking the
  allowlist map. Even though the engine guarantees enum-only output,
  defense in depth requires the map lookup.
- **CSS regression on adjacent sections** — change scope must stay
  inside the new selectors; do not modify existing rules.

## Reviewer Guidance

1. `grep -nE 'innerHTML|outerHTML|insertAdjacentHTML' web/app.js` returns
   no new matches.
2. The renderer is wired into the existing render pipeline — search for
   the call site and confirm exactly one call per report-render cycle.
3. Open the page on the severe-MCP fixture and run the DOM-grep probe
   from T022; paste the result into the review comment.
4. Confirm Primary, Secondary, and no-op note each render correctly via
   three test fixtures.

## Working Directory and Hand-off

```bash
./scripts/smoke-local.sh
```

Open the rendered report; do the DOM privacy check; capture a
screenshot. Then commit:

```bash
git add web/index.html web/app.js web/styles.css
git commit -m "feat(WP04): web UI for token-saving recommendation panel"
```

Mark subtasks done and move to `for_review`:

```bash
spec-kitty agent tasks mark-status T017 T018 T019 T020 T021 T022 --status done --mission token-saving-recommendation-phase-b-01KS0JZ4
spec-kitty agent tasks move-task WP04 --to for_review --note "Ready for review" --mission token-saving-recommendation-phase-b-01KS0JZ4
```
