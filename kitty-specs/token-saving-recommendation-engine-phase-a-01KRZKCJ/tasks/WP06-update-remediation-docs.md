---
work_package_id: WP06
title: Update existing remediation docs
dependencies:
- WP05
requirement_refs:
- FR-019
- FR-021
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T031
- T032
- T033
agent: "claude:opus-4-7:reviewer-rina:reviewer"
shell_pid: "47410"
history:
- '2026-05-19': created from mission token-saving-recommendation-engine-phase-a-01KRZKCJ
agent_profile: curator-carla
authoritative_surface: docs/remediation/token-saving-tooling-matrix.md
execution_mode: planning_artifact
owned_files:
- docs/remediation/token-saving-tooling-matrix.md
- docs/remediation/plugin-artifacts.md
role: curator
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading the rest of this prompt, load the assigned agent profile:

```text
/ad-hoc-profile-load curator-carla
```

Then continue with **Objective** below.

## Objective

Additively update `docs/remediation/token-saving-tooling-matrix.md` and
`docs/remediation/plugin-artifacts.md` so they cross-reference the new
registry and the dedupe-aware recommendation contract from WP05. **Do not
rewrite existing content.** Every change is additive: new paragraphs, new
"See also" lines, no deletions, no table rewrites.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. Rebase onto
WP05's merged head per `lanes.json` (so the link target file already
exists).

## Context

Read both existing docs in full before editing:

- `docs/remediation/token-saving-tooling-matrix.md` (current tier
  tables + recommendation mapping)
- `docs/remediation/plugin-artifacts.md` (current paid-plugin guidance)
- `docs/remediation/token-saving-recommendation-engine.md` (the new doc
  from WP05 — link target)

Also reference `spec.md` FR-019 and FR-021 for what counts as additive.

## Owned files

This WP owns and is the only writer of:

- `docs/remediation/token-saving-tooling-matrix.md`
- `docs/remediation/plugin-artifacts.md`

**Do not edit** any source file or any other docs file.

## Implementation command

```bash
spec-kitty agent action implement WP06 --agent claude
```

---

### Subtask T031 — Update the tooling matrix

**Purpose.** Tie the existing tier doc to the new code registry without
breaking what's already there.

**Steps.**

1. Open `docs/remediation/token-saving-tooling-matrix.md`. Read it
   completely; note every existing heading. Do not edit any of them.
2. Append (at the end of the document, after the existing "Guardrails"
   section) a new section titled
   `## Registry cross-reference (Phase A)`. Inside, add 2–4 paragraphs:
   - Point at `internal/analyzer/token_saving_tools.go` as the
     canonical machine-readable registry.
   - Note that the matrix doc is human reference; the registry is what
     the engine actually consults.
   - State the dedupe-aware recommendation contract briefly (≤ 1 primary
     + ≤ 1 secondary; tools that are `active_high` for a signal are
     skipped, not re-recommended).
   - Cross-reference `docs/remediation/token-saving-recommendation-engine.md`
     for the full state model and rule precedence.
3. **Do not modify** any existing tier table row, paragraph, or
   recommendation-mapping table.

**Validation.** `git diff` should show **additions only** — no deletions
of any non-whitespace content. Reviewer should `git diff --stat` and
expect a single file added-line count, zero deleted-line count.

---

### Subtask T032 — Update `plugin-artifacts.md`

**Purpose.** Note that paid plugin artifacts can now embed an additive
`TokenSavingRecommendation` object without breaking existing tests.

**Steps.**

1. Open `docs/remediation/plugin-artifacts.md`. Read it completely.
2. Append a new section at the end titled
   `## Token-saving recommendation embedding (Phase A, additive)`.
   2-3 paragraphs:
   - Phase A introduces `TokenSavingRecommendation` (link the contract).
   - The recommendation object is optional in paid plugin artifacts and
     may be added without breaking current artifact-shape tests.
   - Refer readers to the new engine doc for the full contract.
3. **Do not modify** any existing section, table, or example. No
   reorderings.

**Validation.** Same `git diff` rule: additions only.

---

### Subtask T033 — Cross-reference lines

**Purpose.** Two-way discoverability between the three remediation docs.

**Steps.**

1. In **both** updated files, add or extend a `## See also` section at
   the very end with bullets pointing to the other two:
   - `docs/remediation/token-saving-recommendation-engine.md` (always present after WP05)
   - The sibling existing doc (matrix ↔ plugin-artifacts).
2. If a `See also` section already exists in either file, append to it
   without removing existing bullets.

**Validation.** Each updated file has a single `## See also` section near
the end with at least the two new bullets.

---

## Definition of Done

- [ ] `docs/remediation/token-saving-tooling-matrix.md` has the new
      "Registry cross-reference" section and updated `See also`.
- [ ] `docs/remediation/plugin-artifacts.md` has the new "Token-saving
      recommendation embedding" section and updated `See also`.
- [ ] `git diff` shows additions only (no deletions of existing
      non-whitespace content).
- [ ] All internal links resolve.
- [ ] No file outside `owned_files` is modified.

## Risks & reviewer guidance

- Easy to inadvertently rewrite a tier-table row when adding the
  cross-reference paragraph at the bottom; the reviewer should diff with
  `git diff --diff-filter=M` (modified) and visually confirm zero rows
  vanished.
- The link to the new engine doc must be a path relative to the
  docs/remediation/ directory (i.e., the filename only).

## Out of scope for WP06

- Writing the new engine doc (WP05).
- Editing any source file.
- Restructuring or trimming existing tier tables.

## Activity Log

- 2026-05-19T09:37:11Z – claude:opus-4-7:curator-carla:curator – shell_pid=47016 – Started implementation via action command
- 2026-05-19T09:38:51Z – claude:opus-4-7:curator-carla:curator – shell_pid=47016 – Additive cross-references in matrix + plugin-artifacts docs; new Registry cross-reference and Recommendation embedding sections; See also links updated; no existing content modified
- 2026-05-19T09:39:06Z – claude:opus-4-7:reviewer-rina:reviewer – shell_pid=47410 – Started review via action command
- 2026-05-19T09:39:59Z – claude:opus-4-7:reviewer-rina:reviewer – shell_pid=47410 – Review passed: purely additive (28 insertions, 0 deletions) across both owned docs; new Registry cross-reference and Token-saving recommendation embedding sections present with correct registry path, ≤1+≤1 contract, active_high skip note, and bidirectional See also links.
- 2026-05-19T10:03:10Z – claude:opus-4-7:reviewer-rina:reviewer – shell_pid=47410 – Moved to done
