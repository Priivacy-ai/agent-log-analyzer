# Contract: Paid Aggregate Merge — `WorkflowFingerprints` and `ToolingUtilization`

**Backs:** FR-007, FR-008, FR-009, NFR-002, NFR-005, C-006, C-007
**Implementation site:** `internal/analyzer/aggregate.go` (`mergeEcosystems` and new helpers)
**Consumer site:** `internal/remediation/artifact.go`
**Test sites:** `internal/analyzer/aggregate_test.go`, `internal/analyzer/golden_test.go`, `internal/analyzer/leak_test.go`, `internal/remediation/artifact_test.go`

## Goal

Merge two newer Ecosystem fields across N input reports without:
1. Dropping any input fingerprint by id (SC-4).
2. Introducing any private name into the merged output (NFR-002).
3. Producing order-dependent output (associativity invariant).

## Merge semantics (definitive)

### `Ecosystem.WorkflowFingerprints` (`[]EcosystemFingerprint`)

Group by `id`. For each group, the merged fingerprint is:

- `id` = group key.
- `sources` = sorted union of `sources` across all inputs in the group (deduplicated).
- `evidence_count` = `Σ evidence_count_i` (C-007: sum, not max).
- `confidence` = `argmax_rank(confidence_i)` where rank is `low < medium < high`.
- `active` = `OR_i(active_i)`.
- `installed` = `OR_i(installed_i)`.
- `version_bucket` =
  - The shared value if all `version_bucket_i` are non-empty and equal.
  - Empty string otherwise. (`mixed` is **not** introduced.)

### `Ecosystem.ToolingUtilization.MCP` (`MCPUtilization`)

- `KnownServerIDs` = sorted union.
- `UnknownServerCount` = `Σ`.
- `ExposedToolCount` (if present as a numeric field) = `Σ`.
- `CallCount` = `Σ`.
- `KnownCallCount` = `Σ`.
- `UniqueKnownCalledIDs` = sorted union.
- `UtilizationRatioPct` =
  - If `ExposedToolCount_total > 0`: `round(100 * KnownCallCount_total / ExposedToolCount_total)`, clamped to `[0, 100]`.
  - Else: `0`.
- `ServerCountBucket`, `ExposedToolCountBucket`, `ContextTokenBucket` = recompute from summed totals when bucket boundaries are well-defined; otherwise hold `argmax_rank(bucket_i)`.
- `WarningBand` = `argmax_rank(band_i)` with rank `unknown < normal < watch < high < severe`.

### `Ecosystem.ToolingUtilization.Skill` (`SkillUtilization`)

- `KnownExposedIDs` = sorted union.
- `UnknownExposedCount` = `Σ`.
- `ExposedCountBucket` = recompute-or-max-rank.
- `ExecutedCount` = `Σ`.
- `KnownExecutedIDs` = sorted union.
- `UtilizationRatioPct` =
  - If `KnownExposedCount_total > 0`: `round(100 * KnownExecutedCount_total / KnownExposedCount_total)`, clamped.
  - Else: `0`.
- `ContextEfficiencyBucket` = recompute-or-max-rank.
- `WarningBand` = `argmax_rank(band_i)` (same rank as MCP).

## Invariants

For any inputs `A`, `B`, `C`, let `m(x, y)` denote `mergeEcosystems(x, y)`.

1. **Identity:** `m(A, empty) == A` for all fields covered above.
2. **Commutativity:** `m(A, B) == m(B, A)`.
3. **Associativity:** `m(m(A, B), C) == m(A, m(B, C))`.
4. **Coverage:** for every fingerprint `f ∈ A.WorkflowFingerprints ∪ B.WorkflowFingerprints`, `m(A, B).WorkflowFingerprints` contains a fingerprint with `id == f.id`.
5. **Privacy:** if neither `A` nor `B` contains any private name string in any allowlisted ID list, `m(A, B)` contains no private name strings either. Unknown name counts merge as integers; the names themselves are not part of any input field.
6. **Bounded-cardinality:** every closed-enum field in `m(A, B)` has a value from the input enum domain — no new values introduced.

Tests assert all six invariants on a generated input pair (and triple for associativity) per FR-007 / FR-008.

## Consumer contract (FR-009)

`internal/remediation/artifact.go:Generate` and `toolingRecommendations` (around `artifact.go:120`) read from `report.Ecosystem.ToolingUtilization` and `report.Ecosystem.WorkflowFingerprints`. After this mission:

- Paid scan flow (`internal/paidscan/bundle.go:44` → `AnalyzeBundle` → `AggregateReports`) populates these fields in the aggregate `Report`.
- The generated paid plugin artifact JSON includes the merged tooling utilization and fingerprint summary in the fields it already exposes (no new artifact fields).
- A new test in `internal/remediation/artifact_test.go` asserts that a paid artifact generated from a multi-report merge contains the merged values (not the pre-merge values from any single input).

## Performance contract (NFR-005)

- `mergeEcosystems` is `O(I × U)` where `I` is the number of input reports and `U` is the cardinality of all union sets. For a 100-input scan with realistic golden-fixture sizes, total merge wall time MUST be `< 5s` on a developer laptop equivalent to the GitHub Actions runner.
- Bench/timing test (optional but recommended): `BenchmarkAggregateReports100` in `aggregate_test.go` records the runtime; CI is not gated on it, but a `testing.B` failure on `> 5s` for the standard fixture set is a regression signal.

## Privacy canary (NFR-002)

`internal/analyzer/leak_test.go` is extended:

1. Build two input reports `A` and `B` where:
   - `A.Ecosystem.MCPServersKnown = []string{"<private>", "github_mcp"}` — wait, **no.** `MCPServersKnown` is supposed to contain *only* allowlisted IDs. Instead, simulate the failure mode: build `A` and `B` from raw input bytes containing private names, parse through the analyzer, then merge. The privacy canary verifies that no private name from the **raw input** appears in the merged output.
2. Serialize `mergeEcosystems(A.Ecosystem, B.Ecosystem)`.
3. Assert none of the leak-string fixtures (the existing `leak_test.go` strings such as `acme_internal_secret`, `private_corp_mcp`) appear in the serialized merged Ecosystem JSON.
4. Assert the same against the generated paid artifact built from the merged report.
5. Assert the same against the aggregate event payload built from the merged report.

## Out of scope

- Adding a `mixed` value to the `version_bucket` enum.
- Switching `evidence_count` to `max`.
- Changing the upload schema (C-001).
- Changing the per-report computation of `ToolingUtilization` (already done in `internal/analyzer/ecosystem.go`).
