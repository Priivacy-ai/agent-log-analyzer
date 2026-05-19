# Data Model: MCP and Skill Bloat Analytics

**Mission**: `mcp-skill-bloat-analytics-01KRZJGV`
**Date**: 2026-05-19

This document specifies the Go types added by this mission and the synthetic fixtures that pin their expected outputs.

## Types

All new types live in `internal/analyzer/types.go`.

### `Ecosystem` (modified, additive)

```go
type Ecosystem struct {
    // ... all existing fields preserved unchanged ...
    KnownPlugins          []string `json:"known_plugins"`
    UnknownPluginCount    int      `json:"unknown_plugin_count"`
    PackageManagers       []string `json:"package_managers"`
    VersionControl        string   `json:"version_control"`

    // NEW (this mission):
    ToolingUtilization ToolingUtilization `json:"tooling_utilization"`
}
```

The new field is appended; no existing fields are renamed, reordered semantically, or removed. Existing JSON consumers see a new key but every previous key remains.

### `ToolingUtilization` (new)

```go
type ToolingUtilization struct {
    MCP   MCPUtilization   `json:"mcp"`
    Skill SkillUtilization `json:"skill"`
}
```

### `MCPUtilization` (new)

```go
type MCPUtilization struct {
    KnownServerIDs           []string `json:"known_server_ids"`             // sorted, allowlist hits only
    UnknownServerCount       int      `json:"unknown_server_count"`         // count; names never stored
    ServerCountBucket        string   `json:"server_count_bucket"`          // closed enum, see below
    ExposedToolCountBucket   string   `json:"exposed_tool_count_bucket"`    // closed enum
    ContextTokenBucket       string   `json:"context_token_bucket"`         // closed enum
    ExposureKnown            bool     `json:"exposure_known"`               // true iff a signal observed
    InferenceSource          string   `json:"inference_source"`             // closed enum: "header"|"calls"|"none"

    CallCount                int      `json:"call_count"`                   // total MCP calls observed
    KnownCallCount           int      `json:"known_call_count"`             // calls to allowlist servers
    UnknownCallCount         int      `json:"unknown_call_count"`           // calls to non-allowlist servers
    UniqueKnownCalledIDs     []string `json:"unique_known_called_ids"`      // sorted, allowlist IDs only
    UniqueUnknownCalledCount int      `json:"unique_unknown_called_count"`  // count; names never stored

    UtilizationRatioPct      int      `json:"utilization_ratio_pct"`        // integer 0..100; 0 when exposure unknown
    ContextEfficiencyBucket  string   `json:"context_efficiency_bucket"`    // closed enum
    WarningBand              string   `json:"warning_band"`                 // closed enum: normal|watch|high|severe|unknown
}
```

### `SkillUtilization` (new)

```go
type SkillUtilization struct {
    KnownExposedIDs          []string `json:"known_exposed_ids"`           // sorted
    UnknownExposedCount      int      `json:"unknown_exposed_count"`       // count
    ExposedCountBucket       string   `json:"exposed_count_bucket"`        // closed enum
    ContextTokenBucket       string   `json:"context_token_bucket"`        // closed enum
    ExposureKnown            bool     `json:"exposure_known"`
    InferenceSource          string   `json:"inference_source"`            // closed enum

    ExecutedCount            int      `json:"executed_count"`              // skill executions observed
    KnownExecutedIDs         []string `json:"known_executed_ids"`          // sorted
    UnknownExecutedCount     int      `json:"unknown_executed_count"`      // count

    UtilizationRatioPct      int      `json:"utilization_ratio_pct"`
    ContextEfficiencyBucket  string   `json:"context_efficiency_bucket"`
    WarningBand              string   `json:"warning_band"`
}
```

## Closed enumerations

All string fields above come from these closed sets. Tests assert no other values are ever emitted.

### Count buckets (`ServerCountBucket`, `ExposedToolCountBucket`, `ExposedCountBucket`)

`none`, `1-3`, `4-10`, `11-25`, `26-50`, `51-100`, `100+`, `unknown`.

### Context-token buckets (`ContextTokenBucket`)

`none`, `<1k`, `1k-5k`, `5k-15k`, `15k-50k`, `50k+`, `unknown`.

### Context-efficiency buckets (`ContextEfficiencyBucket`)

