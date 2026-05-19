---
work_package_id: WP02
title: 'Detection: Exposure, Calls, Footprint'
dependencies:
- WP01
requirement_refs:
- FR-001
- FR-002
- FR-003
- FR-004
- FR-007
planning_base_branch: main
merge_target_branch: main
branch_strategy: Lane worktree branches from WP01's lane head; runs in parallel with WP03.
subtasks:
- T005
- T006
- T007
- T008
- T009
agent: claude
history:
- event: generated
  at: '2026-05-19T08:00:33Z'
  by: /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/tooling_detect.go
execution_mode: code_change
mission_id: 01KRZJGVG3MCCCY9MKB1YRDBQR
mission_slug: mcp-skill-bloat-analytics-01KRZJGV
owned_files:
- internal/analyzer/tooling_detect.go
- internal/analyzer/tooling_detect_test.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load the agent profile by invoking `/ad-hoc-profile-load` with `profile_id: "implementer-ivan"` and `role: "implementer"`. Then return here.

## Objective

Implement the pure detection layer that extracts privacy-safe signals from a parsed transcript. This WP delivers four pure functions and their tests: a header-based exposure detector, a call inference helper for MCP, a skill execution counter that preserves the existing path-avoidance behavior, and a hybrid footprint estimator. All functions return only counts, allowlist IDs, and closed-enum labels — never raw names, paths, schema text, or skill text.

All output is consumed by WP04 (wiring) which calls these functions from `DetectEcosystem` and feeds the results into the classifier from WP03.

## Context

