# Tasks: Launch Correctness Fixes

**Mission:** `launch-correctness-01KRZZVK`
**Spec:** [./spec.md](./spec.md)
**Plan:** [./plan.md](./plan.md)
**Branch contract:** planning_base=`main`, merge_target=`main` (single PR on feature branch `codex/launch-correctness`).

## Summary

Three bug-fix work packages. WP01 is independent. WP02 and WP03 land in sequence (WP03 depends on WP02) because they share concern over `internal/analyzer/golden_test.go` and `testdata/golden/sample-report.json`: WP02 may shift per-report MCP call counts, and WP03 picks up the golden refresh after rebasing.

| WP   | Title                                       | Subtasks | Est. lines | Parallel | Dependencies | Issues       |
|------|---------------------------------------------|----------|------------|----------|--------------|--------------|
| WP01 | CLI positional log path handling            | 5        | ~280       | [P]      | —            | #74          |
| WP02 | MCP exposure-header call mask               | 6        | ~360       |          | —            | #70          |
| WP03 | Paid aggregate merge for ecosystem fields   | 7        | ~460       |          | Depends on WP02 | #72       |

Lanes after `finalize-tasks` collapse: two lanes (WP01 in one, WP02 → WP03 in another).

MVP scope: any one of the three lanes; correctness story requires all three WPs.

## Subtask Index

| ID    | Description                                                       | WP   | Parallel |
|-------|-------------------------------------------------------------------|------|----------|
| T001  | Implement positional argument resolution in `runAnalyze`          | WP01 |          |
| T002  | Update `usage()` text to document positional form                 | WP01 | [P]      |
| T003  | Create `cmd/claude-analyzer/main_test.go` with FR-001..FR-003 tests | WP01 | [P]      |
| T004  | Update `README.md` and `docs/testing-plan.md` for positional form | WP01 | [P]      |
| T005  | Update `web/app.js` command generator copy if it advertises `--log` only | WP01 | [P]      |
| T006  | Add `byteRange` type and `HeaderRanges` field to `mcpExposure` / `skillExposure` | WP02 |          |
| T007  | Populate `HeaderRanges` in `detectMCPExposureFromHeaders` and skill exposure detector | WP02 |          |
| T008  | Add `maskedOffset` helper; apply mask inside `detectMCPCallsFromToolUse` raw-byte rescan | WP02 |          |
| T009  | Create fixture `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log` | WP02 | [P]      |
| T010  | Extend `tooling_detect_test.go`: add fixture-08 case + unit test for masking primitive | WP02 |          |
| T011  | Verify C-006 no-op stability across fixtures 00..06; update golden snapshot for fixture 07 only if header double-counts existed | WP02 |          |
| T012  | Add merge helpers (`maxWarningBand`, `maxConfidence`, `unionSorted`, `mergeMCPUtilization`, `mergeSkillUtilization`, `mergeWorkflowFingerprints`) in `aggregate.go` | WP03 |          |
| T013  | Extend `mergeEcosystems` to call the three field-level helpers     | WP03 |          |
| T014  | Add `aggregate_test.go` cases for FR-007 / FR-008 + identity / commutativity / associativity invariants | WP03 |          |
| T015  | Extend `leak_test.go` privacy canary across merged ecosystem, paid artifact, aggregate event (NFR-002) | WP03 |          |
| T016  | Update `golden_test.go` to assert merged `WorkflowFingerprints` instead of nilling them | WP03 |          |
| T017  | Add `internal/remediation/artifact_test.go` case proving artifact consumes merged data (FR-009) | WP03 | [P]      |
| T018  | Add timing test for NFR-005 (100-input merge < 5s)                | WP03 |          |

## Work Package 1 — CLI positional log path handling

**Goal:** Make `claude-analyzer analyze <path>` analyze `<path>`. Refuse all invalid combinations with clear errors. Document the new form.

**Independent test:** `go test ./cmd/claude-analyzer/...` passes with the new tests.

**Priority:** High. Issue #74. Required for charter `verification baseline` to give meaningful results when a developer points at a specific log.

**Issue thread:** #74. Implementer must post a start-of-work comment on #74 referencing this WP and a ready-for-review comment when the PR opens (FR-010 partial coverage).

### Included subtasks

- [x] T001 Implement positional argument resolution in `runAnalyze` (WP01)
- [x] T002 Update `usage()` text to document positional form (WP01)
- [x] T003 Create `cmd/claude-analyzer/main_test.go` with FR-001..FR-003 tests (WP01)
- [x] T004 Update `README.md` and `docs/testing-plan.md` for positional form (WP01)
- [x] T005 Update `web/app.js` command generator copy if it advertises `--log` only (WP01)

