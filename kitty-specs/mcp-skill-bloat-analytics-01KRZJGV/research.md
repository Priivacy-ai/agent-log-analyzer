# Phase 0 Research: MCP and Skill Bloat Analytics

**Mission**: `mcp-skill-bloat-analytics-01KRZJGV`
**Date**: 2026-05-19

This document consolidates the design unknowns raised during planning, the decisions reached, the rationale, and the alternatives considered.

## R-1: Context-token footprint estimator

**Question**: How should the analyzer estimate the context footprint of exposed MCPs/skills without uploading raw schema or skill text?

**Decision**: Hybrid estimator.
- When the local transcript contains inline schema/description text (e.g., system-reminder blocks listing tools or skills with their descriptions), measure that block's byte length, divide by 4 to estimate tokens (matches existing `estimateTokens` heuristic in `internal/analyzer/analyzer.go:492`), bucket the result.
- When no schema text is observed but exposure count is known, multiply count × fixed per-item constants: ~150 tokens per MCP tool, ~250 tokens per MCP server overhead, ~400 tokens per skill. Constants documented in `docs/ecosystem-signatures.md`.
- When neither is observed, emit `context_token_bucket: unknown` and `exposure_known: false`.

**Rationale**: The brief explicitly permits *local* measurement of schema text and forbids *uploading* it. Using local length when available gives a more faithful number for the user; falling back to constants keeps the metric meaningful even when the log lacks schema headers. The bucketing throws away enough precision that no private content survives in the upload.

**Alternatives considered**:
- *Constants-only*: simpler but ignores observable signal. Rejected because it would emit the same bucket for two users whose exposure footprints differ by an order of magnitude.
- *Length-only*: would mark exposure as `unknown` whenever schema text isn't in the log, even when we can count exposure another way. Rejected because it under-reports for users whose transcripts include tool-use blocks but no header lists.

## R-2: Exposure signal sources

**Question**: How does the analyzer determine how many MCPs/skills were *exposed* (vs. just *called*)?

**Decision**: Three-tier signal hierarchy.
1. **Explicit availability headers in transcript** (preferred). Match patterns like `Available MCP servers:`, `following skills are available`, `following deferred tools are now available`. Parse the list, count allowlist hits as known IDs, count everything else into `unknown_*_count`. Names never stored.
2. **Tool-use inference** (fallback for MCP). The existing regex `mcp__([A-Za-z0-9_-]+)__` in `ecosystem.go:101` gives the set of MCP servers actually called. Use distinct prefixes as a lower bound for exposure when no header is present. Set `inference_source: "calls"` (closed enum) so consumers know precision is limited.
3. **No signal**. `exposure_known: false`, all exposure buckets `unknown`, warning band `unknown`.

**Rationale**: Honest about confidence. Conservative: an `unknown` band is acceptable; a misleading warning is not.

**Alternatives considered**:
- *Assume exposure equals max-observed-call-count globally*: rejected — produces utilization ratios near 100% for everyone, which masks the actual bloat signal.
- *Require explicit header always*: rejected — too many real transcripts won't have one. Mission would emit `unknown` for the majority of users, defeating the product goal.

## R-3: Warning band threshold values

**Question**: What concrete numeric thresholds gate each warning band?

**Decision**: Pinned in `plan.md` §D-4. Locked by fixtures in Phase 1. Summary:
- `normal` whenever utilization is meaningful (≥40% MCP, ≥30% skill) OR count bucket is low (≤4-10). Count alone never triggers anything above `normal`.
- `watch` requires moderate exposure (`11-25` or `26-50`) AND low utilization (<40% MCP, <30% skill) AND no degradation signals.
- `high` requires significant exposure (`11-25`+) AND very low utilization (<20% MCP, <15% skill) AND high context footprint OR many tools.
- `severe` is `high` plus at least one observable degradation signal (Rereads≥3 OR RetryDepthMax≥3 OR ContextGrowthEvents≥2 — the same thresholds already used elsewhere in `buildFindings`).

