# Spec: Token-Saving Recommendation Engine (Phase A)

| Field | Value |
| --- | --- |
| Mission slug | `token-saving-recommendation-engine-phase-a-01KRZKCJ` |
| Mission ID | `01KRZKCJN2VCSE6M6T29VHZS96` |
| Mission type | software-dev |
| Target branch | `main` |
| Work branch (planned) | `codex/token-recommendations-phase-a` (timestamp suffix if collision) |
| Source brief | `start-here.md` (workspace root) |
| Upstream issue | [robertDouglass/claude-log-analyzer#68](https://github.com/robertDouglass/claude-log-analyzer/issues/68) |
| Parallel epics | #38 fingerprint registry; #39 MCP/skill bloat utilization analytics |

## Purpose

Build the additive, deterministic, privacy-safe foundation of the token-saving
tool recommendation engine called for by issue #68. Phase A defines the
allowlist, state model, decision policy, synthetic tests, and remediation
documentation so the engine can later consume signals from the parallel #38 and
#39 epics without rework. Generic "install another tool" advice wastes user
tokens; the engine must diagnose the dominant waste pattern and recommend the
highest-impact next step while skipping tools already active.

## User Scenarios & Testing

### Primary scenario

An analyst (or a downstream paid-pack generator acting on the analyst's behalf)
has just finished an analyzer pass over a local log corpus and now has:

- a set of dominant waste **Signals** (e.g. `shell_output_bloat`,
  `repeated_file_reads`), and
- a **Tool-State Map** mapping each allowlisted tool ID to a `ToolState` value
  (`unknown` / `mentioned_low` / `installed_medium` / `configured_medium` /
  `active_high` / `rejected_medium`) for that user.

The analyst calls the recommendation engine with those two inputs. The engine
returns at most one **primary** and one **secondary** `TokenSavingRecommendation`,
each pointing at an allowlisted tool ID, with a reason enum, signal enum list,
confidence enum, risk-level enum, install-policy enum, and bounded evidence
counts. Recommendations skip any tool whose state for the relevant signal is
`active_high`, and never stack untrusted shell/proxy/MCP tools without a waiver
gate.

### Acceptance scenarios (synthetic-input tests)

| # | Inputs | Expected outcome |
| --- | --- | --- |
| AS-01 | No tools active, signal: `no_usage_visibility` | Recommend `ccusage` (primary), no secondary |
| AS-02 | `ccusage` active + server-quota mismatch evidence | Recommend server-quota visibility / local-vs-server divergence note; do not re-recommend `ccusage` |
| AS-03 | `shell_output_bloat`, RTK absent | Recommend `rtk` |
| AS-04 | `shell_output_bloat`, RTK `installed_medium` or `configured_medium` | Recommend RTK activation/config audit, not another tool |
| AS-05 | `shell_output_bloat` + persistent rereads, RTK `active_high` | Skip RTK; recommend `leanctx` (and optionally `headroom` as secondary) |
| AS-06 | `shell_output_bloat`, RTK `rejected_medium` | Skip RTK; recommend `leanctx` with risk notes |
| AS-07 | `mcp_tool_output_bloat`, context_mode absent | Recommend `context_mode` |
| AS-08 | `mcp_tool_output_bloat`, context_mode `configured_medium` | Recommend usage/activation check, not another tool |
| AS-09 | `mcp_tool_output_bloat`, context_mode `active_high` + bloat persists | Recommend `distill` or a narrower low-token tool as next complementary tool |
| AS-10 | `repeated_file_reads`, Serena `active_high` | Do not recommend Serena again; prefer the next retrieval-tier tool only if waste persists in a different retrieval mode |
| AS-11 | `mcp_skill_bloat` dominant | Recommend MCP/skill pruning/lazy-loading/scoping; do **not** add another MCP |
| AS-12 | `output_verbosity` only | Recommend `claude_token_efficient`; `caveman` only as opt-in |
| AS-13 | Multiple severe signals | At most one primary + one secondary; prefer input/context reductions over output-style tweaks |
| AS-14 | Privacy probe: tool-state map carries no private names; recommendation output for any synthetic input | No raw/private string appears in the marshalled recommendation JSON; only allowlisted enums + bounded counts |

### Edge cases

- All signals empty / no waste detected ⇒ engine returns an empty
  recommendation list and a deterministic "no-op" reason enum.
- Tool ID present in input state map but not in registry ⇒ engine ignores the
  unknown ID (counts only) and emits no recommendation referencing it.
- Conflicting states for the same tool (e.g. `configured_medium` and
  `rejected_medium` from different evidence sources) ⇒ engine resolves
  deterministically by a documented precedence order (higher-trust evidence
  wins; tie-breaker is `rejected_medium > active_high > configured_medium >
  installed_medium > mentioned_low > unknown`).
- Two distinct rules would recommend the same tool ⇒ engine emits the tool
  once, with the union of `signal_ids` attached and the highest applicable
  confidence.

## Domain Language

| Canonical term | Meaning |
| --- | --- |
| **Token-saving tool** | Allowlisted third-party tool whose adoption is expected to reduce token usage or context bloat. |
| **Allowlist** | Versioned registry of public tool IDs the engine is permitted to recommend. Anything not in the allowlist is never emitted. |
| **ToolState** | Per-user state of a tool: `unknown` / `mentioned_low` / `installed_medium` / `configured_medium` / `active_high` / `rejected_medium`. |
| **Signal** | Enum identifier for a dominant waste pattern detected by the analyzer (`shell_output_bloat`, `repeated_file_reads`, …). |
| **Evidence source** | Enum identifier for *how* a tool's state was inferred (`cli_presence`, `mcp_active`, `report_mention`, `failure_or_rejection`, …). |
| **Recommendation class** | Static class (e.g. `usage_visibility`, `shell_output_reducer`, `mcp_output_reducer`, `retrieval`, `reread_guard`, `output_verbosity`, `mcp_skill_hygiene`). |
| **Class rank** | Integer ordering within a class, used for deterministic primary/secondary selection. |
| **Install policy** | Enum: `bundle` / `recommend` / `recommend_with_waiver` / `research_only` / `reference_only`. |
| **Risk level** | Enum: `low` / `medium` / `high` covering install/data-movement risk surface. |
| **Waiver gate** | Explicit user confirmation required before recommending tools with `high` risk or that rewrite shell/proxy/MCP behavior. |

Ambiguous synonyms to avoid in code and docs:

- "installed" alone (use the explicit `ToolState`) ; "mentioned" alone (always
  `mentioned_low`); "MCP plugin" (prefer "MCP" or "skill" per evidence source);
  "rejection" (use `rejected_medium` or the `failure_or_rejection` evidence
  source).

## Functional Requirements

| ID | Requirement | Status |
| --- | --- | --- |
| FR-001 | Phase A ships a versioned `TokenSavingTool` registry whose IDs are the **union** of the brief's allowlist and the entries already documented in `docs/remediation/token-saving-tooling-matrix.md`, deduped by canonical lowercase-underscored ID. Reference-only and research-only entries are flagged via `install_policy` / `research_only`. | proposed |
| FR-002 | Each registry entry carries: `id`, `display_name`, `source_url`, `category`, `recommendation_class`, `class_rank`, `detector_sources[]`, `install_risk`, `data_movement_risk`, `rollback_guidance`, `free_report_allowed`, `paid_pack_allowed`, `research_only`. Strings come from static enums where applicable. | proposed |
| FR-003 | Public lookup API: `GetTool(id) (TokenSavingTool, bool)`, `AllTools() []TokenSavingTool`, `RegistryVersion() string`. No mutation API. | proposed |
| FR-004 | `ToolState` enum: `unknown`, `mentioned_low`, `installed_medium`, `configured_medium`, `active_high`, `rejected_medium`. Each value's semantics match the brief. | proposed |
| FR-005 | Evidence-source enum: `cli_presence`, `cli_version`, `log_active_command`, `mcp_configured`, `mcp_active`, `plugin_configured`, `skill_configured`, `hook_configured`, `statusline_configured`, `report_mention`, `failure_or_rejection`. | proposed |
| FR-006 | Signal enum: `no_usage_visibility`, `tool_output_bloat`, `shell_output_bloat`, `mcp_tool_output_bloat`, `repeated_file_reads`, `broad_repo_exploration`, `unchanged_file_rereads`, `mcp_skill_bloat`, `output_verbosity`, `retry_loop`, `context_growth_spikes`. | proposed |
| FR-007 | Output struct `TokenSavingRecommendation` carries `recommendation_id`, `primary_tool_id`, `skipped_tool_ids[]`, `reason` (enum), `signal_ids[]` (enum), `confidence` (enum), `risk_level` (enum), `install_policy` (enum), `evidence_counts` (map of static evidence-source enum key → int). | proposed |
| FR-008 | Recommendation engine is a pure deterministic function of `(signals, tool_state_map)` — no time, no RNG, no I/O, no environment reads. | proposed |
| FR-009 | Rule **no_usage_visibility** ⇒ recommend `ccusage`; if `ccusage` is `active_high` and `server_quota_mismatch` evidence is supplied, recommend server-quota visibility / local-vs-server divergence note instead. | proposed |
| FR-010 | Rule **shell_output_bloat** ⇒ RTK state machine: absent → `rtk`; installed/configured → activation/config audit; active + persistent → `leanctx` (and optionally `headroom`); rejected → skip RTK + `leanctx` with risk notes. | proposed |
| FR-011 | Rule **mcp_tool_output_bloat** ⇒ context_mode state machine: absent → `context_mode`; configured-unused → activation; active + persistent bloat → `distill` or a narrower low-token tool. | proposed |
| FR-012 | Rule **repeated_file_reads** / **broad_repo_exploration** ⇒ retrieval tier: prefer `serena` / official code-intelligence first; large-repo graph-style ⇒ `codegraph` or `codebase_memory_mcp`; local semantic search ⇒ `semble` or `grepai`. No stacking unless waste persists in a different retrieval mode. | proposed |
| FR-013 | Rule **unchanged_file_rereads** ⇒ `read_once` or `leanctx`; if `openwolf` / `leanctx` / `read_once` already active and rereads persist ⇒ recommend configuration audit, not another tool. | proposed |
| FR-014 | Rule **mcp_skill_bloat** ⇒ recommend pruning / lazy-loading / scoping. **Never** add another MCP by default. | proposed |
| FR-015 | Rule **output_verbosity** only ⇒ recommend `claude_token_efficient`. `caveman` only as an opt-in style profile, not a default. | proposed |
| FR-016 | Multi-severe-signal arbitration: emit at most one primary and one secondary `TokenSavingRecommendation`. Prefer input/context-token reductions over output-style tweaks. Never stack untrusted shell / proxy / MCP tools without the `recommend_with_waiver` install policy. | proposed |
| FR-017 | Engine MUST skip any tool whose state for the relevant signal is `active_high` and record it in `skipped_tool_ids[]` with the relevant reason enum. | proposed |
| FR-018 | Engine resolves conflicting per-tool states deterministically using the documented precedence order (see Edge Cases). | proposed |
| FR-019 | Synthetic-input test suite exercises every acceptance scenario AS-01 through AS-14 with table-driven Go tests. | proposed |
| FR-020 | Privacy test marshals recommendation output for a representative set of inputs and asserts: only allowlisted enum strings and bounded integer counts appear; no raw user data, private tool name, file path, branch, host, repo URL, session ID, or version string is present. | proposed |
| FR-021 | Update `docs/remediation/token-saving-tooling-matrix.md` and `docs/remediation/plugin-artifacts.md` to describe the registry, dedupe-aware recommendation contract, and state-vs-active distinctions. Additive edits only. | proposed |
| FR-022 | Add new doc `docs/remediation/token-saving-recommendation-engine.md` covering token-saving tool classes, allowlist policy, dedupe contract, state model, risk levels, install policy, waiver requirement, privacy constraints, and the Phase B wire-up plan for #38/#39. | proposed |

## Non-Functional Requirements

| ID | Requirement | Measure / Threshold | Status |
| --- | --- | --- | --- |
| NFR-001 | Determinism | For any identical `(signals, tool_state_map)` input, the engine MUST produce byte-identical marshalled recommendation JSON across runs and across machines. Verified by a Go test that runs the engine twice and `bytes.Equal`s the output. | proposed |
| NFR-002 | Privacy budget | Zero raw/private strings in any engine output. Verified by FR-020's privacy test scanning for any non-allowlisted token in the marshalled JSON. | proposed |
| NFR-003 | Additivity | Phase A introduces no broad rewrites of `internal/analyzer/types.go` or `internal/analyzer/ecosystem.go`; pre-existing `go test ./...` stays green. Verified by running the existing suite plus new tests with no edits to those files beyond additive struct fields if any. | proposed |
| NFR-004 | Hermetic tests | `go test ./...` runs with no network access, no env reads, no filesystem writes outside Go's test workdir. | proposed |
| NFR-005 | Registry versioning | `RegistryVersion()` returns a monotonically increasing identifier (e.g. `"phase-a-2026-05-19"` or integer build counter); changing any registry entry without bumping the version is a test failure. | proposed |
| NFR-006 | Recommendation latency | Engine completes one call in < 1 ms on a developer laptop for synthetic inputs (≤ 50 tools × ≤ 11 signals). | proposed |

## Constraints

| ID | Constraint | Status |
| --- | --- | --- |
| C-001 | No CLI / binary probing in Phase A — defer to #67. Allowlist metadata only (binary name, safe version command, version parser policy, `cli_probing_disabled` flag). | proposed |
| C-002 | Do not finalize report JSON shape that should consume #38/#39 outputs. The recommendation struct must be embeddable without forcing schema changes upstream. | proposed |
| C-003 | Do not force-merge with #38 fingerprint structs or #39 utilization structs; Phase A talks only to its own synthetic `ToolStateMap` type. | proposed |
| C-004 | Work branch is `codex/token-recommendations-phase-a`. If it already exists, append a UTC timestamp suffix (e.g. `-20260519T0749Z`). | proposed |
| C-005 | All recommendation output keys and values are static enum strings; maps may only be keyed by static enum keys, never by raw log data. | proposed |
| C-006 | At most two recommendations per engine call (one primary + one secondary). | proposed |
| C-007 | A start comment must be posted on issue #68 when Phase A branch work begins and a completion comment when Phase A is review-ready (with files changed, tests run, and what remains for Phase B). Issue #68 MUST NOT be closed until Phase B is also complete. | proposed |
| C-008 | `gofmt -w` + `go test ./...` must pass before the branch is pushed; smoke (`./scripts/smoke-local.sh`) is run when changes affect report generation or paid artifacts, and documented as skipped if not applicable. | proposed |
| C-009 | Do not alter paid plugin generation deeply. Additive consumption of the new recommendation object is allowed only if it does not break current tests. | proposed |
| C-010 | Tools that rewrite shell commands, proxy execution, or move data off-device must carry `install_policy = recommend_with_waiver` and a non-empty `rollback_guidance`. | proposed |

## Success Criteria

- **SC-001** A reviewer can read the new doc + tests and answer, in under five
  minutes, "what does the engine recommend for `shell_output_bloat` when RTK is
  already active?" — and the answer matches the engine's output for an
  equivalent synthetic input.
- **SC-002** For every acceptance scenario AS-01 through AS-14, the engine's
  output matches the row's expected outcome, and `go test ./...` is green.
- **SC-003** Privacy assertion (FR-020 / NFR-002) detects any future regression
  that leaks raw user data into recommendation JSON; CI fails fast on such a
  regression.
- **SC-004** A maintainer can add or remove an allowlist entry by editing only
  the registry file, bumping `RegistryVersion()`, and adding a row to the
  matrix doc — no engine code changes required for additive entries.
- **SC-005** Phase B can wire #38 fingerprint outputs and #39 utilization
  signals into the engine by populating `ToolStateMap` and `signals` only, with
  no edits to Phase A code beyond additive registry entries.

## Key Entities

- **`TokenSavingTool`** (registry entry) — see FR-002.
- **`ToolState`** (enum) — see FR-004.
- **`EvidenceSource`** (enum) — see FR-005.
- **`Signal`** (enum) — see FR-006.
- **`ToolStateMap`** — `map[ToolID]ToolStateEntry`, where `ToolStateEntry`
  carries `State`, the `EvidenceSource[]` that produced it, and a bounded
  per-source count. **No raw strings from logs.**
- **`TokenSavingRecommendation`** (engine output) — see FR-007.
- **`RecommendationSet`** — `{ Primary *Rec, Secondary *Rec, Skipped []Skip }`
  enforcing the ≤ 1 + ≤ 1 invariant from C-006.
- **`RegistryVersion`** — string identifier of the loaded registry snapshot.

## Assumptions

- The brief (`start-here.md`) is authoritative where it conflicts with the
  existing matrix doc; matrix-doc tools missing from the brief are folded in as
  `reference_only` or `research_only`.
- Public source URLs for each tool either already exist in the matrix doc or
  the brief; Phase A does **not** invent or guess URLs — any tool whose public
  URL cannot be verified during implementation is marked `research_only` with
  `source_url` empty and a `notes` field explaining the gap.
- The eventual upstream signal producer (#38/#39) will deliver allowlist-safe
  inputs; the engine still defensively counts and discards anything outside the
  registry rather than echoing it.
- The deterministic precedence order documented in *Edge Cases* is sufficient
  for Phase A's synthetic tests; richer evidence-merging policy is a Phase B
  concern.

## Scope

### In scope (Phase A)

- New additive Go files (registry, recommendations, tests) under
  `internal/analyzer/`.
- New remediation doc and additive edits to two existing remediation docs.
- Synthetic-input table-driven tests, including the privacy test.
- Branch creation, two GitHub-issue comments on #68 (start + completion).

### Out of scope (deferred to Phase B or other epics)

- Reading real analyzer logs into the engine; the engine consumes synthetic
  signals only in Phase A.
- Wiring #38 fingerprint structs or #39 utilization structs.
- Finalizing the report JSON shape that embeds recommendations.
- CLI / binary probing (defer to #67).
- Closing issue #68.
- Aggregate ecosystem intelligence (#40), report UX (#41), confidence scoring
  (#49), paid MCP/skill profile (#64) — referenced but not implemented.