### Implementation sketch

1. Read `cmd/claude-analyzer/main.go:36..86`. After `fs.Parse(args)`, branch on `(len(fs.Args()), --log empty?)` per the FR-001/002/003 truth table in `contracts/cli-analyze.md`.
2. Use clear error messages whose substrings are part of the test contract (`cannot combine positional log path with --log`, `unexpected extra argument`).
3. Update `usage()` text in the same file.
4. Add `cmd/claude-analyzer/main_test.go` invoking `runAnalyze` directly (no need to `os.Exec`). Cover the six cases listed in `contracts/cli-analyze.md`.
5. Update README and docs for the new positional form (FR-004).
6. Update `web/app.js` only if its current local-command generator copy advertises `--log` exclusively.

### Parallel opportunities

T002, T003, T004, T005 can run after T001 lands in any order.

### Dependencies

None.

### Risks

- Accidentally changing `latestClaudeLog()` semantics. Mitigation: existing-callers no-args case is the first new test (sanity check, not new behavior).
- Error message wording drift between message and tests. Mitigation: tests assert substrings, not full strings.

### Reference contracts

- `contracts/cli-analyze.md`

---

## Work Package 2 — MCP exposure-header call mask

**Goal:** Tokens of shape `mcp__server__tool` appearing inside MCP/skill exposure-header byte ranges never count as calls in `MCPUtilization.CallCount` (and analogous skill counts).

**Independent test:** `go test ./internal/analyzer/ -run TestDetectMCPExposure -v` passes the new `08-header-only-zero-calls` fixture with `CallCount == 0`. `go test ./internal/analyzer/ -run TestGolden -v` produces byte-identical output for fixtures 00..06 (C-006).

**Priority:** High. Issue #70. The MCP utilization numbers are the single most visible report metric the user sees today; a stale double-count erodes trust immediately.

**Issue thread:** #70. Implementer comments on #70 at start and ready-for-review (FR-010 partial coverage).

### Included subtasks

- [x] T006 Add `byteRange` type and `HeaderRanges` field to `mcpExposure` / `skillExposure` (WP02)
- [x] T007 Populate `HeaderRanges` in `detectMCPExposureFromHeaders` and skill exposure detector (WP02)
- [x] T008 Add `maskedOffset` helper; apply mask inside `detectMCPCallsFromToolUse` raw-byte rescan (WP02)
- [x] T009 Create fixture `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log` (WP02)
- [x] T010 Extend `tooling_detect_test.go`: add fixture-08 case + unit test for masking primitive (WP02)
- [x] T011 Verify C-006 no-op stability across fixtures 00..06; update golden snapshot for fixture 07 only if header double-counts existed (WP02)

### Implementation sketch

1. T006: in `internal/analyzer/tooling_detect.go:17..48`, introduce unexported `byteRange struct{ Start, End int }` and add `HeaderRanges []byteRange` to both `mcpExposure` and `skillExposure`. These fields are in-memory only — confirm zero JSON serialization by structural inspection or test.
2. T007: at the existing header-block boundary detection (around `tooling_detect.go:114`), record the matched header block's `[start, end)` offsets into `HeaderRanges`. Do not change the existing exposed-tool counting logic.
3. T008: in `detectMCPCallsFromToolUse` (`tooling_detect.go:212..271`), before the raw-byte rescan at line ~242, build a combined ranges slice from MCP + skill headers. For each candidate match offset, skip if it falls inside any header range. The parsed-line scan at lines 250..263 is unchanged.
4. T009: create `internal/analyzer/testdata/tooling/08-header-only-zero-calls.log` — one system message advertising at least 5 `mcp__server__tool` identifiers in an exposure header (mix of public-allowlisted and obviously-synthetic names), zero tool-use records.
5. T010: extend `internal/analyzer/tooling_detect_test.go` table-driven test to include fixture 08 with assertion `CallCount == 0`; add a small unit test for the masking primitive (given header ranges + candidate offsets, returns filtered set).
6. T011: run `go test ./internal/analyzer/...`. Fixtures 00..06 MUST be byte-identical (C-006). If fixture 07's golden shifts, the change is the fix being applied; update the snapshot in the same WP.

### Parallel opportunities

T009 (fixture authoring) is independent of T006..T008 and can run in parallel.

### Dependencies

None outside WP02 itself. WP02 internal sequence: T006 → T007 → T008 → T010 → T011. T009 free.

### Risks

