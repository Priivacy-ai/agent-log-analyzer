# Implementation Plan: Report Intelligence UX

**Branch**: `main` (target) | **Date**: 2026-05-19 | **Spec**: [spec.md](spec.md)
**Mission**: `report-intelligence-ux-01KS070G`
**Mission ID**: `01KS070GDSG3W56YBCS2C8SHVY`

## Summary

Ship two compact, profiler-style report-page sections that surface the bounded-cardinality ecosystem intelligence Phase 1 (PR #75) made aggregate-safe: a **Workflow Fingerprints** section (issue #62) and a **MCP & Skill Utilization** section with band-keyed pruning advice (issue #63). All copy is sourced from already-emitted Go strings (`Finding.Recommendation` for the four `*_bloat_*` IDs); no new serialized fields, no JS framework, no LLM calls. The page never renders unknown private MCP/skill/plugin names — counts only.

## Technical Context

**Language/Version**: Go 1.23 (project root); vanilla HTML5 + ES2017+ JavaScript for `web/` (no transpile pipeline).
**Primary Dependencies**: standard library (`encoding/json`, `net/http`), the existing analyzer/remediation packages under `internal/`. Frontend: zero runtime dependencies — `web/index.html`, `web/app.js`, `web/styles.css` are served as static assets.
**Storage**: N/A (view layer; report JSON is already produced by the analyzer).
**Testing**: `go test ./...` for backend assertions; Go-side renderer-input fixture tests for privacy invariants; manual browser QA under `docker compose up --build` for FR-001..009 final acceptance.
**Target Platform**: Linux/macOS developer machines + the existing Docker compose stack (web served on `http://localhost:8080`).
**Project Type**: web (Go HTTP server with embedded static frontend).
**Performance Goals**: First paint of new sections < 5 ms over a representative fixture report (NFR-005).
**Constraints**: No new serialized fields (C-001), no unknown private names rendered anywhere (C-002), deterministic advice copy only (C-003), no new JS framework (NFR-004).
**Scale/Scope**: Two new DOM sections, two new render functions, zero new Go data types, zero new HTTP routes.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Charter context loaded (`spec-kitty charter context --action plan --json`, compact mode): template set `software-dev-default`, paradigms `domain-driven-design`, tools `git`, `spec-kitty`, Directives `DIR-001`, `DIR-002`. No org charter present.

| Gate | Status | Evidence |
|---|---|---|
| Privacy invariant honored (no private name leakage) | PASS | NFR-001 + C-002 in spec; INV-1 in data-model.md; verified by extending `internal/analyzer/leak_test.go`. |
| Bounded-cardinality invariant (no new serialized fields) | PASS | C-001 in spec; INV-2 in data-model.md; will diff `internal/analyzer/types.go` vs `main` to verify zero schema changes. |
| Deterministic-advice constraint (no LLM, no runtime fetch) | PASS | C-003 + NFR-003 in spec; D1 in research.md sources advice from existing `Finding.Recommendation`. |
| No new framework / build pipeline | PASS | NFR-004 in spec; design uses only vanilla JS edits to existing files. |
| DIR-001 / DIR-002 | PASS | These directives are charter-template gates; the design introduces no policy exceptions and no test-skipping. |

**Post-Phase-1 re-check**: no new gates surfaced during contract design — the renderer contracts (`contracts/render-workflow-fingerprints.md`, `contracts/render-tooling-utilization.md`) impose explicit prohibitions (P1..P5) that operationalize the charter rules. PASS.

## Branch Strategy

- Current branch at plan start: `main`
- Planning/base branch: `main`
- Merge target for completed changes: `main`
- `branch_matches_target`: `true` (Spec Kitty `setup-plan --json` payload, plan start)
- PR branch (created downstream during implement): `codex/report-intelligence-ux` (timestamp suffix if taken).

## Project Structure

### Documentation (this feature)

```
kitty-specs/report-intelligence-ux-01KS070G/
├── plan.md              # This file
├── spec.md              # Already committed
├── research.md          # Phase 0 — decisions D1..D8
├── data-model.md        # Phase 1 — entities consumed (no new entities)
├── contracts/
│   ├── render-workflow-fingerprints.md   # WP01 renderer contract
│   └── render-tooling-utilization.md     # WP02 renderer contract
├── quickstart.md        # Verification + browser QA recipe
├── meta.json
└── tasks/               # Populated by /spec-kitty.tasks (next phase)
```

### Source Code (repository root)

In-scope files (touched by WP01 and/or WP02):

```
web/
├── index.html           # New <section> blocks for fingerprints + utilization
├── app.js               # renderWorkflowFingerprints, renderToolingUtilization
└── styles.css           # Scoped styles for new sections, band-chip styles

internal/analyzer/
├── types.go             # READ-ONLY (no schema changes)
├── analyzer.go          # READ-ONLY (advice source — already emits *_bloat_* findings)
└── leak_test.go         # Possibly extended; or new sibling test for renderer-input shape

testdata/golden/
└── sample-report.json   # Possibly updated to surface fingerprints + utilization for browser QA
```

Out-of-scope (touched by no WP this mission):

- `internal/backend/`, `internal/awsstore/`, `cmd/`, `infra/`, `docs/cloud-launch-todo.md`, `internal/remediation/` (Phase 4+ scope).
- Any file under `web/privacy/`, `web/security/` (separate trust-page content).

## Phase 0: Outline & Research

Phase 0 is complete — see `research.md`. Eight design decisions resolved:

| ID | Topic | Outcome |
|---|---|---|
| D1 | Pruning advice copy source | Reuse `Finding.Recommendation` for the four `*_bloat_*` IDs |
| D2 | WP decomposition | Two parallel WPs aligned to issues #62 and #63 |
| D3 | Test strategy | Go-side renderer-input fixture + extended leak canary; manual browser QA as final gate |
| D4 | Existing flat ecosystem `<pre>` | Keep, place new sections above it |
| D5 | Confidence / source label vocabulary | Render the enum value verbatim |
| D6 | Sources rendering | Compact badge row |
| D7 | Ratio when `exposure_known==false` | Suppress ratio; show `inference_source` label |
| D8 | MCP/Skill row ordering | MCP first, then Skill (struct field order) |

## Phase 1: Design & Contracts

Phase 1 is complete:

- **`data-model.md`** documents the read-only entities (`Ecosystem`, `EcosystemFingerprint`, `MCPUtilization`, `SkillUtilization`, `Finding`) and their renderer-vs-data privacy classification, plus five invariants (INV-1..5).
- **`contracts/render-workflow-fingerprints.md`** specifies the WP01 renderer's input shape, behavior, prohibitions (P1..P5), and verification checks (C1..C5).
- **`contracts/render-tooling-utilization.md`** specifies the WP02 renderer's input shape, behavior including the band → finding-ID lookup table, prohibitions (P1..P5), and verification checks (C1..C10).
- **`quickstart.md`** documents the full verification recipe including the Docker browser-QA gate and an explicit privacy-grep over rendered HTML.

## Work Package Outline (drafted; finalized in /spec-kitty.tasks)

**Revision during /spec-kitty.tasks**: the original two-WP split was collapsed to a single 6-subtask WP because both proposed WPs would have needed to own `web/index.html`, `web/app.js`, and `web/styles.css` — the finalizer rejects overlapping `owned_files`. The merged WP still covers both issues (#62 and #63) and stays within the 3–7 subtask ideal range.

### WP01 — Workflow Fingerprints section (issue #62) — superseded by merged WP below

**Scope**:
- `web/index.html`: add `<section id="workflow-fingerprints" hidden>` block above the existing `Ecosystem` block. Markup follows the contract's row shape (id, confidence, sources badges, evidence count, active/installed indicators, optional version_bucket).
- `web/app.js`: implement `renderWorkflowFingerprints(report)` per `contracts/render-workflow-fingerprints.md`. Wire it into the existing `applyReport(report)` flow alongside the current renderers.
- `web/styles.css`: add scoped styles for the new section (`.fingerprint-row`, `.fingerprint-sources`, `.fingerprint-indicator`, confidence-label variants).
- Tests: small Go-side fixture test that loads a synthetic report JSON with one fingerprint per confidence band and asserts the renderer prohibitions hold at the data layer (`textContent`-only would be a JS check — backstop with a hostile-fixture leak assertion).

**Acceptance**:
- FR-001, FR-002 satisfied
- NFR-001, NFR-002, NFR-004, NFR-006 satisfied
- C-001, C-002 honored
- All five verification checks (C1..C5) in the contract pass.

**Mapped FRs**: FR-001, FR-002, FR-008 (graceful handling), FR-009 (preserve existing ecosystem block).

### WP02 — MCP & Skill Utilization section + pruning advice (issue #63)

**Scope**:
- `web/index.html`: add `<section id="tooling-utilization" hidden>` block above the existing `Ecosystem` block (below WP01's section). Sub-blocks for MCP row and Skill row, each with bucket cells, count cells, warning-band chip slot, and an advice-block slot.
- `web/app.js`: implement `renderToolingUtilization(report)` per `contracts/render-tooling-utilization.md`. Includes the band → finding-ID lookup (4 keys) and `exposure_known` gating for the ratio.
- `web/styles.css`: scoped styles for utilization rows, band-chip variants (`.band-severe`, `.band-high`, `.band-watch`, `.band-normal`, `.band-unknown`), advice-block container.
- Tests: small Go-side fixture test with five band-permutation fixtures + a hostile-fixture leak assertion proving no canary string would reach the renderer input.

**Acceptance**:
- FR-003 .. FR-007 satisfied
- NFR-001 .. NFR-007 satisfied
- C-001 .. C-007 honored
- All ten verification checks (C1..C10) in the contract pass.

**Mapped FRs**: FR-003, FR-004, FR-005, FR-006, FR-007, FR-008.

### Merged WP01 — Report Intelligence UX sections (issues #62 + #63)

Owns the entire `web/` UI delta for this mission plus the renderer-input leak test.

- `web/index.html`: both `<section id="workflow-fingerprints">` and `<section id="tooling-utilization">` blocks (initially `hidden`).
- `web/app.js`: `renderWorkflowFingerprints(report)`, `renderToolingUtilization(report)`, wiring into `applyReport(report)`.
- `web/styles.css`: scoped styles for fingerprint rows, utilization rows, band-chip variants, advice block.
- `internal/analyzer/view_render_inputs_test.go` (new): renderer-input leak canary covering fingerprint + utilization input fields + all 5 band permutations.

### FR Coverage Matrix (single merged WP)

| FR | WP01 |
|---|---|
| FR-001 | ✓ |
| FR-002 | ✓ |
| FR-003 | ✓ |
| FR-004 | ✓ |
| FR-005 | ✓ |
| FR-006 | ✓ |
| FR-007 | ✓ |
| FR-008 | ✓ |
| FR-009 | ✓ |

Every FR is owned by the merged WP. NFR/C invariants are honored within it.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| A fixture used for browser QA does not surface a `*_bloat_*` band | Medium | Slows manual QA only | Use `testdata/golden/sample-report.json` or hand-craft a small fixture in `testdata/fixtures/` that produces a `high`/`severe` band. |
| Future schema change removes one of the four `*_bloat_*` findings | Low | Advice block silently disappears for that surface | Add a comment in `app.js` referencing `internal/analyzer/analyzer.go:368-394`; if the analyzer changes those IDs, this UI must be updated in lockstep. |
| Goldens change in a way that requires `WorkflowFingerprints` nilling | Medium | CI flake on hosts where SDD CLIs are present in `$PATH` | Phase 1 already nils fingerprints in single-report goldens; if a new golden surfaces this, follow the same pattern. |
| Reviewer flags `<pre>` JSON dump as implementation detail | Low | Minor spec-quality nit | Spec checklist already notes the FR-009 framing — defer to D4 in research.md. |

## Verification Baseline

Before merge:

```bash
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
terraform -chdir=infra/aws fmt -check -recursive
./scripts/smoke-local.sh
docker compose up --build   # plus manual browser QA per quickstart.md
```

Optional after report/API changes (none expected here, but available):

```bash
./scripts/load-local.sh 25
```

## Mission Out-of-Scope (re-affirm)

- Phase 3 next-best-recommendation card (#73 — `codex/recommendation-phase-b`).
- Phase 4 paid-pack personalization, Stripe (#24/#27), waiver-gated plugin (#30/#31/#33), signed releases (#34).
- Phase 5 privacy analytics gates (#58–#61, #65).
- Phase 6 trust/distribution (#34/#36/#37).
- Phase 7 cloud launch hardening.
- Any change to `internal/analyzer/types.go` schema.

## Branch Strategy (final restate)

- Current branch: `main`
- Planning/base branch: `main`
- Merge target: `main`
- `branch_matches_target`: `true`

## Next Suggested Command

`/spec-kitty.tasks` — generate WP01 and WP02 work-package files mapped to FR coverage matrix above. Run `spec-kitty agent action finalize-tasks --validate-only` before the mutating finalize call per the Phase 1 tactical hints.
