---
work_package_id: WP03
title: Privacy and determinism gates (leak + golden)
dependencies:
- WP01
requirement_refs:
- NFR-001
- NFR-002
- NFR-004
- C-001
- C-003
- C-005
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
created_at: '2026-05-19T20:30:00+00:00'
subtasks:
- T013
- T014
- T015
- T016
agent_profile: reviewer-rina
role: reviewer
agent: claude:sonnet:reviewer-rina:reviewer
authoritative_surface: internal/analyzer/testdata/recommendation
execution_mode: code_change
owned_files:
- internal/analyzer/leak_test.go
- internal/analyzer/golden_test.go
- internal/analyzer/testdata/recommendation
history:
- '2026-05-19': created from mission token-saving-recommendation-phase-b-01KS0JZ4
tags:
- privacy
- determinism
- tests
---

## ⚡ Do This First: Load Agent Profile

```text
/ad-hoc-profile-load reviewer-rina
```

Note: this WP uses the reviewer profile because its work is
gate-enforcement (tests + privacy probes), not new product code. After
profile load, continue with **Objective** below.

## Objective

Extend the existing privacy gate (`internal/analyzer/leak_test.go`) and
the existing golden snapshot (`internal/analyzer/golden_test.go`) to
cover the new `Report.Recommendation` field. Add a Report fixture that
includes private MCP/skill names and prove they never reach the
recommendation JSON.

This WP must NOT modify any file outside `owned_files`. Engine internals,
WP01's helper, the aggregate path, and the paid artifact path are all
out of scope. Tests are the only product of this WP.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. Execution
worktree per `lanes.json`; resolve via context-resolve.

## Context

- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/spec.md` (NFR-001, NFR-002, AS-07, AS-08)
- `kitty-specs/token-saving-recommendation-phase-b-01KS0JZ4/contracts/report-recommendation-json-envelope.md` (forbidden substrings list)
- Existing `internal/analyzer/leak_test.go` (the canonical privacy gate)
- Existing `internal/analyzer/golden_test.go` (the canonical snapshot)
- Existing fixtures under `internal/analyzer/testdata/`

## Owned files

- `internal/analyzer/leak_test.go` — extend existing assertions with
  new probes targeting `Report.Recommendation` JSON.
- `internal/analyzer/golden_test.go` — extend existing snapshot to
  include the new field.
- `internal/analyzer/testdata/recommendation/` — new subdirectory holding
  any new fixture data added by this WP.

## Subtasks

### T013 — Extend leak test for recommendation-JSON forbidden substrings

**Purpose**: Lock in NFR-002 over the new field.

**Steps**:

1. Open `internal/analyzer/leak_test.go`. Read the entire file before
   making changes (it is the canonical privacy gate; do not destabilize
   existing assertions).
2. Add `TestLeakRecommendationJSON`:
   - Build a synthetic `Report` whose `Ecosystem` carries:
     - `UnknownMCPServerCount = 5`.
     - `UnknownSkillCount = 3`.
     - `UnknownPluginCount = 2`.
     - At least one `WorkflowFingerprints` entry with `ID` outside the
       registry (synthetic "private" ID that mimics a private MCP name,
       e.g. `"private_mcp_acme"`).
   - Call `AttachRecommendation(&report)`.
   - Marshal `report.Recommendation` to JSON.
   - Assert the JSON byte string contains zero matches for the
     forbidden substrings: `"mcp__"`, `"skill__"`, `"plugin__"`,
     `"private_mcp_"`, `"acme"`. (The exact pattern set should match
     existing leak-test patterns — re-use the existing
     `forbiddenSubstrings` slice if one exists in the file; otherwise
     declare a local one and document why.)

**Validation**: `go test -run TestLeakRecommendationJSON
./internal/analyzer/...` passes.

### T014 — Extend leak test with a private-name probe

**Purpose**: Prove that unknown private names contribute only to
`UnknownIDCount` and never become any string field in the recommendation
output.

**Steps**:

1. Add `TestLeakRecommendationPrivateNamesAreOnlyCounted`.
2. Build a `Report` with explicit private names in `Ecosystem`:
   - `WorkflowFingerprints` containing one entry whose ID is a private
     name not in the registry.
   - `KnownMCPServerIDs` containing one private name (this is a
     synthetic abuse of the field; the production analyzer only puts
     known names here, but the test is a worst-case probe).
3. Call `AttachRecommendation(&report)`.
4. Marshal `report.Recommendation` to JSON.
5. Assert:
   - The JSON does not contain any of the private names supplied.
   - `report.Recommendation.UnknownIDCount > 0`.

**Validation**: `go test -run
TestLeakRecommendationPrivateNamesAreOnlyCounted ./internal/analyzer/...`
passes.

### T015 [P] — Add golden test for recommendation JSON

**Purpose**: Lock in the field-level shape so accidental field-order
changes or accidental new fields are caught by the snapshot.

**Steps**:

1. Open `internal/analyzer/golden_test.go`. Read the entire file before
   making changes.
2. The existing severe-MCP fixture is referenced by current golden
   tests; reuse it. Run the existing analysis path against the fixture,
   capture `report.Recommendation`, marshal to indented JSON
   (`json.MarshalIndent(..., "", "  ")`).
3. Compare against a new golden file:
   `internal/analyzer/testdata/recommendation/severe-mcp.golden.json`.
4. On mismatch, the test fails with a diff and a hint to re-generate
   via `UPDATE_GOLDEN=1 go test ./...` (existing pattern in this
   package — use the same env-var if it already exists).

**Validation**: `go test -run TestGoldenRecommendation
./internal/analyzer/...` passes. First run requires generating the
golden via the update-env-var.

### T016 — Add fixture with private names

**Purpose**: Provide a stable test fixture that future WPs can reuse.

**Steps**:

1. Create `internal/analyzer/testdata/recommendation/leak-fixture.json`.
2. Populate it with a Report shape that:
   - Carries 2-3 `WorkflowFingerprints` entries with both known and
     unknown IDs.
   - Carries `Ecosystem.ToolingUtilization.MCP.UnknownCallCount > 0`.
   - Carries `Ecosystem.ToolingUtilization.Skill.UnknownExecutedCount > 0`.
3. Have T013 and T014 load this fixture rather than constructing a
   Report in-code.

**Validation**: tests T013 and T014 use the fixture and still pass.

## Test strategy

- Extend, do not rewrite. Existing leak-test and golden-test assertions
  must stay green.
- Use the same fixture loading pattern as the rest of `testdata/`.
- Make the golden update path explicit (env var name documented in the
  test).

## Definition of Done

- [ ] `go test ./...` passes (full repo).
- [ ] New tests `TestLeakRecommendationJSON`,
      `TestLeakRecommendationPrivateNamesAreOnlyCounted`, and
      `TestGoldenRecommendation` all pass.
- [ ] Existing leak/golden tests still pass.
- [ ] New fixture file
      `internal/analyzer/testdata/recommendation/leak-fixture.json`
      exists and is non-empty.
- [ ] New golden file
      `internal/analyzer/testdata/recommendation/severe-mcp.golden.json`
      exists.
- [ ] No file outside `owned_files` is modified.

## Risks

- **Existing leak/golden tests break because the new `Recommendation`
  field appears in their snapshot/JSON-grep**: this is the most likely
  failure mode. Triage: extend the snapshot, do not delete it. If the
  forbidden-substring assertion now hits a string that legitimately
  appears in the new recommendation JSON, that's a sign WP01's
  derivation is leaking — file a bug against WP01 immediately rather
  than weakening the assertion.
- **Golden file too large** — recommendation JSON is small (one
  pointer's worth of bounded fields). If the golden snapshot grows by
  more than a few KB, something is wrong.

## Reviewer Guidance

1. Confirm the forbidden-substring list is at least as strict as the
   existing list. New strings should be added; no existing forbidden
   substring should be removed.
2. Confirm the private-name probe actually constructs private names
   (not just the synthetic IDs already in the registry).
3. Confirm the golden file is checked into git and its bytes match what
   the test computes for the canonical fixture.

## Working Directory and Hand-off

```bash
gofmt -w internal/analyzer
go vet ./internal/analyzer/...
UPDATE_GOLDEN=1 go test ./internal/analyzer/... # first run to create golden
go test ./internal/analyzer/...                 # second run, no env var
```

All must exit 0. Then commit:

```bash
git add internal/analyzer/leak_test.go \
        internal/analyzer/golden_test.go \
        internal/analyzer/testdata/recommendation/
git commit -m "test(WP03): privacy and determinism gates for recommendation JSON"
```

Mark subtasks done and move to `for_review`:

```bash
spec-kitty agent tasks mark-status T013 T014 T015 T016 --status done --mission token-saving-recommendation-phase-b-01KS0JZ4
spec-kitty agent tasks move-task WP03 --to for_review --note "Ready for review" --mission token-saving-recommendation-phase-b-01KS0JZ4
```