- Mutating raw bytes vs masking offsets: must NOT mutate. The mask is purely a predicate over candidate match offsets.
- Off-by-one on header byte ranges. Mitigation: T010 unit test for the masking primitive exercises boundary cases.
- Skill side is "defensive symmetry" — no current bug, but the structural addition keeps future skill exposure schema changes safe (rationale in `research.md`).

### Reference contracts

- `contracts/mcp-call-counting.md`

---

## Work Package 3 — Paid aggregate merge for ecosystem fields

**Goal:** `mergeEcosystems` preserves `Ecosystem.WorkflowFingerprints` and `Ecosystem.ToolingUtilization` across N input reports using the FR-007 / FR-008 deterministic rules. The generated paid plugin artifact consumes the merged values.

**Independent test:** `go test ./internal/analyzer/ -run TestAggregateReports -v` passes new cases for fingerprint merge, MCP/Skill merge, identity / commutativity / associativity invariants. `go test ./internal/remediation/...` passes the merged-artifact case.

**Priority:** High. Issue #72. The paid 100-log scan currently silently drops the most informative two ecosystem signals; this is a paid-flow correctness blocker.

**Issue thread:** #72. Implementer comments on #72 at start and ready-for-review (FR-010 partial coverage).

### Included subtasks

- [x] T012 Add merge helpers in `aggregate.go` (WP03)
- [x] T013 Extend `mergeEcosystems` to call the three field-level helpers (WP03)
- [x] T014 Add `aggregate_test.go` cases for FR-007 / FR-008 + invariants (WP03)
- [x] T015 Extend `leak_test.go` privacy canary across merged ecosystem / paid artifact / aggregate event (WP03)
- [x] T016 Update `golden_test.go` to assert merged `WorkflowFingerprints` instead of nilling them (WP03)
- [x] T017 Add `internal/remediation/artifact_test.go` case proving artifact consumes merged data (WP03)
- [x] T018 Add timing test for NFR-005 (100-input merge < 5s) (WP03)

### Implementation sketch

1. T012: in `internal/analyzer/aggregate.go`, add helpers as defined in `contracts/aggregate-merge.md` and `data-model.md`. Helpers are unexported.
2. T013: extend `mergeEcosystems` (around `aggregate.go:128..143`) to delegate to the three new field-level merge functions. Preserve the existing 13-field merges exactly as today.
3. T014: extend `internal/analyzer/aggregate_test.go` (or create it if absent) with the six invariants from `contracts/aggregate-merge.md` (identity, commutativity, associativity, coverage, privacy, bounded-cardinality) and explicit FR-007 / FR-008 row-by-row cases.
4. T015: extend `internal/analyzer/leak_test.go` privacy canary: build two input reports whose raw input contained synthetic private names, run the merge, serialize the merged ecosystem JSON and the generated paid artifact and the aggregate event payload — assert zero leak strings.
5. T016: in `internal/analyzer/golden_test.go:55..59`, stop nilling `WorkflowFingerprints`. Update the aggregate-golden snapshot to reflect the merged shape.
6. T017: add an artifact test in `internal/remediation/artifact_test.go` that constructs a multi-input paid scan, runs `Generate()`, and asserts the artifact reflects merged `ToolingUtilization` (not pre-merge values from any single input).
7. T018: add a timing test in `aggregate_test.go` composing 100 inputs from the largest bundled fixtures and asserting wall time < 5s.

### Parallel opportunities

T017 is in `internal/remediation/` and can run in parallel with the analyzer-side work once T013 is in place.

### Dependencies

**Depends on WP02.** WP03 rebases on the WP02 lane head before starting code work; if WP02's hand-off note flags a per-report MCP `CallCount` shift, the refreshed `testdata/golden/sample-report.json` is committed as part of this WP together with the aggregate-merge updates. See "WP02 rebase / hand-off" in the WP03 prompt.

WP03 internal sequence: T012 → T013 → (T014 || T015 || T016 || T017 || T018).

### Risks

- Privacy regression by accidentally retaining private name strings in union semantics. Mitigation: every union operates on public-allowlisted ID lists; canary in T015 asserts zero leakage.
- Associativity bug from non-commutative bucket-rank handling. Mitigation: T014 invariants exercise `merge(merge(A,B), C) == merge(A, merge(B,C))`.
- Performance ceiling: a naive O(N²) union per merge could blow NFR-005. Mitigation: helpers use sorted-union with deduplication (linear in input size), and `mergeEcosystems` runs linearly over input reports.

### Reference contracts

- `contracts/aggregate-merge.md`
