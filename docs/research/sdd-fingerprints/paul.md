# PAUL (paul)

- Status: verified
- Category: structured AI-assisted development framework for Claude Code (Plan-Apply-Unify Loop)
- Competitor priority: 12
- Official repository: https://github.com/ChristopherKahler/paul
- Official docs: https://github.com/ChristopherKahler/paul#readme
- Release / package source: https://www.npmjs.com/package/paul-framework (installed via `npx paul-framework`)
- Aliases: ["PAUL", "paul", "paul-framework", "Plan-Apply-Unify Loop"]

## Markers (public-source only)

### config_dir
- `.paul/` — workspace directory created by PAUL to hold project state, roadmap, phases, and config. Documented in the repository README.
- `.paul/phases/` — per-phase artifact subtrees (e.g., `01-foundation/`, `02-features/`).

### config_file
- `.paul/PROJECT.md`, `.paul/ROADMAP.md`, `.paul/STATE.md`, `.paul/config.md`, `.paul/SPECIAL-FLOWS.md` — top-level workspace files inside `.paul/`. Documented in the README.

### package_manifest
- npm `paul-framework` (https://www.npmjs.com/package/paul-framework).

### command_name
- `npx paul-framework` invocation in scrubbed transcripts.

### slash_command
- `/paul:init`, `/paul:plan`, `/paul:apply`, `/paul:unify`, `/paul:progress`, `/paul:help` — Claude Code slash commands (the framework registers 26 commands total; documented in README).

### mcp_server_name
- (none documented)

### skill_name
- (none documented)

### plugin_manifest
- (none documented as a stand-alone plugin)

### cli_binary
- (no globally installed binary documented; invocation is via `npx paul-framework`)

### cli_version_probe
- (not applicable)

## Negative-test markers (must NOT trigger this detector alone)
- The proper name "Paul" — extremely common. Must NOT match on the name alone.
- Generic `STATE.md` / `PROJECT.md` / `ROADMAP.md` outside `.paul/` — generic; require pairing.
- `.kittify/`, `kitty-specs/`, `.specify/`, `openspec/`, `.kiro/`, `.bmad-core/`, `.planning/`, `.s2s/` — other SDD tools; veto.

## Confidence wiring
- **High**: `.paul/` directory present **and** any of (`/paul:*` slash command, `paul-framework` npm reference).
- **Medium**: any single `/paul:*` slash command OR `paul-framework` package reference.
- **Low**: bare `npx paul-framework` text mention.

## Source references (citations)
- https://github.com/ChristopherKahler/paul — official repository (README documents `.paul/`, slash commands, npm install).
- https://www.npmjs.com/package/paul-framework — npm package.
