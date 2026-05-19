---
work_package_id: WP01
title: Token-saving tool registry
dependencies: []
requirement_refs:
- FR-001
- FR-002
- FR-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning base main; the work-branch and lane assignment come from lanes.json after finalize-tasks.
subtasks:
- T001
- T002
- T003
- T004
- T005
agent: claude
history:
- 2026-05-19: created from mission token-saving-recommendation-engine-phase-a-01KRZKCJ
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/token_saving_tools
execution_mode: code_change
owned_files:
- internal/analyzer/token_saving_tools.go
- internal/analyzer/token_saving_tools_test.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading the rest of this prompt, load the assigned agent profile:

```text
/ad-hoc-profile-load implementer-ivan
```

The profile defines your identity, governance scope, allowed file surface, and
initialization declaration for this work package. After it loads, return here
and continue with **Objective** below.

## Objective

Ship the additive, code-owned **token-saving tool registry** plus its small
public lookup API. After this work package lands, the package
`internal/analyzer/` exposes `TokenSavingTool`, `GetTool`, `AllTools`,
`RegistryVersion`, and a registry literal that downstream WPs (engine, tests,
docs) can consume.

This WP is purely additive. It must not touch `internal/analyzer/types.go`,
`internal/analyzer/ecosystem.go`, or any other existing source file.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. The implementing
agent operates on the work-branch and execution worktree assigned by
`lanes.json` (read it via `spec-kitty agent context resolve --mission
token-saving-recommendation-engine-phase-a-01KRZKCJ --wp WP01 --json`); do
not invent or hop branches manually.

## Context

The full picture lives in:

- `kitty-specs/token-saving-recommendation-engine-phase-a-01KRZKCJ/spec.md`
- `kitty-specs/token-saving-recommendation-engine-phase-a-01KRZKCJ/data-model.md` (TokenSavingTool struct, registry invariants)
- `kitty-specs/token-saving-recommendation-engine-phase-a-01KRZKCJ/research.md` §"Per-tool research notes" (the canonical allowlist)
- `kitty-specs/token-saving-recommendation-engine-phase-a-01KRZKCJ/contracts/token_saving_engine_go_api.md` (frozen Go surface)

You do not need anything outside this mission directory to complete WP01.

## Owned files

This WP owns and is the only writer of:

- `internal/analyzer/token_saving_tools.go` (new)
- `internal/analyzer/token_saving_tools_test.go` (new)

Both files are new in this WP. **Do not modify any other file**, including
`internal/analyzer/types.go`. If you discover a need to do so, stop and
report.

## Implementation command

```bash
spec-kitty agent action implement WP01 --agent claude
```

---

### Subtask T001 — Declare the `TokenSavingTool` struct

**Purpose.** Stand up the registry's primary type in a new file.

**Steps.**

1. Create `internal/analyzer/token_saving_tools.go` with `package analyzer`
   and a short file-header comment naming the WP and mission slug.
2. Declare the named type `type ToolID string`. Document in a comment that
   the canonical form is lowercase + underscore-separated.
3. Declare the struct exactly as specified in `data-model.md` (every field
   present, JSON tags as documented). Fields:
   `ID, DisplayName, SourceURL, Category, RecommendationClass, ClassRank,
   DetectorSources, InstallRisk, DataMovementRisk, RollbackGuidance,
   FreeReportAllowed, PaidPackAllowed, ResearchOnly, InstallPolicy, Notes`.
4. The struct's enum-typed fields refer to named string types declared in
   WP02 (`RecommendationClass`, `RiskLevel`, `InstallPolicy`, `EvidenceSource`).
   To avoid a circular dependency, declare these as `type FooName = string`
   placeholders in this file with a comment "redeclared as named types in
   `token_saving_types.go` once WP02 lands" — or, simpler, **wait for WP02
   to land first if the agent runs the WPs sequentially**. (Finalize-tasks
   marks WP02 as a dependency for WP03 and WP04 only because of file
   ownership; if you'd rather declare the registry without leaning on
   WP02's types, leave the fields as `string` with a TODO and let WP02 do
   the rename. The acceptance test in WP04 covers either layout.)

**Validation.** `go vet ./internal/analyzer/` clean; `go build ./...`
succeeds with WP01 alone.

---

### Subtask T002 — Encode the registry Go literal

**Purpose.** Populate the registry with every entry the brief and existing
matrix doc require, deduped, with `research_only` set wherever the source URL
is unverified.

**Steps.**

1. Add `var registry = []TokenSavingTool{ … }` as a package-level value.
2. Populate one entry per tool from `research.md` §"Per-tool research notes".
   Use the IDs verbatim (lowercase + underscore). Sample entry (truncated):

   ```go
   {
       ID:                  "ccusage",
       DisplayName:         "ccusage",
       SourceURL:           "https://github.com/ryoppippi/ccusage",
       Category:            "observability",
       RecommendationClass: "usage_visibility",
       ClassRank:           1,
       DetectorSources:     []EvidenceSource{"cli_presence", "cli_version", "log_active_command"},
       InstallRisk:         "low",
       DataMovementRisk:    "low",
       RollbackGuidance:    "",
       FreeReportAllowed:   true,
       PaidPackAllowed:     true,
       ResearchOnly:        false,
       InstallPolicy:       "recommend",
       Notes:               "Independent metrics layer.",
   },
   ```

