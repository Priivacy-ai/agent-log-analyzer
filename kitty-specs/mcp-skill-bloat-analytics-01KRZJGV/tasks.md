# Tasks: MCP and Skill Bloat Analytics

**Mission**: `mcp-skill-bloat-analytics-01KRZJGV` (mission_id `01KRZJGVG3MCCCY9MKB1YRDBQR`)
**Planning base**: `main`
**Merge target**: `main`
**Total work packages**: 6
**Total subtasks**: 25
**Generated**: 2026-05-19

## Branch Strategy

- Planning artifacts live on `main` (already committed by `/spec-kitty.specify` and `/spec-kitty.plan`).
- Implementation lanes are computed by `finalize-tasks` and materialized as worktrees by `/spec-kitty.implement`.
- All lanes ultimately merge into `main`.
- Single implementation branch per mission per spec C-005: `codex/mcp-skill-utilization`. Lane worktrees branch from this name + lane suffix.

## Subtask Index (reference only — not the tracking surface)

| ID   | Description                                                                 | WP   | Parallel |
|------|-----------------------------------------------------------------------------|------|----------|
| T001 | Add ToolingUtilization/MCPUtilization/SkillUtilization types + Ecosystem field | WP01 |          |
| T002 | Bucketing helpers: countBucket, tokenBucket, efficiencyBucket               | WP01 |          |
| T003 | Closed-enum constants: warning bands, inference sources                     | WP01 |          |
| T004 | Bucketing unit tests                                                        | WP01 |          |
| T005 | Header-based exposure detector (MCP + skill)                                | WP02 | [P]      |
| T006 | Tool-use call inference for MCP                                             | WP02 | [P]      |
| T007 | Skill execution counter with preserved path-avoidance                       | WP02 | [P]      |
| T008 | Hybrid footprint estimator (schema length + constants)                      | WP02 | [P]      |
| T009 | Detection unit tests                                                        | WP02 | [P]      |
| T010 | Warning band classifier (pure function, D-4 thresholds)                     | WP03 | [P]      |
| T011 | Efficiency bucket classifier                                                | WP03 | [P]      |
| T012 | Classifier unit tests for all D-4 band transitions                          | WP03 | [P]      |
| T013 | Invariant tests: count-alone-never-warns, exposure_known=false → unknown   | WP03 | [P]      |
| T014 | Pipeline reorder: compute metrics before ecosystem so bands see degradation | WP04 |          |
| T015 | Extend DetectEcosystem to populate ToolingUtilization                       | WP04 |          |
| T016 | Add bloat findings (mcp_bloat_high/severe, skill_bloat_high/severe)         | WP04 |          |
| T017 | Update normalizeEcosystemCollections for new nested slices                  | WP04 |          |
| T018 | Update existing analyzer_test.go assertions for new Ecosystem shape         | WP04 |          |
| T019 | Create 7 synthetic fixtures under testdata/tooling/                         | WP05 | [P]      |
| T020 | Golden test entries pinning buckets, ratios, bands, remediation strings     | WP05 | [P]      |
| T021 | Privacy-leak corpus test: zero forbidden substrings in serialized output    | WP05 | [P]      |
| T022 | Update docs/ecosystem-signatures.md                                         | WP06 | [P]      |
| T023 | Update docs/data-retention-and-analytics.md                                 | WP06 | [P]      |
| T024 | Update docs/logging-policy.md (cross-link)                                  | WP06 | [P]      |
| T025 | Update docs/testing-plan.md with the 7 fixture scenarios                    | WP06 | [P]      |

## Work Packages

### WP01 — Foundation: Types & Bucketing Helpers

**Goal**: Establish the type contracts and bucketing primitives that every later WP consumes. No business logic — just shapes, helpers, and constants.
**Priority**: P1 (blocks everything else)
**Independent test**: `go test ./internal/analyzer/ -run TestBucket` passes.
**Dependencies**: none
**Estimated prompt size**: ~280 lines
**Prompt**: [WP01-foundation-types-buckets.md](./tasks/WP01-foundation-types-buckets.md)

