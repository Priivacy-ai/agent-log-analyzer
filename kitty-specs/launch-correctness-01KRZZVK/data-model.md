# Phase 1: Data Model — Launch Correctness Fixes

This mission introduces no new serialized fields. It refines existing in-memory and merged shapes. All paths are absolute against the repo root `/Users/robert/code-analyzer-dev/launch-completion-20260519-125840-dfxXcb/agent-log-analyzer`.

## In-memory additions (never serialized)

### `byteRange`

New unexported type local to `internal/analyzer/tooling_detect.go`:

```go
// byteRange is a [Start, End) byte offset pair inside a parsed log buffer.
// In-memory only — never written to disk, JSON, logs, or telemetry.
type byteRange struct {
    Start int
    End   int
}
```

### `mcpExposure.HeaderRanges`

Add an unexported field to the existing `mcpExposure` struct in `internal/analyzer/tooling_detect.go:19..26`:

```go
type mcpExposure struct {
    KnownIDs         []string
    UnknownCount     int
    ExposedToolCount int
    SchemaTextBytes  int
    ExposedToolKnown bool
    InferenceSource  string
    HeaderRanges     []byteRange // NEW — in-memory only
}
```

### `skillExposure.HeaderRanges`

Mirror the same field on `skillExposure` (`tooling_detect.go:28..35`). Defensive: today the skill detector doesn't have an MCP-style regex rescan bug, but the structure makes future skill schema changes safe by default.

### Why these are private to the file

- `byteRange` is unexported (lowercase `b`).
- `HeaderRanges` is exported only within the package; the field is not part of any serialized JSON payload.
- The privacy canary in `leak_test.go` will explicitly assert that no `HeaderRanges` data reaches the report, aggregate event, or paid artifact.

## Existing structs — semantics changed, shape unchanged

### `Ecosystem` (`internal/analyzer/types.go:51..67`)

No new fields. The mission changes how two existing fields are aggregated across inputs:

- `ToolingUtilization ToolingUtilization` (`types.go:65`)
- `WorkflowFingerprints []EcosystemFingerprint` (`types.go:66`)

### `ToolingUtilization` (`internal/analyzer/types.go:83..119`)

Containers: `MCP MCPUtilization`, `Skill SkillUtilization`.

#### `MCPUtilization` (`types.go:88..104`) — merge semantics for FR-008

| Field | Type | Merge rule |
|-------|------|------------|
| `KnownServerIDs` | `[]string` | Union (deduplicated). Order: sorted ascending for determinism. |
| `UnknownServerCount` | `int` | Sum. |
| `ServerCountBucket` | `string` (closed enum) | Recompute from summed total exposed count if input bucket boundaries agree, else hold max by bucket rank. |
| `ExposedToolCountBucket` | `string` (closed enum) | Same recompute-or-max-rank rule. |
| `ContextTokenBucket` | `string` (closed enum) | Same recompute-or-max-rank rule. |
| `CallCount` | `int` | Sum. |
| `KnownCallCount` | `int` | Sum. |
| `UniqueKnownCalledIDs` | `[]string` | Union (deduplicated, sorted). |
| `UtilizationRatioPct` | `int` | Recompute from summed `KnownCallCount` and summed `ExposedToolCount` (clamped to `[0, 100]`). If denominator is zero, set to `0`. |
| `WarningBand` | `string` (closed enum) | Max by rank: `severe > high > watch > normal > unknown`. |

#### `SkillUtilization` (`types.go:106..119`) — merge semantics for FR-008

| Field | Type | Merge rule |
|-------|------|------------|
| `KnownExposedIDs` | `[]string` | Union (deduplicated, sorted). |
| `UnknownExposedCount` | `int` | Sum. |
| `ExposedCountBucket` | `string` (closed enum) | Recompute-or-max-rank. |
| `ExecutedCount` | `int` | Sum. |
| `KnownExecutedIDs` | `[]string` | Union (deduplicated, sorted). |
| `UtilizationRatioPct` | `int` | Recompute (`KnownExecutedCount / max(1, KnownExposedCount) * 100`). |
| `ContextEfficiencyBucket` | `string` (closed enum) | Recompute-or-max-rank. |
| `WarningBand` | `string` (closed enum) | Max by rank: same rank as MCP. |

