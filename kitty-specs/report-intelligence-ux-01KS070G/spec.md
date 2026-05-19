# Feature Specification: Report Intelligence UX

**Mission**: `report-intelligence-ux-01KS070G`
**Mission ID**: `01KS070GDSG3W56YBCS2C8SHVY`
**Mission Type**: `software-dev`
**Target Branch**: `main`
**Created**: 2026-05-19

## Purpose

**TLDR**: Surface SDD fingerprints and MCP/skill utilization in the report UI with deterministic pruning advice and zero private-name leakage.

**Context**: Developers running claude-log-analyzer need to see which spec-driven-development tooling and MCP/skill bloat their workflow has, plus actionable pruning guidance, without the report ever rendering unknown private names. This mission ships the view layer over the already-bounded ecosystem data Phase 1 made aggregate-safe.

## Primary User Scenario

A developer runs `claude-analyzer analyze <log>` (or uploads a sanitized report) and opens the resulting report page. Today the page shows only a flat `Ecosystem` JSON-style dump and gives no signal about which spec-driven-development workflow they're running, how much of their MCP/skill surface they actually use, or what to do about bloat. After this mission, the developer sees two new compact, profiler-style sections:

- **Workflow Fingerprints** — which public SDD tooling was detected (Spec Kitty, GitHub Spec Kit, OpenSpec, …), with the detector's confidence, source classes, and whether the tooling appears active or merely installed.
- **MCP & Skill Utilization** — per-class rows for MCP servers and Skills showing exposed-count bucket, calls/executions, utilization ratio, context-footprint bucket, and a warning band. When the warning band is `high` or `severe`, a deterministic pruning/lazy-loading/scoping advice block renders beneath the row.

The developer leaves the page understanding which tooling is bloated and what to consider pruning, without anything private having appeared on screen or in the DOM source.

### Acceptance Scenario (happy path)

Given a report whose `ecosystem.workflow_fingerprints` contains a high-confidence Spec Kitty fingerprint and whose `ecosystem.tooling_utilization.mcp.warning_band == "severe"`,
when the developer opens the report,
then the Workflow Fingerprints section shows the Spec Kitty row with its confidence label, source-class labels, and active/installed indicators; the MCP row shows the severe warning band; and a pruning advice block appears under the MCP row with deterministic, band-keyed copy.

### Edge Cases & Exception Paths

1. **Empty fingerprints array** — section is hidden entirely (no empty header, no "none detected" placeholder).
2. **Tooling utilization with `exposure_known == false`** — render the row with the inference-source label and the `unknown` warning band; suppress the utilization ratio when it would mislead.
3. **Unknown counts only (no known IDs)** — render the unknown count and band, but never a private name; the row remains useful as a bloat signal.
4. **Warning band `watch`** — show the band as a label, do **not** show the pruning advice block.
5. **Warning band `normal` / `unknown`** — show the band as a label, do **not** show the pruning advice block.
6. **Report JSON with `ecosystem == null` or missing `tooling_utilization` block** — both new sections hidden; no console errors.

## Domain Language

