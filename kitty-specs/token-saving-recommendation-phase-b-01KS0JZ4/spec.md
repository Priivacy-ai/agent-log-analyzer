# Spec: Token-Saving Recommendation Engine (Phase B Wiring)

| Field | Value |
| --- | --- |
| Mission slug | `token-saving-recommendation-phase-b-01KS0JZ4` |
| Mission ID | `01KS0JZ495XV0PCKSVBNDVAY16` |
| Mission type | software-dev |
| Target branch | `main` |
| Source brief | `start-here.md` (workspace root) |
| Upstream issue | [robertDouglass/claude-log-analyzer#73](https://github.com/robertDouglass/claude-log-analyzer/issues/73) |
| Depends on | Mission `token-saving-recommendation-engine-phase-a-01KRZKCJ` (engine, frozen contract); PR #76 report intelligence UX |

## Purpose

Phase A produced a deterministic, privacy-safe recommendation engine
(`Recommend(signals, state) -> RecommendationSet`) but the engine is not yet
consumed anywhere. Phase B turns that engine into visible product value: it
derives the engine's `Signal` inputs from real analyzer findings, builds the
engine's `ToolStateMap` from detected fingerprints and tooling utilization, calls
the engine once per `Report`, attaches the bounded output to the free `Report`
and the paid plugin artifact, and renders Primary + Secondary recommendations
in the web report. When the engine has nothing to recommend, the UI shows a
short "no action needed" note so users know the engine looked.

Phase B does not modify the engine signature, the engine's rule precedence, or
the registry. The engine's dedupe rules (active/effective tools are never
recommended) and prune-first rules (MCP/skill bloat → pruning recommendation,
not a new MCP) are honored because Phase B populates `ToolStateMap.State` and
the `signals` slice correctly. The contract Phase B owns is everything from
`Report` outward; the engine is a frozen dependency.

## User Scenarios & Testing

### Primary scenario

An engineer runs `claude-analyzer analyze` on their Claude Code logs. The
analyzer produces a `Report` containing ecosystem findings (utilization bands,
SDD/workflow fingerprints, finding IDs such as `tool_output_bloat`,
`repeated_file_reads`, `retry_loop`, `context_growth_spikes`). The analyzer
also calls the recommendation engine once with the derived signals and tool
state. The `Report` carries an additional `Recommendation` field containing the
`RecommendationSet` produced by the engine. The user inspects the local report
JSON, sees a single Primary recommendation (with optional Secondary), and
follows the deterministic explanation rendered in the web UI to take the next
action — or sees a "no action needed" note if their tooling is already in
shape.

If the user later runs the paid 100-log bundle path, the merged aggregate
report carries the same `RecommendationSet` (re-run on merged signals and tool
state), and the generated paid plugin artifact embeds the bounded JSON object
alongside its existing `VettedRecommendations` list.

### Acceptance scenarios (integration-level)

| # | Inputs | Expected outcome |
| --- | --- | --- |
| AS-01 | Report with finding `tool_output_bloat` and no known shell-output reducer in tool state | `Report.Recommendation.Primary.PrimaryToolID` points at the engine's emitted shell-output reducer; report UI renders the Primary card |
| AS-02 | Report with finding `repeated_file_reads` and Serena evidence `active_high` (via SDD fingerprint) | Engine output skips Serena; if no other retrieval tool fits, `Primary == nil` and the UI shows "no action needed" |
| AS-03 | Report with `Ecosystem.ToolingUtilization` MCP warning band `severe` | Recommendation is from class `mcp_skill_hygiene` (prune/lazy-load); no new MCP is recommended even when other bloat signals are present |
| AS-04 | Report with no findings and an empty utilization map | Engine returns no Primary and no Secondary; UI renders "no action needed" note; `RecommendationSet.RegistryVersion` and `EngineVersion` are still present on the report |
| AS-05 | Report with `WorkflowFingerprints` showing `ccusage` `active_high` and a `no_usage_visibility` signal | Engine skips `ccusage`; no usage-visibility recommendation is emitted |
| AS-06 | Paid aggregate merging 3 reports with mixed signals and utilization bands | Merged paid aggregate carries one `RecommendationSet` produced by re-running `Recommend` on union signals and resolved tool state (per documented precedence); plugin artifact JSON embeds the same set |
| AS-07 | DOM privacy probe: free report HTML rendered from a Report containing severe-MCP utilization and the engine's recommendation | Rendered DOM contains zero `mcp__*` / `skill__*` / `plugin__*` token strings, zero raw command strings, zero unknown private names |
| AS-08 | Determinism probe: same `Report` analyzed twice via `analyzer.Analyze` | Both runs produce byte-identical `Report.Recommendation` JSON |
| AS-09 | Render perf probe: free report renders with recommendation section enabled on the existing severe-MCP fixture | Render p95 stays under 500ms; no observable regression from the PR #76 baseline |
| AS-10 | A tool whose state resolves to `configured_medium` (configured but not active) for the recommended class | Engine emits an "audit_config" or "active-check" recommendation (reason enum), not an "install another tool" recommendation; UI surfaces this distinction in the deterministic explanation |

