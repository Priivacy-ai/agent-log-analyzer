---
work_package_id: WP01
title: Report Intelligence UX sections (Workflow Fingerprints + Tooling Utilization)
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-005
- FR-006
- FR-007
- FR-008
- FR-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-report-intelligence-ux-01KS070G
base_commit: 2d040ad6a77c63b5f884c7b3c1d9ff7b93b0ca34
created_at: '2026-05-19T14:08:24.761679+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
agent: "claude:opus-4-7:reviewer-renata:reviewer"
shell_pid: "80149"
history:
- timestamp: '2026-05-19T13:47:54Z'
  event: drafted
  note: Initial WP draft during /spec-kitty.tasks for mission report-intelligence-ux-01KS070G.
agent_profile: frontend-freddy
authoritative_surface: web/
execution_mode: code_change
mission_id: 01KS070GDSG3W56YBCS2C8SHVY
mission_slug: report-intelligence-ux-01KS070G
model: claude-opus-4-7
owned_files:
- web/index.html
- web/app.js
- web/styles.css
- internal/analyzer/view_render_inputs_test.go
role: implementer
tags: []
---

# WP01 — Report Intelligence UX sections (Workflow Fingerprints + Tooling Utilization)

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load your assigned agent profile so your role, scope boundaries, and initialization declaration are bound to this session:

```bash
/ad-hoc-profile-load frontend-freddy
```

If `/ad-hoc-profile-load` is unavailable in your harness, the equivalent CLI is `list` (this Spec Kitty build has no `show` subcommand). Pull the single profile with a filter:

```bash
spec-kitty agent profile list --json | python3 -c "import sys,json; [print(json.dumps(p,indent=2)) for p in json.load(sys.stdin) if p['profile_id']=='frontend-freddy']"
```

Read the profile's identity, internalize the role boundaries (you are an **implementer** for **frontend** code — HTML, vanilla JS, CSS), and acknowledge those boundaries before touching any code.

## Objective

Ship two compact, profiler-style sections on the existing report page:

1. **Workflow Fingerprints** — surfaces detected public SDD tooling (Spec Kitty, GitHub Spec Kit, OpenSpec, …) with confidence, sources, and active/installed flags. Closes issue #62.
2. **MCP & Skill Utilization** — surfaces per-class bucket labels, call counts, utilization ratio, context-footprint, and a warning-band chip. When the warning band is `high` or `severe`, render a band-keyed pruning advice block sourced from existing `Finding.Recommendation` strings. Closes issue #63.

Both sections render only when their underlying data is present (hidden otherwise). The page never renders unknown private MCP/skill/plugin names — counts only. No new serialized fields, no JS framework, no LLM calls.

## Context

