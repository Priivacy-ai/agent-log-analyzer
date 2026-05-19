# SDD Pilot (sdd_pilot)

- Status: verified
- Category: spec-driven-development pilot/workflow framework for AI coding agents (Specify → Plan → Tasks → Implement → QC)
- Competitor priority: 8
- Official repository: https://github.com/attilaszasz/sdd-pilot
- Official docs: https://sdd-pilot.szaszattila.com/
- Release / package source: Distributed as versioned platform archives (e.g., `sdd-pilot-copilot-vX.Y.Z.zip`) per the GitHub releases page. No npm/PyPI package.
- Aliases: ["SDD Pilot", "sdd-pilot", "sddp"]

## Markers (public-source only)

### config_dir
- (none unique to SDD Pilot; it uses generic `specs/` and `.agents/`. **Do not** trigger on `specs/` alone — FR-012.)

### config_file
- `.github/sddp-config.md` — shared project context file. Documented in the SDD Pilot repository.

### package_manifest
- (no npm/PyPI; distribution is via release archives)

### command_name
- `sddp-` filename prefix in `.claude/commands/`, `.codex/commands/`, etc. — recognizable command-pack identifier.

### slash_command
- `/sddp-prd`, `/sddp-specify`, `/sddp-clarify`, `/sddp-plan`, `/sddp-checklist`, `/sddp-tasks`, `/sddp-analyze`, `/sddp-implement`, `/sddp-qc`, `/sddp-autopilot`, `/sddp-systemdesign`, `/sddp-devops`, `/sddp-projectplan`, `/sddp-amend`, `/sddp-init`, `/sddp-devsetup` — documented in the SDD Pilot repository.

### mcp_server_name
- (none documented)

### skill_name
- (none documented as a registered Claude Code skill)

### plugin_manifest
- `gemini-extension.json` is a Gemini-specific extension manifest; not Claude Code plugin shape.

### cli_binary
- (no installable CLI binary documented)

### cli_version_probe
- (not applicable)

## Negative-test markers (must NOT trigger this detector alone)
- Bare `specs/` directory — generic SDD shared artifact.
- Bare `.agents/` directory — generic.
- `prd.md`, `sad.md`, `dod.md`, `project-plan.md` filenames alone — generic.
- The English word "pilot" — generic.
- `.kittify/`, `kitty-specs/`, `.specify/`, `openspec/`, `.kiro/`, `.bmad-core/` — other SDD tools; veto.

## Confidence wiring
- **High**: any `/sddp-*` slash command **and** `.github/sddp-config.md` config file (two distinct source classes).
- **Medium**: any single `/sddp-*` slash command match.
- **Low**: bare textual mention of `sddp-` command name.

## Source references (citations)
- https://github.com/attilaszasz/sdd-pilot — official repository (README documents the workflow and `/sddp-*` command set).
- https://sdd-pilot.szaszattila.com/ — official documentation site.
