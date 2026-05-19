# Specification: MCP and Skill Bloat Analytics

**Mission ID**: 01KRZJGVG3MCCCY9MKB1YRDBQR
**Slug**: mcp-skill-bloat-analytics-01KRZJGV
**Mission type**: software-dev
**Target branch**: main
**Created**: 2026-05-19
**Source**: GitHub Epic #39 (parent), required children #51–#57. Brief: `../start-here.md`.

## Purpose

**TLDR**: Quantify whether exposed MCPs and skills are actually used, with deterministic warnings and privacy-safe aggregates.

Some Claude Code and Codex users expose many MCP servers, skills, plugins, and instructions to the agent context but execute only a small fraction of them. The unused surface creates hidden token overhead, slower tool choice, instruction conflict, noisy context, and degraded session behavior — and users have no deterministic way to see this in their own logs.

This mission extends the existing analyzer report with a `tooling_utilization` section that answers: **"Are the user's MCPs and skills actually being used enough to justify the context they add?"** It does so without ingesting, storing, logging, or uploading private MCP/skill names, schemas, prompts, paths, URLs, args, or skill text.

## Out of Scope

The following are explicitly **out of scope** for this mission (tracked as separate downstream work per user confirmation):

- **#41** Report UX for ecosystem fingerprints and tooling bloat.
- **#58** Upload schema additions for bounded ecosystem aggregate fields.
- **#65** CI privacy and bounded-cardinality gates for the new fields.
- **#38** Top-20 SDD fingerprint registry. This mission must remain additive and merge-friendly with #38 but does not modify its types.
- Network upload behavior changes (this mission only changes what the report contains; it does not change transports).

## User Scenarios & Testing

### Primary Actor

A developer who runs the `claude-log-analyzer` CLI against one or more session transcripts and reads the resulting report (JSON and/or rendered output).

### Primary Scenario (happy path)

1. The user has been working with Claude Code (or Codex) for several sessions and has installed many MCP servers and skills.
2. The user runs the analyzer on their transcript export.
3. The analyzer emits a report containing the existing `ecosystem` block **plus** a new `tooling_utilization` section with MCP and skill sub-objects.
4. For each of MCP and skill, the report shows: known public IDs (from the allowlist), an unknown-count, exposure buckets (server count, exposed tool count, exposed skill count, context-token footprint), executed-call counts, integer utilization ratio (%), context-efficiency bucket, and a warning band (`normal`/`watch`/`high`/`severe`/`unknown`).
5. When the warning band is `high` or `severe`, the report's `immediate_fixes` list includes one or more deterministic remediation strings (e.g., "Disable unused MCP servers by default.").
6. Aggregate report JSON contains **zero** private names, schema text, skill text, paths, URLs, args, or hashes of any private string.

### Exception / Edge Cases

- **Exposure cannot be observed.** When neither the transcript nor any structured manifest reveals the count of exposed MCPs or skills, exposure buckets are `unknown`, `exposure_known` is `false`, and the warning band is `unknown` regardless of usage. The report must not warn on usage alone when exposure is unknown.
- **Zero exposure, zero usage.** Report emits `none` buckets, `0` ratios with `exposure_known=true`, and warning band `normal`.
- **High exposure, high usage.** Report emits high buckets, high ratio, and warning band `normal` — high count alone never triggers a warning.
- **High exposure, near-zero usage, no degradation signals.** Warning band is `high`.
- **High exposure, near-zero usage, with degradation signals (rereads/retries/context growth above threshold).** Warning band is `severe`.
- **Mix of known public and unknown private.** Known IDs from the allowlist are listed by ID; unknown items are counted but never named.
- **Slash command on a file path** (e.g., a quoted `"/etc/passwd"` substring). Existing path-avoidance behavior must be preserved; such substrings must not be counted as skill executions.
- **Schema text appears in the log.** Local analysis may count tool definitions to feed exposure buckets, but the upload (`AggregateSafeEvent`) must contain only bucket labels and known IDs, never the schema text or tool names.

### Acceptance Scenarios

- AS-1: With a fixture containing no MCP and no skill signals, `tooling_utilization.mcp` and `tooling_utilization.skill` emit `none` buckets and `unknown` exposure with band `unknown` (no exposure observed) or `normal` (exposure confirmed zero by a structured signal) — fixtures pin the expected band.
- AS-2: With a fixture containing 1 known MCP, 1 known skill, and matching usage, both bands are `normal` and known IDs appear in `known_*` arrays.
- AS-3: With a fixture containing many MCP server names and tool calls covering most of them, the band is `normal` despite high count.
- AS-4: With a fixture containing many private MCPs and zero calls, the band is `high` (or `severe` with degradation), the report includes remediation strings, and no private names appear in the report JSON.
- AS-5: With a fixture containing private skills and no executions, the skill band is `high`, the report includes skill remediation strings, and no skill text or private names appear.
- AS-6: With a fixture mixing known public allowlisted IDs and unknown private entries, known IDs appear as IDs and unknown entries appear only as counts.
- AS-7: With a transcript containing strings that look like slash commands but are file paths, none of those strings are counted as skill executions.

