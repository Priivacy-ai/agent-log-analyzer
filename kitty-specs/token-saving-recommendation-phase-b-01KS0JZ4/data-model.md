# Data Model: Token-Saving Recommendation Phase B Wiring

This document captures the entity-level diffs Phase B introduces. The engine
data model (`ToolStateMap`, `RecommendationSet`, `Signal`, `ToolState`, etc.)
is **frozen** by C-002 and unchanged here. Only the analyzer-side and
remediation-side entities change.

## Entities Changed

### `analyzer.Report`

Existing fields are unchanged. One new optional pointer field is added.

| Field | Type | JSON tag | Purpose |
| --- | --- | --- | --- |
| `Recommendation` | `*RecommendationSet` | `recommendation,omitempty` | Engine output produced by `AttachRecommendation`. `nil` only for legacy reports written by code that predates this mission. |

**Invariants**:

- `omitempty` ensures legacy `Report` JSON deserializes cleanly with
  `Recommendation == nil` (FR-012, NFR-004).
- After `AttachRecommendation` runs, the pointer is always non-nil. Even
  when the engine returns `Primary == nil && Secondary == nil`, the field
  carries `RegistryVersion`, `EngineVersion`, the evaluated signal slice,
  and `UnknownIDCount`.
- The struct is marshaled in field-declaration order; this is the
  determinism contract for NFR-001 (combined with the engine's existing
  sorted-key invariant on `EvidenceCounts`).

### `remediation.PluginArtifact`

(Defined in `internal/remediation/artifact.go`.) One new optional pointer
field is added; existing fields including `VettedRecommendations` are
unchanged (C-004).

| Field | Type | JSON tag | Purpose |
| --- | --- | --- | --- |
| `Recommendation` | `*analyzer.RecommendationSet` | `recommendation,omitempty` | Engine output carried through to the paid plugin artifact. Populated from `Report.Recommendation` verbatim. |

**Invariants**:

- The legacy `VettedRecommendations` slice continues to be populated by
  `toolingRecommendations(report)`; it is not gated on
  `Recommendation`.
- A `PluginArtifact` produced from a pre-Phase-B `Report` (where
  `Report.Recommendation == nil`) yields `PluginArtifact.Recommendation ==
  nil`; both fields stay `omitempty`.

## New Internal Entities

### `signalSource` (package-private, in `recommendation_wiring.go`)

Internal struct used by signal-derivation tests; not exported.

| Field | Type | Purpose |
| --- | --- | --- |
| `source` | `string` | Human-readable origin (e.g. `"finding:tool_output_bloat"`, `"utilization:mcp.warning_band"`). For test diagnostics only — never marshalled, never logged in production. |
| `signal` | `Signal` | The engine signal emitted by that source. |

This struct exists to make derivation tests readable. Production code paths
return `[]Signal` only.

## Entities Unchanged

These are listed for clarity; Phase B reads them but does not modify their
shape, JSON tags, or marshal order.

- `analyzer.Ecosystem`
- `analyzer.EcosystemFingerprint`
- `analyzer.ToolingUtilization`
- `analyzer.MCPUtilization`
- `analyzer.SkillUtilization`
- `analyzer.Finding`
- `analyzer.AggregateSafeEvent`
- `analyzer.WasteRange`
- All engine entities: `ToolState`, `EvidenceSource`, `Signal`,
  `RecommendationClass`, `Confidence`, `RiskLevel`, `InstallPolicy`,
  `Reason`, `ToolStateEntry`, `ToolStateMap`,
  `TokenSavingRecommendation`, `SkipNote`, `RecommendationSet`.

## State / Lifecycle

`AttachRecommendation` is a single deterministic function call with no state
of its own. It is invoked exactly once per `Report`:

```
build Report (Findings, Ecosystem, AggregateEvent, ...)
  └── AttachRecommendation(report)
        ├── deriveSignals(report)         -> []Signal
        ├── deriveToolStateMap(report)    -> ToolStateMap
        ├── Recommend(signals, state)     -> RecommendationSet
        └── report.Recommendation = &set
```

The same call sequence is used by both the per-report path
(`analyzer.Analyze`) and the aggregate path (`analyzer.AggregateReports`,
after merging completes).

## Validation Rules

Validation enforced at compile-time and via tests:

- `Recommendation` is a pointer field; `omitempty` is required so legacy
  reports round-trip cleanly.
- The helper does not panic on a nil `report`; callers in
  `Analyze`/`AggregateReports` must pass a non-nil pointer (their contract).
- Tests assert:
  - `Report.Recommendation.EngineVersion == analyzer.EngineVersion()` after
    `AttachRecommendation`.
  - `Report.Recommendation.RegistryVersion == analyzer.RegistryVersion()`
    after `AttachRecommendation` (Phase A already exposes this accessor in
    `token_saving_tools.go`).
  - Re-running `AttachRecommendation` on the same `Report` produces a
    byte-identical `Recommendation` JSON.

## Aggregate Merge Boundary

`AggregateReports`:

1. Merges classic ecosystem fields.
2. Merges `Ecosystem.WorkflowFingerprints` (already implemented in PR #75).
3. Merges `Ecosystem.ToolingUtilization` (already implemented in PR #75).
4. **(new)** Calls `AttachRecommendation(merged)` once.

Re-running the engine on merged inputs is the canonical semantics; no
field-merger for `RecommendationSet` is introduced.
