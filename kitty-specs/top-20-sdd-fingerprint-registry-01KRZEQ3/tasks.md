# Tasks — Top-20 SDD Fingerprint Registry

**Mission**: `top-20-sdd-fingerprint-registry-01KRZEQ3`
**Plan**: [plan.md](plan.md)
**Spec**: [spec.md](spec.md)
**Branch contract**: planning on `main`; implementation commits later on `codex/sdd-fingerprint-registry`; final merge target `main`.

## Subtask index

| ID | Description | WP | Parallel |
| --- | --- | --- | --- |
| T001 | Create `internal/analyzer/sdd/detector.go` with typed registry record (`SDDDetector`, `SourceClass`, `Confidence`, `Status`, `Marker`, `ConfidenceRule`, `SourceRef`) | WP01 | | [D] |
| T002 | Create `internal/analyzer/sdd/registry.go` with embed loader and startup validation | WP01 | | [D] |
| T003 | Add `WorkflowFingerprints []EcosystemFingerprint` to `analyzer.Ecosystem` and define `EcosystemFingerprint` type | WP01 | | [D] |
| T004 | Add `detector_test.go` covering loader validation, enum rejection, regex compile failure handling | WP01 | | [D] |
| T005 | `gofmt` + `go test ./...` runs green with empty registry | WP01 | | [D] |
| T006 | Create `internal/analyzer/sdd/probe.go` with `CLIProbe` interface, `RealProbe`, `FakeProbe` | WP02 | | [D] |
| T007 | Create `internal/analyzer/sdd/version_bucket.go` with `normalizeVersionBucket` | WP02 | [D] |
| T008 | Add `probe_test.go` + `version_bucket_test.go` (FakeProbe + one safe RealProbe integration test) | WP02 | | [D] |
| T009 | Add `version_args` deny-list enforcement to registry loader | WP02 | [P] |
| T010 | Create `internal/analyzer/sdd/evaluator.go` with `Evaluate(text, lines, probe, registry) []EcosystemFingerprint` | WP03 | | [D] |
| T011 | Implement `ConfidenceRule` scoring and `Active` derivation | WP03 | | [D] |
| T012 | Implement `Marker.Negative` veto logic | WP03 | | [D] |
| T013 | Wire `internal/analyzer/ecosystem.go` to call `sdd.Evaluate` and assign `Ecosystem.WorkflowFingerprints` | WP03 | | [D] |
| T014 | Add `evaluator_test.go` with synthetic detectors verifying scoring, ordering, Active | WP03 | | [D] |
| T015 | Scaffold `docs/research/sdd-fingerprints/` + per-tool stubs + `README.md` | WP04 | | [D] |
| T016 | Research and fill: Spec Kitty, GitHub Spec Kit, OpenSpec | WP04 | [D] |
| T017 | Research and fill: Kiro, BMAD-METHOD, GSD | WP04 | [D] |
| T018 | Research and fill: Spec Workflow MCP, SDD Pilot, Spec-Driven Develop, spec2ship, ChatDev, PAUL | WP04 | [D] |
| T019 | Research and fill: fspec, whenwords, Intent, Cognition/Devin, Microsoft Agent Framework, Tessl, Agentic Code, CodeSpeak | WP04 | [D] |
| T020 | Add `docs/sdd-fingerprint-registry.md` cross-linking per-tool research files | WP04 | | [D] |
| T021 | Seed `spec_kitty` entry in `sdd_detectors.json` | WP05 | | [D] |
| T022 | Seed `github_spec_kit` entry | WP05 | | [D] |
| T023 | Seed `openspec` entry | WP05 | | [D] |
| T024 | Add `testdata/fixtures/{spec_kitty,github_spec_kit,openspec,generic_only}.txt` | WP05 | | [D] |
| T025 | Add 3×3 cross-negative + generic-only assertions to `evaluator_test.go` (NFR-004, FR-012) | WP05 | | [D] |
| T026 | Seed `kiro` entry + fixture | WP06 | [D] |
| T027 | Seed `bmad` entry + fixture | WP06 | [D] |
| T028 | Seed `gsd` entry + fixture | WP06 | [P] |
| T029 | Extend evaluator tests with positive cases for Kiro/BMAD/GSD; assert no cross-trigger with first-class | WP06 | | [D] |
| T030 | Seed 7 entries + fixtures: Spec Workflow MCP, SDD Pilot, Spec-Driven Develop, spec2ship, ChatDev, PAUL, fspec | WP07 | | [D] |
| T031 | Seed 7 entries + fixtures: whenwords, Intent, Cognition/Devin, Microsoft Agent Framework, Tessl, Agentic Code, CodeSpeak | WP07 | | [D] |
| T032 | Add positive-detection tests for all 14 long-tail tools | WP07 | | [D] |
| T033 | Cross-negative assertions: no long-tail tool triggers a first-class or second-ring detector | WP07 | | [D] |
| T034 | Add `internal/analyzer/sdd/leak_test.go` with 16-category canary fixture | WP08 | | [D] |
| T035 | Extend `internal/analyzer/analyzer_test.go` for end-to-end unknown-count + fingerprint privacy | WP08 | | [D] |
| T036 | Structural test: `Ecosystem.WorkflowFingerprints` sorted, deduplicated, bounded keys (NFR-003) | WP08 | | [D] |
| T037 | Write `docs/sdd-fingerprint-registry.md` (top-20 table, taxonomy, confidence, status, privacy rules) | WP09 | | [D] |
| T038 | Update `docs/ecosystem-signatures.md` with pointer to new doc | WP09 | [D] |
| T039 | Update `docs/data-retention-and-analytics.md` with fingerprint privacy notes | WP09 | [D] |
| T040 | Update `docs/logging-policy.md` with CLI probe rules | WP09 | [D] |
| T041 | Document the GitHub issue comments to post on #38 / #42 / #43 / #44–#48 / #49 / #50 / #66 / #67 | WP09 | | [D] |
| T042 | Update `internal/analyzer/golden_test.go` for `WorkflowFingerprints` shape | WP10 | | [D] |
| T043 | `gofmt -w` + `go test ./...` + `./scripts/smoke-local.sh` clean | WP10 | | [D] |
| T044 | Switch to `codex/sdd-fingerprint-registry` branch, commit consolidated changes, push | WP10 | | [D] |

