---
work_package_id: WP05
title: Seed first-class detectors (Spec Kitty / GitHub Spec Kit / OpenSpec)
dependencies:
- WP03
- WP04
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-005
- FR-006
- FR-012
- FR-014
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T021
- T022
- T023
- T024
- T025
phase: Phase 3 — Seed detectors
agent: "claude:opus-4.7:reviewer-renata:reviewer"
shell_pid: "23568"
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/sdd/testdata/
execution_mode: code_change
owned_files:
- internal/analyzer/signatures/sdd_detectors_first_class.json
- internal/analyzer/sdd/testdata/fixtures/spec_kitty.txt
- internal/analyzer/sdd/testdata/fixtures/github_spec_kit.txt
- internal/analyzer/sdd/testdata/fixtures/openspec.txt
- internal/analyzer/sdd/testdata/fixtures/generic_only.txt
- internal/analyzer/sdd/evaluator_first_class_test.go
role: implementer
tags: []
---

# Work Package Prompt: WP05 — Seed first-class detectors

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.

## Objectives & Success Criteria

- `sdd_detectors.json` contains three `verified` entries: `spec_kitty`, `github_spec_kit`, `openspec`. Each carries source-class markers grounded in the WP04 research files and at least one `source_references` entry.
- Four fixtures under `testdata/fixtures/`: one per first-class tool plus `generic_only.txt`.
- The 3×3 cross-negative matrix in `evaluator_test.go` (≥9 assertions per NFR-004) passes.
- The generic-only fixture triggers zero detectors (FR-012).
- Spec Kitty / GitHub Spec Kit / OpenSpec are correctly distinguished — no cross-trigger.

## Context & Constraints

- Read research files for the three tools (produced by WP04):
  - `docs/research/sdd-fingerprints/spec-kitty.md`
  - `docs/research/sdd-fingerprints/github-spec-kit.md`
  - `docs/research/sdd-fingerprints/openspec.md`
- Read: `data-model.md` (registry schema), `contracts/sdd-detector.schema.json` (JSON validation).
- Read: `research.md` §R-07 (cross-negative design).
- FR-001, FR-002, FR-003, FR-005, FR-012, FR-014, NFR-004 are mapped to this WP.

## Subtasks & Detailed Guidance

### Subtask T021 — Seed `spec_kitty`

