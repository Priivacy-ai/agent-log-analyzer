# spec2ship (spec2ship)

- Status: verified
- Category: spec-driven, multi-agent Claude Code plugin for moving from specs to shipping via roundtable collaboration
- Competitor priority: 10
- Official repository: https://github.com/spec2ship/spec2ship
- Official docs: https://github.com/spec2ship/spec2ship#readme
- Release / package source: Distributed as a Claude Code plugin via `/plugin marketplace add spec2ship/spec2ship` and `/plugin install s2s`. No npm/PyPI package.
- Aliases: ["spec2ship", "Spec2Ship", "s2s"]

## Markers (public-source only)

### config_dir
- `.s2s/` — workspace directory created by the plugin to hold context, sessions, and backlog. Documented in the repository README.
- `.s2s/sessions/` — per-session yaml artifacts (`YYYYMMDD-specs-*.yaml`).

### config_file
- `.s2s/CONTEXT.md` — session-spanning context.
- `.s2s/BACKLOG.md` — backlog projection.

### package_manifest
- (no npm/PyPI; the tool is a Claude Code plugin)

### command_name
- (no global CLI binary; commands are slash commands inside Claude Code)

### slash_command
- `/s2s:init`, `/s2s:brainstorm`, `/s2s:specs`, `/s2s:design`, `/s2s:plan`, `/s2s:roundtable`, `/s2s:session`, `/s2s:session:list`, `/s2s:session:validate`, `/s2s:session:close` — Claude Code slash commands documented in the repository.

### mcp_server_name
- (none documented)

### skill_name
- (none documented)

### plugin_manifest
- `.claude-plugin/` directory at the spec2ship plugin install location. The plugin is registered through the Claude Code marketplace install path; the marketplace command `/plugin install s2s` is the canonical install signal.

### cli_binary
- (none)

### cli_version_probe
- (not applicable)

## Negative-test markers (must NOT trigger this detector alone)
- Bare `s2s` token — could be ship-to-ship, server-to-server, etc. Must NOT match on the token alone.
- Bare `CONTEXT.md` or `BACKLOG.md` filenames — generic.
- `.kittify/`, `kitty-specs/`, `.specify/`, `openspec/`, `.kiro/`, `.bmad-core/`, `.planning/`, `.spec-workflow/`, `.paul/`, `.tessl/` — other SDD tools; veto.

## Confidence wiring
- **High**: `.s2s/` directory present **and** any of (`/s2s:*` slash command, `.s2s/CONTEXT.md`).
- **Medium**: any single `/s2s:*` slash command match.
- **Low**: bare textual mention of `spec2ship/spec2ship` repo path or `/plugin install s2s`.

## Source references (citations)
- https://github.com/spec2ship/spec2ship — official repository (README documents `.s2s/`, slash commands, plugin install).