Included subtasks:
- [ ] T001 Add ToolingUtilization, MCPUtilization, SkillUtilization types + Ecosystem.ToolingUtilization field (WP01)
- [ ] T002 Bucketing helpers (countBucket, tokenBucket, efficiencyBucket) (WP01)
- [ ] T003 Closed-enum constants for warning bands and inference sources (WP01)
- [ ] T004 Bucketing unit tests (WP01)

Parallel opportunities: subtasks within this WP are sequential (T001 → T002 → T003 → T004). After WP01, WP02 and WP03 can run in parallel.

### WP02 — Detection: Exposure, Calls, Footprint

**Goal**: Build the pure detection layer that extracts privacy-safe signals from a parsed transcript: who is exposed, who was called, and how much context they use.
**Priority**: P1
**Independent test**: `go test ./internal/analyzer/ -run TestDetect` passes.
**Dependencies**: WP01
**Estimated prompt size**: ~400 lines
**Prompt**: [WP02-detection-exposure-calls-footprint.md](./tasks/WP02-detection-exposure-calls-footprint.md)

Included subtasks:
- [ ] T005 Header-based exposure detector for MCP and skill (WP02)
- [ ] T006 Tool-use call inference for MCP (WP02)
- [ ] T007 Skill execution counter with preserved path-avoidance (WP02)
- [ ] T008 Hybrid footprint estimator (schema-length when present, constants otherwise) (WP02)
- [ ] T009 Detection unit tests (WP02)

Parallel opportunities: parallel with WP03. Sub-subtasks within WP02 mostly independent (T005-T008 each touch separate function); T009 runs last.

### WP03 — Classification: Warning Bands

**Goal**: Implement the deterministic D-4 band classifier and efficiency bucket. Pure functions; no I/O, no state.
**Priority**: P1
**Independent test**: `go test ./internal/analyzer/ -run TestClassify` passes, including invariant tests.
**Dependencies**: WP01
**Estimated prompt size**: ~320 lines
**Prompt**: [WP03-classification-bands.md](./tasks/WP03-classification-bands.md)

Included subtasks:
- [ ] T010 Warning band classifier (pure function over buckets, ratio, degradation signals) (WP03)
- [ ] T011 Efficiency bucket classifier (WP03)
- [ ] T012 Classifier unit tests covering every D-4 transition (WP03)
- [ ] T013 Invariant tests: count alone never warns; exposure_known=false → unknown band (WP03)

Parallel opportunities: parallel with WP02.

### WP04 — Wiring: DetectEcosystem, Bloat Findings, Aggregate

**Goal**: Wire the detectors and classifiers into the existing `Analyze` pipeline. Populate `Ecosystem.ToolingUtilization`, add bloat findings, update normalization, and reconcile existing tests with the new shape.
**Priority**: P1 (integration)
**Independent test**: `go test ./internal/analyzer/...` passes (full package).
**Dependencies**: WP01, WP02, WP03
**Estimated prompt size**: ~440 lines
**Prompt**: [WP04-wiring-detect-findings-aggregate.md](./tasks/WP04-wiring-detect-findings-aggregate.md)

Included subtasks:
- [ ] T014 Reorder pipeline so metrics are computed before ecosystem (so bands see degradation) (WP04)
- [ ] T015 Extend DetectEcosystem to populate Ecosystem.ToolingUtilization (WP04)
- [ ] T016 Add bloat findings (mcp_bloat_high/severe, skill_bloat_high/severe) (WP04)
- [ ] T017 Update normalizeEcosystemCollections for new nested slice fields (WP04)
- [ ] T018 Update existing analyzer_test.go assertions to accommodate the new shape (WP04)

Parallel opportunities: none within WP04. Unblocks WP05 and WP06.

### WP05 — Fixtures & Golden Tests

