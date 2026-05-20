# Implementation Plan: Top-20 SDD Fingerprint Registry

**Branch**: `main` (planning + spec); implementation work later on `codex/sdd-fingerprint-registry` per the brief
**Date**: 2026-05-19
**Spec**: [spec.md](spec.md)
**Mission**: `top-20-sdd-fingerprint-registry-01KRZEQ3` (`mission_id` `01KRZEQ372C78Z0KGN69MXRXZV`)
**Input**: Feature specification at `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/spec.md`

## Summary

Build a privacy-safe, deterministic registry for the top-20 spec-driven-development (SDD) tools. The work extends the existing `internal/analyzer/signatures/` registry with a new typed schema (`SDDDetector` / `EcosystemFingerprint`) that carries source classes, confidence levels, status, and source references. The analyzer gains an injectable CLI probe abstraction wrapping `exec.LookPath` plus an allowlisted version-command runner; the resolved path and raw version output never leave the process. The `Ecosystem` report type grows a new `WorkflowFingerprints` collection while keeping `WorkflowFrameworks []string` for backward compatibility. Three first-class detectors (Spec Kitty, GitHub Spec Kit, OpenSpec) get explicit cross-negative tests. Eighteen additional verified detectors round out the top-20 from public-source research. A serialization-leak test asserts that none of the 16 forbidden raw-string categories appear in any report or aggregate event.

## Technical Context

**Language/Version**: Go 1.25
**Primary Dependencies**: Go standard library only (`embed`, `encoding/json`, `regexp`, `os/exec`, `context`, `testing`). No new third-party dependencies.
**Storage**: Embedded JSON registry files under `internal/analyzer/signatures/` (existing pattern); no databases, no on-disk caches.
**Testing**: Go `testing` package; table-driven unit tests; golden-file tests for report shape (existing pattern in `golden_test.go`); fixture-based fixtures for tool-specific detection; injectable `CLIProbe` interface for hermetic CLI tests.
**Target Platform**: Cross-platform local CLI / library — macOS, Linux, Windows. CLI probes must work on all three; gracefully degrade on Windows where `exec.LookPath` semantics differ.
**Project Type**: Single Go module (`github.com/priivacy-ai/agent-log-analyzer`); analyzer is a library package consumed by the CLI and the web service.
**Performance Goals**: No measurable regression on existing `go test ./...` runtime (target: ≤ 10% increase). CLI version probes complete within 2 s wall-clock per binary (NFR-002). Detector evaluation over typical scrubbed transcript input adds < 50 ms total at p95.
**Constraints**:
- Zero forbidden raw strings in any aggregate output (NFR-001, 16 categories).
- Aggregate output cardinality bounded — no maps keyed by raw user strings (NFR-003).
- All 20 SDD tools must ship as `verified` from public sources before this mission merges (C-001). Tools with no public fingerprintable surface trigger a scope conversation with the user, not a silent downgrade.
- CLI probes target only allowlisted public binary names; no network, no auth, no project-data reads, no file modification, no shell expansion, no stdin.
**Scale/Scope**: 20 SDD tool detectors (3 named first-class + 17 long-tail). Each detector typically has 1–6 markers across source classes. Total registry size estimated at ~80–120 marker entries plus per-tool metadata.

## Charter Check

`spec-kitty charter context --action plan --json` reported `mode: missing` (no `.kittify/charter/charter.md` present). Charter Check is **skipped** for this mission. The brief's privacy stance functions as the de-facto governance policy and is enforced via NFR-001 (forbidden-string leakage test) and C-002…C-006.

## Project Structure

### Documentation (this feature)

```
kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/
├── plan.md              # This file
├── spec.md              # /spec-kitty.specify output (committed)
├── research.md          # Phase 0 output (this command)
├── data-model.md        # Phase 1 output (this command)
├── quickstart.md        # Phase 1 output (this command)
├── contracts/           # Phase 1 output (this command)
├── checklists/
│   └── requirements.md  # Spec quality checklist (committed)
└── tasks/               # Populated later by /spec-kitty.tasks
```

### Source Code (repository root)

```
internal/analyzer/
├── analyzer.go                  # existing: top-level Analyze()
├── ecosystem.go                 # existing: DetectEcosystem(); will call new SDD detector pass
├── registry.go                  # existing: embed.FS + loader; will gain SDD registry loader
├── types.go                     # existing: Ecosystem; will gain WorkflowFingerprints field
├── aggregate.go                 # existing: AggregateSafeEvent shaping
├── scrubber.go                  # existing: input scrubber (unchanged)
├── analyzer_test.go             # existing: extended with new assertions
├── golden_test.go               # existing: extended for WorkflowFingerprints
├── test_helpers_test.go         # existing
│
├── sdd/                         # NEW package — SDD fingerprint registry & evaluator
│   ├── detector.go              # SDDDetector, SourceClass, Confidence, Status types
│   ├── fingerprint.go           # EcosystemFingerprint emission + scoring
│   ├── evaluator.go             # Evaluate(text, lines, probe) → []EcosystemFingerprint
│   ├── probe.go                 # CLIProbe interface + RealProbe + FakeProbe
│   ├── version_bucket.go        # Normalize raw version output → bounded bucket
│   ├── registry.go              # Loader for embedded sdd_detectors.json
│   ├── detector_test.go         # Unit tests for source-class matching + confidence
│   ├── evaluator_test.go        # Cross-negative tests (Spec Kitty / GitHub Spec Kit / OpenSpec)
│   ├── probe_test.go            # FakeProbe-driven tests, no real exec
│   ├── version_bucket_test.go   # Normalization & leak suppression tests
│   ├── leak_test.go             # Serialization-leak test over fully populated report
│   └── testdata/
│       ├── fixtures/            # Per-tool detection fixtures (sanitized)
│       │   ├── spec_kitty.txt
│       │   ├── github_spec_kit.txt
│       │   ├── openspec.txt
│       │   ├── kiro.txt
│       │   ├── bmad.txt
│       │   ├── gsd.txt
│       │   └── ... (one per top-20 tool)
│       └── generic_only.txt     # Generic `specs/`, `tasks.md`, etc. — must trigger nothing
│
└── signatures/
    ├── frameworks.json          # existing — left in place for back-compat
    ├── mcp_servers.json         # existing
    ├── skills.json              # existing
    ├── plugins.json             # existing
    ├── coding_agents.json       # existing
    ├── package_managers.json    # existing
    └── sdd_detectors.json       # NEW — typed top-20 SDD registry

docs/
├── ecosystem-signatures.md            # existing — link to new doc
├── sdd-fingerprint-registry.md        # NEW — registry overview, source-class taxonomy, top-20 table, status semantics
├── research/
│   └── sdd-fingerprints/              # NEW — one .md per tool with public-source references
│       ├── spec-kitty.md
│       ├── github-spec-kit.md
│       ├── openspec.md
│       └── ... (one per top-20 tool)
├── data-retention-and-analytics.md    # existing — update with fingerprint privacy notes
└── logging-policy.md                  # existing — update with CLI probe rules

(research clone area — outside the repo, under the workspace:
 /Users/robert/code-analyzer-dev/claude-code-analyzer-20260519-082245-0QWuF7/research/<tool-id>/)
```

