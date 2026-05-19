---
work_package_id: WP03
title: Evaluator + ecosystem wiring
dependencies:
- WP01
- WP02
requirement_refs:
- FR-006
- FR-007
- FR-010
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T010
- T011
- T012
- T013
- T014
phase: Phase 2 — Engine
agent: "claude:opus-4.7:reviewer-renata:reviewer"
shell_pid: "18315"
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/sdd/
execution_mode: code_change
owned_files:
- internal/analyzer/sdd/evaluator.go
- internal/analyzer/sdd/evaluator_test.go
- internal/analyzer/ecosystem.go
role: implementer
tags: []
---

# Work Package Prompt: WP03 — Evaluator + ecosystem wiring

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

If no profile is specified, run `spec-kitty agent profile list` and select the best
match for this work package's `task_type` and `authoritative_surface`.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.

## Objectives & Success Criteria

- `sdd.Evaluate(text string, lines []parsedLine, probe CLIProbe, registry []SDDDetector) []EcosystemFingerprint` implemented.
- `Active` derivation: true iff `Confidence == "high"` AND at least one runtime-touch source matched (`slash_command`, `mcp_server_name`, or `cli_binary`).
- `Marker.Negative` veto: a matched negative marker suppresses the whole detector for that input.
- `analyzer.DetectEcosystem` calls `sdd.Evaluate` and stores the result on `Ecosystem.WorkflowFingerprints`.
- All existing tests pass; new evaluator tests pass.

## Context & Constraints

- Read `data-model.md` (Evaluator behavior).
- Read `research.md` §R-05 (confidence rule rationale), §R-10 (call site).
- FR-006 (source classes), FR-007 (confidence rules), FR-010 (emission), FR-012 (generic names alone do not trigger).
- C-005: low-confidence textual mentions MUST NOT count as installed in aggregate adoption metrics. This is a downstream consumer convention; the evaluator returns the fingerprint with `confidence: "low"` and `installed: false`. Downstream aggregation respects the convention.

## Subtasks & Detailed Guidance

### Subtask T010 — `sdd/evaluator.go`

- **Purpose**: Central evaluator that matches markers, computes confidence, and emits fingerprints.
- **Steps**:
  1. Create `internal/analyzer/sdd/evaluator.go`.
  2. Define:
     ```go
     // parsedLine is the existing type from internal/analyzer.
     // To avoid an import cycle, define a tiny interface or duplicate
     // the minimal shape here:
     type Line interface { TextLower() string; IsToolCall() bool }
     // (Or: change package layout to put parsedLine in a shared internal subpackage.)
     ```
     Pragmatic choice: pass `text string` plus an already-prepared `[]string` of free-text lines (computed by `ecosystem.go`), so `sdd` doesn't depend on `analyzer`'s private types. The evaluator then operates on `text` for textual markers and on the line slice for slash-command markers.
  3. Implement:
     ```go
     func Evaluate(text string, slashHits []string, probe CLIProbe, registry []SDDDetector) []EcosystemFingerprint
     ```
     where `slashHits` is the slice of slash-command tokens the analyzer already extracts. (Compute this in `ecosystem.go` before calling.)
  4. For each `SDDDetector` whose `Status == StatusVerified`:
     - Iterate markers; for each match, record the source class.
     - Check `Negative` markers first; if any negative pattern matches the input, skip this detector entirely.
     - For `cli_binary` markers: call `probe.LookPath(marker.Binary)`. If true, record `cli_binary` source and set `installed = true`.
     - For `cli_version_probe` markers: only run if the corresponding `cli_binary` for the same binary returned true. Then `probe.Version(ctx, name, args)` → normalize → set `version_bucket` if non-empty; record `cli_version_probe` source.
     - After all markers processed, if no markers matched, skip this detector.
     - Compute confidence by iterating `ConfidenceRules` and picking the highest-confidence rule that satisfies its `RequiresAnyOf` / `RequiresAllOf` / `RequiresDistinctClasses` constraint. Tier order: high > medium > low.
     - If no rule matched but markers did match, default to `low`.
     - Compute `Active`: true iff confidence is `high` AND the matched sources contain any of `{slash_command, mcp_server_name, cli_binary}`.
     - Build `EcosystemFingerprint{ID, Confidence, Sources, EvidenceCount, Active, Installed, VersionBucket}` and append.
  5. Sort the result slice by `(competitor_priority asc, ID asc)`. Dedup by ID (should already be unique).
  6. Return.
- **Files**: `internal/analyzer/sdd/evaluator.go` (new, ~250 lines).

### Subtask T011 — Confidence scoring + Active derivation

- Covered inside T010. Add a private helper `scoreConfidence(matched map[SourceClass]int, rules []ConfidenceRule) Confidence`. Unit-test it in T014.

### Subtask T012 — Negative-marker veto

