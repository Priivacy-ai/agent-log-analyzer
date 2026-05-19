---
work_package_id: WP07
title: Seed long-tail detectors (14 tools)
dependencies:
- WP06
requirement_refs:
- FR-005
- FR-006
- FR-012
- FR-014
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T030
- T031
- T032
- T033
phase: Phase 3 — Seed detectors
agent: "claude:opus-4.7:reviewer-renata:reviewer"
shell_pid: "25752"
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/sdd/testdata/
execution_mode: code_change
owned_files:
- internal/analyzer/signatures/sdd_detectors_long_tail.json
- internal/analyzer/sdd/testdata/fixtures/spec_workflow_mcp.txt
- internal/analyzer/sdd/testdata/fixtures/sdd_pilot.txt
- internal/analyzer/sdd/testdata/fixtures/spec_driven_develop.txt
- internal/analyzer/sdd/testdata/fixtures/spec2ship.txt
- internal/analyzer/sdd/testdata/fixtures/chatdev.txt
- internal/analyzer/sdd/testdata/fixtures/paul.txt
- internal/analyzer/sdd/testdata/fixtures/fspec.txt
- internal/analyzer/sdd/testdata/fixtures/whenwords.txt
- internal/analyzer/sdd/testdata/fixtures/intent.txt
- internal/analyzer/sdd/testdata/fixtures/cognition_devin.txt
- internal/analyzer/sdd/testdata/fixtures/microsoft_agent_framework.txt
- internal/analyzer/sdd/testdata/fixtures/tessl.txt
- internal/analyzer/sdd/testdata/fixtures/agentic_code.txt
- internal/analyzer/sdd/testdata/fixtures/codespeak.txt
- internal/analyzer/sdd/evaluator_long_tail_test.go
role: implementer
tags: []
---

# Work Package Prompt: WP07 — Seed long-tail detectors (14 tools)

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.

## Objectives & Success Criteria

- `sdd_detectors_long_tail.json` gains 14 `verified` entries: `spec_workflow_mcp`, `sdd_pilot`, `spec_driven_develop`, `spec2ship`, `chatdev`, `paul`, `fspec`, `whenwords`, `intent`, `cognition_devin`, `microsoft_agent_framework`, `tessl`, `agentic_code`, `codespeak`.
- 14 new fixtures under `testdata/fixtures/`.
- Positive-detection tests for each of the 14.
- Cross-negative assertions: no long-tail tool triggers any first-class or second-ring detector.

## Context & Constraints

- Read research files for each of the 14 tools under `docs/research/sdd-fingerprints/`.
- All markers come from the public-source citations in those files. No guessed markers.
- Issue: #48 (Detector group: second-ring SDD tools).
- Special generic-name risks: "Intent" and "Agentic Code" are extremely generic terms. The detector entry MUST anchor on a tool-specific config file, CLI name, or package manifest entry; mere textual mention earns `low` confidence at most.

## Subtasks & Detailed Guidance

### Subtask T030 — Seed first 7 long-tail tools

- **Tools**: `spec_workflow_mcp`, `sdd_pilot`, `spec_driven_develop`, `spec2ship`, `chatdev`, `paul`, `fspec`.
- **Steps**:
  1. Create `internal/analyzer/signatures/sdd_detectors_long_tail.json` (new tier file owned by this WP, starts as `[]`). The loader globs `sdd_detectors*.json` and picks it up automatically.
- **For each tool**:
  1. Add a `verified` detector entry derived from the corresponding research file.
  2. Each entry needs at least one tool-specific marker plus, where applicable, a `cli_binary` or `mcp_server_name`.
  3. Set `competitor_priority` in the 7–13 range, ordered by general visibility.
  4. Add the fixture file `testdata/fixtures/<id>.txt` with hand-crafted scrubbed content that contains the tool-specific markers and nothing else.
- **Specific guidance**:
  - `spec_workflow_mcp`: primary marker is `mcp_server_name` matching `mcp__spec[_-]workflow__`. Add an MCP-server-name fixture, not a config-dir fixture.
  - `chatdev`: from OpenBMB. Distinguish from generic "chat" — anchor on `ChatDev` exact phrasing and the repo's known directory names.
  - `paul`: confirm correct casing and any tool-specific config name from research.
  - `spec2ship`: anchor on `spec2ship` as a CLI name plus any tool-specific config.

