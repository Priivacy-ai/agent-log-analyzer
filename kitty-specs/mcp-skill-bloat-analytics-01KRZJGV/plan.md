# Implementation Plan: MCP and Skill Bloat Analytics

**Branch**: `codex/mcp-skill-utilization` (cut from `main` at implementation time) | **Date**: 2026-05-19 | **Spec**: [spec.md](./spec.md)
**Mission**: `mcp-skill-bloat-analytics-01KRZJGV` (mission_id `01KRZJGVG3MCCCY9MKB1YRDBQR`)
**Branch contract**: current=`main`, planning base=`main`, merge target=`main`, matches target=true.

## Summary

Extend the existing `agent-log-analyzer` Go report with a `tooling_utilization` object nested inside `Ecosystem`, containing `mcp` and `skill` sub-objects. Each sub-object emits inventory (known IDs from allowlist, unknown count, bounded buckets for count and context footprint, `exposure_known` flag), usage (executed counts, integer utilization ratio %, context-efficiency bucket), and a deterministic warning band (`normal`/`watch`/`high`/`severe`/`unknown`). Bands are gated on the combination of count, footprint, utilization, and existing degradation signals from `Metrics` (Rereads/RetryDepthMax/ContextGrowthEvents). Remediation strings are appended to `Report.ImmediateFixes` via new findings when bands reach `high` or `severe`. Nesting inside `Ecosystem` causes the new fields to auto-flow into `AggregateSafeEvent.Ecosystem` with no additional plumbing. The aggregate is privacy-safe because every string field comes from a closed enumeration (bucket labels, band labels, allowlist IDs).

## Technical Context

**Language/Version**: Go 1.25 (from `go.mod`)
**Primary Dependencies**: Go stdlib only — `regexp`, `embed`, `encoding/json`, `sort`, `strings`, `bytes`, `bufio`. No new third-party packages.
**Storage**: None. The analyzer is a pure function over the input transcript bytes.
**Testing**: `go test ./...` with table-driven tests under `internal/analyzer/`. Golden fixtures live alongside under `internal/analyzer/testdata/` (existing pattern in `golden_test.go`).
**Target Platform**: Same as parent CLI — Linux/macOS/Windows binary, plus the AWS Lambda deployment used by `scripts/smoke-aws-local.sh`. No platform-specific code added.
**Project Type**: Single project (Go module `github.com/priivacy-ai/agent-log-analyzer`). All work is contained in `internal/analyzer/` plus `docs/`.
**Performance Goals**: New code adds O(n) passes over the transcript (n = byte size and line count). Total analyzer runtime on a 10MB transcript stays under 1 second on a developer laptop — the existing analyzer is already well under this and new passes are simple regex+string ops over the same `parsedLine` slice already produced.
**Constraints**:
- Determinism — same input → byte-identical `tooling_utilization` JSON (no maps in serialization order, no goroutine-induced reordering, no time-based values).
- Privacy — every string emitted in `AggregateSafeEvent.Ecosystem.tooling_utilization` comes from a closed enumeration. Free-form strings (unknown server names, tool names, skill names, paths, schema text) are counted only, never emitted.
- Backward compatibility — all existing `Ecosystem` fields and JSON keys preserved (`MCPServersKnown`, `UnknownMCPServerCount`, `KnownSkills`, `UnknownSkillCount`, `KnownPlugins`, `UnknownPluginCount`).
**Scale/Scope**: Touch ~6 existing files (`internal/analyzer/{types,ecosystem,analyzer,scrubber}.go`, `internal/analyzer/analyzer_test.go`, `internal/analyzer/golden_test.go`), add 3–4 new files (`internal/analyzer/tooling.go`, `internal/analyzer/tooling_test.go`, ~6 testdata fixtures), and update 4 docs.

## Charter Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

No charter exists (`spec-kitty charter context --action plan` returned `mode: missing`). Section is skipped per the planner workflow. If a charter is added later, re-evaluate before merge.

## Key Design Decisions

### D-1: Type placement — nested inside `Ecosystem`

**Decision**: Add `ToolingUtilization ToolingUtilization \`json:"tooling_utilization"\`` as a new field on `Ecosystem`. Recorded as resolved decision `01KRZJR0S1VJBYX6TQR2J6WVMZ`.

**Rationale**: `AggregateSafeEvent` already embeds `Ecosystem` directly (`internal/analyzer/analyzer.go:451`). Nesting auto-propagates the new fields into the upload-safe shape with zero additional plumbing. Lives next to the existing `MCPServersKnown`/`KnownSkills` fields it relates to.

