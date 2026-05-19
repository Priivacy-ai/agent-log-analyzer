# Contract: `analyzer.AttachRecommendation` Go API

This is the only new public Go function introduced by Phase B.

## Signature

```go
// AttachRecommendation derives engine signals and tool state from the given
// Report, calls the frozen Phase A Recommend engine, and assigns the result
// to report.Recommendation. The function is deterministic, has no side
// effects beyond the report mutation, and never returns an error.
//
// Callers (analyzer.Analyze and analyzer.AggregateReports) must pass a
// non-nil pointer. The function is safe to re-run on the same Report; the
// second call overwrites Recommendation with a byte-identical value.
func AttachRecommendation(report *Report)
```

## Package

`package analyzer` (the same package as `Recommend`).

## File

`internal/analyzer/recommendation_wiring.go` (new file).

## Test file

`internal/analyzer/recommendation_wiring_test.go` (new file).

## Behavioral contract

1. **No panics on empty input.** Given a `Report` with empty `Findings`,
   empty `Ecosystem`, and empty `Recommendation`, `AttachRecommendation`
   returns normally with `report.Recommendation != nil` and
   `report.Recommendation.Primary == nil`.

2. **Deterministic.** Two calls on the same input produce a byte-identical
   marshaled `report.Recommendation`. Tested via a 100-iteration loop.

3. **No `range` over maps in derivation paths.** Derivation helpers iterate
   `ToolStateMap` only via `state.SortedTools()` and signal slices only via
   `sortedSignalIDs`. Tested by a static analysis test that greps the file
   for `range\s+state` and `range\s+m\b` patterns and fails when found.

4. **Engine contract preserved.** `report.Recommendation.EngineVersion` ==
   `analyzer.EngineVersion()` and `report.Recommendation.RegistryVersion`
   == the registry-version accessor in `token_saving_tools.go`.

5. **Frozen engine.** No code path inside `recommendation_wiring.go`
   mutates engine internals or the registry.

6. **Privacy budget.** The function does not read or store any raw command
   string, raw file path, prompt text, or raw version string. The only
   string values touched are: bounded `Finding.ID` enums, bounded
   `WarningBand` strings, bounded `EcosystemFingerprint.ID` enums (which
   are public allowlisted tool IDs), and `EcosystemFingerprint.VersionBucket`
   enum presence (as a boolean trigger only).

## Concurrency

Pure function. Safe to call from multiple goroutines on disjoint `*Report`
pointers. The function does not read or write package-level state.

## Error model

The function never returns an error. The engine's `Recommend` never returns
an error (its signature is frozen). Internal helpers also never return
errors; an unrecognized finding ID is silently skipped (mapping is a closed
set; unknown IDs are not signals).

## Internal helpers (package-private)

The following package-private functions live alongside `AttachRecommendation`
in `recommendation_wiring.go`. They are not part of the public contract but
are documented here so reviewers know what to look for.

```go
func deriveSignals(r *Report) []Signal
func deriveToolStateMap(r *Report) ToolStateMap
```

Each has its own table-driven test in
`recommendation_wiring_test.go`.

## Call sites

- `internal/analyzer/analyzer.go`: at the bottom of `Analyze`, after the
  `Report` is fully constructed (including `Ecosystem`, `Findings`,
  `AggregateEvent`).
- `internal/analyzer/aggregate.go`: at the bottom of `AggregateReports`,
  after the merged `Report` is fully constructed.

A grep verification asserts that exactly these two call sites exist; a
third call site is a code smell and must be justified in PR description.