Derived from `UtilizationRatioPct` × `ContextTokenBucket`. Closed set:
`unused` (ratio < 5% and footprint ≥ `1k-5k`),
`underutilized` (ratio in 5..29%),
`moderate` (ratio in 30..69%),
`well-utilized` (ratio ≥ 70%),
`unknown` (`exposure_known=false`).

### Warning bands (`WarningBand`)

`normal`, `watch`, `high`, `severe`, `unknown`.

### Inference sources (`InferenceSource`)

`header`, `calls`, `none`.

## Invariants

- **I-1**: `ExposureKnown == false` ⟹ all of `ServerCountBucket`, `ExposedToolCountBucket`, `ExposedCountBucket`, `ContextTokenBucket` are `unknown`; `UtilizationRatioPct == 0`; `WarningBand == "unknown"`.
- **I-2**: `ExposureKnown == true` ⟹ no bucket is `unknown`; `WarningBand ∈ {normal, watch, high, severe}`.
- **I-3**: `WarningBand == "severe"` ⟹ `WarningBand` of the corresponding `high` conditions holds AND at least one of `Metrics.Rereads ≥ 3`, `Metrics.RetryDepthMax ≥ 3`, `Metrics.ContextGrowthEvents ≥ 2`.
- **I-4**: `UtilizationRatioPct` is computed as `min(100, round(executed_known / max(1, exposed_total) * 100))` when `ExposureKnown` is true; `0` otherwise.
- **I-5**: `len(KnownServerIDs) == len(unique_known_seen_in_exposure)` and `len(UniqueKnownCalledIDs) ⊆ allowlist`.
- **I-6**: Aggregate output contains only values from the closed enumerations above and integer counts. Free-form strings are never present.

## Bucketing function

Pseudocode for the count bucketer (new helper in `internal/analyzer/tooling.go`):

```go
func countBucket(n int, known bool) string {
    if !known { return "unknown" }
    switch {
    case n == 0:        return "none"
    case n <= 3:        return "1-3"
    case n <= 10:       return "4-10"
    case n <= 25:       return "11-25"
    case n <= 50:       return "26-50"
    case n <= 100:      return "51-100"
    default:            return "100+"
    }
}
```

Token bucketer is analogous with thresholds 1000, 5000, 15000, 50000.

The existing `bucket()` in `analyzer.go:506` (with `"%d_%d"` labels) is **not** reused — that function's label format is incompatible with the spec's mandated buckets, and changing it would break existing tests. New tooling buckets live in `tooling.go`.

## Synthetic Fixtures

Stored under `internal/analyzer/testdata/tooling/`. Each fixture has a `.log` input and a `.golden.json` expected output (or expected subset, asserting specific keys). All names in fixtures are synthetic and clearly distinguishable from anything real.

| Fixture | Input shape | Expected MCP band | Expected skill band | Asserts |
|---------|-------------|-------------------|--------------------|---------|
| `00-empty` | Minimal log, no MCPs, no skills | `normal` (`exposure_known=true`, `0` everywhere) or `unknown` if header missing | `normal` or `unknown` | All buckets resolve correctly under zero. |
| `01-healthy-small` | 2 known public MCPs both called; 1 known skill executed | `normal` | `normal` | Known IDs preserved; ratio ~100%. |
| `02-many-high-util` | 30 MCPs exposed via header, 25 called across many tools | `normal` | n/a | Count alone never triggers warning. |
| `03-many-low-util` | 30 MCPs exposed, only 2 called; no degradation | `high` | n/a | Footprint + low util triggers `high`; remediation strings appended. |
| `04-many-low-util-degraded` | Same as 03 plus `Rereads=4`, `RetryDepthMax=5` | `severe` | n/a | Degradation upgrades to `severe`; severe remediation strings appended. |
| `05-skill-bloat` | 20 skills exposed via header, 0 executed | n/a | `high` | Skill band fires independently of MCP. |
| `06-private-only` | Private MCP/skill names mixed with private schema text and repo paths in fake `system-reminder` | `high` or `severe` with `unknown_*_count` populated | `high` with `unknown_exposed_count` populated | **Privacy**: serialized report contains zero substring matches for any private name, path, or schema fragment. Known IDs arrays empty. |
| `07-mixed-known-unknown` | Mix of allowlist IDs + private names + path-like slash-command strings | varies | varies | Known IDs emitted by ID; private names counted only; path-like strings not counted as skill executions. |

Fixture `06-private-only` is also wired into a privacy-leak corpus (see NFR-004) that scans the JSON serialization of `Report` (not just `AggregateSafeEvent`) for forbidden substrings.