### Edge cases

- **Engine returns no-op** (`Primary == nil && Secondary == nil`,
  `RecommendationSet.Skipped == nil`): the report still carries a non-nil
  `RecommendationSet` field with `RegistryVersion`, `EngineVersion`, and the
  signal slice the engine evaluated. The UI renders a short "no action
  needed" note that names how many candidate tools were considered (count
  only, no IDs).
- **`Ecosystem.ToolingUtilization` carries unknown/private names**: those
  names contribute only to `unknown_id_count` on the engine output and never
  appear in `Report.Recommendation`. Counts are bounded.
- **A finding ID isn't mapped to any engine signal**: it is ignored by Phase
  B's signal derivation (no fabricated signals). The list of mapped finding
  IDs is documented in `plan.md`.
- **Conflicting evidence for the same tool** (e.g. `installed_medium` from
  CLI probe AND `rejected_medium` from a prior failure log): Phase B passes
  both sources to `ToolStateMap.Resolve`, which applies the documented
  precedence order. Phase B never invents a state value.
- **`Recommendation` field present in legacy free reports loaded by a newer
  UI**: the UI tolerates a missing `Recommendation` and shows nothing. The
  field is additive.
- **Paid aggregate with one report producing no recommendation and another
  producing a Primary**: the merged set is the result of re-running
  `Recommend` on the union of signals and the resolved tool state, not a
  field-level merge of the two `RecommendationSet` objects.

## Domain Language

| Canonical term | Meaning |
| --- | --- |
| **Engine** | The Phase A function `Recommend(signals []Signal, state ToolStateMap) RecommendationSet`. Signature is frozen. |
| **RecommendationSet** | The bounded engine output struct: `Primary`, `Secondary`, `Skipped`, `Signals`, `UnknownIDCount`, `RegistryVersion`, `EngineVersion`. Enum/allowlist fields only. |
| **Signal** | One of the closed `Signal` enum values (`tool_output_bloat`, `repeated_file_reads`, `retry_loop`, `context_growth_spikes`, `mcp_skill_bloat`, …). |
| **ToolStateMap** | Map from `ToolID` to `ToolStateEntry{State, Sources}` consumed by the engine. |
| **Free report** | The web HTML report served from `web/` and rendered by `web/app.js` from a `Report` JSON document. |
| **Paid artifact** | The plugin zip produced by `internal/remediation` from a (possibly merged) `Report`. |
| **No-op note** | The UI message rendered when the engine emits neither Primary nor Secondary. Names the candidate count only, no tool IDs. |
| **Signal derivation** | The new Phase B function mapping `Report` content → `[]Signal`. |
| **Tool-state derivation** | The new Phase B function mapping `Report` content → `ToolStateMap`. |

Avoid drift: never call the engine output a "Recommendation" (singular) — that
collides with the existing per-finding `recommendation` string field on
`Finding`. Use `RecommendationSet` for the engine output and `Recommendation`
only when referring to the `Report.Recommendation` field that carries it.

## Functional Requirements

