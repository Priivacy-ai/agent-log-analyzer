---
work_package_id: WP06
title: Seed second-ring detectors (Kiro / BMAD-METHOD / GSD)
dependencies:
- WP05
requirement_refs:
- FR-005
- FR-006
- FR-012
- FR-014
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T026
- T027
- T028
- T029
phase: Phase 3 — Seed detectors
agent: "claude:opus-4.7:implementer-ivan:implementer"
shell_pid: "23806"
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/sdd/testdata/
execution_mode: code_change
owned_files:
- internal/analyzer/signatures/sdd_detectors_second_ring.json
- internal/analyzer/sdd/testdata/fixtures/kiro.txt
- internal/analyzer/sdd/testdata/fixtures/bmad.txt
- internal/analyzer/sdd/testdata/fixtures/gsd.txt
- internal/analyzer/sdd/evaluator_second_ring_test.go
role: implementer
tags: []
---

# Work Package Prompt: WP06 — Seed second-ring detectors

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.

## Objectives & Success Criteria

- `sdd_detectors_second_ring.json` gains three `verified` entries: `kiro`, `bmad`, `gsd`.
- Three new fixtures under `testdata/fixtures/`.
- Positive-detection tests for each of the three; cross-negative against the first-class trio (Spec Kitty / GitHub Spec Kit / OpenSpec).
- Generic markers (FR-012 list) still trigger nothing.

## Context & Constraints

- Read research files: `docs/research/sdd-fingerprints/{kiro,bmad,gsd}.md`.
- Read: `sdd_detectors_second_ring.json` entries from WP05 for shape.
- Issue: #47 (Detector group: Kiro, BMAD, and GSD).
- BMAD ID `bmad` already exists in legacy `frameworks.json`. Reuse the same ID in `sdd_detectors_second_ring.json`. The legacy entry stays per C-004.

## Subtasks & Detailed Guidance

### Subtask T026 — Seed `kiro`

- **Steps**:
  1. Create `internal/analyzer/signatures/sdd_detectors_second_ring.json` (new tier file owned by this WP, starts as `[]`). The loader globs `sdd_detectors*.json` and picks it up automatically.
  2. Add a `kiro` detector entry to that tier file using markers from research.
  2. Kiro's source-class signal includes config dir, slash commands, possibly an MCP server name. Note that `hooks` alone is in the FR-012 negative-name list — never use `hooks` as a Kiro-specific marker without an additional tool-specific qualifier.
  3. Add `internal/analyzer/sdd/testdata/fixtures/kiro.txt` (~10 lines, sanitized).

### Subtask T027 — Seed `bmad`

- **Steps**:
  1. Add a `bmad` detector entry. ID `"bmad"`, display `"BMAD-METHOD"`, aliases `["bmad-method"]`.
  2. Markers: tool-specific config_dir / config_file from research, CLI binary name, slash commands.
  3. `competitor_priority`: `5`.
  4. Add `internal/analyzer/sdd/testdata/fixtures/bmad.txt`.

### Subtask T028 — Seed `gsd`

- **Steps**:
  1. Add a `gsd` detector entry. The brief flags `STATE.md` as a false-positive marker, so any GSD pattern must require an additional tool-specific qualifier.
  2. `competitor_priority`: `6`.
  3. Add `internal/analyzer/sdd/testdata/fixtures/gsd.txt`.

### Subtask T029 — Extend evaluator tests

- **Steps**:
  1. In `evaluator_second_ring_test.go`, extend the existing fixture-driven harness from WP05 to add positive cases:
     ```go
     {"kiro.txt", "kiro"},
     {"bmad.txt", "bmad"},
     {"gsd.txt", "gsd"},
     ```
  2. Also add cross-negative assertions: each of these fixtures does NOT trigger `spec_kitty`, `github_spec_kit`, or `openspec`.
  3. Add a regression assertion: `generic_only.txt` still triggers nothing (re-run from WP05).

## Test Strategy

- `go test ./internal/analyzer/sdd/...` passes.
- Manual: read each new fixture line by line and confirm no private content slipped in.

## Risks & Mitigations

- **`STATE.md` over-trigger** (FR-012): never use bare `STATE.md` as a GSD marker. The detector entry's primary marker must be a tool-specific identifier.
- **BMAD ID collision** with legacy `frameworks.json`: both entries can coexist because they live in different registries and feed different output fields (`WorkflowFrameworks` vs `WorkflowFingerprints`). Reviewer should verify nothing tries to deduplicate them.

## Review Guidance

- Reviewer should mentally apply each of these three entries' regex patterns against the four first-class-WP fixtures and confirm zero false positives.
- Reviewer should confirm the cross-trigger assertions in the evaluator test pass.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
- 2026-05-19T07:58:45Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=23806 – Started implementation via action command
- 2026-05-19T08:04:19Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=23806 – Kiro + BMAD detectors shipped (verified, second-ring tier). GSD deferred per FR-013: WP04 research marked it research_needed (no canonical upstream repo identified). T028 intentionally left in planned — scope decision (C-001 / A-04) awaiting mission-review. Tests pass: go test ./internal/analyzer/sdd/... green; cross-negative matrix holds across first-class + second-ring registry; generic_only.txt still triggers zero fingerprints. (Note: TestGoldenSampleReport in internal/analyzer was already failing on the WP05 base commit a25a5b2 — pre-existing, unrelated to WP06; sample-report.json is not WP06-owned.)