- **Steps**:
  1. Create `internal/analyzer/signatures/sdd_detectors_first_class.json` (new file, starting as `[]`). The WP01 loader globs `signatures/sdd_detectors*.json`, so this tier file is picked up automatically.
  2. Add the `spec_kitty` entry to this tier file (the registry loader globs `sdd_detectors*.json`; this WP's tier file is the authoritative home for first-class entries). Required fields:
     - `id`: `"spec_kitty"`.
     - `display_name`: `"Spec Kitty"`.
     - `aliases`: `["spec-kitty"]`.
     - `category`: `"sdd"`.
     - `competitor_priority`: `1`.
     - `status`: `"verified"`.
     - `source_references`: from `docs/research/sdd-fingerprints/spec-kitty.md`. At least the official repo URL.
     - `markers`: derived from research. At minimum:
       - `config_dir` matching `(?i)\.kittify(/|\\b)`.
       - `config_dir` matching `(?i)kitty-specs(/|\\b)`.
       - `cli_binary` with `binary: "spec-kitty"` and `version_args: ["--version"]`.
       - `slash_command` matching `(?i)\bspec-kitty\.(specify|plan|tasks|implement|review|merge)\b`.
     - `confidence_rules`:
       ```json
       [
         {"confidence": "high",   "requires_distinct_classes": 2},
         {"confidence": "medium", "requires_any_of": ["cli_binary", "slash_command"]},
         {"confidence": "low",    "requires_any_of": ["command_name"]}
       ]
       ```
  3. Validate JSON against the schema by running `go test ./internal/analyzer/sdd/...`. The loader will panic if a regex is malformed.

### Subtask T022 — Seed `github_spec_kit`

- **Steps**:
  1. Add the `github_spec_kit` entry to the same tier file (`sdd_detectors_first_class.json`). Required fields mirror T021 but use the GitHub Spec Kit research:
     - `id`: `"github_spec_kit"`. `display_name`: `"GitHub Spec Kit"`. `aliases`: `["spec-kit", "github/spec-kit"]`.
     - `competitor_priority`: `2`.
     - `markers` distinguished from `spec_kitty`:
       - tool-specific `config_dir` or `config_file` from research.
       - `cli_binary`: `"specify"` or `"spec-kit"` per research — confirm in research file.
       - Slash commands unique to GitHub Spec Kit.
     - **Negative marker** matching Spec Kitty-specific artifacts (e.g., `\.kittify/`, `kitty-specs/`) — i.e., presence of those vetoes the GitHub Spec Kit detector.

### Subtask T023 — Seed `openspec`

- **Steps**:
  1. Add the `openspec` entry to the same tier file (`sdd_detectors_first_class.json`). From research:
     - `id`: `"openspec"`. `display_name`: `"OpenSpec"`.
     - `competitor_priority`: `3`.
     - `markers`: tool-specific `config_dir` or `config_file` (e.g., `openspec.yaml`), `cli_binary`: `"openspec"`, slash commands per research.
     - **Negative markers** for `\.kittify/` and the GitHub Spec Kit-specific artifacts.

### Subtask T024 — Build fixtures

- **Files**:
  - `internal/analyzer/sdd/testdata/fixtures/spec_kitty.txt` — contains Spec Kitty-specific paths, slash commands, and the `spec-kitty` CLI name. Hand-crafted (~20 lines).
  - `internal/analyzer/sdd/testdata/fixtures/github_spec_kit.txt` — contains GitHub Spec Kit init artifacts (a `specify` CLI mention, the GitHub Spec Kit-specific slash commands).
  - `internal/analyzer/sdd/testdata/fixtures/openspec.txt` — contains OpenSpec config name (`openspec.yaml`), OpenSpec CLI name, etc.
  - `internal/analyzer/sdd/testdata/fixtures/generic_only.txt` — contains only `specs/`, `tasks.md`, `design.md`, `STATE.md`, `hooks`, `requirements.md`. NO tool-specific markers.
- **Notes**: keep fixtures small, sanitized, and verbatim from public sources (do not include any private path or username).

### Subtask T025 — Cross-negative matrix in tests

- **Steps**:
  1. Create `internal/analyzer/sdd/evaluator_first_class_test.go` (new file owned by this WP). Other tiers add their own `evaluator_<tier>_test.go` files later.
  2. Add:
     ```go
     func TestCrossNegativeFirstClass(t *testing.T) {
         cases := []struct {
             fixture string
             want    string // which detector ID should fire
         }{
             {"spec_kitty.txt", "spec_kitty"},
             {"github_spec_kit.txt", "github_spec_kit"},
             {"openspec.txt", "openspec"},
         }
         for _, c := range cases {
             c := c
             t.Run(c.fixture, func(t *testing.T) {
                 input := readFixture(t, c.fixture)
                 fps := evaluateWithRealRegistry(t, input)
                 // Assertion 1: expected detector fires.
                 assertOneOf(t, fps, c.want)
                 // Assertion 2 & 3: other two first-class detectors do NOT fire.
                 for _, other := range []string{"spec_kitty", "github_spec_kit", "openspec"} {
                     if other == c.want {
                         continue
                     }
                     assertNotPresent(t, fps, other)
                 }
             })
         }
     }
     func TestGenericOnlyTriggersNothing(t *testing.T) {
         input := readFixture(t, "generic_only.txt")
         fps := evaluateWithRealRegistry(t, input)
         if len(fps) != 0 {
             t.Fatalf("expected no fingerprints; got %v", fps)
         }
     }
     ```
  3. `evaluateWithRealRegistry` constructs a `FakeProbe` with `Installed = nil` (so CLI presence doesn't artificially fire) and calls `sdd.Evaluate(text, slashHits, fake, sdd.LoadRegistry())`.
  4. Confirm the test file produces ≥9 assertions across the matrix and ≥1 from the generic-only test (NFR-004 floor met).

- **Files**: `internal/analyzer/sdd/evaluator_first_class_test.go` (edit, add ~120 lines).

## Test Strategy

- `go test ./internal/analyzer/sdd/...` passes.
- Manual: open each fixture and confirm it does not contain forbidden private content (path with `/Users/`, etc.).

## Risks & Mitigations

- **Pattern over-match**: a Spec Kitty regex like `(?i)\bspec-kitty\b` could match free-text mentions in the GitHub Spec Kit fixture if someone writes "unlike Spec Kitty, GitHub Spec Kit ...". Mitigated by anchoring patterns on real artifacts (`.kittify/`, `kitty-specs/`) and by using `Negative` markers to suppress cross-trigger.
- **Loader rejects an entry**: run tests early after each entry to catch validation errors.

## Review Guidance

- Reviewer should grep `sdd_detectors.json` for every regex pattern and mentally apply it to each of the three fixtures. The 3×3 matrix should be obviously correct.
- Reviewer should verify each detector's `source_references` cites a public source.
- Reviewer should confirm `generic_only.txt` triggers zero detectors.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
- 2026-05-19T07:47:50Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=21245 – Started implementation via action command
- 2026-05-19T07:56:12Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=21245 – 3 first-class detectors + 4 fixtures + 3x3 cross-negative matrix + generic-only veto test
- 2026-05-19T07:56:54Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=23568 – Started review via action command
- 2026-05-19T07:58:09Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=23568 – Review passed: 3 verified detectors with mutual negative markers; 4 fixtures clean; NFR-004 cross-negative matrix (9+ assertions); FR-012 generic-only veto passes. TestGoldenSampleReport drift is expected and is WP10/T042's scope.
