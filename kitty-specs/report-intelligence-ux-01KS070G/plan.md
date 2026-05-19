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

## Work Package Outline

> **History note**: The plan originally proposed two WPs (one per issue). Both would have needed to own `web/index.html`, `web/app.js`, and `web/styles.css`, which the finalizer rejects as overlapping `owned_files`. Collapsed to a single 6-subtask WP that stays within the 3–7 ideal range. The collapsed shape was confirmed by pre-implementation review (Architect Alphonso + Reviewer Renata, 2026-05-19) and the review findings are folded into the artifact set below.

### WP01 — Report Intelligence UX sections (issues #62 + #63)

Owns the entire `web/` UI delta for this mission plus the renderer-input leak test.

- `web/index.html`: both `<section id="workflow-fingerprints">` and `<section id="tooling-utilization">` blocks (initially `hidden`).
- `web/app.js`: `renderWorkflowFingerprints(report)`, `renderToolingUtilization(report)`, wiring into `renderReport(report)`.
- `web/styles.css`: scoped styles for fingerprint rows, utilization rows, band-chip variants, advice block.
- `internal/analyzer/view_render_inputs_test.go` (new): renderer-input leak canary covering fingerprint + utilization input fields + all 5 band permutations.

### Requirement Coverage Matrix

| Requirement | WP01 subtask(s) | Verification artifact |
|---|---|---|
| FR-001 | T001, T002 | renderer hidden when array empty; manual browser QA |
| FR-002 | T002 | row content matches fingerprint contract C1..C5; manual browser QA |
| FR-003 | T001, T003 | renderer hidden when block missing; manual browser QA |
| FR-004 | T003 | bucket/count/band cells match utilization contract C2; manual browser QA |
| FR-005 | T003 | advice block renders for `high`/`severe` (C3, C4); manual browser QA |
| FR-006 | T003 | no advice for `watch`/`normal`/`unknown` (C5, C6, C7); source grep on `ADVICE_LOOKUP`; T006 band-pairing test |
| FR-007 | T003 | `exposure_known === false` → no ratio shown (C8); manual browser QA |
| FR-008 | T002, T003 | renderer no-throw on missing inputs; defensive entry verified in source |
| FR-009 | T001, T004 | existing `Ecosystem` block intact; DoD non-regression checkbox |
| NFR-001 | T002, T003, T006 | T006 `TestRenderInputs_NoCanaryInRendererJSON` + manual DOM grep |
| NFR-002 | All | DoD: `git diff main -- internal/analyzer/types.go` is empty |
| NFR-003 | T003 | DoD: source grep shows no new `fetch`/HTTP/model call in `web/app.js` |
| NFR-004 | All | DoD: `git diff main -- web/` introduces no `package.json` / bundler |
| NFR-005 | T004 | DoD: `performance.now()` delta pasted into PR (< 5 ms budget) |
| NFR-006 | T002, T003 | bucket/enum/count fields enumerated in `data-model.md` |
| NFR-007 | T006 | byte-equal pin of advice strings to `analyzer.go:381-393` constants |
| C-001 | All | DoD: `types.go` diff empty |
| C-002 | T002, T003, T006 | renderer prohibitions P1; T006 canary scan over all findings |
| C-003 | T003, T006 | advice sourced only from existing findings; T006 byte-equal pin |
| C-004 | scope reviews | scope reminders in WP file and quickstart (no Phase 3 work) |
| C-005 | scope reviews | scope reminders in WP file and quickstart (no Phase 4+ work) |
| C-006 | All | DoD: no Terraform apply, no production deploy |
| C-007 | T002 | renderer never reads `evidence` field; fingerprint contract P1 |

Every FR, NFR, and Constraint has at least one subtask owner and at least one verification artifact.

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
