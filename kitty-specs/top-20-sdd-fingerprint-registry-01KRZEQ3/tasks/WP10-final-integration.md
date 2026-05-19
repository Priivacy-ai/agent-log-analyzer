---
work_package_id: WP10
title: Final integration, smoke, branch hygiene
dependencies:
- WP07
- WP08
requirement_refs:
- FR-001
- FR-002
- FR-003
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T042
- T043
- T044
phase: Phase 6 — Integration
agent: claude
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: implementer-ivan
authoritative_surface: internal/analyzer/
execution_mode: code_change
owned_files:
- internal/analyzer/golden_test.go
role: implementer
tags: []
---

# Work Package Prompt: WP10 — Final integration, smoke, branch hygiene

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. **Implementation commits go to `codex/sdd-fingerprint-registry` per the brief.** Create the branch in this WP if it does not already exist (per the brief: `git switch -c codex/sdd-fingerprint-registry`; use a timestamped suffix if the branch already exists).

## Objectives & Success Criteria

- Golden test in `internal/analyzer/golden_test.go` updated for the new `WorkflowFingerprints` field shape (still tolerant of `omitempty`).
- `gofmt -w` + `go test ./...` + `./scripts/smoke-local.sh` run clean.
- All changes consolidated on `codex/sdd-fingerprint-registry`, pushed to origin.
- GitHub issue comments (from WP09) posted with commit hash and PR placeholders filled in.

## Context & Constraints

- Read: existing `internal/analyzer/golden_test.go` for the assertion pattern.
- The brief's Definition of Done is the acceptance bar for this WP.
- A-06: if `./scripts/smoke-local.sh` is unrelated or blocked, document the reason and still run `go test ./...`.

## Subtasks & Detailed Guidance

### Subtask T042 — Update `golden_test.go`

- **Steps**:
  1. Open `internal/analyzer/golden_test.go`.
  2. The existing test serializes a known `Report` and compares against a golden file. With the new `WorkflowFingerprints` field at `omitempty`:
     - If the test's fixture input contains no SDD markers, the field is absent from output — golden file unchanged. Verify.
     - If the fixture happens to contain SDD markers (some existing tests use sanitized Claude Code transcripts that may incidentally trigger Spec Kitty or similar), update the golden file to include the expected `workflow_fingerprints` array, deterministically sorted.
  3. Re-run the golden test; if it fails, update the golden artifact and commit.
  4. Also extend the assertion to validate that any `workflow_fingerprints` entry in the golden output has the seven-field shape (no surprises).

### Subtask T043 — Format + tests + smoke

- **Steps**:
  1. `gofmt -w $(find . -name '*.go' -not -path './.git/*')`.
  2. `go vet ./...`.
  3. `go test ./...` — must pass.
  4. `./scripts/smoke-local.sh` — must pass. If blocked, document the exact blocker in the activity log and proceed with `go test ./...` only (per A-06).
  5. Manually verify: pick a small sanitized Claude Code transcript (e.g., `testdata/` if it exists, or construct a synthetic one) and run the analyzer CLI against it. Inspect the output to confirm `workflow_fingerprints` appears with expected entries and no private content.

### Subtask T044 — Commit, push, GitHub comments

- **Steps**:
  1. From repo root:
     ```sh
     git switch -c codex/sdd-fingerprint-registry  # or timestamped suffix if exists
     git status --short
     git add internal/analyzer/sdd/ internal/analyzer/signatures/sdd_detectors.json \
             internal/analyzer/{ecosystem.go,types.go,registry.go,analyzer_test.go,golden_test.go} \
             docs/research/sdd-fingerprints/ docs/sdd-fingerprint-registry.md \
             docs/ecosystem-signatures.md docs/data-retention-and-analytics.md docs/logging-policy.md
     git commit -m "Add SDD fingerprint registry (epic #38)"
     git push -u origin codex/sdd-fingerprint-registry
     ```
  2. Capture the resulting commit hash.
  3. Open `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/github-issue-comments.md` (created in WP09) and fill in the commit hash and PR URL placeholders.
  4. Post each comment to its respective GitHub issue using `gh issue comment <issue-number> --body-file <inline-body.md>`. The brief lists the issues: #38, #42, #43, #44, #45, #46, #47, #48, #49, #50, #66, #67.
  5. Do NOT close any issue. The brief is explicit: closes only when acceptance criteria are actually satisfied.

## Test Strategy

- The complete test suite from WP01–WP09 is the implicit test strategy for this WP.
- Smoke test confirms end-to-end CLI behavior.

## Risks & Mitigations

- **Smoke test failure unrelated to this work**: document the failure mode, file a separate issue, and proceed if the failure is clearly orthogonal.
- **Branch already exists**: use a timestamped suffix per the brief.
- **Golden test churn**: keep changes minimal — only update for the new field.

## Review Guidance

- Reviewer should run `go test ./...` locally and confirm green.
- Reviewer should verify the branch `codex/sdd-fingerprint-registry` exists on origin and contains the consolidated commit.
- Reviewer should spot-check that the posted GitHub issue comments do NOT contain private content from the developer's machine (paths, usernames, hostnames). The brief's privacy stance applies to issue comments too.

## Definition of Done (Mission-level checklist for this WP)

- [ ] Registry schema supports privacy-safe typed fingerprints.
- [ ] All 20 SDD tools seeded with `verified` status (C-001).
- [ ] Spec Kitty, GitHub Spec Kit, OpenSpec separate with cross-negative tests.
- [ ] CLI fingerprinting implemented behind allowlisted probes.
- [ ] Unknown/private tool names remain counts only.
- [ ] Aggregate output contains no raw private evidence.
- [ ] Docs explain registry and privacy model.
- [ ] `go test ./...` passes.
- [ ] Smoke test passes or blocker documented.
- [ ] Work committed and pushed.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