## Work packages

### WP01 — Foundation: typed registry schema + loader

- **Goal**: Land the typed registry types (`SDDDetector`, `SourceClass`, `Confidence`, `Status`, `Marker`, `ConfidenceRule`, `SourceRef`), the embedded loader with startup validation, and the new `EcosystemFingerprint` type on `analyzer.Ecosystem` — all without any detector entries yet.
- **Priority**: P0 (blocking).
- **Independent test**: `go test ./internal/analyzer/sdd/...` passes; `go test ./...` passes with no behavior change to existing reports.
- **Included subtasks**: T001, T002, T003, T004, T005.
- **Estimated prompt size**: ~300 lines.
- **Implementation sketch**: New package `internal/analyzer/sdd`. New embed `signatures/sdd_detectors.json` (empty array initially). Add `WorkflowFingerprints []EcosystemFingerprint` to `analyzer.Ecosystem`. Define `EcosystemFingerprint` in `analyzer/types.go` (avoid import cycle with `sdd`). Update `registry.go` if needed to also surface SDD loader.
- **Parallel opportunities**: None inside the WP.
- **Dependencies**: none.
- **Risks**: import cycle if `EcosystemFingerprint` is defined in `sdd` package and `analyzer.Ecosystem` references it — mitigated by putting the report-shaped type in `analyzer/types.go`.
- **Prompt**: [tasks/WP01-foundation-typed-registry.md](tasks/WP01-foundation-typed-registry.md)

### WP02 — CLI probe abstraction + version-bucket normalizer

- **Goal**: Land the `CLIProbe` interface, `RealProbe` and `FakeProbe` implementations, and the version-bucket normalizer. Enforce the `version_args` deny-list at registry load time.
- **Priority**: P0.
- **Independent test**: `go test ./internal/analyzer/sdd/...` covers `FakeProbe`-driven calls, a single safe `RealProbe` integration test on the `go` binary, and the `normalizeVersionBucket` test cases (including leak-suppression on path-bearing inputs).
- **Included subtasks**: T006, T007, T008, T009.
- **Estimated prompt size**: ~320 lines.
- **Parallel opportunities**: T007 and T009 are independent of T006.
- **Dependencies**: WP01.
- **Risks**: `RealProbe` accidentally returns the resolved path — guarded by the contract and a dedicated leak assertion in `probe_test.go`.
- **Prompt**: [tasks/WP02-cli-probe-version-bucket.md](tasks/WP02-cli-probe-version-bucket.md)

