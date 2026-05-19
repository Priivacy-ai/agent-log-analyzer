# Contract: Signal Derivation Map

`deriveSignals(report *Report) []Signal` returns the deduplicated, sorted
slice of engine signals derived from the `Report`.

## Source rules (exhaustive)

| Rule | Source | Trigger condition | Emitted Signal |
| --- | --- | --- | --- |
| S-01 | `report.Findings` | A finding with `ID == "tool_output_bloat"` is present (any severity) | `SignalToolOutputBloat` (`"tool_output_bloat"`) |
| S-02 | `report.Findings` | A finding with `ID == "repeated_file_reads"` is present | `SignalRepeatedFileReads` (`"repeated_file_reads"`) |
| S-03 | `report.Findings` | A finding with `ID == "retry_loop"` is present | `SignalRetryLoop` (`"retry_loop"`) |
| S-04 | `report.Findings` | A finding with `ID == "context_growth_spikes"` is present | `SignalContextGrowthSpikes` (`"context_growth_spikes"`) |
| S-05 | `report.Ecosystem.ToolingUtilization.MCP.WarningBand` | Value ∈ `{"high", "severe"}` | `SignalMCPSkillBloat` (`"mcp_skill_bloat"`) |
| S-06 | `report.Ecosystem.ToolingUtilization.Skill.WarningBand` | Value ∈ `{"high", "severe"}` | `SignalMCPSkillBloat` |
| S-07 | `report.Ecosystem.WorkflowFingerprints` + tool registry | **No** fingerprint with `Active == true` whose `ID` belongs to the registry's `usage_visibility` class | `SignalNoUsageVisibility` (`"no_usage_visibility"`) |

## Determinism rules

1. Rules are evaluated in the order S-01 → S-07.
2. The result slice is deduplicated and sorted via `sortedSignalIDs`
   (the same helper Phase A uses for engine output).
3. The output is therefore byte-stable for identical input.

## Edge cases

- **Multiple findings with the same ID** (e.g. two `tool_output_bloat`
  findings from different shards): the signal is emitted **once** (dedupe
  via `sortedSignalIDs`).
- **MCP band `severe` AND Skill band `high`**: `SignalMCPSkillBloat` is
  emitted once.
- **Unknown `WarningBand` value** (anything outside `low`/`normal`/`watch`/`high`/`severe`):
  the rule does not fire. Phase A's bucket vocabulary is closed; an unknown
  value is treated as "not bloat" rather than fabricating a signal.
- **Empty `Findings` AND empty `Ecosystem`** (a report from an unsupported
  input format): only S-07 may fire, since no usage-visibility fingerprint
  is present. The engine then evaluates `SignalNoUsageVisibility` alone.
- **Phase A registry empty for `usage_visibility` class** (cannot happen
  with the shipping registry; documented for completeness): S-07 short-
  circuits to no-emit. Without a candidate tool, recommending one would
  fail downstream anyway.

## Not in scope

The following Phase A signals are intentionally **not** derived by Phase B:

- `SignalShellOutputBloat` (`shell_output_bloat`) — analyzer does not yet
  emit a finding that distinguishes shell from generic tool output bloat.
- `SignalMCPToolOutputBloat` (`mcp_tool_output_bloat`) — same.
- `SignalBroadRepoExploration` (`broad_repo_exploration`) — no finding ID.
- `SignalUnchangedFileRereads` (`unchanged_file_rereads`) — would require
  a new finding; deferred.
- `SignalOutputVerbosity` (`output_verbosity`) — would require a verbosity
  detector; deferred.

A future phase may extend this map; the engine already supports those
signal values.

## Test fixtures

Three table-driven tests in `recommendation_wiring_test.go`:

1. Single-source fixtures (one rule fires per case) — 7 cases (one per rule).
2. Multi-source fixtures (multiple rules fire) — at least 3 representative
   combinations including the MCP-and-Skill-bloat dedupe case.
3. Empty fixtures (no rules fire vs only S-07 fires).

A golden test asserts the marshaled `[]Signal` for the existing
severe-MCP fixture matches an expected byte string.
