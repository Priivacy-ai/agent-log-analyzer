---
work_package_id: WP05
title: Engine documentation (new doc)
dependencies: []
requirement_refs:
- FR-022
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning base main; can run in parallel with WP01 per lanes.json (independent owned_files).
subtasks:
- T026
- T027
- T028
- T029
- T030
agent: claude
history:
- 2026-05-19: created from mission token-saving-recommendation-engine-phase-a-01KRZKCJ
agent_profile: curator-carla
authoritative_surface: docs/remediation/token-saving-recommendation-engine.md
execution_mode: planning_artifact
owned_files:
- docs/remediation/token-saving-recommendation-engine.md
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

Write the new `docs/remediation/token-saving-recommendation-engine.md` doc
that covers (a) token-saving tool classes, (b) allowlist policy, (c) the
dedupe-aware recommendation contract, (d) how installed/configured/active/
rejected differ, (e) risk levels and install policies, (f) the waiver gate
requirement, (g) privacy constraints, and (h) the Phase B integration plan
for issues #38 (fingerprint registry) and #39 (MCP/skill utilization).

This is a brand-new file. The doc must read well in isolation and link to
the registry source + spec/plan/research instead of duplicating their
content.

## Branch Strategy

Planning base branch: `main`. Final merge target: `main`. This WP can run
in parallel with WP01–WP04 because it owns a distinct file. The
lane-assigned worktree is in `lanes.json`.

## Context

References to draw from (do **not** duplicate verbatim):

- `spec.md` (FR-022 spells out what this doc must cover)
- `plan.md` §"Project Structure" (cite the new files)
- `research.md` (decisions for every section)
- `data-model.md` (the canonical state model)
- `contracts/token_saving_engine_go_api.md` (link, don't restate)

## Owned files

This WP owns and is the only writer of:

- `docs/remediation/token-saving-recommendation-engine.md` (new)

**Do not edit** existing `docs/remediation/token-saving-tooling-matrix.md`
or `docs/remediation/plugin-artifacts.md` — those belong to WP06.

## Implementation command

```bash
spec-kitty agent action implement WP05 --agent claude
```

---

### Subtask T026 — Skeleton + ToC [P]

**Purpose.** Stand up the file with a clear scope statement.

**Steps.**

1. Create `docs/remediation/token-saving-recommendation-engine.md`.
2. H1: `# Token-Saving Recommendation Engine`.
3. Add a one-paragraph **Scope** statement: this doc covers the additive
   recommendation-engine surface introduced in Phase A of issue #68. Link
   to the spec and the brief. State that Phase B will wire #38 and #39
   inputs.
4. Add a Table of Contents listing the seven sections (Classes, Allowlist
   Policy, State Model, Recommendation Contract, Risk & Install Policy,
   Privacy, Phase B Plan).

**Validation.** Markdown renders cleanly in any GitHub-flavoured viewer.

---

### Subtask T027 — Classes + allowlist policy [P]

**Purpose.** Describe the eight `RecommendationClass` values and the
allowlist policy.

**Steps.**

1. Section "Token-saving tool classes": list the eight classes with a
   one-paragraph description each (usage_visibility, mcp_skill_hygiene,
   mcp_output_reducer, shell_output_reducer, retrieval, reread_guard,
   context_hygiene, output_verbosity). Mention which signals each class
   responds to.
2. Section "Allowlist policy": explain that the registry is a Go literal
   at `internal/analyzer/token_saving_tools.go`; tools outside it are
   counted but never recommended. Note that adding or removing entries
   bumps `RegistryVersion()`. Explicitly cross-reference research.md's
   per-tool notes and remind readers that any unverified URL ships as
   `research_only` with an empty `source_url`.

**Validation.** Cross-check each class name against
`data-model.md` §"RecommendationClass".

---

### Subtask T028 — State model + recommendation contract [P]

**Purpose.** Make the dedupe-aware contract obvious to a non-engineer
stakeholder.

**Steps.**

1. Section "State model": describe the six `ToolState` values and the
   conflict precedence (`rejected_medium > active_high > … > unknown`).
2. Section "Recommendation contract": describe the fixed 8-step rule
   precedence (point at research.md §3 for the canonical list). Explain
   the **≤ 1 primary + ≤ 1 secondary** invariant, the **different-class
   secondary** rule, and what `Skipped` entries mean.
3. Include a small worked example in prose:
   "If the analyzer reports `shell_output_bloat` and Serena is already
   `active_high`, the engine emits `rtk` as the primary (subject to its
   state) and skips Serena from the retrieval class because the signal
   it would have been recommended for is unrelated to shell-output
   bloat." (Adjust to a realistic walked example after writing the test
   suite in WP04.)

**Validation.** State enum names match `data-model.md` exactly.

---

### Subtask T029 — Risk, install policy, waiver gate [P]

**Purpose.** Document the operational contract.

**Steps.**

1. Section "Risk levels": list `low`/`medium`/`high` with one-sentence
   guidance each ("low: standard plugin install; medium: requires user
   approval; high: rewrites shell/proxy/MCP behaviour and requires
   waiver gate").
2. Section "Install policies": list the five values
   (`bundle`/`recommend`/`recommend_with_waiver`/`research_only`/`reference_only`)
   and explain what the engine actually does for each (e.g.
   `research_only` is **never** emitted as a default recommendation;
   `reference_only` is documentation-only, never installed).
3. Section "Waiver gate": describe the user-facing requirement. The
   engine surfaces `install_policy = recommend_with_waiver` in the
   recommendation JSON; the caller (paid-pack generator, plugin runtime)
   is responsible for the waiver UI.

**Validation.** All five `InstallPolicy` values present and spelled
correctly.

---

### Subtask T030 — Privacy + Phase B plan [P]

**Purpose.** Lock the privacy contract and point at Phase B work.

**Steps.**

1. Section "Privacy constraints": state that recommendation JSON contains
   only allowlisted enum strings, registered `ToolID` values, structural
   JSON characters, and integer counts. Describe the positive-list
   scanner test (`TestRecommendPrivacyBudget`) as the enforcement
   mechanism. Reproduce the list of forbidden data classes from
   `spec.md`'s "Non-Negotiable Privacy Stance".
2. Section "Phase B integration plan": describe how Phase B will populate
   `ToolStateMap` from #38 (fingerprint registry) and #39 (MCP/skill
   utilization analytics) without touching the engine. State the four
   public functions (`Recommend`, `GetTool`, `AllTools`, `RegistryVersion`)
   are the only surface Phase B needs. Note that #67 (safe CLI presence
   and version probes) will populate `EvidenceCLIPresence` /
   `EvidenceCLIVersion` entries.

**Validation.** Document is internally consistent with spec.md, plan.md,
research.md, data-model.md, contracts/.

---

## Definition of Done

- [ ] `docs/remediation/token-saving-recommendation-engine.md` exists
      with sections matching the seven topics above.
- [ ] Every enum string mentioned in the doc matches the value in
      `data-model.md` exactly.
- [ ] All internal links resolve (`internal/analyzer/token_saving_tools.go`,
      `spec.md`, `research.md`, `contracts/token_saving_engine_go_api.md`).
- [ ] Document is < 500 lines.
- [ ] No other file is modified.

## Risks & reviewer guidance

- Reviewer should compare every enum string in the doc against
  `data-model.md`; a single typo (`config_medium` vs `configured_medium`)
  rots silently.
- Keep the doc narrative — don't reproduce field-by-field tables from
  data-model.md.

## Out of scope for WP05

- Editing the existing matrix doc or plugin-artifacts doc (WP06).
- Editing any source file.
