---
work_package_id: WP09
title: Documentation + GitHub issue hygiene
dependencies:
- WP04
requirement_refs:
- FR-013
- FR-015
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T037
- T038
- T039
- T040
- T041
phase: Phase 5 — Docs
agent: "claude:opus-4.7:reviewer-renata:reviewer"
shell_pid: "14321"
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: curator-carla
authoritative_surface: docs/
execution_mode: planning_artifact
owned_files:
- docs/sdd-fingerprint-registry.md
- docs/ecosystem-signatures.md
- docs/data-retention-and-analytics.md
- docs/logging-policy.md
- kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/github-issue-comments.md
role: curator
tags: []
---

# Work Package Prompt: WP09 — Documentation + GitHub issue hygiene

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.

## Objectives & Success Criteria

- `docs/sdd-fingerprint-registry.md` is a complete maintainer-facing guide.
- The three existing docs are updated with pointers and policy text.
- A file at `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/github-issue-comments.md` holds ready-to-post comment text for issues #38 / #42 / #43 / #44–#48 / #49 / #50 / #66 / #67.

## Context & Constraints

- Read: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/*` for source-of-truth content.
- Read: existing `docs/ecosystem-signatures.md`, `docs/data-retention-and-analytics.md`, `docs/logging-policy.md` for tone and structure.
- Do NOT duplicate prose; the doc should reference contracts and spec sections.

## Subtasks & Detailed Guidance

### Subtask T037 — `docs/sdd-fingerprint-registry.md`

- **Sections**:
  1. **Overview** — what the registry is, why it exists, the privacy stance.
  2. **Top-20 table** — id, display name, status, link to per-tool research file.
  3. **Source-class taxonomy** — the 10 source classes from FR-006.
  4. **Confidence levels** — the three tiers from FR-007 with rule summaries.
  5. **Status semantics** — `verified` / `research_needed` / `blocked` from FR-015. Note C-001 (this mission ships zero in `research_needed`).
  6. **CLI probe privacy rules** — link to `contracts/cli-probe.md`.
  7. **What must never be uploaded** — the 16 categories from `contracts/forbidden-strings.md`.
  8. **How to add a detector** — link to `quickstart.md`.
  9. **Spec Kitty vs GitHub Spec Kit vs OpenSpec** — explicit note that these three products are distinct and never conflate.
- **Length target**: ~250 lines.

### Subtask T038 — `docs/ecosystem-signatures.md`

- **Steps**:
  1. Add a short "See also" section near the top: "The SDD fingerprint registry (`docs/sdd-fingerprint-registry.md`) is the canonical source for spec-driven-development tool detection. This file continues to document the broader ecosystem-signature pattern for the legacy `WorkflowFrameworks []string` field."
  2. Note that `WorkflowFrameworks` is preserved for back-compat per C-004 but new SDD detection lives in `WorkflowFingerprints`.

### Subtask T039 — `docs/data-retention-and-analytics.md`

- **Steps**:
  1. Add a section "SDD fingerprint privacy" that:
     - Confirms `WorkflowFingerprints` records carry only the seven bounded fields.
     - Confirms unknown MCP/skill/plugin names remain counts only (FR-011).
     - References the leak test as the build-time enforcement (NFR-001).
     - Re-states the 16 forbidden-string categories at a high level (link to the contract for the full list).

### Subtask T040 — `docs/logging-policy.md`

- **Steps**:
  1. Add a section "CLI presence and version probes" that:
     - Forbids logging resolved executable paths.
     - Forbids logging raw `--version` output.
     - Restates the `version_args` deny-list.
     - Notes the 2-second timeout requirement.

### Subtask T041 — GitHub issue comment text

- **File**: `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/github-issue-comments.md` (new).
- **Steps**: produce ready-to-post Markdown for each issue named by the brief. One section per issue. Each section has:
  - Status (e.g., "Starting work" / "Implementation complete" / "Schema implemented" / "CLI probing implemented" / "Research summary").
  - One paragraph summary.
  - Bullet list of evidence (PR links, commit hashes — leave placeholders that the WP10 implementer fills in).
- **Issues to cover**: #38 start, #38 done, #42 (schema), #43 (top-20 seed), #44 (Spec Kitty), #45 (GitHub Spec Kit), #46 (OpenSpec), #47 (Kiro/BMAD/GSD), #48 (long-tail), #49 (confidence + evidence), #50 (privacy tests), #66 (research summary; note any `research_needed` if it ended up that way per A-04), #67 (CLI probing).
- **Do not post** the comments here — that's a human/WP10 step.

## Test Strategy

No code tests. The acceptance criterion is "a new maintainer can land a new detector entry by reading these docs alone".

## Risks & Mitigations

- **Drift between docs and code**: prefer linking to file paths and contracts (`internal/analyzer/sdd/...`, `contracts/sdd-detector.schema.json`) rather than copying field lists.

## Review Guidance

- Reviewer should walk through `quickstart.md` while consulting `docs/sdd-fingerprint-registry.md` and verify a new detector could be added with no other reference material.
- Reviewer should grep the four docs for any inadvertent copies of forbidden raw-string examples.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
- 2026-05-19T07:37:44Z – claude:opus-4.7:curator-carla:curator – shell_pid=10970 – Started implementation via action command
- 2026-05-19T07:42:30Z – claude:opus-4.7:curator-carla:curator – shell_pid=10970 – Maintainer-facing registry doc + 3 related-doc updates + GH issue comment template
- 2026-05-19T07:42:51Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=14321 – Started review via action command
- 2026-05-19T07:45:02Z – claude:opus-4.7:reviewer-renata:reviewer – shell_pid=14321 – Review passed: registry doc covers 9 sections; top-20 table reflects 9 verified / 11 research_needed; cross-product docs link to it; GH comment templates ready for WP10.