**Trade-off accepted**: `Ecosystem` becomes slightly larger (identity + behavior in one struct). Acceptable — `Ecosystem` is already a mixed bag (`PackageManagers`, `WorkflowFrameworks`, `KnownPlugins` are arguably more "behavioral" than "identity" anyway).

### D-2: Context-token footprint estimator

**Decision**: Hybrid estimator.

1. **Schema-text path** (preferred when available): when the local transcript contains MCP tool definitions or skill descriptions inline (e.g., as `system-reminder` blocks listing available tools/skills with their descriptions), measure the byte length of those blocks divided by 4 (matching the existing `estimateTokens` heuristic at `internal/analyzer/analyzer.go:492`) and bucket the resulting token count. This length stays **local**; it is never emitted.
2. **Constant-per-item fallback** (when no schema text observed): use fixed deterministic constants — `~150 tokens` per exposed MCP tool, `~250 tokens` per exposed MCP server overhead, `~400 tokens` per exposed skill. These constants are documented in `docs/ecosystem-signatures.md` and pinned in fixtures.
3. **Unknown path**: when neither exposure count nor schema text is observable, the context-token bucket is `unknown` and `exposure_known` is `false`.

**Rationale**: The brief explicitly allows local measurement of schema text but forbids emitting it. The hybrid keeps measurements honest where possible while degrading gracefully.

### D-3: Exposure detection

**Decision**: Three signals, evaluated in order; first match wins, otherwise `exposure_known=false`.

1. **Structured availability headers**. Match patterns like `Available MCP servers?:`, `MCP tools available:`, `following skills are available`, `following deferred tools are now available` in the scrubbed transcript. When matched, parse the immediately following list (allowlist IDs are emitted by ID; everything else is counted into `unknown_*_count` only — names are NEVER stored or emitted).
2. **Tool-use inference for MCP**. Distinct `mcp__server__*` prefixes observed in tool-use blocks (using the existing regex at `internal/analyzer/ecosystem.go:101`) give a **lower bound** on exposed servers and tools — exposed ≥ called. When (1) is absent but tool-use blocks are present, set `exposure_known=true`, `server_count_bucket` ≥ the bucket containing distinct prefixes, and emit the called counts as both call counts and the inferred-exposed lower bound. Mark `inference_source: "calls"` (a fixed enum value) so downstream consumers know precision is limited.
3. **No signal**. `exposure_known=false`, all exposure buckets `unknown`, warning band `unknown`. No usage-based warning may fire.

**Rationale**: Honest about confidence. Conservative: it is better to say "unknown" than to invent a denominator that produces a misleading warning.

### D-4: Warning band thresholds (deterministic constants)

The brief mandates the bands not fire on count alone. Concrete thresholds, pinned by fixtures:

**MCP**:
- `unknown`: `exposure_known=false`.
- `normal`: any of {`server_count_bucket` ≤ `4-10`} **OR** {`utilization_ratio_pct` ≥ 40}.
- `watch`: `server_count_bucket` ∈ {`11-25`, `26-50`} **AND** `utilization_ratio_pct` < 40 **AND** no degradation signals.
- `high`: `server_count_bucket` ≥ `11-25` **AND** `utilization_ratio_pct` < 20 **AND** (`context_token_bucket` ≥ `5k-15k` **OR** `exposed_tool_count_bucket` ≥ `26-50`).
- `severe`: `high` conditions **AND** at least one degradation signal present (`Metrics.Rereads ≥ 3` **OR** `Metrics.RetryDepthMax ≥ 3` **OR** `Metrics.ContextGrowthEvents ≥ 2`).

**Skill**:
- `unknown`: `exposure_known=false`.
- `normal`: any of {`exposed_count_bucket` ≤ `4-10`} **OR** {`utilization_ratio_pct` ≥ 30}.
- `watch`: `exposed_count_bucket` ∈ {`11-25`, `26-50`} **AND** `utilization_ratio_pct` < 30 **AND** no degradation signals.
- `high`: `exposed_count_bucket` ≥ `11-25` **AND** `utilization_ratio_pct` < 15 **AND** `context_token_bucket` ≥ `5k-15k`.
- `severe`: `high` conditions **AND** at least one degradation signal present (same thresholds as MCP).

Both bands lock to `normal` whenever utilization is meaningfully high, regardless of exposure count. Count alone never triggers anything above `normal`.

### D-5: Remediation wording

**Decision**: Bloat findings are added through the existing `buildFindings` pathway as new finding IDs (`mcp_bloat_high`, `mcp_bloat_severe`, `skill_bloat_high`, `skill_bloat_severe`). Their `Recommendation` strings flow into `Report.ImmediateFixes` via the existing `immediateFixes` helper. Wording draws from the fixed set named in spec FR-006. No private content.

