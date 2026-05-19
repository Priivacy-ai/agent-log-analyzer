---
work_package_id: WP02
title: MCP exposure-header call mask
dependencies: []
requirement_refs:
- FR-005
- FR-006
- FR-010
- NFR-001
- NFR-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts were generated on main; the implementer lane worktree branches off main and the completed PR merges back into main. The shared feature branch name for all three WPs of this mission is codex/launch-correctness (timestamp suffix if taken).
subtasks:
- T006
- T007
- T008
- T009
- T010
- T011
phase: Phase 1 — Launch Correctness
assignee: ''
agent: claude
history:
- at: '2026-05-19T11:55:54Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/tooling_detect
execution_mode: code_change
model: ''
owned_files:
- internal/analyzer/tooling_detect.go
- internal/analyzer/tooling_detect_test.go
- internal/analyzer/testdata/tooling/**
role: implementer
tags: []
---

# Work Package Prompt: WP02 – MCP exposure-header call mask

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
- **Actual execution workspace**: `/spec-kitty.implement` selects the lane worktree and records the lane branch. Trust the printed lane workspace; do not guess.
- **If human instructions contradict these fields**: stop and resolve before coding.

## Objectives & Success Criteria

This WP delivers issue **#70** in full. After this WP:

1. Tokens of the shape `mcp__<server>__<tool>` (or analogous skill identifiers) appearing inside an MCP/skill exposure-header byte range contribute zero to `MCPUtilization.CallCount`, `MCPUtilization.KnownCallCount`, `SkillUtilization.ExecutedCount`, and `SkillUtilization.KnownExecutedIDs` (FR-005).
2. A new fixture `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log` proves the mask works: many header-only `mcp__server__tool` tokens, zero actual tool-use records, expected `CallCount == 0` (FR-006).
3. After the fix, the MCP exposure-header call false-positive rate across the full bundled fixture set is exactly 0 (NFR-003).
4. Fixtures `00..06` produce byte-identical `ToolingUtilization` before and after the change (C-006 no-op stability). Fixture `07` may shift; that shift is the fix being applied, not a regression.
5. The charter verification baseline still passes after the change (NFR-001).

The implementer must also post a "starting work" comment on issue #70 and a "ready for review" comment on issue #70 when the PR opens — this WP carries one-third of FR-010.

**Independent test:**

```bash
go test ./internal/analyzer/ -run TestDetectMCPExposure -v
go test ./internal/analyzer/ -run TestGolden -v
go vet ./internal/analyzer/...
```

## Context & Constraints

- Spec: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/spec.md` — FR-005, FR-006, NFR-003, C-006, FR-010.
- Plan: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/plan.md`.
- Data model: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/data-model.md` — read the `byteRange` and `HeaderRanges` sections in full.
- Contract: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/contracts/mcp-call-counting.md`. Read this in full — it contains the operational mask definition, the C-006 no-op rule, and the new fixture composition.
- Research: `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/claude-log-analyzer/kitty-specs/launch-correctness-01KRZZVK/research.md` — Bug #70 section explains why mask-then-skip beats alternative approaches.
- Existing code: `internal/analyzer/tooling_detect.go`. The bug site is `detectMCPCallsFromToolUse` (lines 212..271), specifically the raw-byte rescan around line 242. The header detector at line 114 already identifies header blocks; this WP teaches it to retain their byte ranges.

**Constraints carried from spec/charter:**
- C-001 (bounded-cardinality schema): `HeaderRanges` is **in-memory only**, never serialized. The privacy canary in WP03's T015 will explicitly verify this once aggregate coverage exists.
- C-002 (privacy stance): no new field exposes private names, raw bytes, or stable hashes.
- C-006 (no-op stability): fixtures `00..06` MUST be byte-identical before and after this WP.

## Subtasks & Detailed Guidance

### Subtask T006 — Add `byteRange` type and `HeaderRanges` field

- **Purpose**: Make header byte ranges available to the call detector.
- **Files**: `internal/analyzer/tooling_detect.go`.
- **Steps**:
  1. Add an unexported `byteRange` type at the top of the file (or just above the struct definitions):
     ```go
     // byteRange is a [Start, End) byte offset pair inside a parsed log buffer.
     // In-memory only — never written to disk, JSON, logs, or telemetry.
     type byteRange struct {
         Start int
         End   int
     }
     ```
  2. Add a `HeaderRanges []byteRange` field to `mcpExposure` (struct around lines 19..26) and to `skillExposure` (struct around lines 28..35).
  3. Confirm by inspection that neither struct has any `json:` tags on these fields and neither is exposed via a serialized type. (The structs are package-private and not part of any external API.)
- **Parallel?**: No — T007 needs T006.

### Subtask T007 — Populate `HeaderRanges`

- **Purpose**: Make the header detector record block boundaries.
- **Files**: `internal/analyzer/tooling_detect.go`.
- **Steps**:
  1. In `detectMCPExposureFromHeaders` (around `tooling_detect.go:114`), at the point where the matched header block is iterated, capture the block's `[Start, End)` byte offsets in the input buffer and append to `mcpExposure.HeaderRanges`.
  2. Do the same in the skill exposure header detector for `skillExposure.HeaderRanges`. The skill side has no current bug, but the symmetric structural addition is defensive (per `research.md` premortem).
  3. Do NOT change any other observable behavior of these detectors. `ExposedToolCount`, `KnownIDs`, `UnknownCount`, and `SchemaTextBytes` semantics stay identical.
- **Parallel?**: No — T008 needs T007.
- **Notes**: If the matched block is identified via regex submatch indices, use those (`FindSubmatchIndex` style) to compute precise byte offsets. If the block is built differently, capture the offsets at the point of construction.

### Subtask T008 — Mask helper and call-detector integration

- **Purpose**: Make `detectMCPCallsFromToolUse` skip raw-byte matches inside header ranges (FR-005).
- **Files**: `internal/analyzer/tooling_detect.go`.
- **Steps**:
  1. Add a small unexported helper:
     ```go
     // insideAny reports whether off lies inside any range in ranges.
     // O(len(ranges)). Header range count is bounded and small (typically 0..3).
     func insideAny(off int, ranges []byteRange) bool {
         for _, r := range ranges {
             if off >= r.Start && off < r.End {
                 return true
             }
         }
         return false
     }
     ```
  2. Inside `detectMCPCallsFromToolUse` (around `tooling_detect.go:212..271`), build the combined ranges slice once per call:
     ```go
     combined := append([]byteRange{}, mcpEx.HeaderRanges...)
     combined = append(combined, skillEx.HeaderRanges...)
     ```
  3. At the raw-byte rescan (around line 242, where `mcpCallPairRe.FindAllSubmatch(rawBytes, -1)` lives), switch to `FindAllSubmatchIndex` to get offsets. For each match, call `insideAny(matchStart, combined)`; if true, skip the match entirely. Otherwise, count it as today.
  4. The parsed-line scan (lines 250..263) is unchanged.
- **Parallel?**: No — T010 covers tests.
- **Notes**:
  - When `len(combined) == 0`, the predicate trivially returns false for every offset, and the rescan behavior is identical to today. This is the load-bearing C-006 no-op guarantee — verify by inspection.
  - Do not allocate `combined` inside a loop; build it once per call.

### Subtask T009 — Create fixture `08-header-only-zero-calls.log`

- **Purpose**: Provide the load-bearing assertion for FR-006 and NFR-003.
- **Files**: `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log` (NEW).
- **Steps**:
  1. Mirror the JSONL shape used in `01-healthy-small.log` for the system message that carries the MCP exposure header — one system message with a header block enumerating at least 5 `mcp__server__tool` identifiers.
  2. At least 2 identifiers should be on the public allowlist (so they would otherwise be counted as known calls). At least 1 should be obviously synthetic (e.g. `mcp__synthetic-fixture__placeholder`). DO NOT use names that resemble real private MCPs.
  3. The rest of the log contains zero `tool_use` records.
- **Parallel?**: [P] — independent of T006..T008.
- **Notes**:
  - Look at the existing fixtures `01-healthy-small.log` and `07-mixed-known-unknown.log` for header format conventions.
  - Keep the file small (< 5 KB). Include only the minimum messages needed.
  - Privacy: do not import real names. Synthetic names must be obviously synthetic.

### Subtask T010 — Extend `tooling_detect_test.go`

- **Purpose**: Cover FR-006 with the new fixture and exercise the masking primitive (NFR-003).
- **Files**: `internal/analyzer/tooling_detect_test.go`.
- **Steps**:
  1. Add a case to the existing table-driven test for `detectMCPExposureFromHeaders` and the call detector that loads fixture 08 and asserts `mcpUtilization.CallCount == 0`, `mcpUtilization.KnownCallCount == 0`, and `mcpUtilization.UniqueKnownCalledIDs` is empty.
  2. Add a separate unit test for `insideAny(off int, ranges []byteRange) bool`:
     - Empty ranges → always false.
     - Single range `[10, 20)` → `9` false, `10` true, `15` true, `19` true, `20` false.
     - Multiple ranges → first-hit short-circuit returns true.
- **Parallel?**: No — depends on T008.

### Subtask T011 — Verify C-006 no-op stability + update fixture-07 golden if needed

- **Purpose**: Prove that the fix does not perturb header-free fixtures and update the only fixture whose snapshot is allowed to shift.
- **Files**: `internal/analyzer/testdata/golden/**` (only if a snapshot shifts).
- **Steps**:
  1. Run `go test ./internal/analyzer/ -run TestGolden -v`.
  2. For fixtures 00..06, expect byte-identical results. If any of them shifts, **STOP** and investigate — that violates C-006 and means the mask is over-matching.
  3. For fixture 07 (`07-mixed-known-unknown.log`), the snapshot may shift downward (fewer false-positive MCP calls). Update the golden file in place.
  4. Capture the diff in the PR description so the reviewer can confirm the shift is in the expected direction (counts go down, never up).
- **Parallel?**: No — runs last in this WP.
- **Notes**:
  - If fixture 07's snapshot does NOT shift, it means fixture 07 had no header-token double-count to begin with. That's fine — note it in the PR and move on.
  - The `golden_test.go` file itself is owned by WP03; do not modify it. This WP only touches golden snapshot JSON files (data) in `testdata/golden/`.

## Test Strategy

Tests are required for FR-005, FR-006, NFR-003, and C-006.

Run during development:

```bash
go test ./internal/analyzer/ -run TestDetectMCPExposure -v
go test ./internal/analyzer/ -run TestGolden -v
```

Charter verification baseline at the end:

```bash
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
terraform -chdir=infra/aws fmt -check -recursive
./scripts/smoke-local.sh
```

## Risks & Mitigations

- **Risk**: Off-by-one on header byte ranges (e.g. inclusive vs exclusive end).
  - **Mitigation**: T010 unit test exercises boundary offsets `9/10/19/20` against `[10, 20)`.
- **Risk**: Mutating the input byte buffer instead of masking offsets.
  - **Mitigation**: Use `FindAllSubmatchIndex` to get offsets; never call `bytes.Replace` or similar on the raw input.
- **Risk**: Fixture 07 snapshot shifts in an unexpected direction (counts go up).
  - **Mitigation**: T011 explicitly checks the direction; an increase is a bug, not a fix.
- **Risk**: Skill-side defensive code introduces new false positives because skill detector does not produce header ranges yet.
  - **Mitigation**: If `skillExposure.HeaderRanges` stays empty for current logs, `insideAny` returns false for skill offsets and the call detector behavior is identical to today.
- **Risk**: Privacy regression by accidentally serializing `HeaderRanges` somewhere.
  - **Mitigation**: Fields are package-private with no `json:` tag; structs are not exported. WP03's T015 will add aggregate-level canary coverage.

## Review Guidance

Reviewer checkpoints for `/spec-kitty.review`:

1. **C-006 no-op stability**: confirm fixtures 00..06 golden snapshots are byte-identical pre/post-merge.
2. **FR-005 mask correctness**: confirm fixture 08 produces `CallCount == 0` after the fix.
3. **Fixture 07 direction**: if its snapshot shifted, it must have shifted downward (fewer calls), not upward.
4. **No serialized HeaderRanges**: confirm structurally that no JSON output anywhere mentions header ranges.
5. **Skill defensive symmetry**: confirm `skillExposure.HeaderRanges` was added and populated, even though no current skill bug exists.
6. **Issue #70 comments**: implementer commented at start and ready-for-review.

## Activity Log

> **CRITICAL**: Activity log entries MUST be in chronological order (oldest first, newest last). Append at the END.

Initial entry:

- 2026-05-19T11:55:54Z -- system -- Prompt created.

---

### Status Management

```bash
spec-kitty agent tasks move-task WP02 --to <status> --note "message"
```

### File Structure

All WP files live in a flat `tasks/` directory.