| ID | Description | Status |
| --- | --- | --- |
| FR-001 | Derive engine `Signal` values from `Report` findings. Mapped finding IDs include `tool_output_bloat`, `repeated_file_reads`, `retry_loop`, `context_growth_spikes`. The mapping table is exhaustive and lives in `plan.md`. | Draft |
| FR-002 | Derive engine `Signal` values from `Ecosystem.ToolingUtilization` warning bands. A `high` or `severe` band on MCP utilization or skill utilization emits `mcp_skill_bloat`. | Draft |
| FR-003 | Build `ToolStateMap` entries from `Ecosystem.WorkflowFingerprints` using only allowlisted public tool IDs. Fingerprint `active`/`installed` flags map to `active_high`/`installed_medium` respectively, and confidence values map to `EvidenceSource` counts (bounded). | Draft |
| FR-004 | Build `ToolStateMap` entries from `Ecosystem.ToolingUtilization` for known MCP and skill IDs. Calls executed against a known MCP ID contribute `active_high` evidence; mere exposure contributes `configured_medium`. | Draft |
| FR-005 | Carry safe CLI probe evidence already captured by SDD fingerprints into `ToolStateMap.Sources` as `cli_presence` / `cli_version` counts (counts only; no raw version strings, no raw paths). | Draft |
| FR-006 | Call the frozen `Recommend(signals, state)` engine once per analyzer pass and attach the returned `RecommendationSet` as an optional field on `Report` (`Report.Recommendation`). | Draft |
| FR-007 | Carry the same `RecommendationSet` into the paid plugin artifact JSON without modifying the existing `VettedRecommendations` list. The new field is additive. | Draft |
| FR-008 | Aggregate-merge `RecommendationSet` across multi-report paid scans by re-running `Recommend` on the union of derived signals and a resolved tool-state map (using `ToolStateMap.Resolve` per-tool). The merged set is not a field-level merge of two `RecommendationSet` objects. | Draft |
| FR-009 | Free web report renders Primary and Secondary recommendations with allowlisted human-readable text composed from enum values only. When both are absent, render a short "no action needed" note containing only the candidate count. | Draft |
| FR-010 | When MCP/skill bloat band is `high` or `severe`, the surfaced recommendation set must prioritize pruning/lazy-loading (class `mcp_skill_hygiene`) over adding new tooling. This is enforced by the engine; FR-010 validates the wiring populates inputs such that the engine reaches that branch. | Draft |
| FR-011 | A tool whose state resolves to `active_high`, `configured_medium`, or `rejected_medium` for the relevant class must never appear as `Primary.PrimaryToolID` or `Secondary.PrimaryToolID`. Enforced by the engine; FR-011 validates the wiring populates state correctly. | Draft |
| FR-012 | When `Report.Recommendation` is absent (legacy report JSON), the free report UI renders no recommendation section and produces no console error. | Draft |

## Non-Functional Requirements

| ID | Description | Threshold | Status |
| --- | --- | --- | --- |
| NFR-001 | Determinism: identical `Report` inputs produce byte-identical `Report.Recommendation` JSON. | 100 repeated runs over the fixture corpus produce identical JSON bytes. | Draft |
| NFR-002 | Privacy: `Report.Recommendation` and any aggregate-paid recommendation field contain zero raw command strings, raw file paths, private/unknown tool names, prompt text, raw version strings, and raw CLI output. | Leak tests assert zero high-cardinality strings and zero matches for `mcp__*`, `skill__*`, `plugin__*` patterns in `Report.Recommendation` JSON and in the paid artifact JSON. | Draft |
| NFR-003 | Render performance: free report render p95 does not regress after adding the recommendation section. | p95 < 500ms on the existing severe-MCP fixture; no console errors. | Draft |
| NFR-004 | Backwards compatibility: a `Report` JSON written by code that predates this mission deserializes cleanly when read by the new analyzer, with `Recommendation == nil`. | Round-trip test using a pre-Phase-B fixture passes. | Draft |

## Constraints