**Goal**: Pin the implementation behavior with 7 synthetic golden fixtures spanning the full matrix, plus a privacy-leak corpus that proves zero leakage to the report and aggregate event.
**Priority**: P1 (acceptance)
**Independent test**: `go test ./internal/analyzer/ -run TestGolden` and `-run TestPrivacy` pass.
**Dependencies**: WP04
**Estimated prompt size**: ~380 lines
**Prompt**: [WP05-fixtures-golden-privacy.md](./tasks/WP05-fixtures-golden-privacy.md)

Included subtasks:
- [ ] T019 Create 7 synthetic fixtures under internal/analyzer/testdata/tooling/ (WP05)
- [ ] T020 Golden test entries pinning expected buckets, ratios, bands, remediation strings (WP05)
- [ ] T021 Privacy-leak corpus test: zero forbidden substrings in serialized Report and AggregateSafeEvent (WP05)

Parallel opportunities: parallel with WP06.

### WP06 — Documentation

**Goal**: Document the new metrics, footprint estimator, bucket meanings, warning band thresholds, privacy stance, and fixture scenarios so future maintainers understand what each field means and why.
**Priority**: P2 (post-merge facing)
**Independent test**: human read-through; cross-links resolve.
**Dependencies**: WP04
**Estimated prompt size**: ~260 lines
**Prompt**: [WP06-documentation.md](./tasks/WP06-documentation.md)

Included subtasks:
- [ ] T022 Update docs/ecosystem-signatures.md (new metrics + privacy stance) (WP06)
- [ ] T023 Update docs/data-retention-and-analytics.md (aggregate shape additions) (WP06)
- [ ] T024 Update docs/logging-policy.md (cross-link) (WP06)
- [ ] T025 Update docs/testing-plan.md (7 fixture scenarios) (WP06)

Parallel opportunities: parallel with WP05.

## MVP Scope

The MVP is **WP01 + WP02 + WP03 + WP04 + WP05**. WP06 (docs) can land in the same PR but is independently mergeable.

## Parallelization Map

```
WP01 ─┬─→ WP02 ─┬─→ WP04 ─┬─→ WP05
      │         │         │
      └─→ WP03 ─┘         └─→ WP06
```

After WP01: 2 parallel lanes (WP02, WP03).
After WP04: 2 parallel lanes (WP05, WP06).

## Requirement Coverage

| Requirement | Covered by |
|-------------|-----------|
| FR-001 (MCP inventory)               | WP02 (T005-T008), WP04 (T015) |
| FR-002 (MCP usage)                   | WP02 (T006), WP04 (T015) |
| FR-003 (Skill inventory)             | WP02 (T005, T008), WP04 (T015) |
| FR-004 (Skill usage)                 | WP02 (T007), WP04 (T015) |
| FR-005 (Warning bands deterministic) | WP03 (T010, T012, T013), WP04 (T015) |
| FR-006 (Remediation wording)         | WP04 (T016) |
| FR-007 (Path-avoidance preserved)    | WP02 (T007) |
| FR-008 (Emit in Report + Aggregate)  | WP01 (T001), WP04 (T015) |
| FR-009 (Golden fixtures)             | WP05 (T019, T020) |
| FR-010 (Preserve existing fields)    | WP01 (T001 — additive) |
| FR-011 (Existing immediate-fixes preserved) | WP04 (T016) |
| FR-012 (Docs updated)                | WP06 (T022-T025) |
| NFR-001 (Determinism)                | WP01 (T002 sort/enum), WP05 (T020) |
| NFR-002 (`go test` passes)           | all WPs |
| NFR-003 (gofmt clean)                | all WPs |
| NFR-004 (Zero privacy leakage)       | WP02 (T005-T008 count-only), WP05 (T021) |
| NFR-005 (Smoke test or blocker)      | reviewer responsibility at PR time |
| NFR-006 (Bounded cardinality)        | WP01 (T002, T003 enums), WP05 (T021) |
| NFR-007 (Backward compatibility)     | WP01 (T001 additive), WP04 (T018) |