## Domain Language

Canonical terms used throughout the report and code (the synonyms in parentheses must be avoided):

- **MCP server** (not "MCP plugin", not "MCP integration"): a Model Context Protocol server exposing tools to the agent.
- **MCP tool**: an individual callable function exposed by an MCP server.
- **Skill** (not "command", not "macro"): an agent-context skill or slash command surface.
- **Exposure**: tools/skills available to the agent in context.
- **Execution / call**: actual invocation observed in the transcript.
- **Utilization ratio**: integer percentage of exposure that was actually used.
- **Warning band**: one of `normal`/`watch`/`high`/`severe`/`unknown` — never a numeric score.
- **Allowlist**: the curated set of public MCP/skill IDs that may appear by name in reports. All other IDs are unknown and counted only.
- **Bucket**: a label from a fixed, low-cardinality set used in place of exact counts in aggregate output.

## Functional Requirements

| ID | Status | Requirement |
|----|--------|-------------|
| FR-001 | proposed | The analyzer SHALL compute MCP inventory metrics: `known_server_ids` (sorted allowlist IDs found), `unknown_server_count` (integer), `server_count_bucket`, `exposed_tool_count_bucket`, `context_token_bucket`, and `exposure_known` (boolean indicating whether exposure was directly observed). |
| FR-002 | proposed | The analyzer SHALL compute MCP usage metrics: `call_count`, `known_call_count`, `unknown_call_count`, `unique_known_called_ids` (sorted), `unique_unknown_called_count`, `utilization_ratio_pct` (integer 0–100 or 0 with `exposure_known=false`), and `context_efficiency_bucket`. |
| FR-003 | proposed | The analyzer SHALL compute skill inventory metrics: `known_exposed_ids` (sorted allowlist IDs found), `unknown_exposed_count`, `exposed_count_bucket`, `context_token_bucket`, and `exposure_known` (boolean). |
| FR-004 | proposed | The analyzer SHALL compute skill usage metrics: `executed_count`, `known_executed_ids` (sorted), `unknown_executed_count`, `utilization_ratio_pct`, and `context_efficiency_bucket`. |
| FR-005 | proposed | The analyzer SHALL assign a deterministic `warning_band` (`normal`/`watch`/`high`/`severe`/`unknown`) to each of MCP and skill, computed only from documented thresholds combining count bucket, footprint bucket, utilization ratio, and degradation signals (rereads, retry depth, context growth). A high count alone MUST NOT trigger any band above `normal`. When `exposure_known` is false, the band MUST be `unknown`. |
| FR-006 | proposed | When a band is `high` or `severe`, the analyzer SHALL append deterministic remediation strings (drawn from a fixed set covering: disable unused MCP servers, scope project-specific MCPs to projects, prefer narrower servers, lazy-load heavy servers, split general from project-specific skills, move rarely used instructions out of always-loaded context, keep only high-signal skills in defaults) to the report's `immediate_fixes`. No remediation string MAY contain private names, paths, schema text, or skill content. |
| FR-007 | proposed | Existing slash-command path-avoidance behavior in skill detection SHALL be preserved and extended; tests covering quoted file paths, code-fenced paths, and URL-like paths SHALL continue to assert zero false-positive skill executions. |
| FR-008 | proposed | The analyzer SHALL emit a `tooling_utilization` object (containing `mcp` and `skill` sub-objects) in both `Report` and `AggregateSafeEvent` (the upload-safe shape). The aggregate event MUST NOT contain any field whose value is a private name, raw path, URL, hash of a private string, or any free-form text derived from user content. |
| FR-009 | proposed | The mission SHALL provide synthetic golden fixtures covering at minimum: (a) no MCPs and no skills, (b) small healthy setup, (c) many MCPs with high utilization, (d) many MCPs with near-zero utilization, (e) many skills with near-zero execution, (f) unknown private MCPs/skills only, (g) mixed known public + unknown private. Each fixture pins expected bucket labels, ratios, and warning bands. |
| FR-010 | proposed | Existing `Ecosystem` fields `MCPServersKnown`, `UnknownMCPServerCount`, `KnownSkills`, `UnknownSkillCount`, `KnownPlugins`, `UnknownPluginCount` SHALL be preserved with their current JSON keys and semantics. New types SHALL be additive. |
| FR-011 | proposed | The report's existing immediate-fixes generation SHALL be extended additively; existing fix strings MUST continue to fire under their existing conditions. New strings MUST be deterministic and gated on the new warning bands. |
| FR-012 | proposed | The mission SHALL update `docs/ecosystem-signatures.md`, `docs/data-retention-and-analytics.md`, `docs/logging-policy.md`, and `docs/testing-plan.md` to describe what MCP/skill utilization means, how context footprint is estimated, what each bucket means, when each warning band fires, why unknown private names are counts only, and how this differs from #38 fingerprinting. New docs (if added) SHALL be cross-linked from existing docs. |

