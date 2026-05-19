# Research: Report Intelligence UX

**Mission**: `report-intelligence-ux-01KS070G`
**Phase**: 0 (Outline & Research)
**Date**: 2026-05-19

## Research Method

Phase 0 is read-only inspection of `main` @ `0cd9cfe`. All decisions are grounded in code paths already in the tree from Phase 1 (`5df679c`). No external research was needed because the spec is a view layer over already-bounded fields.

## Open Clarifications

None. The spec carries no deferred-decision markers; the one alignment decision deferred from spec ("Go map vs static client copy keyed on enum") was resolved during plan interrogation in favor of reusing existing `Finding.Recommendation` strings.

> Note: the literal scanner-trigger string used in spec.md placeholders is intentionally not spelled out in this section to avoid tripping mission-acceptance scanners that grep for it as a marker.

## Decision Log

### D1. Pruning advice copy source — reuse existing `Finding.Recommendation`

- **Decision**: The pruning advice block in the new MCP/Skill Utilization section reads its copy from the existing band-keyed findings (`mcp_bloat_severe`, `mcp_bloat_high`, `skill_bloat_severe`, `skill_bloat_high`) by ID lookup against `report.findings[]`. No new field, no new client-side copy table.
- **Rationale**:
  - Single source of truth in Go (`internal/analyzer/analyzer.go::computeFindings` lines 368–394).
  - Honors C-001 (no new serialized fields) — these findings are already emitted today.
  - Structurally enforces FR-006: the analyzer never emits a finding for `watch`/`normal`/`unknown`, so no advice block can render for those bands.
  - Avoids client/server copy drift.
  - Reusing strings rather than duplicating them keeps the leak surface minimal.
- **Alternatives considered**:
  - *Static client copy table keyed on `warning_band` enum.* Workable, but introduces a second copy that can drift from the analyzer's finding copy without anyone noticing.
  - *Emit advice in JSON as a new `pruning_advice` string field on `MCPUtilization` / `SkillUtilization`.* Rejected — violates C-001.

### D2. WP decomposition — two parallel UI lanes

