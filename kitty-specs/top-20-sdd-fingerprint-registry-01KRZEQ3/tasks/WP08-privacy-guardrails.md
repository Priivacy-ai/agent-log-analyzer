---
work_package_id: WP08
title: 'Privacy guardrails: leak test + structural bounds'
dependencies:
- WP05
requirement_refs:
- FR-011
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning on main; implementation lands on codex/sdd-fingerprint-registry; final merge target main.
subtasks:
- T034
- T035
- T036
phase: Phase 4 — Guardrails
agent: claude
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/sdd/
execution_mode: code_change
owned_files:
- internal/analyzer/sdd/leak_test.go
- internal/analyzer/sdd/structural_test.go
- internal/analyzer/analyzer_test.go
role: implementer
tags: []
---

# Work Package Prompt: WP08 — Privacy guardrails

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.

## Objectives & Success Criteria

- `TestReportSerializationContainsNoForbiddenStrings` exists, runs as part of `go test ./...`, and asserts that **none** of the 16 forbidden-string canaries from `contracts/forbidden-strings.md` appear in `json.Marshal(report)` or `json.Marshal(report.AggregateEvent)` (NFR-001).
- A structural test asserts `Ecosystem.WorkflowFingerprints` has bounded field shape (NFR-003): no `map[string]string`, no free-text fields.
- Existing analyzer privacy tests (unknown MCP/skill counts, etc.) still pass with the new fingerprint pass active.

## Context & Constraints

- Read: `contracts/forbidden-strings.md` (canonical canary list).
- Read: `data-model.md` (`EcosystemFingerprint` field list — exactly seven primitive fields).
- Read: existing `internal/analyzer/analyzer_test.go` for the privacy assertion pattern already in use.
- FR-011, NFR-001, NFR-003 mapped to this WP.

## Subtasks & Detailed Guidance

### Subtask T034 — Canary leak test

- **File**: `internal/analyzer/sdd/leak_test.go` (new, ~200 lines).
- **Steps**:
  1. Define the 16 canary values exactly as in `contracts/forbidden-strings.md`.
  2. Build a fully populated `analyzer.Report` value programmatically. Use realistic field values for ID-like fields (use real allowlisted detector IDs from the registry); use the canary values for any field that could plausibly carry private content.
  3. For the `Ecosystem` portion:
     - Populate `MCPServersKnown`, `KnownSkills`, etc. with allowlisted IDs only.
     - Populate `WorkflowFingerprints` with one entry per first-class detector: realistic `confidence`, `sources`, etc.
     - Crucially: do NOT put any canary value into any field of `Ecosystem`. The canaries enter only via the upstream input bytes that *would be* scrubbed/discarded, not via report fields. The test simulates a hostile input that contains the canaries, runs it through `analyzer.Analyze`, and asserts the resulting `Report` contains none of them.
  4. Test body:
     ```go
     func TestReportSerializationContainsNoForbiddenStrings(t *testing.T) {
         canaries := []string{ /* ... 16 values ... */ }
         input := buildHostileInput(canaries) // see helper below
         rep, err := analyzer.Analyze("job-1", input)
         if err != nil { t.Fatal(err) }
         for _, target := range []any{rep, rep.AggregateEvent} {
             b, err := json.Marshal(target)
             if err != nil { t.Fatal(err) }
             s := string(b)
             for _, c := range canaries {
                 if strings.Contains(s, c) {
                     t.Errorf("forbidden canary %q leaked into %T JSON", c, target)
                 }
             }
         }
     }

     func buildHostileInput(canaries []string) []byte {
         var lines []string
         for _, c := range canaries {
             lines = append(lines, `{"user": "`+c+`"}`)
             lines = append(lines, `{"tool_input": "`+c+`"}`)
             lines = append(lines, `{"tool_output": "`+c+`"}`)
             lines = append(lines, c)
         }
         return []byte(strings.Join(lines, "\n"))
     }
     ```
  5. The test lives in the `sdd` package but imports `analyzer`. To avoid an import cycle, place it in `analyzer_test` package (`package analyzer_test` in `analyzer/leak_test.go`). Update `owned_files` accordingly in the WP frontmatter if needed.
- **Important**: this test asserts behavior of `analyzer.Analyze`, not just `sdd.Evaluate`. It's the end-to-end privacy guardrail.

### Subtask T035 — Update existing analyzer tests

- **File**: `internal/analyzer/analyzer_test.go` (edit).
- **Steps**:
  1. Find existing tests asserting unknown MCP/skill counts.
  2. Add an assertion: after the fingerprint pass runs, unknown MCP/skill counts are unchanged.
  3. Add a small test that an input containing only an unknown MCP server name (e.g., `mcp__company_private_42__do_thing`) yields:
     - `Ecosystem.UnknownMCPServerCount = 1`
     - `Ecosystem.WorkflowFingerprints` does not include any entry whose markers match `company_private_42` (because no detector with that ID exists).
     - The serialized report contains the string `"unknown_mcp_server_count":1` but not `"company_private_42"`.

### Subtask T036 — Structural NFR-003 test

- **File**: `internal/analyzer/sdd/structural_test.go` (new, ~80 lines).
- **Steps**:
  1. Use `reflect` to walk `analyzer.EcosystemFingerprint`'s exported fields.
  2. Assert:
     - All fields are basic types (`string`, `bool`, `int`) or `[]string`.
     - No field is `map[K]V`.
     - The exact field name set is `{ID, Confidence, Sources, EvidenceCount, Active, Installed, VersionBucket}`.
  3. Similarly walk `Ecosystem.WorkflowFingerprints` element type and assert the same.
  4. The test fails loudly with an actionable message if anyone adds a new field — the canary leak test must also be updated whenever this happens (cross-reference in the failure message).

## Test Strategy

- `go test ./...` passes after these three tests are added.
- The leak test must run in under 200 ms (small synthetic input, no exec).

## Risks & Mitigations

- **False positive in leak test** if a canary collides with a legitimate registry ID: canaries are chosen to be obviously non-public (e.g., `FORBIDDEN-PROMPT-CANARY`, `jdoe-private`). Real registry IDs are lowercase snake. No collision possible.
- **Scrubber suppresses canary** before analyzer sees it (good for safety, bad for testing the post-scrub leak surface): if the existing `scrubber.go` redacts emails / paths, the canary `private.user@example.internal` and `/Users/robert/private-project` will be replaced with placeholders. That's still a pass — the report contains the placeholder, not the canary. Verify the leak test passes regardless of scrubber behavior.

## Review Guidance

- Reviewer should run the leak test and inspect the failure message format.
- Reviewer should temporarily add a new `string` field to `EcosystemFingerprint` and confirm the structural test fails with a clear message.
- Reviewer should temporarily plant a canary into `EcosystemFingerprint.ID` and confirm the leak test fails.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