**Rationale**: Reuses an existing pipeline; preserves the existing contract that immediate fixes are derived from findings; keeps band → finding → fix traceable.

### D-6: Plugins out of scope

`Ecosystem.KnownPlugins` and `UnknownPluginCount` are **not** modified. The brief is scoped to MCP and skill. Plugins are a separate concept and adding a `PluginUtilization` is left to a future mission.

## Project Structure

### Documentation (this feature)

```
kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/
├── plan.md              # This file
├── spec.md              # Already committed
├── research.md          # Phase 0 output (this command)
├── data-model.md        # Phase 1 output (this command)
├── quickstart.md        # Phase 1 output (this command)
├── contracts/           # Phase 1 output (this command)
│   └── tooling-utilization.json    # JSON Schema for the new fields
├── decisions/           # Decision Moments (01KRZJR0S1VJBYX6TQR2J6WVMZ already minted)
├── checklists/
│   └── requirements.md  # Already committed
└── tasks/               # Populated by /spec-kitty.tasks
```

### Source Code (repository root)

```
internal/analyzer/
├── types.go                 # MODIFIED: add ToolingUtilization, MCPUtilization, SkillUtilization, BucketLabels
├── ecosystem.go             # MODIFIED: extend DetectEcosystem to populate new fields
├── tooling.go               # NEW: bucketing helpers, band classifier, footprint estimator
├── tooling_test.go          # NEW: unit tests for bucketing, bands, footprint, exposure detection
├── analyzer.go              # MODIFIED: register new findings via buildFindings; extend normalizeEcosystemCollections
├── analyzer_test.go         # MODIFIED: extend existing tests where shapes are asserted
├── golden_test.go           # MODIFIED: add new fixture cases
├── scrubber.go              # MODIFIED only if scrubber needs to preserve a marker (probably not)
├── registry.go              # UNCHANGED
├── signatures/              # UNCHANGED (allowlists consumed as-is)
└── testdata/                # NEW: 7 synthetic fixtures (see Phase 1 data-model.md)

docs/
├── ecosystem-signatures.md             # MODIFIED: document new metrics + privacy stance
├── data-retention-and-analytics.md     # MODIFIED: document aggregate shape
├── logging-policy.md                   # MODIFIED: cross-link
└── testing-plan.md                     # MODIFIED: add the 7 fixture scenarios
```

**Structure Decision**: Single-project Go module. All new logic lives in `internal/analyzer/tooling.go` to keep the diff against `ecosystem.go` minimal and merge-friendly with Epic #38 (which also touches `ecosystem.go`). `ecosystem.go` gets only a small wiring change to invoke the new functions; the bulk of new code is in `tooling.go`.

## Complexity Tracking

No Charter Check violations (no charter exists). No complexity entries.

## Phase 0 — Research

See [research.md](./research.md). Summary of unknowns resolved:

- **R-1 (footprint estimator)** → D-2 hybrid (schema length when present, constants otherwise).
- **R-2 (exposure signal sources)** → D-3 three-tier (header → call inference → unknown).
- **R-3 (band threshold concrete values)** → D-4 pinned by fixtures.
- **R-4 (remediation surfacing path)** → D-5 via existing findings → immediate-fixes pipeline.
- **R-5 (allowlist coverage)** → existing `signatures/{mcp_servers,skills}.json` is sufficient for v1; no new allowlist entries needed to land the mission (fixtures use synthetic public IDs that are in the allowlist + synthetic private names that are not).

## Phase 1 — Design & Contracts

- **Data model**: see [data-model.md](./data-model.md).
- **JSON contract**: see [contracts/tooling-utilization.json](./contracts/tooling-utilization.json).
- **Quickstart**: see [quickstart.md](./quickstart.md).

## Charter Re-check (post-design)

Skipped — no charter. Privacy stance is enforced instead by C-001/C-002 in `spec.md`, NFR-004/NFR-006, and SC-2 — three overlapping enforcement points. The mission must pass all three before merge.

## Branch Contract (final restatement)

- Workflow start branch: `main`.
- Planning/base branch: `main`. Plan artifacts commit here.
- Final merge target: `main`.
- Implementation branch (cut later by `/spec-kitty.tasks` / `/spec-kitty.implement`): `codex/mcp-skill-utilization` (timestamp suffix if it already exists, per spec C-005).

Once tasks are generated, the implementation worktree(s) will be created under `.worktrees/mcp-skill-bloat-analytics-01KRZJGV-lane-a/` (or similar lane naming).