### `EcosystemFingerprint` — merge semantics for FR-007

Per fingerprint `id`, across the set of input reports:

| Field | Merge rule |
|-------|------------|
| `id` | Identity key (group by). |
| `sources` (`[]string`, closed enum) | Union (deduplicated, sorted). |
| `evidence_count` (`int`) | **Sum** across inputs (C-007). |
| `confidence` (`string`, closed enum) | Max by confidence rank: `high > medium > low`. |
| `active` (`bool`) | OR across inputs. |
| `installed` (`bool`) | OR across inputs. |
| `version_bucket` (`string`, closed enum) | Retain when all inputs agree on a non-empty value; otherwise empty. No `mixed` value introduced. |

## Aggregate merge implementation surface

### `mergeEcosystems` (`internal/analyzer/aggregate.go:128..143`)

Currently merges 13 simple fields and returns the combined Ecosystem. Extend to also merge:

1. `ToolingUtilization.MCP` using the MCPUtilization rules above.
2. `ToolingUtilization.Skill` using the SkillUtilization rules above.
3. `WorkflowFingerprints` using the EcosystemFingerprint rules above.

Helper functions to introduce in `aggregate.go` (unexported):

- `mergeMCPUtilization(a, b MCPUtilization) MCPUtilization`
- `mergeSkillUtilization(a, b SkillUtilization) SkillUtilization`
- `mergeWorkflowFingerprints(a, b []EcosystemFingerprint) []EcosystemFingerprint`
- `maxWarningBand(a, b string) string` (rank-based; reused for MCP + Skill)
- `maxConfidence(a, b string) string` (rank-based; for fingerprints)
- `unionSorted(a, b []string) []string` (small helper; sort.Strings on dedup)

These helpers are private to `aggregate.go` and have no external API surface.

### `mergeEcosystems` post-condition invariants (for tests)

For any inputs `A` and `B`:

- `len(merge(A,B).KnownServerIDs) == len(union(A.KnownServerIDs, B.KnownServerIDs))`.
- `merge(A,B).UnknownServerCount == A.UnknownServerCount + B.UnknownServerCount`.
- `merge(A,B).WarningBand >= max(A.WarningBand, B.WarningBand)` under the band-rank order.
- `merge(A,B)` is associative on the set of inputs and produces a result that does not depend on input order. (Test: `merge(merge(A,B), C) == merge(A, merge(B,C))`.)
- `merge(A,B)` contains zero private name strings if `A` and `B` each contain zero private name strings in their public-allowlisted ID lists. (Privacy canary.)

## Touched test data and fixtures

| File | Action | Why |
|------|--------|-----|
| `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log` | NEW | FR-006: many `mcp__server__tool` tokens inside an exposure header, zero actual tool-use records. Expected MCP `CallCount` after fix: `0`. |
| `internal/analyzer/testdata/tooling/07-mixed-known-unknown.log` | UNCHANGED | Existing fixture. Golden assertion may shift if it had header tokens treated as calls — that shift is the fix, not a regression. |
| `internal/analyzer/testdata/tooling/{00..06}-*.log` | UNCHANGED | C-006: must be a no-op for these. |
| `internal/analyzer/golden_test.go:55..59` | MODIFIED | Stop nilling `WorkflowFingerprints` in aggregate golden compare. Instead, assert merged shape. |
| `internal/analyzer/leak_test.go` | EXTENDED | Add private MCP/skill names and raw paths into input reports; assert their absence in the merged aggregate output. |

## Post-Phase-1 Charter Re-check

| Gate | Verdict | Notes |
|------|---------|-------|
| Privacy stance | PASS | No new serialized field. Header byte ranges are integer offsets, in-memory only, asserted via leak test. |
| Bounded-cardinality | PASS | All merged values are allowlisted IDs / closed enums / numeric counts / bounded buckets. No new shape introduced. |
| C-006 (no-op stability) | PASS | Header-range mask is a strict no-op on input bytes that contain no header-range tokens. |
| C-007 (`evidence_count = sum`) | PASS | Locked. |
| Test coverage for new merge semantics | PASS | New aggregate tests cover associativity, privacy canary across merge, and FR-007/008 row-by-row rules. |
