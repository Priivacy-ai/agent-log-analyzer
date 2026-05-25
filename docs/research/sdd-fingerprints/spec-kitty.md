# Spec Kitty (spec_kitty)

- Status: verified
- Category: spec-driven-development orchestrator
- Competitor priority: 1
- Official repository: https://github.com/spec-kitty/spec-kitty (alias of the canonical org); user-facing artifact `kittify` CLI
- Official docs: https://spec-kitty.ai/
- Release / package source: https://pypi.org/project/spec-kitty/ (Python CLI; install yields the `spec-kitty` command)
- Aliases: ["Spec Kitty", "spec-kitty", "kittify", "kitty-specs"]

## Markers (public-source only)

### config_dir
- `.kittify/` — directory created at repo root by `spec-kitty setup`. Observed in this very repo (`.kittify/skills-manifest.json`, `.kittify/config.yaml`, `.kittify/workspaces/`, `.kittify/memory/`). Docs at https://spec-kitty.ai/.
- `kitty-specs/` — directory holding per-mission planning artifacts (`<mission-slug>/spec.md`, `plan.md`, `tasks.md`, `research.md`). Observed in this repo at `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/`.

### config_file
- `.kittify/config.yaml` — Spec Kitty machine config (per-repo). Observed in this repo.
- `.kittify/skills-manifest.json` — manifest of installed skills, including the `spec-kitty-*` family. Observed in this repo.
- `.kittify/command-skills-manifest.json` — command-skill mapping. Observed in this repo.
- `kitty-specs/<mission>/meta.json` — mission metadata. Observed in this repo.

### package_manifest
- PyPI: `spec-kitty` (https://pypi.org/project/spec-kitty/).

### command_name
- `spec-kitty` — primary CLI binary documented at https://spec-kitty.ai/ and installed via `pip install spec-kitty`.
- `kittify` — secondary alias historically associated with the project; the `.kittify/` directory name is the durable evidence.

### slash_command
- `/spec-kitty.specify`, `/spec-kitty.plan`, `/spec-kitty.tasks`, `/spec-kitty.implement`, `/spec-kitty.review`, `/spec-kitty.merge`, `/spec-kitty.accept`, `/spec-kitty.dashboard`, `/spec-kitty.status`, `/spec-kitty.charter`, `/spec-kitty.tasks-packages`, `/spec-kitty.tasks-outline`, `/spec-kitty.tasks-finalize`, `/spec-kitty.analyze`, `/spec-kitty.research` — namespaced under the `spec-kitty.` prefix in the Claude Code slash command surface. Visible in this session's available-skills list.

### mcp_server_name
- (none documented as `mcp__spec_kitty__...`; Spec Kitty uses skills + CLI, not an MCP server)

### skill_name
- `ad-hoc-profile-load`
- `debugger-debbie`
- `spec-kitty-glossary-context`
- `spec-kitty-bulk-edit-classification`
- `spec-kitty-git-workflow`
- `spec-kitty-orchestrator-api-operator`
- `spec-kitty-charter-doctrine`
- `spec-kitty-implement-review`
- `spec-kitty-program-orchestrate`
- `spec-kitty-runtime-next`
- `spec-kitty-runtime-review`
- `spec-kitty-spdd-reasons`
- `spec-kitty-mission-review`
- `spec-kitty-setup-doctor`
- `spec-kitty-mission-system`

Observed at `.claude/skills/spec-kitty-*/SKILL.md` in this repo.

### plugin_manifest
- (not applicable — Spec Kitty installs as skills + CLI; no `.plugin.json`)

### cli_binary
- `spec-kitty` — allowlisted binary.

### cli_version_probe
- args: `["--version"]`
- expected output pattern: `spec-kitty\s+\d+\.\d+(\.\d+)?` → bucket `MAJOR.MINOR`.

## Negative-test markers (must NOT trigger this detector alone)
- `Spec Kit` / `spec-kit` (different product — see `github-spec-kit.md`).
- `OpenSpec` / `openspec` (different product — see `openspec.md`).
- Generic `specs/`, `tasks.md`, `plan.md` (also produced by GitHub Spec Kit and OpenSpec; ambiguous in isolation).
- Mention of "spec kitty" inside an unrelated narrative file with no `.kittify/` or `kitty-specs/` evidence is a Low-confidence signal at most.

## Confidence wiring
- **High**: `.kittify/` config_dir present **and** any of (`kitty-specs/<mission>/spec.md`, a `spec-kitty-*` skill, `/spec-kitty.*` slash command transcript). Or: `cli_binary spec-kitty` installed + `.kittify/config.yaml` present.
- **Medium**: exactly one of (`/spec-kitty.*` slash command in transcript, `cli_binary spec-kitty` installed) without `.kittify/` corroboration.
- **Low**: free-text mention `(?i)\bspec[\s-]?kitty\b` without artifact or CLI evidence.

## Source references (citations)
- https://github.com/Priivacy-ai/spec-kitty/tree/main/src/doctrine/skills — canonical doctrine skill inventory.
- https://spec-kitty.ai/ — product home page and docs (lists CLI, skills, and `.kittify/` workspace layout).
- https://pypi.org/project/spec-kitty/ — package source of the `spec-kitty` CLI.
- This repository, `.kittify/skills-manifest.json` — observed manifest of the `spec-kitty-*` skill family (in-tree evidence).
- This repository, `.claude/skills/spec-kitty-runtime-next/SKILL.md` — example skill file shipped with the tool.
- This repository, `kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/` — example mission directory layout.