**Structure Decision**: Single Go module, single project. A new `internal/analyzer/sdd/` package owns the typed registry, the evaluator, and the CLI probe so the existing `analyzer` package keeps its current shape. `internal/analyzer/ecosystem.go` calls into `sdd.Evaluate(...)` once per analyze run and assigns the result to `Ecosystem.WorkflowFingerprints`. The embedded JSON registry stays under `internal/analyzer/signatures/sdd_detectors.json` so it ships in the binary; per-tool research docs live under `docs/research/sdd-fingerprints/`.

## Phase 0 — Research

See [research.md](research.md) for the full output. Phase 0 covers:

1. **Public-source fingerprints for the top-20 SDD tools** — official repository URL, docs URL, install/init flow, observed artifacts, allowlisted public CLI binary names, allowlisted slash commands, MCP server names, skill names, plugin manifests, package names, negative-test markers. The implementation cloning step happens in WP02 (research gate); `research.md` captures the methodology and the per-tool template that researchers fill in.
2. **CLI probe safety model** — what flags are safe per binary, what timeout to use, how to normalize version output, how to suppress raw output deterministically.
3. **Confidence-rule rationale** — why "config dir + tool-specific marker" earns `high` and why a textual mention alone earns `low`.
4. **Bulk-edit decision** — confirmed `change_mode: feature` (no occurrence_map.yaml needed); the work adds new code and new fingerprint data and does not rename any existing identifier across many files.
5. **Privacy threat model** — what NFR-001's leak test must cover; how serialization of `EcosystemFingerprint` is shaped to make leakage structurally impossible (typed fields only, enum-like source classes, allowlisted IDs).

## Phase 1 — Design

### Data model
See [data-model.md](data-model.md) for the typed schema. Highlights:

- `sdd.SDDDetector` — registry record (ID, display name, aliases, category, competitor priority, status, source references, marker entries per source class, confidence weights).
- `sdd.SourceClass` — enum of ten values per FR-006.
- `sdd.Confidence` — `high`, `medium`, `low` per FR-007.
- `sdd.Status` — `verified`, `research_needed`, `blocked` per FR-015.
- `analyzer.EcosystemFingerprint` — report-shaped value (ID, confidence, sources, evidence_count, optional `active`, `installed`, `version_bucket`).
- `analyzer.Ecosystem.WorkflowFingerprints []EcosystemFingerprint` — new field; existing `WorkflowFrameworks []string` retained per C-004.

### Contracts
See [contracts/](contracts/) for:

- `contracts/sdd-detector.schema.json` — JSON schema for entries in `sdd_detectors.json`.
- `contracts/ecosystem-fingerprint.schema.json` — JSON schema for the report-emitted record.
- `contracts/cli-probe.md` — Go interface contract for the injectable CLI probe.
- `contracts/forbidden-strings.md` — canonical list of the 16 forbidden raw-string categories used by the leak test.

### Quickstart
See [quickstart.md](quickstart.md) for the run-and-verify flow contributors follow.

### Re-evaluated Charter Check (post-design)
Skipped (no charter). Privacy invariants from C-002…C-006 are enforced via NFR-001 leak tests and structural test asserting bounded cardinality (NFR-003).

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| New `internal/analyzer/sdd/` package | The existing `signatureSpec` (id + patterns only) cannot express source classes, confidence, status, references, or CLI probes (FR-004 / FR-006 / FR-007). | Inlining the new types into `internal/analyzer` would bloat that package and entangle the CLI probe with the rest of the analyzer; tests would lose the package boundary that lets us inject `FakeProbe`. |
| Backward-compat `WorkflowFrameworks []string` kept alongside new `WorkflowFingerprints` | C-004 explicitly preserves the legacy field for the web UI / golden tests. | Dropping the legacy field would break existing report consumers and golden tests in this same commit; out of scope. |
| New embedded `sdd_detectors.json` alongside existing `frameworks.json` | The existing `frameworks.json` schema is too small to express the new fields and is consumed by code that wouldn't tolerate the richer shape. | Migrating `frameworks.json` to the new schema would force a coordinated change in the legacy detector path during the same commit; rejected as scope creep. The legacy frameworks file may be retired in a later mission. |
