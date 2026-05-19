---
work_package_id: WP06
title: Documentation
dependencies:
- WP04
requirement_refs:
- FR-012
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this mission were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T022
- T023
- T024
- T025
agent: claude
history:
- event: generated
  at: '2026-05-19T08:00:33Z'
  by: /spec-kitty.tasks
agent_profile: curator-carla
authoritative_surface: docs/ecosystem-signatures.md
execution_mode: code_change
mission_id: 01KRZJGVG3MCCCY9MKB1YRDBQR
mission_slug: mcp-skill-bloat-analytics-01KRZJGV
owned_files:
- docs/ecosystem-signatures.md
- docs/data-retention-and-analytics.md
- docs/logging-policy.md
- docs/testing-plan.md
role: curator
tags: []
---

## ⚡ Do This First: Load Agent Profile

Before reading anything else, load the agent profile by invoking `/ad-hoc-profile-load` with `profile_id: "curator-carla"` and `role: "curator"`. This WP is about maintaining documentation consistency, taxonomy, and cross-links — a curator's job. Then return here.

## Objective

Document the new MCP/skill bloat analytics surface so a maintainer reading only the docs can answer:
1. What does each new metric mean?
2. How is the context footprint estimated?
3. What do the buckets mean?
4. When does each warning band fire?
5. Why are unknown private names counts only?
6. How does this differ from Epic #38's SDD fingerprint registry?

This WP must NOT include private content (no private names, real schemas, real paths) — same privacy stance as the runtime.

## Context

Read first:
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/spec.md` — FR-012 (the four docs to update), SC-6 (what readers must be able to answer).
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md` — §D-1..D-5 for all the decisions to summarize.
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md` — full reference for buckets, enums, invariants.
- `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json` — the JSON Schema.
- Existing docs that you will modify:
  - `docs/ecosystem-signatures.md`
  - `docs/data-retention-and-analytics.md`
  - `docs/logging-policy.md`
  - `docs/testing-plan.md`

Branch contract:
- Planning base: `main`. Merge target: `main`. Lane base resolved from `lanes.json`.

## Detailed Guidance

### Subtask T022 — Update `docs/ecosystem-signatures.md`

**Purpose**: This is the primary user-facing reference for what the analyzer detects. Add a new section explaining the MCP/skill utilization metrics.

**Steps**:
1. Read the existing file end-to-end first (don't paste content into the middle of a section without reading the surrounding flow).
2. Add a new top-level section titled `## MCP and Skill Utilization (Epic #39)` at a natural location (after the existing MCP-detection section if there is one, otherwise after the SDD-fingerprint section).
3. The new section must cover:
   - **What this is**: One paragraph explaining the question the metric answers ("Are the user's MCPs and skills actually being used enough to justify the context they add?").
   - **The fields**: A description of each top-level key under `tooling_utilization.mcp` and `tooling_utilization.skill`, with one sentence per field. Match `data-model.md` and link to it.
   - **Buckets**: A small table reproducing the closed enumerations:
     | Bucket family | Values |
     |---------------|--------|
     | Count buckets | `none`, `1-3`, `4-10`, `11-25`, `26-50`, `51-100`, `100+`, `unknown` |
     | Token buckets | `none`, `<1k`, `1k-5k`, `5k-15k`, `15k-50k`, `50k+`, `unknown` |
     | Efficiency | `unused`, `underutilized`, `moderate`, `well-utilized`, `unknown` |
     | Warning bands | `normal`, `watch`, `high`, `severe`, `unknown` |
   - **Footprint estimator**: brief explanation that local schema text length is preferred when present, else fixed per-item constants (cite the numeric constants from plan §D-2: 150 tokens per MCP tool, 250 per MCP server overhead, 400 per skill), else `unknown`.
   - **Band thresholds**: a prose summary of D-4. Do not reproduce the full numeric table — link to `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md#d-4` for the authoritative source. But do state the **product rule**: "Bands never fire on count alone. A user with many MCPs but high utilization is `normal`."
   - **Privacy stance**: short paragraph stating that unknown/private MCP and skill names are counted only — never stored, logged, or emitted in any aggregate output. Link to `data-retention-and-analytics.md` for the full policy.
   - **Differences from Epic #38**: one paragraph clarifying that #38 is the SDD (Spec-Driven Development) tool fingerprint registry — focused on identifying which tools the user has installed — while #39 is the utilization analytics, focused on whether installed MCPs/skills are actually used. Both can coexist in the same report.

4. Cross-link to the spec and plan files.

**Files**:
- `docs/ecosystem-signatures.md` (extend).

**Validation**:
- [ ] All six SC-6 questions can be answered by reading the new section.
- [ ] No private names appear (use generic phrasing like "a private MCP server name" or "an internal company MCP").
- [ ] Cross-links resolve (relative paths to spec.md, plan.md, data-model.md).

### Subtask T023 — Update `docs/data-retention-and-analytics.md`

**Purpose**: Document the aggregate shape changes — specifically that `AggregateSafeEvent.Ecosystem.tooling_utilization` is now part of the upload contract, with every string field bounded.