| ID | Description | Status |
| --- | --- | --- |
| C-001 | Output fields use only the existing enum/allowlist set: `recommendation_id`, `primary_tool_id`, `skipped_tool_ids`, `reason`, `signal_ids`, `confidence`, `risk_level`, `install_policy`, `evidence_counts`. No new free-form string fields may be added to the recommendation surface. | Draft |
| C-002 | The frozen Phase A engine signature `func Recommend(signals []Signal, state ToolStateMap) RecommendationSet` may not be modified. Engine internals, rule precedence, and registry are also out of scope. | Draft |
| C-003 | PR #76 DOM-privacy invariant is preserved: the rendered DOM contains no `mcp__*` / `skill__*` / `plugin__*` token strings. Top Problems and report intelligence sections continue to use `textContent`, not `innerHTML`. | Draft |
| C-004 | The existing paid artifact `VettedRecommendations` list is preserved alongside the new bounded recommendation object. The two coexist; this mission does not consolidate them. | Draft |
| C-005 | Signal derivation, tool-state derivation, and aggregate-merge functions must order their iteration deterministically (sorted keys), matching the engine's NFR-001 contract. | Draft |
| C-006 | No new external dependency may be added for this mission. All wiring uses standard library packages and the existing `analyzer` / `remediation` modules. | Draft |

## Success Criteria

| ID | Outcome | How it is measured |
| --- | --- | --- |
| SC-01 | A user inspecting the local report JSON sees a deterministic next-best recommendation when the engine has one. | `Report.Recommendation.Primary != nil` for fixtures that match a known waste pattern with an inactive recommended tool. |
| SC-02 | When the user's tooling is already in good shape, the report visibly says so. | "No action needed" note renders for fixtures where the engine returns no Primary and no Secondary. |
| SC-03 | A user reading the free report cannot see any private tool name, raw command, raw file path, or raw version string. | Leak-test grep over `Report.Recommendation` JSON and rendered DOM returns zero hits for the forbidden patterns. |
| SC-04 | A user running the paid scan path receives a plugin artifact whose embedded recommendation is identical to what they saw in the free report (per single-report input) or aggregated deterministically (per multi-report input). | Round-trip test over the paid aggregate flow produces a JSON object equal to the engine output. |
| SC-05 | Reviewing the report intelligence UI does not feel slower than before this mission shipped. | p95 < 500ms on the existing severe-MCP fixture; no console warnings. |

## Key Entities

| Entity | Where it lives | Phase B role |
| --- | --- | --- |
| `Report` | `internal/analyzer/types.go` | Gains an optional `Recommendation *RecommendationSet` field. |
| `Ecosystem` | `internal/analyzer/types.go` | Source of `WorkflowFingerprints` and `ToolingUtilization` used by tool-state derivation. |
| `Finding` | `internal/analyzer/types.go` | Source of finding IDs used by signal derivation. |
| `RecommendationSet` | `internal/analyzer/token_saving_types.go` | Engine output; carried verbatim in `Report.Recommendation` and in the paid artifact. |
| `ToolStateMap` | `internal/analyzer/token_saving_types.go` | Engine input; built by Phase B's tool-state derivation. |
| `Signal` | `internal/analyzer/token_saving_types.go` | Engine input; produced by Phase B's signal derivation. |
| Paid plugin artifact | `internal/remediation/artifact.go` | Gains an embedded `RecommendationSet` field alongside `VettedRecommendations`. |
| Free report renderer | `web/app.js`, `web/index.html` | Gains a Primary/Secondary recommendation panel and a no-op note path. |

## Assumptions

- The engine's `Recommend` signature, conflict-resolution helper, registry, and
  rule precedence remain frozen. Any change to those is a separate mission.
- Finding IDs `tool_output_bloat`, `repeated_file_reads`, `retry_loop`, and
  `context_growth_spikes` exist (or can be confirmed in the analyzer) at plan
  time. If any are missing, plan.md will record the gap and either map an
  equivalent existing finding ID or document a stub.
- Existing severe-MCP fixture under `internal/analyzer/testdata/` continues to
  exercise the report path and is the canonical render-performance baseline.
- The `web/` HTML report is the only free render surface for now; CLI text
  output of the recommendation can come later and is out of scope.

## Out of Scope

- Modifying the Phase A engine (`Recommend`, registry, rule precedence,
  `RecommendationSet` shape).
- Consolidating the legacy `VettedRecommendations` hand-curated list with the
  engine output.
- Stripe checkout, paid-session entitlement, or other monetization paths
  (covered in later launch-completion PRs).
- CLI text rendering of the recommendation (web UI only for this mission).
- WASM/browser local-analysis demo (issue #37; not blocked by this mission).