Read first:
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/spec.md` — FR-001 through FR-004, FR-007 (path avoidance), C-001 (privacy), C-002 (counts only).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md` — §D-2 (footprint estimator), §D-3 (exposure detection — three tiers).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/research.md` — R-1 and R-2 for full rationale.
- `internal/analyzer/ecosystem.go` — existing `countUnknownMCP` (line 100, regex `mcp__([A-Za-z0-9_-]+)__`) and `countUnknownSlashCommands` (line 119, with path-avoidance via `matchEnd` check). **Reuse the same regex and path-avoidance logic** — do not weaken it.
- `internal/analyzer/registry.go` — `ecosystemRegistry()` exposes `MCPServers`, `Skills`, and `KnownSlashCommandIDs()`.
- `internal/analyzer/types.go` — `parsedLine` has `IsTool`, `ToolName`, `Text`, `Command` fields you can use.
- `internal/analyzer/tooling_buckets.go` (from WP01) — `countBucket`, `tokenBucket`, `InferenceSourceHeader`/`Calls`/`None` constants.

Branch contract:
- Planning base: `main`. Merge target: `main`. Lane worktree allocated by `/spec-kitty.implement`.

## Detailed Guidance

### Subtask T005 — Header-based exposure detector

**Purpose**: Find structured availability headers in the transcript and parse the immediately following list. Allowlist hits go to `known_*_ids`; everything else increments `unknown_*_count`. Names from the "everything else" path are NEVER stored or emitted.

**Steps**:
1. In a new file `internal/analyzer/tooling_detect.go`, add two functions:
   ```go
   type mcpExposure struct {
       KnownIDs           []string // sorted, lowercased, underscored — allowlist IDs only
       UnknownCount       int
       ExposedToolCount   int  // 0 when only server count is known
       ExposedToolKnown   bool
       SchemaTextBytes    int  // bytes of header block; 0 if no header
       InferenceSource    string // "header" when header found, "" otherwise
   }
   type skillExposure struct {
       KnownIDs        []string
       UnknownCount    int
       SchemaTextBytes int
       InferenceSource string
   }

   func detectMCPExposureFromHeaders(input []byte, registry signatureRegistry) mcpExposure { ... }
   func detectSkillExposureFromHeaders(input []byte, registry signatureRegistry) skillExposure { ... }
   ```
2. Patterns to match (case-insensitive, anchored on phrases observed in real Claude Code system reminders). Compile once at package init:
   - MCP server list headers: `(?i)available mcp servers?:`, `(?i)mcp tools? available`, `(?i)following deferred tools? are now available`.
   - Skill list headers: `(?i)following skills are available`, `(?i)available skills:`.
3. Algorithm per match:
   a. Find the header position in `input`.
   b. Read until the next blank line or until N=200 lines (defensive limit) to capture the list block.
   c. Record `SchemaTextBytes = len(block)`.
   d. For each line in the block, extract candidate IDs using a conservative regex (e.g., `^[\s\-*•]*([a-z0-9_][a-z0-9_:-]+)` capturing the leading bullet-style entry, lowercased and underscored via the existing `normalizeID` helper in `ecosystem.go:155`).
   e. Bucket each candidate: if its normalized form matches an allowlist ID from `registry.MCPServers` / `registry.Skills`, add to `KnownIDs`; otherwise increment `UnknownCount`.
   f. **Never store unknown candidate strings.** Increment the counter and discard.
   g. For MCP: also look for `mcp__server__tool` tokens in the same block to count exposed tools (set `ExposedToolCount` and `ExposedToolKnown = true`).
   h. Sort `KnownIDs` (use `sortedKeys` pattern). Deduplicate.
   i. Set `InferenceSource = "header"` if any header matched, otherwise leave empty (caller decides the fallback).

**Files**:
- `internal/analyzer/tooling_detect.go` (new file).

**Validation**:
- [ ] When fixture contains `Available MCP servers: github, linear, acme_internal`, `KnownIDs` is `["github", "linear"]` and `UnknownCount` is `1`.
- [ ] The string `"acme_internal"` does NOT appear anywhere in the returned struct (verify via reflection or by serializing the struct to JSON and grepping).
- [ ] When no header is present, returns zero-value structs with `InferenceSource = ""`.

### Subtask T006 — Tool-use call inference for MCP

**Purpose**: For MCP, when no exposure header is found, infer a lower bound on exposed servers/tools from the actual calls observed. Set `inference_source = "calls"` so consumers know precision is limited.

**Steps**:
1. In `tooling_detect.go`, add:
   ```go
   type mcpCalls struct {
       TotalCalls           int
       KnownCallCount       int
       UnknownCallCount     int
       UniqueKnownIDs       []string // sorted
       UniqueUnknownCount   int
       UniqueServerCount    int // total distinct servers observed
       UniqueToolCount      int // total distinct server::tool pairs observed
   }

   func detectMCPCallsFromToolUse(input []byte, lines []parsedLine, registry signatureRegistry) mcpCalls { ... }
   ```
2. Use the existing regex pattern from `ecosystem.go:101` (`mcp__([A-Za-z0-9_-]+)__([A-Za-z0-9_-]+)`) extended to capture both server and tool. Apply it to:
   - the raw `input` bytes (catches references in narrative text), AND
   - each `parsedLine` where `IsTool` is true and `ToolName` starts with `mcp__` (most reliable signal).
3. Bucket each call:
   - Normalize server name with `normalizeID`. If it's in the MCP allowlist, increment `KnownCallCount` and add to `UniqueKnownIDs` set. Otherwise increment `UnknownCallCount` and add to a private unknown-set used **only** for counting `UniqueUnknownCount`.
   - Increment `TotalCalls` for every match.
   - Maintain two more sets: distinct server names and distinct server::tool pairs, used **only** to compute counts.
4. Sort `UniqueKnownIDs`. Return.

**Files**:
- `internal/analyzer/tooling_detect.go` (extend).

**Validation**:
- [ ] A transcript with 5 calls to `mcp__github__create_issue` produces `TotalCalls=5`, `KnownCallCount=5`, `UniqueKnownIDs=["github"]`, `UniqueServerCount=1`, `UniqueToolCount=1`.
- [ ] A transcript with calls to a private server `mcp__acme_secret__send` produces `UnknownCallCount > 0` and `UniqueUnknownCount > 0`; the string `"acme_secret"` does NOT appear in the returned struct.

### Subtask T007 — Skill execution counter with path-avoidance

**Purpose**: Count skill executions while preserving the existing path-avoidance behavior at `ecosystem.go:119`. Distinguish known (allowlist) from unknown.

**Steps**:
1. In `tooling_detect.go`, add:
   ```go
   type skillExecutions struct {
       ExecutedCount    int
       KnownExecutedIDs []string // sorted
       UnknownExecuted  int
   }

   func detectSkillExecutionsFromLines(lines []parsedLine, registry signatureRegistry) skillExecutions { ... }
   ```
2. Reuse the path-avoidance regex from `ecosystem.go:120` exactly: `(?:^|[\s"'(:])/(?:[A-Za-z][A-Za-z0-9_-]{2,})`.
3. Iterate over `parsedLine` entries where `line.IsTool` is false (matches existing behavior at `ecosystem.go:127`).
4. For each match:
   - Apply the same trim/skip logic as the existing function (lines 131-135) — this is what prevents file-path false positives.
   - Lowercase, strip leading `/`, strip leading `gstack-` (matches existing line 137).
   - Normalize with `normalizeID`.
   - If known: increment `ExecutedCount`, add to known-set, no double-count for repeated calls to the same skill (a skill called 3 times is still 1 unique known ID but 3 executions).
   - Wait — re-read existing logic. The current code counts **distinct unknown names**, not executions. We need both:
     - `ExecutedCount`: total executions (one per regex match that passes path-avoidance, including repeats).
     - `KnownExecutedIDs`: distinct known IDs (sorted).
     - `UnknownExecuted`: distinct unknown names (count only).
5. **Do not store unknown names.** Increment the unknown set's distinct count by tracking a `map[string]bool` locally, but discard before returning.

**Critical**: Run the existing `analyzer_test.go` cases for slash-command path-avoidance against your new code path. They must all pass.

**Files**:
- `internal/analyzer/tooling_detect.go` (extend).

**Validation**:
- [ ] A line containing `"/etc/passwd"` produces zero skill executions (path-avoidance preserved).
- [ ] A line containing `/scrape` (with no following slash) where `scrape` is in the allowlist produces one execution and `KnownExecutedIDs=["scrape"]`.
- [ ] Multiple invocations of the same skill produce `ExecutedCount` >= 2 with `KnownExecutedIDs` still length 1.

### Subtask T008 — Hybrid footprint estimator

**Purpose**: Estimate the context-token footprint of exposed MCPs/skills. Prefer measured schema/description text length when present; fall back to fixed per-item constants; emit `unknown` when neither is available.

**Steps**:
1. In `tooling_detect.go`, add:
   ```go
   const (
       // Fixed per-item token-cost constants for the fallback path (see plan §D-2).
       mcpServerOverheadTokens = 250
       mcpToolTokens           = 150
       skillTokens             = 400
   )

   // estimateMCPFootprintTokens returns an integer token estimate.
   // schemaBytes: bytes of inline schema/header block observed; 0 if none.
   // serverCount, toolCount: known exposure counts; -1 if unknown.
   // Returns (tokens, known) where known=false signals no signal observed.
   func estimateMCPFootprintTokens(schemaBytes, serverCount, toolCount int) (int, bool) { ... }
   func estimateSkillFootprintTokens(schemaBytes, skillCount int) (int, bool) { ... }
   ```
2. Algorithm (matches plan §D-2):
   - If `schemaBytes > 0`: return `(schemaBytes / 4, true)` — matches the existing `estimateTokens` heuristic at `analyzer.go:492`.
   - Else if `serverCount >= 0`: return `(serverCount * mcpServerOverheadTokens + max(toolCount, 0) * mcpToolTokens, true)`.
   - Else: return `(0, false)`.
   - Analogous for skill: `schemaBytes/4` else `skillCount * skillTokens` else `(0, false)`.
3. These are pure functions. No side effects.

**Files**:
- `internal/analyzer/tooling_detect.go` (extend).

**Validation**:
- [ ] `estimateMCPFootprintTokens(4000, -1, -1)` returns `(1000, true)`.
- [ ] `estimateMCPFootprintTokens(0, 10, 30)` returns `(10*250 + 30*150, true) = (7000, true)`.
- [ ] `estimateMCPFootprintTokens(0, -1, -1)` returns `(0, false)`.
- [ ] `estimateSkillFootprintTokens(0, 5)` returns `(5*400, true) = (2000, true)`.

### Subtask T009 — Detection unit tests

**Purpose**: Lock the behavior of all four detection functions with table-driven tests, including the privacy guarantee that unknown names are never stored or returned.

**Steps**:
1. Create `internal/analyzer/tooling_detect_test.go` with package `analyzer`.
2. Write four test functions: `TestDetectMCPExposureFromHeaders`, `TestDetectMCPCallsFromToolUse`, `TestDetectSkillExecutionsFromLines`, `TestEstimateFootprint`.
3. Per test, table-driven cases covering:
   - Empty input → zero-value output.
   - Allowlist-only input → all known IDs populated.
   - Unknown-only input → counts populated, names NOT in output struct.
   - Mixed input → both populated correctly.
   - Privacy assertion (critical): after running detection, serialize the result struct to JSON and assert via `strings.Contains` that none of the private name strings appear. Example:
     ```go
     out := detectMCPExposureFromHeaders([]byte(input), registry)
     blob, _ := json.Marshal(out)
     for _, leak := range []string{"acme_internal_secret", "private_corp_mcp"} {
         if strings.Contains(string(blob), leak) {
             t.Errorf("privacy leak: serialized output contains %q", leak)
         }
     }
     ```
4. For skill path-avoidance: include the existing tests' fixtures verbatim (paths like `"/etc/passwd"`, `"/home/user/file"`, code-fenced paths) and assert zero counts.

**Files**:
- `internal/analyzer/tooling_detect_test.go` (new file).

**Validation**:
- [ ] `go test ./internal/analyzer/ -run TestDetect` passes.
- [ ] At least one test per function explicitly asserts zero substring matches for a basket of private-looking strings.

## Test Strategy

All four functions are pure — table-driven tests with no mocks. The privacy guarantee is enforced through substring assertions on serialized output, not just behavioral checks.

## Definition of Done

- [ ] `internal/analyzer/tooling_detect.go` exists with the four functions and their helper types.
- [ ] `internal/analyzer/tooling_detect_test.go` exercises every function with at least 4 cases (empty, known-only, unknown-only, mixed) plus the privacy substring assertion.
- [ ] `go test ./internal/analyzer/ -run TestDetect` passes.
- [ ] `gofmt -l` on both files returns empty.
- [ ] No existing test in `internal/analyzer/` regresses.
- [ ] The path-avoidance regex matches the existing one byte-for-byte.

## Risks

- **Risk**: Reading header blocks too greedily and absorbing post-list narrative as if it were items. **Mitigation**: stop at the first blank line or after N=200 lines, whichever comes first, plus the bullet-style leading regex.
- **Risk**: Subtle regex divergence from the existing path-avoidance code. **Mitigation**: copy the exact regex string and trim logic from `ecosystem.go:120-135`; do not rewrite for "cleanliness".
- **Risk**: A new test fixture accidentally contains a real-looking secret. **Mitigation**: all test inputs use clearly synthetic names like `acme_internal_test_mcp`, `private_corp_skill`, never real product names.

## Reviewer Guidance

When reviewing:
- Verify path-avoidance regex is identical to `ecosystem.go:120`.
- Verify no function ever appends an unknown name to a returned slice — only counts increment.
- Verify the privacy substring assertions exist in tests and target plausible leak strings.
- Spot-check that `KnownIDs` slices are sorted (deterministic ordering).
- Verify constants in `estimateMCPFootprintTokens` match plan §D-2 (`mcpServerOverheadTokens=250`, `mcpToolTokens=150`, `skillTokens=400`).

## Implementation Command

```bash
spec-kitty agent action implement WP02 --agent claude
```