## Non-Functional Requirements

| ID | Status | Requirement | Threshold |
|----|--------|-------------|-----------|
| NFR-001 | proposed | Output determinism: the same input SHALL produce byte-identical `tooling_utilization` JSON across runs. | 100% identical bytes across 10 consecutive runs on the same fixture. |
| NFR-002 | proposed | Test suite passes. | `go test ./...` exits 0 on the implementation branch. |
| NFR-003 | proposed | Lint/format cleanliness. | `gofmt -l` on all changed `.go` files yields zero entries. |
| NFR-004 | proposed | Privacy leak rate. | A privacy test corpus (private MCP/tool/skill names, schema text, args with paths/secrets, file-path-like slash commands) SHALL produce reports whose serialized JSON contains zero substring matches for any private name or content. Zero is the only acceptable value. |
| NFR-005 | proposed | Smoke test. | `./scripts/smoke-local.sh` SHALL be run on the implementation branch; if blocked, the blocker SHALL be documented in the PR description with exact error output. |
| NFR-006 | proposed | Aggregate bounded cardinality. | Every string field in `AggregateSafeEvent.tooling_utilization` SHALL come from a closed enumeration (bucket labels, band labels, allowlist IDs). No free-form strings. |
| NFR-007 | proposed | Backward compatibility. | All existing analyzer test cases in `internal/analyzer/analyzer_test.go` and `internal/analyzer/golden_test.go` continue to pass without modification, except where existing assertions explicitly check the shape of newly added fields. |

## Constraints

| ID | Status | Constraint |
|----|--------|-----------|
| C-001 | proposed | Aggregate output (report JSON + `AggregateSafeEvent`) MUST NEVER contain any of: user prompts, task descriptions, raw transcript excerpts, raw tool inputs/outputs, raw MCP schemas/descriptions/arguments, MCP server URLs, auth scopes, private MCP/tool names, private skill names, skill instruction text, skill examples, user-authored skill docs, raw file paths, repo URLs, branch names, usernames, hostnames, emails, session IDs, transcript paths, or stable hashes of any private string. |
| C-002 | proposed | Unknown items (MCPs, tools, skills) MUST be counted only. Their names MUST NOT be stored, logged, or emitted in any part of the report or aggregate event. |
| C-003 | proposed | Count buckets MUST use the fixed enumeration: `none`, `1-3`, `4-10`, `11-25`, `26-50`, `51-100`, `100+`, `unknown`. Context-token buckets MUST use the fixed enumeration: `none`, `<1k`, `1k-5k`, `5k-15k`, `15k-50k`, `50k+`, `unknown`. Warning bands MUST use the fixed enumeration: `normal`, `watch`, `high`, `severe`, `unknown`. |
| C-004 | proposed | New types MUST be additive. Existing `Ecosystem` fields and their JSON keys MUST be preserved. No rename of `MCPServersKnown`, `UnknownMCPServerCount`, `KnownSkills`, `UnknownSkillCount`, `KnownPlugins`, `UnknownPluginCount`. |
| C-005 | proposed | Implementation branch MUST be `codex/mcp-skill-utilization` (with a timestamp suffix if the branch already exists). |
| C-006 | proposed | Utilization ratios MUST be integer percentages. When the denominator (exposure) is unknown or zero, the ratio MUST be `0` and `exposure_known` MUST be `false`; in that case the warning band MUST be `unknown` and no usage-based warning may fire. |
| C-007 | proposed | The mission MUST NOT modify #38's SDD fingerprint registry types. It MAY consume the existing public MCP/skill allowlists in `internal/analyzer/signatures/`. |
| C-008 | proposed | All fixtures used for tests MUST be synthetic (no real user logs). |

## Success Criteria

Measurable, technology-agnostic outcomes that signal mission success:

- SC-1: A user running the analyzer on a transcript with many MCPs and few calls sees a `high` or `severe` warning band and at least one concrete remediation string in the report within the same single CLI invocation that produces the existing report.
- SC-2: Across every privacy test in the suite, the number of private names, paths, schemas, args, URLs, or skill-text strings that appear in the report JSON is exactly zero.
- SC-3: Across every synthetic fixture, the warning band emitted for that fixture matches the band the fixture pins (100% deterministic match on repeated runs).
- SC-4: A user whose exposure cannot be determined sees `unknown` exposure buckets and an `unknown` warning band — never a usage-based warning, even if their executed call count is zero.
- SC-5: A user with high exposure and high utilization sees `normal` warning bands — high count alone never produces a warning.
- SC-6: The new fields are documented in `docs/ecosystem-signatures.md` and `docs/data-retention-and-analytics.md` such that a maintainer reading only those docs can answer (a) what each bucket means, (b) when each band fires, (c) why unknown names are counts only, (d) how this differs from #38.

