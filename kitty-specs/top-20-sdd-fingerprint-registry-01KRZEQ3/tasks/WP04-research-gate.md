---
work_package_id: WP04
title: 'Research gate: clone + verify top-20 SDD tools'
dependencies: []
requirement_refs:
- FR-005
- FR-013
- FR-014
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning on main; implementation lands on codex/sdd-fingerprint-registry; final merge target main.
subtasks:
- T015
- T016
- T017
- T018
- T019
- T020
phase: Phase 2 — Research
agent: claude
history:
- at: '2026-05-19T06:35:00Z'
  actor: system
  action: Prompt generated via /spec-kitty.tasks
agent_profile: researcher-robbie
authoritative_surface: docs/research/sdd-fingerprints/
execution_mode: planning_artifact
owned_files:
- docs/research/sdd-fingerprints/**
- docs/sdd-fingerprint-registry.md
role: researcher
tags: []
---

# Work Package Prompt: WP04 — Research gate: clone + verify top-20 SDD tools

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the
frontmatter, and behave according to its guidance before parsing the rest of this
prompt.

---

## Branch Strategy

- Planning base: `main`. Merge target: `main`. Implementation lands on `codex/sdd-fingerprint-registry`.

## Objectives & Success Criteria

Produce one verified research file per top-20 SDD tool under `docs/research/sdd-fingerprints/<tool-id>.md`. Each file MUST:

- Cite at least one public source URL (`source_references`).
- Record at least one tool-specific marker that distinguishes this tool from all 19 others.
- Identify generic markers that MUST NOT trigger this detector alone.
- Note whether a `cli_binary` is publicly documented and what `version_args` are safe.
- Carry a status of `verified` (public-source citations present) or `research_needed` (cannot be verified from public sources).

Per C-001: every tool must end `verified`. If a tool genuinely has no public fingerprintable surface, stop and ask the user (do not silently downgrade).

## Context & Constraints

- Read: `research.md` §R-01 (template), §R-02 (clone procedure), §R-09 (initial seed table).
- Public sources only: official repos, public docs, install/init flows run in a disposable scratch directory.
- Cloning location: `/Users/robert/code-analyzer-dev/claude-code-analyzer-20260519-082245-0QWuF7/research/<tool-id>/` — **outside the repo**. Never commit cloned third-party source.
- Important distinctions:
  - **Spec Kitty** (this project's own tool family) ≠ **GitHub Spec Kit** ≠ **OpenSpec**. Each requires explicit tool-specific markers and negative-test markers blocking the others.
  - Be careful with `BMAD-METHOD` — there is an existing `frameworks.json` entry `bmad`; reuse the same ID.

## Subtasks & Detailed Guidance

### Subtask T015 — Scaffold per-tool research directory

- **Steps**:
  1. Create `docs/research/sdd-fingerprints/` with a `README.md` that explains:
     - The methodology (link to `research.md` §R-01).
     - The status semantics (`verified` vs `research_needed`).
     - That citations live inline in each file.
  2. Create one stub file per top-20 tool. Use these slugs:
     `spec-kitty.md`, `github-spec-kit.md`, `openspec.md`, `kiro.md`, `bmad.md`, `gsd.md`, `spec-workflow-mcp.md`, `sdd-pilot.md`, `spec-driven-develop.md`, `spec2ship.md`, `chatdev.md`, `paul.md`, `fspec.md`, `whenwords.md`, `intent.md`, `cognition-devin.md`, `microsoft-agent-framework.md`, `tessl.md`, `agentic-code.md`, `codespeak.md`.
  3. Each stub starts as a copy of the template in `research.md` §R-01 with `status: research_needed`. Subtasks T016–T019 fill them in.
- **Files**: new directory + 21 files.

### Subtask T016 — First-class trio (Spec Kitty / GitHub Spec Kit / OpenSpec)

- **Purpose**: These three are the brief's must-not-conflate set. Most important.
- **Steps for each**:
  1. Locate official repository and docs URLs.
  2. Clone repo into the research clone area (no commit).
  3. Read README, docs, install/init flow output.
  4. Record markers per source class with citation. At a minimum each tool needs:
     - One `config_dir` or `config_file` marker that is unique to this tool.
     - One `slash_command` or `cli_binary` marker.
  5. List generic terms that MUST NOT trigger this tool alone (these become `negative` markers in the registry entry or are excluded from patterns).
  6. Set `status: verified`. Add `source_references`.
- **Files**: `docs/research/sdd-fingerprints/spec-kitty.md`, `github-spec-kit.md`, `openspec.md`.

### Subtask T017 — Second-ring trio (Kiro / BMAD-METHOD / GSD)

- **Steps**: same shape as T016 for `kiro.md`, `bmad.md`, `gsd.md`.
- Note BMAD detail: ID is `bmad`, alias `BMAD-METHOD`. Cite `bmad-code-org/bmad-method` repo.

### Subtask T018 — Long-tail group 1 (6 tools)

- **Steps**: same shape for `spec-workflow-mcp.md`, `sdd-pilot.md`, `spec-driven-develop.md`, `spec2ship.md`, `chatdev.md`, `paul.md`.
- Note: `spec-workflow-mcp` is documented as an MCP server, so `mcp_server_name` is its primary marker. ChatDev is OpenBMB's multi-agent framework — distinguish from generic "chat" patterns.

### Subtask T019 — Long-tail group 2 (8 tools)

- **Steps**: same shape for `fspec.md`, `whenwords.md`, `intent.md`, `cognition-devin.md`, `microsoft-agent-framework.md`, `tessl.md`, `agentic-code.md`, `codespeak.md`.
- Note: `intent.md` is a particularly generic name. Find specific markers (config file name, package name) and treat plain "Intent" mentions as low confidence at most.
- `cognition-devin` may be hosted product rather than a CLI tool; record markers from public docs (e.g., known UI strings, public API names).

### Subtask T020 — `docs/sdd-fingerprint-registry.md`

- **Purpose**: Cross-link the per-tool research files and explain the registry shape to future maintainers.
- **Steps**:
  1. Create `docs/sdd-fingerprint-registry.md` with sections:
     - Overview (what the registry is and why it exists).
     - Source-class taxonomy (10 classes; same definitions as `data-model.md`).
     - Confidence levels (3 tiers; same rules as `research.md` §R-05).
     - Status semantics (`verified` / `research_needed` / `blocked`).
     - CLI probe privacy rules (link to `contracts/cli-probe.md`).
     - The 16 forbidden raw-string categories (link to `contracts/forbidden-strings.md`).
     - Top-20 table linking to each per-tool research file.
  2. The detailed implementation of this doc continues in WP09 task T037. WP04's job is the cross-link skeleton.
- **Files**: `docs/sdd-fingerprint-registry.md` (new, skeleton ~80 lines; WP09 expands).

## Test Strategy

No code tests in this WP. The quality bar is:

- Every tool's research file has at least one citation.
- Every `verified` tool has at least one tool-specific marker.
- A peer reviewer can follow each citation and reach a public source confirming the marker.

## Risks & Mitigations

- **A tool has no public fingerprintable surface** (e.g., proprietary hosted-only product with no public CLI or config file): stop and consult with the user. Do not invent markers. Mark the file `research_needed`, set the parent task status to blocked, and surface this in the WP10 activity log so C-001 can be revisited.
- **Generic naming collisions** (e.g., "Intent", "Agentic Code"): aggressively use negative-test markers and only count tool-specific config/CLI markers as confirmation.
- **Repo move / rename**: pin a commit hash in the citation if uncertain.

## Review Guidance

- Reviewer should sample 5 random tools and click the citation links.
- Reviewer should run `grep -l "status: verified" docs/research/sdd-fingerprints/*.md | wc -l` and confirm it returns 20.

## Activity Log

- 2026-05-19T06:35:00Z -- system -- Prompt created.