**Steps**:
1. Read the existing file.
2. Add a section (or extend an existing "aggregate event" section) describing:
   - The `tooling_utilization` block is included in `AggregateSafeEvent.Ecosystem`.
   - Every string value in that block is from a closed enumeration (list them: count buckets, token buckets, efficiency buckets, warning bands, inference sources) or the public allowlist of MCP/skill IDs.
   - **No free-form strings** are ever included. Unknown private names are counts only.
   - The full schema is at `kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json`.
3. Add a "what we never collect" sub-list quoting the forbidden 18 categories from spec C-001 (user prompts, task descriptions, raw transcript excerpts, raw tool inputs/outputs, raw MCP schemas/descriptions/arguments, MCP server URLs, auth scopes, private MCP/tool names, private skill names, skill instruction text, skill examples, user-authored skill docs, raw file paths, repo URLs, branch names, usernames, hostnames, emails, session IDs, transcript paths, stable hashes of any private string).

**Files**:
- `docs/data-retention-and-analytics.md` (extend).

**Validation**:
- [ ] The new section is consistent with `docs/ecosystem-signatures.md` (no contradictions).
- [ ] The 18 forbidden categories appear explicitly.

### Subtask T024 — Update `docs/logging-policy.md` (cross-link)

**Purpose**: Ensure the logging policy doc cross-links to the new metrics doc so anyone investigating "what gets logged about MCP usage?" finds the answer.

**Steps**:
1. Read the existing file.
2. Add a brief cross-reference in the appropriate section (likely near where existing ecosystem signals are discussed):
   ```markdown
   ## MCP and Skill Utilization

   Analyzer aggregate events include privacy-safe utilization metrics for MCP servers and skills. See [ecosystem-signatures.md#mcp-and-skill-utilization-epic-39](./ecosystem-signatures.md#mcp-and-skill-utilization-epic-39) for what's measured and [data-retention-and-analytics.md](./data-retention-and-analytics.md) for the upload contract. Private MCP/skill names are counted only; nothing identifying is logged, stored, or uploaded.
   ```
3. Do not duplicate content from the other docs — link to them.

**Files**:
- `docs/logging-policy.md` (extend, minimally).

**Validation**:
- [ ] Cross-link added in a natural location.
- [ ] Cross-link is consistent with the other docs.

### Subtask T025 — Update `docs/testing-plan.md`

**Purpose**: Document the 7 fixture scenarios from WP05 so a future contributor can extend them.

**Steps**:
1. Read the existing file.
2. Add a new section titled `## MCP and Skill Bloat Fixtures (Epic #39)`:
   - List the 7 fixtures (`00-empty` through `07-mixed-known-unknown`) with a one-line description of what each one exercises.
   - State that fixtures live under `internal/analyzer/testdata/tooling/`.
   - State the two test functions: `TestGoldenToolingFixtures` (band/bucket assertions) and `TestPrivacyLeakCorpus` (zero-substring privacy assertions).
   - State the rule that the forbidden-substring list in `TestPrivacyLeakCorpus` must stay in sync with whatever synthetic names are introduced in fixtures `06` and `07`.
3. Add a note that all fixture content is synthetic — no real user logs.

**Files**:
- `docs/testing-plan.md` (extend).

**Validation**:
- [ ] All 7 fixtures named with their scenario.
- [ ] Test function names match what's in `internal/analyzer/golden_test.go`.

## Test Strategy

Documentation has no automated test. Validation is human read-through against SC-6 (the six questions a maintainer must be able to answer). Cross-links can be tested via a markdown link checker, but that's not required.

## Definition of Done

- [ ] All four docs updated, each containing the content described above.
- [ ] Cross-links between docs resolve (relative paths).
- [ ] No private names or real product names appear in examples — use generic phrasing.
- [ ] SC-6 reads: a maintainer reading only these docs can answer (a) what each bucket means, (b) when each band fires, (c) why unknown names are counts only, (d) how this differs from #38, (e) what the footprint estimator does, (f) what gets uploaded and what doesn't.

## Risks

- **Risk**: Drift between docs and code as the implementation evolves. **Mitigation**: link to the canonical sources (`data-model.md`, `contracts/tooling-utilization.json`, plan.md §D-4) rather than reproducing values inline wherever possible.
- **Risk**: Accidentally introducing a private name as an example. **Mitigation**: explicit guideline — examples must use generic terms like "a private MCP server" or `acme_internal_example`, never a real product or company name.

## Reviewer Guidance

When reviewing:
- Read each updated doc cold (without referring to the spec) and check whether you can answer the SC-6 questions.
- Click each cross-link; broken links should fail review.
- Verify no real product names appear in examples.
- Verify the docs do not contradict each other (e.g., one says the band thresholds are X and another says Y).

## Implementation Command

```bash
spec-kitty agent action implement WP06 --agent claude
```

## Activity Log

- 2026-05-19T09:34:48Z – claude – Moved to done