### Subtask T031 — Seed remaining 7 long-tail tools

- **Tools**: `whenwords`, `intent`, `cognition_devin`, `microsoft_agent_framework`, `tessl`, `agentic_code`, `codespeak`.
- **Steps**: same shape as T030.
- **Specific guidance**:
  - `intent`: extremely generic name. Use case-sensitive anchors and require a tool-specific config or package manifest entry (e.g., `package.json` containing `"intent-cli"` or similar from research). Mere mentions of the word "intent" must not trigger.
  - `cognition_devin`: Devin is a hosted product. Markers may include known UI strings, an `mcp__devin__` server name, or public CLI binary `devin`. Confirm in research.
  - `microsoft_agent_framework`: primary markers are package-manifest entries (`@microsoft/agent-framework` or the equivalent NuGet package) and slash commands published by the framework.
  - `tessl`: small footprint. Use `cli_binary: "tessl"` and the tool's specific config file name from research.
  - `agentic_code`: generic name; same precautions as `intent`.
  - `codespeak`: anchor on `codespeak` repo / CLI name and `cli_version_probe`.

### Subtask T032 — Positive-detection tests

- **Steps**:
  1. Extend the fixture-driven matrix in `evaluator_long_tail_test.go` with one entry per long-tail tool:
     ```go
     {"spec_workflow_mcp.txt", "spec_workflow_mcp"},
     {"sdd_pilot.txt", "sdd_pilot"},
     ...
     {"codespeak.txt", "codespeak"},
     ```
  2. For each fixture, assert exactly one fingerprint at the expected confidence (use the research's recorded confidence target — usually `medium` for long-tail tools).

### Subtask T033 — Cross-negative assertions

- **Steps**:
  1. For each long-tail fixture, assert NONE of:
     - `spec_kitty`, `github_spec_kit`, `openspec`
     - `kiro`, `bmad`, `gsd`
     fires.
  2. For each first-class and second-ring fixture from earlier WPs, assert NONE of the 14 long-tail detectors fires.
  3. This is a Cartesian assertion sweep, but `assertNotPresent(t, fps, id)` for each (fixture, id) pair is cheap.
- **Output**: a one-line statement at the top of the test like `// 14 fixtures × 6 cross-negative targets + 6 first/second-ring fixtures × 14 cross-negative targets = 168 cross-negative assertions in this WP.`

## Test Strategy

- `go test ./internal/analyzer/sdd/...` passes with the 168+ added assertions.
- Spot-check fixtures for private content — none must appear.

## Risks & Mitigations

- **Generic-name false positives** (especially "Intent" and "Agentic Code"): tightly anchor markers; rely on `package_manifest` / `config_file` rather than bare textual mentions.
- **Tool research is thin** for some entries: if research surfaces no truly tool-specific marker, the entry must remain `research_needed` (which then violates C-001 and triggers a scope conversation with the user). Do not invent.
- **Test runtime regression**: adding 14 fixtures grows test runtime modestly; the registry-level matching is O(detectors × markers × text-length) and should remain sub-second per fixture.

## Review Guidance

- Reviewer should scan the 14 entries for any marker whose regex would plausibly match the four other-WP fixtures. Anything in doubt: add a Negative marker.
- Reviewer should verify the cross-negative assertions are explicit and not just absent — i.e., the test calls `assertNotPresent` for every other-tool ID, not just "non-target detectors don't fire by happy accident".

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
- 2026-05-19T08:06:26Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=24848 – Started implementation via action command
- 2026-05-19T08:14:23Z – claude:opus-4.7:implementer-ivan:implementer – shell_pid=24848 – 4 verified long-tail detectors shipped; 10 deferred per FR-013. Total registry: 3 first-class + 2 second-ring + 4 long-tail = 9 detectors. C-001 scope decision pending at mission-review.
- 2026-05-19T08:15:01Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=25752 – Started review via action command
- 2026-05-19T08:16:33Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=25752 – Review passed: 4 verified long-tail detectors (spec_workflow_mcp, chatdev, cognition_devin, microsoft_agent_framework); generic-name false-positive risks mitigated via tool-specific anchors; cross-negative matrix vs prior 5 detectors holds; 10 unverified tools properly deferred per FR-013. Scope decision pending at mission-review.