- Covered inside T010. The veto runs **before** confidence scoring. Negative markers do not contribute to evidence counts.

### Subtask T013 — Wire `analyzer.DetectEcosystem`

- **Purpose**: Add the single call site.
- **Steps**:
  1. Open `internal/analyzer/ecosystem.go`.
  2. Inside `DetectEcosystem`, after constructing the existing `eco`:
     ```go
     slashHits := extractSlashTokens(lines)
     probe := sdd.NewRealProbe()
     ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // bound for all probes in one call
     defer cancel()
     eco.WorkflowFingerprints = sdd.Evaluate(text, slashHits, sdd.WithContext(ctx, probe), loadSDDRegistry())
     ```
  3. `loadSDDRegistry()` is the sync.Once-guarded loader from WP01.
  4. `extractSlashTokens(lines)` is a small helper that walks `lines` and returns lower-cased slash tokens (mirror the logic in `countUnknownSlashCommands`).
  5. `sdd.WithContext` is a convenience that decorates an existing `CLIProbe` to carry a deadline. Alternative: pass the ctx as a third arg directly into `Evaluate` and have `Evaluate` thread it to `probe.Version`.
- **Files**: `internal/analyzer/ecosystem.go` (edit).
- **Notes**: this WP touches a file outside `internal/analyzer/sdd/`. Document explicitly in the activity log.

### Subtask T014 — Evaluator tests

- **Purpose**: Verify scoring, ordering, Active, and veto logic with synthetic detectors (no real registry yet).
- **Steps**:
  1. Create `internal/analyzer/sdd/evaluator_test.go`.
  2. Construct a tiny in-memory `[]SDDDetector` covering:
     - Tool A: two markers — `config_dir` matching `\.tool-a/` and `cli_binary` matching `tool-a`. High-confidence rule: `requires_distinct_classes: 2`. Active should be true (has `cli_binary`).
     - Tool B: one marker — `command_name` matching `\btoolb\b`. Low-confidence rule.
     - Tool C: same `config_dir` as Tool A but a negative marker `\bnot-tool-c\b`. Should be vetoed when the negative pattern matches the input.
  3. Cases:
     - Input contains `.tool-a/` and `FakeProbe.Installed[tool-a] = true` → Tool A fingerprint, `confidence: high`, `installed: true`, `active: true`, `sources: [cli_binary, config_dir]`, `evidence_count: 2`.
     - Input contains free-text mention `toolb` → Tool B fingerprint, `confidence: low`, no `installed`, no `active`.
     - Input contains `.tool-a/` AND `not-tool-c` → Tool A still detected (negative is on Tool C); Tool C vetoed.
     - Input contains neither → empty slice.
     - Ordering test: two tools, competitor priority 2 and 1, both match → output sorted by priority ascending.
- **Files**: `internal/analyzer/sdd/evaluator_test.go` (new, ~250 lines).

## Test Strategy

- All tests use `FakeProbe`.
- No fixture files yet (those land in WP05+).
- Run `go test ./internal/analyzer/...` and confirm no existing test fails.

## Risks & Mitigations

- **Import cycle** if evaluator imports `analyzer.parsedLine`: resolved by passing `slashHits []string` from `ecosystem.go` instead.
- **CLI probe timing out** during analyzer runs in CI: the 5-second overall budget in `DetectEcosystem` bounds total time; individual probes still have their 2-second NFR-002 budget when WP05+ tests run.
- **Order non-determinism** if multiple detectors share `competitor_priority`: tie-break by ID asc — implement and test this.

## Review Guidance

- Confirm `Evaluate` is pure given `(text, slashHits, probe, registry)` — no globals, no I/O outside `probe`.
- Confirm negative markers fire before confidence scoring.
- Confirm `Active` cannot be true without high confidence.
- Confirm sort order is stable and deterministic.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
- 2026-05-19T07:38:30Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=11958 – Started implementation via action command
- 2026-05-19T07:45:17Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=11958 – Evaluator + confidence scoring + negative veto + ecosystem wiring; WorkflowFingerprints empty until WP05 lands detector entries. Note: registry.go's analyzer import had to be replaced with an injectable ChunksProvider to break the analyzer<->sdd import cycle introduced by this WP's wiring; ecosystem.go init() supplies the provider.
- 2026-05-19T07:45:43Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=18315 – Started review via action command
- 2026-05-19T07:47:24Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=18315 – Review passed: evaluator algorithm correct (negative veto first, tiered confidence high>medium>low with low default, cli_version_probe gated on same-binary cli_binary, stable sort by (competitor_priority asc, ID asc)); ecosystem wiring is a single call site in DetectEcosystem with 5s ctx + RealProbe + extractSlashTokens helper; 7 tests cover all required paths and pass; ChunksProvider scope deviation acceptable (minimal surgical change to break analyzer<->sdd import cycle, nil-provider preserves WP01 empty-registry behavior); go vet clean, all packages green.
