# Contract: Tool-State Derivation Map

`deriveToolStateMap(report *Report) ToolStateMap` returns the engine input
keyed by `ToolID`. Only known public tool IDs (members of the Phase A
allowlist registry) appear as keys; unknown private names are never keys
and never appear in any value.

## Source rules (exhaustive)

Each rule contributes a `ToolStateEntry`. When two rules contribute to the
same `ToolID`, the helper applies `ToolStateMap.Resolve(a, b)` (engine
precedence: `rejected_medium > active_high > configured_medium >
installed_medium > mentioned_low > unknown`). The `Sources` map values are
summed (bounded — capped at 100 per source key per the privacy budget).

### From `report.Ecosystem.WorkflowFingerprints`

For each `EcosystemFingerprint` whose `ID` is a member of the Phase A
registry:

| Rule | Fingerprint condition | Resolved `ToolState` | EvidenceSource increments |
| --- | --- | --- | --- |
| T-F-01 | `Active == true` | `ToolStateActiveHigh` | `EvidenceReportMention` += 1; `EvidenceCLIPresence` += 1 if `Sources` slice contains `"cli_probe"`; `EvidenceCLIVersion` += 1 if `VersionBucket != ""` |
| T-F-02 | `Active == false && Installed == true` | `ToolStateInstalledMedium` | `EvidenceReportMention` += 1; same CLI rules as T-F-01 |
| T-F-03 | `Active == false && Installed == false` | `ToolStateMentionedLow` | `EvidenceReportMention` += 1 |

Notes:

- The `Sources` slice on `EcosystemFingerprint` is bounded vocabulary (CLI
  probe ID, registry-mention markers). The helper checks for membership
  only; it never reads or echoes raw source strings.
- `VersionBucket` (when present) is itself a bounded enum (e.g.
  `"recent"`, `"old"`); only its presence acts as a trigger. The string
  value is not stored anywhere downstream.

### From `report.Ecosystem.ToolingUtilization.MCP`

For each `KnownServerID`:

| Rule | Condition | Resolved `ToolState` | EvidenceSource increment |
| --- | --- | --- | --- |
| T-M-01 | ID ∈ `UniqueKnownCalledIDs` (one or more executed calls) | `ToolStateActiveHigh` | `EvidenceMCPActive` += 1 |
| T-M-02 | ID ∈ `KnownServerIDs` but **not** in `UniqueKnownCalledIDs` | `ToolStateConfiguredMedium` | `EvidenceMCPConfigured` += 1 |

### From `report.Ecosystem.ToolingUtilization.Skill`

For each `KnownExposedID`:

| Rule | Condition | Resolved `ToolState` | EvidenceSource increment |
| --- | --- | --- | --- |
| T-S-01 | ID ∈ `KnownExecutedIDs` | `ToolStateActiveHigh` | `EvidenceSkillConfigured` += 1; `EvidenceReportMention` += 1 |
| T-S-02 | ID ∈ `KnownExposedIDs` but **not** in `KnownExecutedIDs` | `ToolStateConfiguredMedium` | `EvidenceSkillConfigured` += 1 |

Note: Phase A's `EvidenceSource` enum does not have a skill-active value
distinct from `EvidenceSkillConfigured`. T-S-01 uses
`EvidenceSkillConfigured` for the skill-configuration evidence and adds
`EvidenceReportMention` to record the active observation.

### From `report.Ecosystem.KnownPlugins`

Phase B does **not** derive any `ToolStateEntry` from `KnownPlugins`. The
engine registry does not currently include plugin-class IDs. Plugin IDs
are tracked elsewhere (paid artifact's `VettedRecommendations`) and are
explicitly out of scope for engine wiring. The fingerprint pipeline (used
for SDD tools) is the source for plugin-class tools the engine knows about.

### Unknown names

Unknown MCP/skill/plugin counts (`UnknownMCPServerCount`,
`UnknownSkillCount`, `UnknownPluginCount`) contribute **only** to the
engine's `UnknownIDCount` field via the engine's own bookkeeping when
`Recommend` performs registry lookups. Phase B never creates
`ToolStateEntry` rows for unknown names.

## Determinism rules

1. Rules are evaluated in source order (fingerprints first, then MCP,
   then skill), then per-source the iteration follows the slice order
   (which the analyzer guarantees deterministic).
2. The resulting `ToolStateMap` is keyed by `ToolID`; engine paths
   iterate it only via `SortedTools()`.
3. `EvidenceSource` source-map writes go to a fresh map per
   `ToolStateEntry` (no shared map between rules).
4. The maximum `Sources` count per key is capped at 100 to enforce the
   privacy budget on bounded integer counts (a cap that cannot be reached
   in practice by any natural report, but documented for correctness).

## Edge cases

- **Fingerprint ID not in registry**: silently skipped. Never appears in
  the map.
- **Same tool emitted by fingerprint AND MCP utilization** (e.g. a tool
  that is both an SDD framework and an MCP server — rare but possible):
  the helper produces one entry whose state is the
  `Resolve`-resolved value and whose `Sources` map sums both rules'
  increments.
- **Empty `Ecosystem` (no fingerprints, no utilization)**: returns an
  empty `ToolStateMap`. `deriveSignals` may still emit
  `SignalNoUsageVisibility`; the engine then sees an empty state map.
- **Conflicting evidence for the same tool from the same rule** (cannot
  happen — each rule fires at most once per ID).
- **`KnownServerIDs` is nil**: treated as empty slice.

## Test fixtures

Five table-driven tests in `recommendation_wiring_test.go`:

1. Single-source fixtures — one row per rule (T-F-01 through T-S-02), 7 cases.
2. Multi-source dedupe — a tool seen in both fingerprint and MCP utilization.
3. Conflict resolution — a tool with `Installed==true` from fingerprint AND
   not-executed MCP exposure (resolves to `ToolStateConfiguredMedium`).
4. Empty input — empty `Ecosystem` produces empty map.
5. Privacy — unknown MCP/skill names do not appear in the map.

A golden test asserts the marshaled `ToolStateMap` for the existing
severe-MCP fixture matches an expected byte string.