3. For any tool whose `source_url` is marked "unverified" in research.md,
   set `SourceURL: ""`, `ResearchOnly: true`, `InstallPolicy: "research_only"`,
   `FreeReportAllowed: false`, `PaidPackAllowed: false`, and a non-empty
   `Notes` explaining the gap.
4. For tools flagged `recommend_with_waiver` (notably `rtk`), provide a
   non-empty `RollbackGuidance` string describing how a user disables the
   tool (one sentence is enough; e.g. "Remove the rtk hook from
   `.claude/settings.json` and restart the session.").
5. Maintain the ordering: usage_visibility tools first, then
   mcp_skill_hygiene, then mcp_output_reducer, then shell_output_reducer,
   then retrieval, then reread_guard, then context_hygiene, then
   output_verbosity, then reference_only.

**Files.** `internal/analyzer/token_saving_tools.go` (~250 lines after this
step).

**Validation.** Manual eyeball against research.md and a `grep -c '^\tID:' …`
matches the brief's allowlist count.

---

### Subtask T003 — Implement the public lookup API

**Purpose.** Expose three pure functions matching the contract.

**Steps.**

1. `func GetTool(id ToolID) (TokenSavingTool, bool)` — linear scan over
   `registry`; returns `(zero, false)` for unknown IDs.
2. `func AllTools() []TokenSavingTool` — returns a defensive copy sorted by
   `(RecommendationClass, ClassRank)` ascending. Use `sort.SliceStable` with
   a deterministic comparator.
3. `func RegistryVersion() string` — returns the constant
   `"phase-a-2026-05-19"`. Declare the constant at package scope so it can
   be referenced by tests.

**Validation.** Direct unit test in WP01 covers all three (see T004).

---

### Subtask T004 — `TestRegistryInvariants` [P]

**Purpose.** Lock every invariant from data-model.md.

**Steps.**

1. Create `internal/analyzer/token_saving_tools_test.go` with
   `package analyzer` and the import block (`testing`, `regexp`, `strings`).
2. Add `TestRegistryInvariants(t *testing.T)` that iterates `AllTools()` and
   asserts:
   - `ID` is non-empty, matches `^[a-z][a-z0-9_]*$`, and is unique across the
     registry.
   - `InstallPolicy` is one of the five documented values.
   - `ResearchOnly == (InstallPolicy == "research_only")`.
   - `InstallPolicy == "recommend_with_waiver"` ⇒ `RollbackGuidance != ""`.
   - `RecommendationClass` is one of the eight documented values.
   - `(RecommendationClass, ClassRank)` pairs are unique.
   - `SourceURL != "" || ResearchOnly == true`.

3. Each failed assertion calls `t.Errorf` so the run produces a complete
   list rather than stopping on the first violation.

**Validation.** `go test ./internal/analyzer/ -run TestRegistryInvariants`
passes.

---

### Subtask T005 — `TestRegistryAllowlistCoverage` [P]

**Purpose.** Prove every brief-listed tool ID is present exactly once.

**Steps.**

1. In the same test file, define a private variable
   `var briefAllowlist = []ToolID{ "ccusage", "tokenusage", "claude_meter",
   "rtk", "leanctx", "headroom", "context_mode", "distill",
   "token_optimizer_mcp", "serena", "codegraph", "codebase_memory_mcp",
   "code_review_graph", "semble", "jcodemunch_mcp", "grepai",
   "claude_context", "token_savior", "cocoindex_code", "read_once",
   "openwolf", "claude_token_efficient", "caveman" }`.
2. Add `TestRegistryAllowlistCoverage(t *testing.T)` that for each ID in
   `briefAllowlist` asserts `GetTool(id)` returns `(tool, true)` and that
   the slice has zero duplicates.
3. Also add a second loop asserting that every additional registry entry
   beyond the brief list has `InstallPolicy == "reference_only" ||
   InstallPolicy == "research_only"` (the union added from the matrix doc
   must not silently promote a tool to `recommend`).

**Validation.** `go test ./internal/analyzer/ -run TestRegistryAllowlistCoverage`
passes.

---

## Definition of Done

- [ ] `internal/analyzer/token_saving_tools.go` exists with the
      `TokenSavingTool` struct, the registry literal, and three exported
      functions.
- [ ] `internal/analyzer/token_saving_tools_test.go` exists with
      `TestRegistryInvariants` and `TestRegistryAllowlistCoverage`.
- [ ] `go vet ./internal/analyzer/` and `go build ./...` succeed.
- [ ] `go test ./internal/analyzer/ -run TokenSavingTools` passes (both
      tests green).
- [ ] No file outside `owned_files` was modified.
- [ ] `gofmt -w internal/analyzer/token_saving_tools*.go` is clean.

## Risks & reviewer guidance

- The reviewer should diff the registry literal against
  `research.md` §"Per-tool research notes" line by line.
- Pay special attention to `RollbackGuidance` for `rtk` and any other
  `recommend_with_waiver` entries — empty strings here are caught by
  `TestRegistryInvariants` but the reviewer should verify the guidance
  itself is meaningful.
- Confirm `RegistryVersion()` returns a fresh value if the registry
  changes during review iterations (NFR-005).

## Out of scope for WP01

- Any engine logic (lives in WP03).
- Any enum constant declarations not strictly needed by this file (lives
  in WP02).
- Any doc updates (live in WP05/WP06).