### WP03 — Evaluator + ecosystem wiring

- **Goal**: Implement `sdd.Evaluate` with confidence rule scoring, `Active` derivation, and `Marker.Negative` veto. Wire `analyzer.DetectEcosystem` to populate `Ecosystem.WorkflowFingerprints`.
- **Priority**: P0.
- **Independent test**: `go test ./internal/analyzer/sdd/... ./internal/analyzer/...` passes. Synthetic-detector tests cover positive emission, multi-source confidence elevation, negative-veto suppression, and ordering by `(competitor_priority, ID)`.
- **Included subtasks**: T010, T011, T012, T013, T014.
- **Estimated prompt size**: ~360 lines.
- **Dependencies**: WP01, WP02.
- **Risks**: backwards-incompatible behavior in `DetectEcosystem` — golden tests in WP10 catch this.
- **Prompt**: [tasks/WP03-evaluator.md](tasks/WP03-evaluator.md)

### WP04 — Research gate: clone + verify top-20 SDD tools

- **Goal**: Produce per-tool research documents under `docs/research/sdd-fingerprints/` for all 20 SDD tools using public sources only. Each entry carries source-class markers with citations.
- **Priority**: P0 (research gate per brief; #66).
- **Independent test**: Every file under `docs/research/sdd-fingerprints/` passes a manual review against the template in `research.md` §R-01: source references present, at least one tool-specific marker, no guessed fingerprints.
- **Included subtasks**: T015, T016, T017, T018, T019, T020.
- **Estimated prompt size**: ~380 lines.
- **Parallel opportunities**: T016, T017, T018, T019 are independent research tasks. T020 depends on T015-T019.
- **Dependencies**: none.
- **Note**: research can run in parallel with the foundation work packages.
- **Risks**: a tool genuinely has no public fingerprintable surface; A-04 says: stop and have the scope conversation. Do not silently downgrade.
- **Prompt**: [tasks/WP04-research-gate.md](tasks/WP04-research-gate.md)

### WP05 — Seed first-class detectors (Spec Kitty / GitHub Spec Kit / OpenSpec)

- **Goal**: Add the three first-class detector entries to `sdd_detectors.json` using WP04 research. Add fixtures and the 3×3 cross-negative matrix + generic-name negative test.
- **Priority**: P0.
- **Independent test**: NFR-004 cross-negative tests pass (≥9 assertions). FR-012 generic-name negative test passes. Each fixture produces exactly one fingerprint for the matching tool at the expected confidence level.
- **Included subtasks**: T021, T022, T023, T024, T025.
- **Estimated prompt size**: ~340 lines.
- **Dependencies**: WP03, WP04.
- **Risks**: regex patterns over-match — mitigated by the cross-negative matrix and `generic_only.txt` fixture.
- **Prompt**: [tasks/WP05-first-class-detectors.md](tasks/WP05-first-class-detectors.md)

### WP06 — Seed second-ring detectors (Kiro / BMAD-METHOD / GSD)

- **Goal**: Add Kiro, BMAD, and GSD detector entries with verified markers (issue #47).
- **Priority**: P1.
- **Independent test**: Positive detection for each of the three; no cross-trigger with first-class detectors. Generic markers (`STATE.md`, `hooks`, `design.md`, `requirements.md`, `tasks.md`) still do not trigger anything.
- **Included subtasks**: T026, T027, T028, T029.
- **Estimated prompt size**: ~280 lines.
- **Parallel opportunities**: T026, T027, T028 are independent.
- **Dependencies**: WP05.
- **Risks**: BMAD's existing `frameworks.json` entry causes a duplicate-detection scenario in the legacy `WorkflowFrameworks` path — acceptable because C-004 preserves the legacy field unchanged.
- **Prompt**: [tasks/WP06-second-ring-detectors.md](tasks/WP06-second-ring-detectors.md)

### WP07 — Seed long-tail detectors (14 tools)

- **Goal**: Add the remaining 14 tool detector entries (issue #48) with verified markers from WP04.
- **Priority**: P1.
- **Independent test**: Positive detection for each tool; cross-negative against first-class and second-ring.
- **Included subtasks**: T030, T031, T032, T033.
- **Estimated prompt size**: ~360 lines.
- **Dependencies**: WP06.
- **Risks**: noise from generic terms ("Intent", "Agentic Code") — mitigated by tool-specific markers from the research files and by negative-test markers in registry entries.
- **Prompt**: [tasks/WP07-long-tail-detectors.md](tasks/WP07-long-tail-detectors.md)

### WP08 — Privacy guardrails: leak test + structural bounds

- **Goal**: Land the 16-category canary leak test and the NFR-003 structural test. Extend existing analyzer tests for end-to-end unknown-count + fingerprint privacy.
- **Priority**: P0.
- **Independent test**: `TestReportSerializationContainsNoForbiddenStrings` passes (NFR-001); structural test asserting `Ecosystem.WorkflowFingerprints` has no map-keyed-by-user-string field passes; existing analyzer privacy tests continue to pass.
- **Included subtasks**: T034, T035, T036.
- **Estimated prompt size**: ~280 lines.
- **Dependencies**: WP05 (needs first-class detectors so a fully populated report can be constructed).
- **Risks**: a future field addition to `EcosystemFingerprint` accidentally bypasses the canary — mitigated by the structural assertion that lists allowed field names.
- **Prompt**: [tasks/WP08-privacy-guardrails.md](tasks/WP08-privacy-guardrails.md)

### WP09 — Documentation + GitHub issue hygiene

- **Goal**: Land the top-20 documentation, update related docs, and prepare the GitHub issue comment text.
- **Priority**: P1.
- **Independent test**: `docs/sdd-fingerprint-registry.md` covers the top-20 table, source-class taxonomy, confidence levels, status semantics, CLI probe privacy rules, and the "what must never be uploaded" list. The three updated docs cross-link the new doc. T041 produces a single Markdown file in the feature dir listing the comment text for each GitHub issue.
- **Included subtasks**: T037, T038, T039, T040, T041.
- **Estimated prompt size**: ~320 lines.
- **Parallel opportunities**: T038, T039, T040 are independent file edits.
- **Dependencies**: WP04.
- **Note**: research must exist before docs can cross-link it; otherwise this WP runs in parallel with the code-lane work.
- **Risks**: drift between docs and code — mitigated by referring to file paths and contracts rather than restating invariants.
- **Prompt**: [tasks/WP09-documentation.md](tasks/WP09-documentation.md)

### WP10 — Final integration, smoke, branch hygiene

- **Goal**: Update golden tests, run the full test suite + smoke test, switch to the implementation branch `codex/sdd-fingerprint-registry`, commit, push.
- **Priority**: P0.
- **Independent test**: `go test ./...` passes; `./scripts/smoke-local.sh` passes (or the blocker is documented per A-06); the branch is pushed and the commit hash is reported.
- **Included subtasks**: T042, T043, T044.
- **Estimated prompt size**: ~220 lines.
- **Dependencies**: WP07, WP08.
- **Note**: documentation work can land in the same PR but does not block this WP.
- **Risks**: golden test mismatch from `omitempty` collection handling — mitigated by the existing `normalizeReportCollections` path.
- **Prompt**: [tasks/WP10-final-integration.md](tasks/WP10-final-integration.md)

## Dependency graph (linear form)

- WP02 depends on WP01.
- WP03 depends on WP01 and WP02.
- WP04 depends on nothing (runs in parallel with WP01–WP03).
- WP05 depends on WP03 and WP04.
- WP06 depends on WP05.
- WP07 depends on WP06.
- WP08 depends on WP05.
- WP09 depends on WP04 (planning-lane only; runs in parallel with WP05–WP08).
- WP10 depends on WP07 and WP08.

## MVP scope

If we had to ship the smallest viable slice that earns "the analyzer can distinguish SDD tools without leaking", it would be WP01 + WP02 + WP03 + WP04 + WP05 + WP08. That delivers:
- Typed registry, evaluator, probe.
- Three first-class detectors with cross-negative tests.
- Leak test enforcing NFR-001.

Per C-001 ("all 20 verified") the actual mission scope is WP01–WP10.

## Parallelization highlights

- WP04 in parallel with WP01-WP03.
- T016 / T017 / T018 / T019 within WP04 in parallel (per-tool research).
- T026 / T027 / T028 within WP06 in parallel.
- T038 / T039 / T040 within WP09 in parallel.