**Rationale**: Reusing the existing degradation thresholds keeps the analyzer internally consistent. The utilization gates are aggressive enough that a meaningfully-used setup is never flagged, but loose enough to catch genuine bloat. Fixtures lock the boundaries so future contributors can see exactly what each band means.

**Alternatives considered**:
- *Hard numeric thresholds on exact counts (e.g., "≥30 servers triggers severe")*: rejected — bands must not fire on count alone (FR-005 hard constraint).
- *Continuous score 0–100*: rejected — buckets and named bands are the report's existing pattern and are easier to reason about than a score with no labels.

## R-4: How does remediation reach the user-facing report?

**Question**: Where do the new remediation strings get appended?

**Decision**: Add new `Finding` entries through the existing `buildFindings` pipeline (`internal/analyzer/analyzer.go:303`). Their `Recommendation` strings flow into `Report.ImmediateFixes` via the existing `immediateFixes` helper (`analyzer.go:387`).

**New finding IDs**:
- `mcp_bloat_high`, `mcp_bloat_severe`
- `skill_bloat_high`, `skill_bloat_severe`

**Recommendation strings** (fixed deterministic set, drawn from spec FR-006):
- "Disable unused MCP servers by default."
- "Scope project-specific MCPs to project config instead of global config."
- "Prefer narrower MCP servers over all-tools-enabled setups."
- "Lazy-load heavy MCP servers only when needed."
- "Split general skills from project-specific skills."
- "Move rarely used instructions out of always-loaded skill context."
- "Keep only high-signal skills in the default agent context."

Each finding selects 1–3 strings from this list based on the specific band+sub-object.

**Rationale**: Reuses the existing pipeline that already turns findings into immediate fixes. Bands → findings → fixes is a single deterministic chain that's easy to test.

**Alternatives considered**:
- *Append directly to `Report.ImmediateFixes`*: rejected — bypasses the findings model and makes traceability harder.
- *New top-level `Remediation` field*: rejected — out of scope per spec, and unnecessary if the existing `ImmediateFixes` channel works.

## R-5: Is the existing allowlist sufficient for v1?

**Question**: Do we need to expand `internal/analyzer/signatures/{mcp_servers,skills}.json` to land this mission?

**Decision**: No. The existing allowlists are consumed as-is. Fixtures will use:
- a small set of synthetic IDs that match existing allowlist entries (to exercise the "known" path);
- a larger set of synthetic *private* names (clearly not in any allowlist) to exercise the "unknown — count only" path;
- a privacy-leak corpus mixing schema text, private skill text, repo paths, and secrets to validate NFR-004.

Expanding the allowlists is its own concern (separate signature-research mission); it does not block this work.

**Rationale**: Keeping mission scope tight. Allowlist research is already its own track per the brief (#38 is the SDD registry; signature research has its own script at `scripts/research-signatures.sh`).

**Alternatives considered**:
- *Add 10–20 new public MCP IDs to round out coverage*: rejected — separable, would inflate this PR, and risks merge conflicts with #38.

## Cross-cutting concerns confirmed (not standalone unknowns)

- **Determinism**: enforced by sorting all slice outputs (`sortedKeys` already used in `ecosystem.go:146`), using closed enum strings for buckets/bands, and avoiding map iteration order in serialization. No goroutines added.
- **Aggregate-safety**: `AggregateSafeEvent.Ecosystem = report.Ecosystem` (analyzer.go:451) means nesting auto-propagates. Privacy tests must lock this end-to-end by serializing `AggregateSafeEvent` and asserting zero private-string occurrences.
- **#38 merge-friendliness**: all new code lives in `tooling.go`. `ecosystem.go` gets a small wiring change. `types.go` adds new fields to `Ecosystem`. No existing fields are renamed or removed.