| Term | Canonical | Avoid |
|---|---|---|
| Workflow fingerprint | the detected public SDD tool record (`EcosystemFingerprint`) | "SDD detection", "framework hit" |
| Warning band | `severe > high > watch > normal > unknown` rank order | "severity", "alert level" |
| Pruning advice | static, band-keyed copy recommending prune / lazy-load / scope | "recommendation" (reserved for #73 Phase B) |
| Utilization ratio | `utilization_ratio_pct` (integer 0–100) | "usage %" without bound |
| Context-footprint bucket | `context_token_bucket` / `context_efficiency_bucket` | "size" |
| Source class | `EcosystemFingerprint.sources[]` closed-enum entries | "evidence text" (forbidden to render) |

## Functional Requirements

| ID | Description | Status |
|---|---|---|
| FR-001 | The report page renders a Workflow Fingerprints section whenever `ecosystem.workflow_fingerprints` has at least one entry; the section is hidden when the array is empty or missing. | Approved |
| FR-002 | Each fingerprint row displays the fingerprint's allowlisted public `id`, `confidence` label, `sources[]` as discrete labels, and active/installed flags using distinct visual indicators. | Approved |
| FR-003 | The report page renders an MCP & Skill Utilization section whenever `ecosystem.tooling_utilization` is present; the section is hidden when the block is missing. | Approved |
| FR-004 | The MCP row displays `server_count_bucket`, `exposed_tool_count_bucket`, `context_token_bucket`, `call_count`, `utilization_ratio_pct`, `context_efficiency_bucket`, and `warning_band`. The Skill row displays the equivalent fields from `SkillUtilization`. | Approved |
| FR-005 | When `warning_band` is `high` or `severe`, the page renders a deterministic pruning advice block under the row using copy keyed strictly on the warning band (and optionally on a coarse `context_efficiency_bucket` enum). | Approved |
| FR-006 | When `warning_band` is `watch`, `normal`, or `unknown`, the page renders the band label but does NOT render the pruning advice block. | Approved |
| FR-007 | When `exposure_known == false`, the row clearly labels the data as inferred (showing the `inference_source` enum value) and suppresses the utilization ratio display so it cannot be misread. | Approved |
| FR-008 | Both new sections gracefully handle empty/missing fields without throwing JS errors or rendering placeholder strings that look like data. | Approved |
| FR-009 | The existing flat `Ecosystem` `<pre>` summary is preserved or replaced with structured rendering that conveys the same fields; either approach is acceptable provided no previously-shown information regresses. | Approved |

## Non-Functional Requirements

| ID | Description | Threshold | Status |
|---|---|---|---|
| NFR-001 | The DOM rendered for any report must contain zero unknown private MCP, skill, or plugin name strings. | Verifiable by HTML source inspection over the fixture set; automated leak test counts zero occurrences of any string outside the public allowlist. | Approved |
| NFR-002 | The new sections must add no new serialized fields to any report JSON, aggregate event, or upload schema. | Schema diff vs. `main` shows zero new keys in `Report` / `Ecosystem` / `AggregateSafeEvent`; verified by leak-style test or schema dump diff. | Approved |
| NFR-003 | Pruning advice copy is produced deterministically from a small Go map (or equivalent static client copy keyed on the warning-band enum). No LLM/API calls are made to produce advice. | Source-tree grep shows no new outbound HTTP, no model invocation, no JS `fetch()` call introduced by this mission. | Approved |
| NFR-004 | The web UI introduces no new JS framework, no bundler, no build pipeline. Vanilla JS only, served by the existing static file path. | `web/` keeps current file count shape (`index.html`, `app.js`, `styles.css`); no `package.json`, no `node_modules`. | Approved |
| NFR-005 | First paint of the new sections does not measurably regress report page render. | Delta of `performance.now()` around `renderReport(report)` measured twice — once against `main`, once against the WP branch — on the same fixture; the difference (after − before) must be < 5 ms on a developer laptop, captured in the PR description. (Methodology in `quickstart.md` § "NFR-005 measurement".) | Approved |
| NFR-006 | All values displayed in the new sections come from existing bounded-cardinality fields (closed enums, buckets, integer counts, allowlisted IDs). | Verifiable by referencing the field list in FR-002 / FR-004 / FR-007 against `internal/analyzer/types.go`. | Approved |
| NFR-007 | Pruning advice copy passes a "no private name leakage" review: copy is generic and never embeds an unknown name or a count rendered in a way that implies a name. | Manual review against the band-keyed copy table during planning. | Approved |

## Constraints

| ID | Description | Status |
|---|---|---|
| C-001 | No new serialized fields may be added to `internal/analyzer/types.go` for this mission. The UI must read existing fields. | Approved |
| C-002 | Unknown private MCP/skill/plugin names must never appear in the DOM, console, network requests, or any artifact this mission writes. | Approved |
| C-003 | Pruning advice copy must be deterministic (band-keyed lookup). No LLM, no remote call, no template that interpolates user data. | Approved |
| C-004 | The mission must not bundle Phase 3 work (next-best-recommendation card per #73). That mission slot is `codex/recommendation-phase-b`. | Approved |
| C-005 | The mission must not bundle Phase 4+ work (paid pack personalization, Stripe, trust/distribution, cloud). | Approved |
| C-006 | No Terraform apply, no production deploy in this mission. Local Docker smoke is the deployment posture. | Approved |
| C-007 | The mission must not show or persist raw evidence text from SDD detection (counts and source-class enum only). | Approved |

## Success Criteria

1. **Visibility**: A developer reading a report whose JSON contains at least one workflow fingerprint and at least one `tooling_utilization` block can describe both their detected SDD workflow and which tooling class (MCP or Skill) is most bloated within 30 seconds of opening the report.
2. **Actionability**: For any report where `warning_band ∈ {high, severe}` on MCP or Skill, the developer sees at least one concrete pruning/lazy-loading/scoping suggestion that does not require reading external documentation.
3. **Privacy**: Inspection of the rendered DOM over the fixture suite produces zero occurrences of any private MCP / skill / plugin name string. Unknown surface area is conveyed only by counts.
4. **Stability**: All existing report fields that render today (score, waste, findings, timeline, immediate fixes, security receipt, ecosystem summary fields) continue to render correctly. No existing automated test regresses.
5. **Compactness**: The new sections fit visually within the existing profiler-page style; no horizontal scrolling at the existing report's content width.

## Key Entities

- **EcosystemFingerprint** (`internal/analyzer/types.go`): bounded record — `id`, `confidence`, `sources[]`, `evidence_count`, `active`, `installed`, `version_bucket`.
- **MCPUtilization** (`internal/analyzer/types.go`): includes `known_server_ids`, `unknown_server_count`, `server_count_bucket`, `exposed_tool_count_bucket`, `context_token_bucket`, `exposure_known`, `inference_source`, `call_count`, `known_call_count`, `unknown_call_count`, `unique_known_called_ids`, `unique_unknown_called_count`, `utilization_ratio_pct`, `context_efficiency_bucket`, `warning_band`.
- **SkillUtilization** (`internal/analyzer/types.go`): analogous field set scoped to skills.
- **Pruning Advice Source**: a small deterministic lookup keyed strictly on `warning_band` (and optionally on `context_efficiency_bucket`). Lives in Go (preferred — emitted into the report JSON as already-bounded enum-keyed text) **or** as a static client-side copy table keyed on the enum. Decision deferred to plan phase.

## Assumptions

1. Phase 1 (PR #75) is correctly merged into `main`; aggregate merge preserves `WorkflowFingerprints` and `ToolingUtilization`.
2. The existing `web/` static file pipeline serves `index.html`, `app.js`, `styles.css` from the Go HTTP server and is the deployment surface for this mission.
3. Test fixtures `01` and `07` (referenced in the handoff) produce `tooling_utilization` blocks with at least one warning band ≠ `normal` for manual browser QA.
4. The existing leak test infrastructure in `internal/analyzer/leak_test.go` (and friends) can be extended to assert no unknown names appear in any DOM-bound artifact this mission may emit (e.g., the band-keyed pruning advice strings).
5. "Deterministic Go map" vs. "static client copy table" is an implementation detail safely deferred to plan; both satisfy NFR-003 and C-003.

## Out of Scope

- Next-best-recommendation card (issue #73 / Phase 3 / `codex/recommendation-phase-b`).
- Paid pack personalization using fingerprint + utilization data (issue #64 / Phase 4).
- Aggregate analytics schema and bounded-cardinality storage (issues #58–#61, #65 / Phase 5).
- Stripe unlock, signed CLI releases, brand-safe hostname (Phases 4–6).
- Cloud / WAF / TTL / load testing (Phase 7).
- Adding new serialized fields anywhere.
- Re-litigating Phase 1 correctness fixes.