- Mission: `report-intelligence-ux-01KS070G`. Spec: `kitty-specs/report-intelligence-ux-01KS070G/spec.md`. Plan: `kitty-specs/report-intelligence-ux-01KS070G/plan.md`. Renderer contracts under `kitty-specs/report-intelligence-ux-01KS070G/contracts/`.
- Phase 1 (PR #75, merged on `main` at `5df679c`) ensured aggregate merge preserves `Ecosystem.WorkflowFingerprints` and `Ecosystem.ToolingUtilization`. The single-report path already emits these via `internal/analyzer/ecosystem.go::computeToolingUtilization`. **The data is already there. This WP is the view layer.**
- The advice copy you will render is already produced by `internal/analyzer/analyzer.go::computeFindings` (lines 365–394) as `Finding.Recommendation` strings for four IDs: `mcp_bloat_severe`, `mcp_bloat_high`, `skill_bloat_severe`, `skill_bloat_high`. You will read those strings via `report.findings[]` — **do not** duplicate or rewrite the copy.

## Branch Strategy

- Planning/base branch: `main`
- Final merge target: `main`
- Worktree path: assigned by `spec-kitty finalize-tasks` and visible via `lanes.json`. Enter that workspace before editing.

## Owned Files & Boundaries

You may modify only:

- `web/index.html`
- `web/app.js`
- `web/styles.css`
- `internal/analyzer/view_render_inputs_test.go` (new file)

You **must not** modify:

- `internal/analyzer/types.go` (schema is frozen for this mission — C-001).
- `internal/analyzer/analyzer.go` or `internal/analyzer/ecosystem.go` (read-only — they are the upstream sources you're rendering).
- `internal/remediation/*`, `internal/backend/*`, `internal/awsstore/*`, `cmd/*`, `infra/*`.
- Any file under `web/privacy/` or `web/security/` (trust-page content, separate scope).
- `testdata/golden/sample-report.json` unless absolutely required to surface fingerprints/utilization for manual browser QA — if you do touch it, follow the Phase-1 tactical hint to keep `WorkflowFingerprints` nilled for host-portability of golden tests.

## Constraints (re-affirm from spec)

- **C-001 (no new serialized fields)**: do not add any field to `internal/analyzer/types.go`.
- **C-002 (no private-name leakage)**: the rendered DOM must contain zero strings that aren't in the public allowlist. Render counts only for unknown surfaces.
- **C-003 (deterministic advice)**: pruning advice comes from `report.findings[]` for the four `*_bloat_*` IDs. No client copy table that duplicates those strings. No LLM, no `fetch()`.
- **NFR-004 (no new framework / bundler)**: no `package.json`, no `node_modules`, no transpile step. Vanilla ES2017+ JS only.

## Subtasks

### Subtask T001 — Add markup blocks to `web/index.html`

**Purpose**: Provide the two empty section containers the renderers will populate. Markup is initially `hidden` so empty data leaves the page unchanged.

**Steps**:

1. Open `web/index.html`. Locate the existing `Ecosystem` block:
   ```html
   <div>
     <h2>Ecosystem</h2>
     <pre id="ecosystem"></pre>
   </div>
   ```
2. Immediately **above** that block, add two new `<section>` elements:
   ```html
   <section id="workflow-fingerprints" class="intel-section" hidden>
     <h2>Workflow Fingerprints</h2>
     <ol id="workflow-fingerprints-list"></ol>
   </section>

   <section id="tooling-utilization" class="intel-section" hidden>
     <h2>MCP &amp; Skill Utilization</h2>
     <div id="tooling-utilization-rows"></div>
   </section>
   ```
3. Do not change any existing element ID, attribute, or class. Do not introduce inline event handlers, inline styles, or new external resources.

**Files**:

- `web/index.html` — additive change, ~12 new lines.

**Validation**:

- [ ] Both new sections are present in the served HTML.
- [ ] Both sections are `hidden` on initial load (verifiable via DevTools Elements panel).
- [ ] No existing block was moved, renamed, or removed (verify via `git diff main -- web/index.html`).

---

### Subtask T002 — Implement `renderWorkflowFingerprints(report)` in `web/app.js`

**Purpose**: Render one row per fingerprint with bounded fields only.

**Contract**: `kitty-specs/report-intelligence-ux-01KS070G/contracts/render-workflow-fingerprints.md` (read it).

**Steps**:

1. Add a top-level function `renderWorkflowFingerprints(report)` to `web/app.js` (somewhere alongside the existing `summarizeEcosystem` / `summarizeReceipt` helpers).
2. Defensive entry:
   ```js
   const section = document.querySelector("#workflow-fingerprints");
   const list = document.querySelector("#workflow-fingerprints-list");
   if (!section || !list) return;
   const fps = report && report.ecosystem && Array.isArray(report.ecosystem.workflow_fingerprints)
     ? report.ecosystem.workflow_fingerprints
     : [];
   list.replaceChildren();
   if (fps.length === 0) { section.hidden = true; return; }
   section.hidden = false;
   ```
3. For each fingerprint, create a `<li class="fingerprint-row">` and populate child elements using `textContent` only:
   - Row title: the fingerprint `id` (literal text).
   - Confidence label: `<span class="fingerprint-confidence confidence-${fp.confidence}">…</span>` containing the literal `fp.confidence` value (`low` / `medium` / `high`).
   - Sources row: `<ul class="fingerprint-sources">` with one `<li>` per `fp.sources[]` entry, each `<li>.textContent = source`.
   - Evidence count: `<span class="fingerprint-evidence">evidence: N</span>` where N is the numeric `fp.evidence_count`.
   - Active indicator: render only if `fp.active === true` — e.g. `<span class="fingerprint-active">active</span>`.
   - Installed indicator: render only if `fp.installed === true` — e.g. `<span class="fingerprint-installed">installed</span>`.
   - Version bucket: render only if `fp.version_bucket` is a non-empty string — e.g. `<span class="fingerprint-version">version: ${fp.version_bucket}</span>` (assigned via `textContent`, not `innerHTML`).
4. Never use `innerHTML =` with any value derived from the report. Use `document.createElement`, `appendChild`, `textContent`, `classList.add`. (The existing `findings` renderer's `innerHTML` pattern is a legacy mistake — do not copy it.)
5. Multiple calls must fully replace prior content. `list.replaceChildren()` at the top handles that.

**Files**:

- `web/app.js` — new function `renderWorkflowFingerprints(report)`, ~50–70 lines.

**Validation**:

- [ ] Function visible at module scope (`grep -n "renderWorkflowFingerprints" web/app.js`).
- [ ] Function uses only `textContent` / DOM API for field values (verify by source inspection — `grep "innerHTML" web/app.js` shows no new occurrences in the function body).
- [ ] Calling with `{}` does not throw and hides the section.
- [ ] Calling with `{ecosystem:{workflow_fingerprints:[]}}` does not throw and hides the section.
- [ ] Calling with a one-fingerprint fixture shows the row.

---

### Subtask T003 — Implement `renderToolingUtilization(report)` in `web/app.js`

**Purpose**: Render MCP and Skill rows with bucket labels, counts, band chip, and (conditionally) the band-keyed pruning advice block. This is the section that closes #63.

**Contract**: `kitty-specs/report-intelligence-ux-01KS070G/contracts/render-tooling-utilization.md` (read it).

**Steps**:

1. Add `renderToolingUtilization(report)` to `web/app.js`.
2. Defensive entry:
   ```js
   const section = document.querySelector("#tooling-utilization");
   const rowsRoot = document.querySelector("#tooling-utilization-rows");
   if (!section || !rowsRoot) return;
   const tu = report && report.ecosystem && report.ecosystem.tooling_utilization;
   rowsRoot.replaceChildren();
   if (!tu) { section.hidden = true; return; }
   section.hidden = false;
   ```
3. Build a deterministic band → finding-ID lookup. **Hard-code these four pairs only**; never derive the IDs from input data:
   ```js
   const ADVICE_LOOKUP = {
     mcp: { severe: "mcp_bloat_severe", high: "mcp_bloat_high" },
     skill: { severe: "skill_bloat_severe", high: "skill_bloat_high" },
   };
   // No keys for watch/normal/unknown — structurally enforces FR-006.
   ```
4. Implement a small helper:
   ```js
   function findingById(report, id) {
     const list = (report && Array.isArray(report.findings)) ? report.findings : [];
     return list.find((f) => f && f.id === id) || null;
   }
   ```
5. **Normalize band before chip/lookup**:
   ```js
   // Empty/missing warning_band → render as "unknown" (matches analyzer
   // guarantee in tooling_classify.go:149-151 / 191-193 when exposure_known
   // is false; also tolerates a future struct-default zero-value).
   function normalizeBand(b) {
     const v = typeof b === "string" ? b : "";
     return (v === "severe" || v === "high" || v === "watch" || v === "normal") ? v : "unknown";
   }
   ```
6. Render the MCP row using exactly these field names from `internal/analyzer/types.go` lines 88–104 (do not invent any field):
   - **Bucket cells**: `mcp.server_count_bucket`, `mcp.exposed_tool_count_bucket`, `mcp.context_token_bucket`, `mcp.context_efficiency_bucket`.
   - **Counts** (numeric only, never names): `mcp.call_count`, `mcp.known_call_count`, `mcp.unknown_call_count`, `mcp.unknown_server_count`, `mcp.unique_unknown_called_count`. Also display `mcp.unique_known_called_ids.length` and `mcp.known_server_ids.length` (counts of allowlisted arrays — never their contents).
   - **Band chip**: `<span class="band-chip band-${normalizeBand(mcp.warning_band)}">${normalizeBand(mcp.warning_band)}</span>` with `textContent` for the band label.
   - **Ratio cell** (FR-007 gating):
     - If `mcp.exposure_known === true`: render `${mcp.utilization_ratio_pct}%` via `textContent`.
     - Else: render `inferred from: ${mcp.inference_source}` and **do not** render the percentage. (Analyzer guarantees the band is `unknown` in this case — see `tooling_classify.go:149`.)
   - **Advice block (FR-005/006)**:
     ```js
     const adviceId = ADVICE_LOOKUP.mcp[normalizeBand(mcp.warning_band)]; // undefined for watch/normal/unknown
     const finding  = adviceId ? findingById(report, adviceId) : null;
     if (finding && typeof finding.recommendation === "string" && finding.recommendation.length > 0) {
       // render <p class="band-advice">finding.recommendation</p> via textContent
     }
     ```
     Suppressed automatically for `watch`/`normal`/`unknown` (no lookup key) and for `high`/`severe` when no matching finding exists (defense in depth — analyzer single-emission invariant INV-6 in `data-model.md` makes this rare).
7. Render the Skill row using exactly these field names from `internal/analyzer/types.go` lines 106–119 (note: Skill has **no** equivalent of `unique_known_called_ids` or `unique_unknown_called_count` — those are MCP-only):
   - **Bucket cells**: `skill.exposed_count_bucket`, `skill.context_token_bucket`, `skill.context_efficiency_bucket`. (No `exposed_tool_count_bucket` on Skill.)
   - **Counts**: `skill.executed_count`, `skill.unknown_exposed_count`, `skill.unknown_executed_count`. Also display `skill.known_exposed_ids.length` and `skill.known_executed_ids.length` (counts only).
   - **Band chip**: same `normalizeBand(skill.warning_band)` pattern.
   - **Ratio cell**: same `exposure_known` gate; use `skill.inference_source` in the inferred case.
   - **Advice block**: use `ADVICE_LOOKUP.skill[normalizeBand(skill.warning_band)]`.
8. **Do not** render `known_server_ids[]`, `unique_known_called_ids[]`, `known_exposed_ids[]`, or `known_executed_ids[]` as names. The V2 UI shows their `length` only. (The data is allowlisted and would be safe to print, but the section's privacy posture is uniform: counts only.)
9. Add a brief comment above the lookup pointing to the upstream source:
   ```js
   // The four allowlisted advice IDs are emitted by internal/analyzer/analyzer.go:368-394.
   // If those IDs change, this UI must be updated in lockstep.
   ```

**Files**:

- `web/app.js` — new function `renderToolingUtilization(report)` + `findingById` helper + `ADVICE_LOOKUP` constant, ~120–160 lines including both surfaces.

**Validation**:

- [ ] `grep -n "ADVICE_LOOKUP" web/app.js` shows exactly the four `mcp`/`skill` × `severe`/`high` entries; no `watch`/`normal`/`unknown` keys.
- [ ] Section is hidden when `report.ecosystem.tooling_utilization` is missing.
- [ ] When `exposure_known === false`, no percentage character appears in the row's rendered text.
- [ ] When `warning_band === "watch"` (or `normal`/`unknown`), no advice block appears in the row's rendered HTML.
- [ ] When `warning_band === "severe"` and the corresponding `mcp_bloat_severe` finding exists, the advice block renders the finding's recommendation text verbatim.
- [ ] No `innerHTML =` assignment of any field-derived value (source inspection).
- [ ] No `*_ids` array is rendered as text — only `.length` is observed.

---

### Subtask T004 — Wire renderers into `renderReport(report)`

**Purpose**: Call the new renderers from the existing report-display orchestrator. They should render **before** the existing `summarizeEcosystem` text dump so the structured intelligence reads first.

**Steps**:

1. In `web/app.js`, find the `renderReport(report)` function (starts around line 173; currently sets `#score`, `#waste`, builds `#findings`, etc., and ends with `renderTimeline(...)`, `document.querySelector("#ecosystem").textContent = summarizeEcosystem(...)` (around line 201), `summarizeReceipt(...)`, `renderPaidCommandPreview(...)`).
2. Insert two new calls immediately before the `summarizeEcosystem` assignment (around line 201):
   ```js
   renderWorkflowFingerprints(report);
   renderToolingUtilization(report);
   ```
3. Do not change the order of any existing calls. Do not refactor unrelated code in this WP.

**Files**:

- `web/app.js` — 2 new lines inside `renderReport`.

**Validation**:

- [ ] Both renderers are called from `renderReport`.
- [ ] Existing `summarizeEcosystem`, `summarizeReceipt`, `renderTimeline`, `renderPaidCommandPreview` calls are intact and in original relative order.
- [ ] Manual browser QA: loading any report does not produce a JS console error from this code path.

---

### Subtask T005 — Add scoped CSS to `web/styles.css`

**Purpose**: Make the new sections compact, CLI-native, and visually distinguishable per band, without touching any existing rule.

**Steps**:

1. Append a new block at the end of `web/styles.css`:
   ```css
   /* Report Intelligence UX sections — kitty-specs/report-intelligence-ux-01KS070G */

   .intel-section { margin: 1.5rem 0; }
   .intel-section[hidden] { display: none; }

   /* Workflow Fingerprints */
   #workflow-fingerprints-list { list-style: none; padding: 0; margin: 0; }
   .fingerprint-row {
     display: flex; flex-wrap: wrap; gap: 0.5rem 1rem;
     padding: 0.5rem 0; border-bottom: 1px solid var(--rule, rgba(255,255,255,0.08));
     font-family: inherit;
   }
   .fingerprint-confidence { font-size: 0.85em; padding: 0.05em 0.4em; border-radius: 3px; }
   .confidence-high   { background: rgba(80, 200, 120, 0.18); }
   .confidence-medium { background: rgba(240, 200, 80, 0.18); }
   .confidence-low    { background: rgba(200, 200, 200, 0.18); }
   .fingerprint-sources { list-style: none; padding: 0; margin: 0; display: inline-flex; gap: 0.3rem; flex-wrap: wrap; }
   .fingerprint-sources li {
     font-size: 0.8em; padding: 0.05em 0.4em; border-radius: 3px;
     background: rgba(120, 160, 255, 0.16);
   }
   .fingerprint-evidence,
   .fingerprint-version { font-size: 0.85em; opacity: 0.85; }
   .fingerprint-active,
   .fingerprint-installed {
     font-size: 0.8em; padding: 0.05em 0.4em; border-radius: 3px;
     background: rgba(80, 200, 120, 0.18);
   }

   /* Tooling Utilization */
   #tooling-utilization-rows { display: grid; gap: 0.75rem; }
   .utilization-row {
     display: grid; grid-template-columns: minmax(6rem, max-content) 1fr;
     gap: 0.5rem 1rem;
     padding: 0.5rem 0; border-bottom: 1px solid var(--rule, rgba(255,255,255,0.08));
   }
   .utilization-row .surface-header { font-weight: 600; }
   .utilization-row .surface-body { display: flex; flex-wrap: wrap; gap: 0.4rem 0.8rem; align-items: center; font-size: 0.9em; }
   .band-chip {
     font-size: 0.8em; padding: 0.05em 0.4em; border-radius: 3px;
     text-transform: lowercase;
   }
   .band-severe  { background: rgba(220, 80, 80, 0.22); }
   .band-high    { background: rgba(220, 140, 60, 0.22); }
   .band-watch   { background: rgba(240, 200, 80, 0.22); }
   .band-normal  { background: rgba(80, 200, 120, 0.18); }
   .band-unknown { background: rgba(180, 180, 180, 0.18); }
   .band-advice {
     margin-top: 0.4rem; padding: 0.5rem 0.75rem;
     background: rgba(220, 140, 60, 0.10);
     border-left: 2px solid rgba(220, 140, 60, 0.55);
     font-size: 0.9em;
   }
   ```
2. Use only additive rules. Do not change any existing selector.
3. The `var(--rule, …)` fallback covers themes that don't define the variable. Avoid introducing new CSS variables.

**Files**:

- `web/styles.css` — additive block, ~50 lines.

**Validation**:

- [ ] `git diff main -- web/styles.css` shows only appended rules; no existing rule was modified.
- [ ] Band chip variants render with visible color differences in the browser.
- [ ] Section spacing remains compact (no horizontal scrolling at the existing report's content width).

---

### Subtask T006 — Add Go-side renderer-input leak test

**Purpose**: Structurally backstop NFR-001 — assert that the JSON the renderers consume never contains a private canary string, even from a hostile transcript. Also pin the band-finding correspondence so a future analyzer change can't silently break the advice block.

**Steps**:

1. Create `internal/analyzer/view_render_inputs_test.go`. Use the same `package analyzer_test` as `leak_test.go`. Import the project's analyzer package via the same module path used in `leak_test.go` (`github.com/robertdouglass/claude-log-analyzer/internal/analyzer`).

2. Add `TestRenderInputs_NoCanaryInRendererJSON` — extends the existing leak canary to **all** fields the renderers consume:
   - Reuse the `buildHostileInput` helper pattern from `leak_test.go` (clone it locally to avoid editing `leak_test.go`, which is not in `owned_files`).
   - Build the canary list as the union of `leak_test.go`'s 16 canary strings plus these three explicitly tagged for this WP:
     ```
     "mcp__renderprivate__rogue",
     "skill__renderprivate__rogue",
     "plugin__renderprivate__rogue",
     ```
   - Call `analyzer.Analyze("job-render-1", input)`.
   - Marshal each of these renderer-input surfaces separately to JSON and assert no canary substring appears in any of them:
     - `rep.Ecosystem.WorkflowFingerprints`
     - `rep.Ecosystem.ToolingUtilization`
     - `rep.Findings` — the **entire** findings slice, not just the four `*_bloat_*` IDs (the renderer reads all of `report.findings[]` when resolving advice; any current or future finding whose recommendation could carry private text must be caught here).
   - Failure messages should name the surface and the canary so a regression is easy to bisect.

3. Add `TestPruningAdviceRecommendations_BytewiseEqualToConstants` — pin the four advice strings as static copy and prevent silent drift:
   - Drive `analyzer.Analyze` over a hostile input crafted to land MCP and Skill in `severe` and `high` bands (you may need two or three Analyze invocations to cover all four IDs across configurations).
   - For each emission of `mcp_bloat_severe`, `mcp_bloat_high`, `skill_bloat_severe`, `skill_bloat_high`, assert the `Recommendation` field equals the literal string emitted by `internal/analyzer/analyzer.go` lines 381–393 (copy the four strings into the test as `const` values, verbatim).
   - Rationale: locks the advice copy as static; any reword in `analyzer.go` requires updating this test, which forces a contemporaneous review of NFR-007 (no private-name leakage in advice text).

4. Add `TestBandFindingPairings_DrivenByAnalyze` — pin the band→finding-ID contract end-to-end:
   - Build a small fixture set: at least one hostile input that produces MCP `severe`, one that produces MCP `high`, and one that produces neither (so the absence assertion is exercised). Same for Skill if practically separable; otherwise rely on the analyzer's independent classification per surface.
   - For each fixture, run `analyzer.Analyze` and collect `report.Findings`.
   - Assert the pairing table (per surface):
     - When `rep.Ecosystem.ToolingUtilization.MCP.WarningBand == "severe"` → `findings[]` contains exactly one entry with `id == "mcp_bloat_severe"`.
     - When `rep.Ecosystem.ToolingUtilization.MCP.WarningBand == "high"` → exactly one `mcp_bloat_high`.
     - When `rep.Ecosystem.ToolingUtilization.MCP.WarningBand ∈ {watch, normal, unknown}` → zero entries whose `id` starts with `mcp_bloat_`.
     - Same rules for Skill (`skill_bloat_*`).
   - "Exactly one" pins INV-6 (each of the four advice IDs appears at most once per report). The UI lookup uses `.find()` (first match), so duplicate emissions would silently hide one — pinning single-emission protects that invariant.
   - Use the exported constants `analyzer.WarningBandSevere`, `WarningBandHigh`, `WarningBandWatch`, `WarningBandNormal`, `WarningBandUnknown` from `internal/analyzer/tooling_buckets.go` lines 90–94.

5. Run `go test ./internal/analyzer/ -run TestRenderInputs -v` (and the two new sibling tests above) and confirm all pass on a clean tree.

**Files**:

- `internal/analyzer/view_render_inputs_test.go` (new) — approx 150–220 lines including imports, helpers, and the three new test functions.

**Validation**:

- [ ] All three new tests pass (`go test ./internal/analyzer/ -run "TestRenderInputs_NoCanaryInRendererJSON|TestPruningAdviceRecommendations_BytewiseEqualToConstants|TestBandFindingPairings_DrivenByAnalyze" -v`).
- [ ] Canary test fails loudly if any future regression pipes private input into ANY field reachable by the renderers (fingerprints, utilization, full findings slice).
- [ ] Recommendation-byte test fails loudly if `analyzer.go` reworks one of the four advice strings without updating the test (forces NFR-007 re-review).
- [ ] Band-pairing test fails loudly if a future analyzer change removes a `*_bloat_*` finding for `severe`/`high` OR introduces one for `watch`/`normal`/`unknown` OR emits the same advice ID twice (INV-6 breach).

---

## Definition of Done

- [ ] All six subtasks T001..T006 complete with their validation checkboxes ticked.
- [ ] `gofmt -w $(find . -name '*.go' -not -path './.git/*')` — no changes left after formatting.
- [ ] `go test ./...` — all tests pass, including the three new tests in T006.
- [ ] `go vet ./...` — clean.
- [ ] `terraform -chdir=infra/aws fmt -check -recursive` — clean.
- [ ] `./scripts/smoke-local.sh` — green.
- [ ] `docker compose up --build` followed by manual browser QA per `kitty-specs/report-intelligence-ux-01KS070G/quickstart.md`:
  - [ ] Workflow Fingerprints section visible for a fixture with fingerprints.
  - [ ] MCP & Skill Utilization rows render bucket labels, counts, ratio (when `exposure_known`), inference label (when not), and a band chip per surface.
  - [ ] Advice block appears under MCP/Skill rows ONLY when band ∈ {high, severe}.
  - [ ] **FR-009 non-regression**: the existing `Ecosystem` `<pre>` block still renders all of today's fields (client, OS, workflow_frameworks, MCPs). Compare against `main` rendering for the same fixture.
  - [ ] **Privacy grep** (save rendered HTML to `/tmp/report.html` per quickstart §"Browser QA"): `grep -Eo 'mcp__[A-Za-z0-9_-]+|skill__[A-Za-z0-9_-]+|plugin__[A-Za-z0-9_-]+' /tmp/report.html` returns empty or allowlist-only IDs.
  - [ ] **NFR-005 measurement**: wrap the `renderReport(report)` call in a `performance.now()` delta once during browser QA and paste the millisecond result into the PR description. Budget: < 5 ms for the two new renderers combined (compare a "before" and "after" measurement on the same fixture).
- [ ] No new schema field anywhere (`git diff main -- internal/analyzer/types.go` shows zero diff).
- [ ] No new framework/bundler/`package.json`/`node_modules` introduced (`git diff main -- web/` does not contain any such file).
- [ ] No new outbound `fetch()` / `XMLHttpRequest` / HTTP call introduced in `web/app.js` for the new code.
- [ ] WP status moved to `for_review`.

## Risks & Reviewer Guidance

| Risk | Mitigation |
|---|---|
| Implementer copies the existing `innerHTML =` pattern from the findings renderer | Reviewer: `grep "innerHTML" web/app.js` and verify no new occurrences inside `renderWorkflowFingerprints` or `renderToolingUtilization` bodies. |
| Implementer adds a serialized field "for convenience" | Reviewer: `git diff main -- internal/analyzer/types.go` must show zero diff. C-001 violation otherwise. |
| Implementer hand-writes pruning advice copy instead of reading from `findings[]` | Reviewer: confirm the renderer's advice block sources its string from `report.findings.find(f => f.id === …)`. No string literal recommendations in `app.js`. |
| Implementer renders an `*_ids[]` array as names | Reviewer: confirm only `length` is used. The V2 UI shows counts only. |
| New JS introduces an XSS path | Reviewer: confirm `textContent` is used; no `innerHTML =` of report-derived strings; no `eval`, no `Function(...)`. |
| Test in T006 is too tightly coupled to internals and breaks on routine refactors | Reviewer: confirm the test asserts public-surface behavior (canary absence in JSON serialization; ID set in `findings[]`) — not private field internals. |

## Reviewer Quick-Check Script

```bash
git diff main -- internal/analyzer/types.go        # MUST be empty
git diff main -- web/ | grep -E '^\+.*innerHTML\s*=' && echo "FAIL: new innerHTML usage" || echo "ok"
grep -nE 'mcp_bloat_[a-z]+|skill_bloat_[a-z]+' web/app.js   # exactly 4 hits expected
gofmt -l $(find . -name '*.go' -not -path './.git/*')        # MUST be empty
go test ./internal/analyzer/ -run "TestRenderInputs|TestReportSerializationContainsNoForbiddenStrings|TestMergedAggregateContainsNoForbiddenStrings" -v
```

## Implementation Command

```bash
spec-kitty agent action implement WP01 --agent claude --mission report-intelligence-ux-01KS070G
```

## Activity Log

- 2026-05-19T14:08:25Z – claude:opus-4-7:frontend-freddy:implementer – shell_pid=79012 – Assigned agent via action command
- 2026-05-19T14:16:04Z – claude:opus-4-7:frontend-freddy:implementer – shell_pid=79012 – Ready for review — all six subtasks complete, go tests pass, four owned files only
- 2026-05-19T14:16:37Z – claude:opus-4-7:reviewer-renata:reviewer – shell_pid=80149 – Started review via action command
- 2026-05-19T14:18:59Z – claude:opus-4-7:reviewer-renata:reviewer – shell_pid=80149 – Review passed: all 9 FRs verified by diff + tests. Lane diff strictly within 4 owned files (web/index.html, web/app.js, web/styles.css, internal/analyzer/view_render_inputs_test.go); types.go/analyzer.go/ecosystem.go/leak_test.go untouched. ADVICE_LOOKUP has exactly 4 keys (mcp.severe/high, skill.severe/high) structurally enforcing FR-006. Skill row uses only Skill fields (no MCP-only unique_known_called_ids/unique_unknown_called_count). innerHTML count unchanged (6==6, no new innerHTML in new renderers). Renderers defensive (null/empty -> hidden), use textContent + replaceChildren, idempotent. 4 *_bloat_* IDs present. TestRenderInputs_NoCanaryInRendererJSON covers all 16 leak_test.go canaries + 3 WP01 canaries across Findings/WorkflowFingerprints/ToolingUtilization. TestPruningAdviceRecommendations_BytewiseEqualToConstants byte-matches analyzer.go:381-393. TestBandFindingPairings_DrivenByAnalyze pins INV-6 (exactly-one for severe/high, zero for watch/normal/unknown) end-to-end. gofmt clean, go vet clean, go test ./... green, terraform fmt clean, no new fetch/XHR/model invocations, no package.json/bundler, no next-best-recommendation/paid-pack/Stripe changes.
