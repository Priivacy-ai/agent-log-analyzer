---
work_package_id: WP05
title: Fixtures & Golden Tests
dependencies:
- WP04
requirement_refs:
- FR-009
planning_base_branch: main
merge_target_branch: main
branch_strategy: Lane worktree branches from WP04's lane head; runs in parallel with WP06.
subtasks:
- T019
- T020
- T021
agent: claude
history:
- event: generated
  at: '2026-05-19T08:00:33Z'
  by: /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/testdata/tooling/
execution_mode: code_change
mission_id: 01KRZJGVG3MCCCY9MKB1YRDBQR
mission_slug: mcp-skill-bloat-analytics-01KRZJGV
owned_files:
- internal/analyzer/golden_test.go
- internal/analyzer/testdata/tooling/**
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load the agent profile by invoking `/ad-hoc-profile-load` with `profile_id: "implementer-ivan"` and `role: "implementer"`. Then return here.

## Objective

Pin the end-to-end behavior of the MCP/skill bloat pipeline with **7 synthetic golden fixtures** covering the full matrix from `data-model.md`, and prove the privacy guarantee end-to-end with a privacy-leak corpus test that serializes both `Report` and `AggregateSafeEvent` and asserts zero substring matches for a basket of forbidden tokens.

This WP is the acceptance gate for the mission. The fixtures are the spec made executable.

## Context

Read first:
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/spec.md` — FR-009 (the seven fixture scenarios), AS-1..AS-7 acceptance scenarios, NFR-004 (zero leakage).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md` — §Synthetic Fixtures with the full table of expected behaviors per fixture.
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md` — §D-4 thresholds (use these to pre-compute expected bands).
- `internal/analyzer/golden_test.go` — existing golden test pattern. Study it before adding new entries.
- `internal/analyzer/signatures/{mcp_servers,skills}.json` — to choose synthetic IDs that ARE in the allowlist (e.g., `github`, `linear` for MCP) vs. invented private names that are clearly NOT in any allowlist.

Branch contract:
- Planning base: `main`. Merge target: `main`. Lane base resolved from `lanes.json` (depends on WP04).

## Detailed Guidance

### Subtask T019 — Create 7 synthetic fixtures

**Purpose**: Generate the seven `.log` files under `internal/analyzer/testdata/tooling/` that match the matrix in data-model.md.

**Steps**:
1. Create directory `internal/analyzer/testdata/tooling/`.
2. Create the seven fixtures listed below. Each is a plain text log (or JSONL — match whatever the existing golden tests expect). Use **synthetic, clearly-fake names**. Real allowlist IDs (`github`, `linear`, `notion`) may appear by their lowercase canonical form; private/unknown names must use obviously-synthetic strings (e.g., `acme_internal_test_mcp_a`, `private_corp_skill_xyz`).

   - `00-empty.log` — minimal log with one user message, no MCP/skill signals at all.
   - `01-healthy-small.log` — log containing `Available MCP servers: github, linear` header, plus 2-3 tool-use blocks calling `mcp__github__create_issue` and `mcp__linear__list_issues`. Plus `following skills are available:\n- scrape\n- review` and `/scrape` invocation in a user message (where `scrape` is an allowlist skill).
   - `02-many-high-util.log` — header listing 30 MCP server IDs (mostly synthetic, a few allowlist), plus 25 tool-use blocks distributed across those IDs.
   - `03-many-low-util.log` — same 30-server header as 02, but only 2 tool-use blocks. **No** rereads / retries / context growth in the log.
   - `04-many-low-util-degraded.log` — same as 03, plus repeated file reads (use `cat foo.go` twice, `cat bar.go` twice, `cat baz.go` twice — matches the existing `fileReadRE` at `analyzer.go:17`), plus error lines triggering `RetryDepthMax >= 3`, plus a 6000+ token jump between consecutive timeline windows for `ContextGrowthEvents`. Look at `computeMetrics` (analyzer.go:237) to understand exactly what signals you need.
   - `05-skill-bloat.log` — `following skills are available:` header with 20 synthetic skill IDs, ZERO `/skillname` invocations anywhere.
   - `06-private-only.log` — a `system-reminder` block listing 15 PRIVATE MCP server names (e.g., `acme_internal_secret`, `corp_intranet_mcp`) with FAKE schema text (one or two sentences each, including fake repo paths like `/Users/robert/secret/`, fake URLs like `https://internal.acme.test/`, fake "AKIA" tokens), plus 15 PRIVATE skill names with FAKE skill text. ZERO tool-use blocks.
   - `07-mixed-known-unknown.log` — header mixing allowlist IDs (`github`, `notion`) with private IDs, plus path-shaped tokens that look like slash commands (`/etc/passwd`, `/var/log/syslog`, `/home/user/file.txt`) interspersed in user messages.
3. Keep each fixture small (under 2KB ideally) — golden tests are for assertion, not load testing.
4. **Privacy check before committing**: grep each fixture file for the synthetic private names you used. They must appear in the FIXTURE (so the analyzer has something to count) but they must NOT appear in the golden output JSON (asserted in T021).

**Files**:
- `internal/analyzer/testdata/tooling/00-empty.log` through `07-mixed-known-unknown.log` (7 new files).

**Validation**:
- [ ] Each fixture exercises the scenario its name implies (you can spot-check by hand).
- [ ] Every "private" name in fixtures `06` and `07` is obviously synthetic (contains words like `acme`, `corp`, `internal`, `private`, `test`).
- [ ] No real third-party product names appear.

### Subtask T020 — Golden test entries

**Purpose**: For each fixture, run `Analyze` and assert specific values: warning bands, count buckets, token buckets, utilization ratio range, known IDs, and (for `04`) the presence of bloat-remediation strings in `Report.ImmediateFixes`.

**Steps**:
1. Open `internal/analyzer/golden_test.go`. Read the existing pattern (it likely uses a slice of `{name, inputPath, assertions}` structs).
2. Add a new test function `TestGoldenToolingFixtures` (or extend the existing one if natural):
   ```go
   func TestGoldenToolingFixtures(t *testing.T) {
       cases := []struct {
           name              string
           path              string
           wantMCPBand       string
           wantSkillBand     string
           wantMCPExposure   bool
           wantSkillExposure bool
           wantImmediateFixContains []string // substrings expected in ImmediateFixes; nil means no assertion
       }{
           {
               name:        "00-empty",
               path:        "testdata/tooling/00-empty.log",
               wantMCPBand: "unknown", wantSkillBand: "unknown",
               wantMCPExposure: false, wantSkillExposure: false,
           },
           {
               name:        "01-healthy-small",
               path:        "testdata/tooling/01-healthy-small.log",
               wantMCPBand: "normal", wantSkillBand: "normal",
               wantMCPExposure: true, wantSkillExposure: true,
           },
           {
               name:        "02-many-high-util",
               path:        "testdata/tooling/02-many-high-util.log",
               wantMCPBand: "normal", // count alone never triggers
           },
           {
               name:        "03-many-low-util",
               path:        "testdata/tooling/03-many-low-util.log",
               wantMCPBand: "high",
               wantImmediateFixContains: []string{"Scope project-specific MCPs"},
           },
           {
               name:        "04-many-low-util-degraded",
               path:        "testdata/tooling/04-many-low-util-degraded.log",
               wantMCPBand: "severe",
               wantImmediateFixContains: []string{"Disable unused MCP servers", "lazy-load"},
           },
           {
               name:          "05-skill-bloat",
               path:          "testdata/tooling/05-skill-bloat.log",
               wantSkillBand: "high",
               wantImmediateFixContains: []string{"general skills from project-specific"},
           },
           {
               name:        "06-private-only",
               path:        "testdata/tooling/06-private-only.log",
               wantMCPBand: "high", wantSkillBand: "high",
               // privacy is asserted by T021; here we only check the bands fire.
           },
           {
               name: "07-mixed-known-unknown",
               path: "testdata/tooling/07-mixed-known-unknown.log",
               // bands vary; assert at least that path-shaped tokens are NOT counted as skill executions.
           },
       }
       for _, tc := range cases {
           t.Run(tc.name, func(t *testing.T) {
               data, err := os.ReadFile(tc.path)
               if err != nil { t.Fatalf("read fixture: %v", err) }
               report, err := Analyze("test-"+tc.name, data)
               if err != nil { t.Fatalf("analyze: %v", err) }
               if tc.wantMCPBand != "" && report.Ecosystem.ToolingUtilization.MCP.WarningBand != tc.wantMCPBand {
                   t.Errorf("mcp band: got %q want %q", report.Ecosystem.ToolingUtilization.MCP.WarningBand, tc.wantMCPBand)
               }
               // ... analogous for skill band, exposure flags
               for _, want := range tc.wantImmediateFixContains {
                   found := false
                   for _, fix := range report.ImmediateFixes {
                       if strings.Contains(fix, want) { found = true; break }
                   }
                   if !found {
                       t.Errorf("expected immediate_fixes to contain %q; got %v", want, report.ImmediateFixes)
                   }
               }
           })
       }
   }
   ```
3. Run the test once locally and capture failures. For fixtures `02`, `03`, `04` — you may need to tune fixture content (e.g., add more tool-use blocks) so the buckets fall into the right band. The test should drive the fixture, not the other way around: if the test says band should be `severe`, but the fixture only produces `high`, fix the fixture (more degradation signals).
4. Tests run from `internal/analyzer/` so the fixture paths are relative to that directory.

**Files**:
- `internal/analyzer/golden_test.go` (extend).

**Validation**:
- [ ] `go test ./internal/analyzer/ -run TestGolden` passes for all 7 fixtures.
- [ ] Each fixture asserts at minimum the expected MCP and/or skill warning band.

### Subtask T021 — Privacy-leak corpus test

**Purpose**: Prove that fixtures `06-private-only` and `07-mixed-known-unknown` (the worst-case inputs) produce reports whose serialized JSON contains ZERO substring matches for any private name, path, schema fragment, or secret.

**Steps**:
1. In `internal/analyzer/golden_test.go`, add:
   ```go
   func TestPrivacyLeakCorpus(t *testing.T) {
       cases := []struct {
           name           string
           path           string
           forbiddenSubs  []string // strings that MUST NOT appear in serialized output
       }{
           {
               name: "06-private-only",
               path: "testdata/tooling/06-private-only.log",
               forbiddenSubs: []string{
                   "acme_internal_secret",
                   "corp_intranet_mcp",
                   "private_corp_skill_xyz",
                   "/Users/robert/secret",
                   "https://internal.acme.test",
                   "AKIA", // fake secret-like token
                   "fake schema description for", // fake schema text fragment used in the fixture
               },
           },
           {
               name: "07-mixed-known-unknown",
               path: "testdata/tooling/07-mixed-known-unknown.log",
               forbiddenSubs: []string{
                   "acme_internal_test_mcp_a",
                   "private_corp_skill",
                   "/etc/passwd",
                   "/var/log/syslog",
                   "/home/user/file.txt",
               },
           },
       }
       for _, tc := range cases {
           t.Run(tc.name, func(t *testing.T) {
               data, err := os.ReadFile(tc.path)
               if err != nil { t.Fatalf("read: %v", err) }
               report, err := Analyze("privacy-"+tc.name, data)
               if err != nil { t.Fatalf("analyze: %v", err) }

               // Serialize the full Report and assert no forbidden substrings.
               reportJSON, _ := json.Marshal(report)
               for _, sub := range tc.forbiddenSubs {
                   if bytes.Contains(reportJSON, []byte(sub)) {
                       t.Errorf("Report JSON leaks %q (privacy violation)", sub)
                   }
               }
               // Serialize the AggregateSafeEvent separately — it MUST also be clean.
               aggJSON, _ := json.Marshal(report.AggregateEvent)
               for _, sub := range tc.forbiddenSubs {
                   if bytes.Contains(aggJSON, []byte(sub)) {
                       t.Errorf("AggregateSafeEvent JSON leaks %q (privacy violation)", sub)
                   }
               }
           })
       }
   }
   ```
2. The forbidden-substrings list must mirror the synthetic names used in the fixtures from T019. Keep them in sync.
3. This test is the load-bearing privacy enforcement for the entire mission. If it fails, do not weaken it — fix the leaking code path.

**Files**:
- `internal/analyzer/golden_test.go` (extend).

**Validation**:
- [ ] `go test ./internal/analyzer/ -run TestPrivacy` passes.
- [ ] The forbidden-substring lists match what's actually in the fixtures.
- [ ] If a future contributor adds a field that leaks, this test fails loudly.

## Test Strategy

Required (NFR-002, NFR-004). Tests are integration-style — they read fixtures, run `Analyze`, and assert on outputs. No mocks. The privacy test runs at the JSON-serialization boundary because that's where leaks would actually manifest in production.

## Definition of Done

- [ ] 7 fixtures exist under `internal/analyzer/testdata/tooling/` with content matching the data-model.md table.
- [ ] `TestGoldenToolingFixtures` passes for all 7 fixtures.
- [ ] `TestPrivacyLeakCorpus` passes — zero forbidden substring matches in both `Report` and `AggregateSafeEvent` JSON.
- [ ] Existing golden tests still pass (`go test ./internal/analyzer/...`).
- [ ] `gofmt -l internal/analyzer/golden_test.go` returns empty.

## Risks

- **Risk**: Fixture authoring is tedious and easy to get wrong (e.g., not enough tool-use blocks to hit a band threshold). **Mitigation**: develop iteratively — write the test with expected band, run, look at the actual report, adjust the fixture, repeat. The classifier from WP03 is deterministic so this loop converges fast.
- **Risk**: Forbidden-substring list drifts from fixture content. **Mitigation**: keep them named identically (e.g., always `acme_internal_secret`); add a comment block at the top of T021 listing every fixture-name → forbidden-substring mapping.
- **Risk**: The privacy test passes accidentally because the leaked field isn't being serialized. **Mitigation**: spot-check by temporarily removing the privacy guard in WP02's exposure detector — the test must fail. Then put the guard back.

## Reviewer Guidance

When reviewing:
- Open each fixture file. Verify the private/synthetic names are clearly fake. Verify no real product names slipped in.
- Verify `TestPrivacyLeakCorpus` serializes BOTH the full `Report` and the `AggregateEvent` separately — both must be checked.
- Verify the forbidden-substring lists are not empty (at least 5 entries per fixture).
- Spot-check one fixture by running the test manually with `-v` and reading the actual JSON output to confirm it looks reasonable.

## Implementation Command

```bash
spec-kitty agent action implement WP05 --agent claude
```
