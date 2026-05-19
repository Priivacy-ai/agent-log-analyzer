---
work_package_id: WP01
title: CLI positional log path handling
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-010
- NFR-001
- NFR-004
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-launch-correctness-01KRZZVK
base_commit: 40cd62a890ea7a542711898032b685237e955e6f
created_at: '2026-05-19T12:21:33.774696+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
phase: Phase 1 — Launch Correctness
assignee: ''
agent: claude
shell_pid: '46226'
history:
- at: '2026-05-19T11:55:54Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: cmd/claude-analyzer/
execution_mode: code_change
model: ''
owned_files:
- cmd/claude-analyzer/**
- README.md
- docs/testing-plan.md
- web/app.js
role: implementer
tags: []
---

# Work Package Prompt: WP01 – CLI positional log path handling

## ⚡ Do This First: Load Agent Profile

Before reading anything else in this prompt, load the assigned agent profile:

```
/ad-hoc-profile-load implementer-ivan
```

Then return to this file and read top-to-bottom.

## Branch Strategy

- **Planning/base branch at prompt creation**: `main`
- **Final merge target for completed work**: `main`
- **Shared feature branch for all three WPs of this mission**: `codex/launch-correctness` (add timestamp suffix if the name is taken on the remote).
- **Actual execution workspace**: `/spec-kitty.implement` selects the lane worktree and records the lane branch. Trust the printed lane workspace; do not guess a path or branch.
- **If human instructions contradict these fields**: stop and resolve the intended landing branch before coding.

## Objectives & Success Criteria

This WP delivers issue **#74** in full. After this WP:

1. `claude-analyzer analyze <log-path>` analyzes the file at `<log-path>` (FR-001).
2. `claude-analyzer analyze <log-path> --log <other>` refuses with a clear error and exits non-zero (FR-002).
3. `claude-analyzer analyze <path-a> <path-b>` refuses with a clear error and exits non-zero (FR-003).
4. `--help`, `README.md`, `docs/testing-plan.md`, and `web/app.js` all describe the positional form alongside `--log` (FR-004).
5. A new `cmd/claude-analyzer/main_test.go` covers all six cases enumerated in `contracts/cli-analyze.md`.
6. The charter verification baseline still passes after the change (NFR-001), and the CLI does not measurably slow down (NFR-004).

The implementer must also post a "starting work" comment on issue #74 and a "ready for review" comment on issue #74 when the PR opens — this WP carries one-third of FR-010.

**Independent test:**

```bash
go test ./cmd/claude-analyzer/...
go vet ./cmd/claude-analyzer/...
```

## Context & Constraints

- Spec: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/spec.md` — see FR-001..FR-004 and FR-010.
- Plan: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/plan.md`.
- Contract: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/contracts/cli-analyze.md`. Read this file in full — it contains the argument resolution table, the error-message substring contract, and the test list.
- Charter: `.kittify/charter/charter.md`. Privacy stance and verification baseline are non-negotiable; strict exception policy.
- Existing parser: stdlib `flag` package via `flag.NewFlagSet`. Do NOT migrate to Cobra (rejected in `research.md` — out of scope).

**Constraints carried from spec/charter:**
- C-001 (bounded-cardinality schema): not directly relevant to CLI, but no new serialized fields are added.
- C-002 (privacy stance): error messages echo only argument names and the configured paths the user explicitly supplied — never raw log content or filesystem listings.
- C-003 (single branch): all WPs of this mission land on `codex/launch-correctness`.
- C-005 (no `terraform apply`): not relevant here.

## Subtasks & Detailed Guidance

### Subtask T001 — Implement positional argument resolution in `runAnalyze`

- **Purpose**: Make the analyzer obey FR-001 / FR-002 / FR-003 from the CLI argument resolution table.
- **Files**: `cmd/claude-analyzer/main.go` (specifically `runAnalyze`, around lines 36..86).
- **Steps**:
  1. After `fs.Parse(args)`, read `positional := fs.Args()` and the `--log` value already captured in the existing local.
  2. Implement the truth table in `contracts/cli-analyze.md`:
     - `len(positional) == 0` and `--log` empty → call `latestClaudeLog()` (unchanged).
     - `len(positional) == 0` and `--log` set → use `--log` (unchanged).
     - `len(positional) == 1` and `--log` empty → use `positional[0]`. (NEW — FR-001.)
     - `len(positional) == 1` and `--log` set → return an error containing the substring `cannot combine positional log path with --log`. Exit code non-zero. (NEW — FR-002.)
     - `len(positional) >= 2` → return an error containing the substring `unexpected extra argument` (and include the name of the second positional). Exit code non-zero. (NEW — FR-003.)
  3. Make the error return path use the existing failure return shape (so the calling `run()` / `main()` exits non-zero).
- **Parallel?**: No — T002/T003/T004/T005 follow.
- **Notes**:
  - Do not change `latestClaudeLog()` (`main.go:143..175`).
  - The two error-message substrings above are part of the test contract — keep them exactly. The surrounding wording can be edited freely.

### Subtask T002 — Update `usage()` text

- **Purpose**: Make `--help` advertise the positional form (FR-004).
- **Files**: `cmd/claude-analyzer/main.go` (`usage()` around `main.go:183`).
- **Steps**: Replace the existing usage block with:
  ```
  usage: claude-analyzer analyze [<log-path>] [--log <path>] [--out <path>] ...

    <log-path>     path to a Claude Code JSONL log; mutually exclusive with --log.
                   if neither is supplied, the latest log in ~/.claude/projects/
                   is used.
    --log <path>   explicit log path; mutually exclusive with a positional <log-path>.
    --out <path>   output path for the sanitized report JSON (default: ./claude-analyzer-report.json).
  ```
  Keep the rest of the usage text (other flags) as-is.
- **Parallel?**: [P] after T001 lands.

### Subtask T003 — Create `cmd/claude-analyzer/main_test.go`

- **Purpose**: Cover the six FR-001..FR-003 cases enumerated in `contracts/cli-analyze.md`.
- **Files**: `cmd/claude-analyzer/main_test.go` (NEW).
- **Steps**:
  1. Use `testing.T` and invoke `runAnalyze` directly (capture its return error). Do not shell out.
  2. Use a small in-repo log fixture for positive cases — `internal/analyzer/testdata/sample.jsonl` if present, else a tiny inline JSONL written to `t.TempDir()`.
  3. Implement the six cases listed in `contracts/cli-analyze.md`:
     - `TestAnalyze_NoArgs_UsesLatest` (sanity; shim `latestClaudeLog` if necessary).
     - `TestAnalyze_PositionalOnly_UsesPositional`.
     - `TestAnalyze_LogFlagOnly_UsesLogFlag`.
     - `TestAnalyze_PositionalPlusLog_Refuses` — assert error message contains `cannot combine positional log path with --log` and assert no report written.
     - `TestAnalyze_TwoPositionals_Refuses` — assert error message contains `unexpected extra argument` and assert no report written.
     - `TestAnalyze_PositionalNonExistent_Refuses` — assert non-zero return; do NOT assert message text.
- **Parallel?**: [P] after T001 lands.
- **Notes**:
  - Each test uses `t.TempDir()` for any output file paths so cleanup is automatic.
  - For shimming `latestClaudeLog`, the simplest approach is to use a package-level function variable (`var latestClaudeLog = defaultLatestClaudeLog`) and replace it in tests. Confirm this refactor is the minimum scope — if it grows, push the sanity case (`TestAnalyze_NoArgs_UsesLatest`) out of this WP and document the deferral in the PR.

### Subtask T004 — Update `README.md` and `docs/testing-plan.md`

- **Purpose**: Public docs reflect FR-004.
- **Files**: `README.md`, `docs/testing-plan.md`.
- **Steps**:
  1. In `README.md`, where the existing `claude-analyzer analyze` example appears (under the Local Runthrough block), add a short note that both forms are supported:
     ```
     # equivalent to the form using --log:
     claude-analyzer analyze ~/.claude/projects/some-session.jsonl --out ./report.json
     ```
  2. In `docs/testing-plan.md`, add a one-line entry in the CLI section noting that `main_test.go` exists and exercises positional / `--log` exclusivity.
- **Parallel?**: [P] after T001 lands.

### Subtask T005 — Update `web/app.js` command generator

- **Purpose**: Local-command generator on the web UI presents both forms (FR-004).
- **Files**: `web/app.js`.
- **Steps**:
  1. Grep `web/app.js` for `claude-analyzer analyze` template strings.
  2. If the current generator emits only `--log` form, update the snippet to emit the positional form (the more concise form) and leave a one-line comment that `--log` also works.
  3. If the generator already emits the positional form, no change needed — note that in the PR description.
- **Parallel?**: [P] after T001 lands.
- **Notes**: This is the only WP02-level web touch in the mission. Do not introduce any other web changes.

## Test Strategy

Tests are required for FR-001..FR-003. Test work is bundled in T003.

Run:

```bash
go test ./cmd/claude-analyzer/...
go vet ./cmd/claude-analyzer/...
gofmt -d cmd/claude-analyzer/
```

Plus the full charter verification baseline once at the end:

```bash
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
terraform -chdir=infra/aws fmt -check -recursive
./scripts/smoke-local.sh
```

## Risks & Mitigations

- **Risk**: Refactoring `latestClaudeLog` for shimming bleeds beyond this WP.
  - **Mitigation**: If the shim needs more than a one-line `var latestClaudeLog = ...` indirection, drop the `TestAnalyze_NoArgs_UsesLatest` sanity case from T003 and call it out in the PR. The other five FR-backed tests are non-negotiable.
- **Risk**: Error message wording drift between code and tests.
  - **Mitigation**: Tests assert substrings only (`cannot combine positional log path with --log`, `unexpected extra argument`). Keep those substrings exactly.
- **Risk**: `web/app.js` mutation accidentally breaks the static report viewer.
  - **Mitigation**: T005 is doc-string only; no logic changes. Run `./scripts/smoke-local.sh` to confirm.
- **Risk**: Existing scripted users break.
  - **Mitigation**: All existing behaviors are preserved (no-arg uses latest; `--log` still works alone). Only new behaviors are added.

## Review Guidance

Reviewer checkpoints for `/spec-kitty.review`:

1. **Resolution table compliance**: every row in `contracts/cli-analyze.md` truth table is exercised by exactly one test in T003.
2. **Error message substrings**: code error strings contain the contract substrings; tests assert them.
3. **No behavior regression**: existing `--log` flow and no-args flow still work; existing scripted users see no change.
4. **Privacy stance**: error messages do not echo log contents.
5. **Charter verification baseline passes**: the implementer ran all five baseline commands locally.
6. **Issue #74 comments**: implementer commented at start and ready-for-review.

## Activity Log

> **CRITICAL**: Activity log entries MUST be in chronological order (oldest first, newest last).

### How to Add Activity Log Entries

When adding an entry: append at the END (do NOT prepend). Use `- YYYY-MM-DDTHH:MM:SSZ -- agent_id -- <action>` format with UTC.

Initial entry:

- 2026-05-19T11:55:54Z -- system -- Prompt created.

---

### Status Management

Status is managed via `status.events.jsonl`, not in WP frontmatter.

```bash
spec-kitty agent tasks move-task WP01 --to <status> --note "message"
```

### File Structure

All WP files live in a flat `tasks/` directory.