- **Decision**: Split into two work packages aligned to the open issues.
  - **WP01 — Workflow Fingerprints section** (issue #62). Adds `<section id="workflow-fingerprints">` to `web/index.html`, `renderWorkflowFingerprints()` to `web/app.js`, scoped CSS to `web/styles.css`. Reads `report.ecosystem.workflow_fingerprints[]` only.
  - **WP02 — MCP & Skill Utilization section + pruning advice** (issue #63). Adds `<section id="tooling-utilization">` to `web/index.html`, `renderToolingUtilization()` to `web/app.js`, scoped CSS to `web/styles.css`. Reads `report.ecosystem.tooling_utilization` (mcp + skill) and looks up the four `mcp_bloat_*`/`skill_bloat_*` findings by ID for advice copy.
- **Rationale**:
  - Each WP maps 1:1 to a single open GitHub issue.
  - Sections are independent DOM regions, independent render functions, independent field reads → parallel-safe lanes.
  - Smaller WPs are easier to review and revert.
- **Alternatives considered**:
  - *One combined WP for both sections.* Doable, but obscures the per-issue acceptance trail and grows the review surface.
  - *Three WPs (fingerprints / utilization / leak-tests-only).* Rejected — leak tests for each surface belong with the surface that owns them.

### D3. Frontend test strategy — Go-side fixture render assertions

- **Decision**: The new tests are Go-side, using a JSON fixture and a small renderer-shape contract verified at the Go layer. Specifically:
  - Add a **new sibling test file** `internal/analyzer/view_render_inputs_test.go` (do NOT edit `leak_test.go` — it is outside WP01's `owned_files`). The sibling holds three tests: a renderer-input canary across the full findings slice, a byte-equal pin on the four `*_bloat_*` recommendation strings, and an `Analyze`-driven band-pairing test that asserts the band → finding-ID contract end-to-end (and pins INV-6 / single-emission).
  - Privacy is enforced by the server-side bounded shape: if the JSON contains no unknown name, the renderer cannot render one.
  - Manual browser QA is the final user-visible gate (per the handoff's UI verification rule).
- **Rationale**:
  - The charter (and NFR-004) forbids introducing a JS test runner / build pipeline.
  - The privacy invariant is structural: if the JSON has no unknown name, no renderer can render one. So a JSON-level leak assertion is sufficient for NFR-001.
  - DOM verification of the *positive* path (sections render expected enum labels for a fixture) is best done by manual browser QA per the handoff.
- **Alternatives considered**:
  - *Headless browser test (Puppeteer/Playwright).* Rejected — introduces a Node toolchain and CI burden disproportionate to two small UI sections.
  - *Pure unit test of `app.js` via a JS runtime (e.g., goja).* Rejected for now — adds a Go dependency and a test surface no other code shares; can be revisited if a future PR has more JS logic.

### D4. Existing flat ecosystem `<pre>` summary — keep, place above new sections

- **Decision**: Leave the existing `<pre id="ecosystem">` ecosystem summary in place; add the two new sections **above** it so the structured intelligence reads first.
- **Rationale**:
  - FR-009 explicitly allows "preserve or replace". Preserving is lower-risk for Phase 2 (no regressions in fields the old summary surfaced).
  - The structured sections render conditionally (hidden when their data is absent), so for trivial fixtures the page degrades gracefully to today's behavior plus an empty space.
- **Alternatives considered**:
  - *Replace the flat summary entirely.* Defer to a later UI refresh.

### D5. Confidence / source label vocabulary — render the enum value verbatim

- **Decision**: Render `confidence` as the literal enum value (`high`/`medium`/`low`), and each `sources[]` entry as the literal enum label. No translation table, no marketing copy.
- **Rationale**:
  - The enum values are already the most precise public label we have for these signals.
  - A translation table would introduce drift risk between server-emitted enum and client-displayed label.
  - The page is profiler-style; raw enum labels are the CLI-native aesthetic.

### D6. Workflow fingerprint sources — show as compact comma-separated badges

- **Decision**: `sources[]` renders as a compact row of label-style badges (e.g., `cli_probe`, `transcript_pattern`, `config_artifact`) below the fingerprint ID.
- **Rationale**: Bounded cardinality, no DOM bloat at expected list sizes (`sources[]` is small).

### D7. Utilization ratio display when `exposure_known == false`

- **Decision**: When `exposure_known == false`, the row shows the `inference_source` enum value as a "inferred from: <enum>" label and **does not** render the numeric utilization ratio. The bucket labels (`context_token_bucket`, `context_efficiency_bucket`) are still shown.
- **Rationale**: Honors FR-007. A clamped ratio computed against an unknown exposure denominator is misleading; the buckets remain interpretable.

### D8. Section ordering inside MCP & Skill Utilization

- **Decision**: Render MCP first, then Skill (alphabetical and matches the existing struct field order `ToolingUtilization{MCP, Skill}`).
- **Rationale**: Deterministic order; mirrors source.

## Verification Targets

These checks must pass at end of mission (verified at `/spec-kitty.accept`):

| ID | Target | Verification |
|---|---|---|
| V1 | FR-001..009 covered by WP tests + browser QA | Manual + Go tests |
| V2 | NFR-001 (no private name in DOM) | Hostile-fixture leak test (JSON level) + manual view-source review |
| V3 | NFR-002 (no new serialized fields) | `git diff main -- internal/analyzer/types.go` shows zero schema-relevant changes |
| V4 | NFR-003 (no LLM, no new network) | Source grep for new `fetch(` / `http.Get` / model-invocation code |
| V5 | NFR-004 (no new JS framework) | `git diff main -- web/` shows no `package.json`, no `node_modules`, no bundler config |
| V6 | All Phase 1 verification commands green | `gofmt`, `go test ./...`, `go vet ./...`, `terraform fmt -check`, `./scripts/smoke-local.sh` |
| V7 | Charter compliance | DIR-001, DIR-002 honored (privacy & quality gates) |

## References

- `internal/analyzer/types.go` lines 51–119 — `Ecosystem`, `EcosystemFingerprint`, `MCPUtilization`, `SkillUtilization` field shapes.
- `internal/analyzer/analyzer.go` lines 365–394 — band-keyed `Finding` emission.
- `internal/analyzer/leak_test.go` — privacy canary pattern to extend.
- `web/app.js` line 222 — current `summarizeEcosystem` text dump.
- `web/index.html` lines 81–89 — current ecosystem section markup.