## Key Entities

- **Report** (existing). Extended additively with a `tooling_utilization` field.
- **Ecosystem** (existing). Untouched in its existing fields; the new `tooling_utilization` lives alongside it (either as a sibling field on `Report` or nested inside `Ecosystem` — placement chosen during planning to minimize conflict with #38).
- **ToolingUtilization** (new). Container with `mcp: MCPUtilization` and `skill: SkillUtilization` sub-objects.
- **MCPUtilization** (new). Holds known IDs, unknown count, count/footprint/efficiency buckets, call counts, utilization ratio %, `exposure_known` flag, and `warning_band`.
- **SkillUtilization** (new). Holds known exposed IDs, unknown count, count/footprint/efficiency buckets, executed counts, utilization ratio %, `exposure_known` flag, and `warning_band`.
- **Public Allowlist** (existing, in `internal/analyzer/signatures/`). Source of truth for which MCP/skill IDs may appear by name; everything else is counted unnamed.
- **AggregateSafeEvent** (existing). Receives a copy of `tooling_utilization` whose only string values come from closed enumerations (bucket labels, band labels, allowlist IDs).
- **Fixtures** (new). Synthetic transcripts covering zero, healthy, high-util, low-util, unknown-private, and mixed-known-plus-unknown scenarios.

## Dependencies & Assumptions

### Dependencies

- Existing `internal/analyzer` package (`ecosystem.go`, `types.go`, `analyzer.go`, `scrubber.go`, `aggregate.go`).
- Existing public allowlists at `internal/analyzer/signatures/{mcp_servers,skills,plugins}.json`.
- Existing test infrastructure: `analyzer_test.go`, `golden_test.go`, `test_helpers_test.go`.
- Existing docs to update: `docs/ecosystem-signatures.md`, `docs/data-retention-and-analytics.md`, `docs/logging-policy.md`, `docs/testing-plan.md`.

### Assumptions

- The user has Go installed and can run `go test ./...` locally and in CI.
- The existing parsed-line abstraction (`parsedLine`) exposes enough structure to identify tool-use blocks and slash-command-shaped tokens; if it does not, the planning phase will scope the parser extensions needed.
- The existing allowlists are sufficient for v1. Adding new public IDs to the allowlists is out of scope for this mission unless required to keep an existing test fixture passing.
- Context-token footprint is *estimated* from observable signals (number of exposed servers/tools × a fixed per-tool estimate, or directly counted from schema text length when present in the log) and is then bucketed — the exact estimator is a planning decision, but the bucket labels in C-003 are fixed.
- Exact threshold values for each warning band (e.g., "11-25 servers + utilization < 20% → high") are deterministic constants chosen during planning and locked by fixtures. They are not specified here as numeric tables; this spec only requires that the bands be deterministic, fixture-pinned, and never triggered by count alone.

## Definition of Done

This mission is ready for review when all of the following hold:

- [ ] MCP inventory emits known IDs, unknown counts, exposure buckets, and footprint buckets (FR-001, FR-008).
- [ ] MCP usage emits call counts, utilization ratio, context-efficiency bucket, and warning band (FR-002, FR-005).
- [ ] Skill inventory emits known IDs, unknown counts, exposure buckets, and footprint buckets (FR-003, FR-008).
- [ ] Skill usage emits execution counts, utilization ratio, context-efficiency bucket, and warning band (FR-004, FR-005).
- [ ] Warning bands are deterministic and do not fire on count alone (FR-005, AS-3, AS-5, SC-5).
- [ ] Remediation wording is specific, privacy-safe, and gated on band severity (FR-006, NFR-004).
- [ ] Unknown MCP/skill names never appear in aggregate JSON (C-001, C-002, NFR-004).
- [ ] Raw schemas, descriptions, args, skill text, paths, URLs, and prompts never appear in aggregate JSON (C-001, NFR-004).
- [ ] Synthetic fixtures cover zero, healthy, high-utilization, low-utilization, unknown-private, and mixed setups (FR-009).
- [ ] Docs explain the metrics and privacy model (FR-012, SC-6).
- [ ] `go test ./...` passes (NFR-002).
- [ ] `./scripts/smoke-local.sh` is run or a precise blocker is documented (NFR-005).
- [ ] Work is committed to branch `codex/mcp-skill-utilization` and pushed to GitHub (C-005).
- [ ] GitHub issue comments posted on #39 (start + complete) and on #51–#57 as each is implemented.
