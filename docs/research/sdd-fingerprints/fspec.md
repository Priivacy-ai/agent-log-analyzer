# fspec (fspec)

- Status: verified
- Category: spec-driven multi-agent coding factory; auto-generates tests from Gherkin scenarios, enforces TDD with auto checkpoints
- Competitor priority: 13
- Official repository: https://github.com/sengac/fspec
- Official docs: https://fspec.dev/ and https://github.com/sengac/fspec#readme
- Release / package source: https://www.npmjs.com/package/@sengac/fspec (installed via `npm install -g @sengac/fspec`)
- Aliases: ["fspec", "FSPEC", "@sengac/fspec"]

## Markers (public-source only)

### config_dir
- (the fspec source tree uses a `spec/` directory for its own feature specs; this is generic and **must not** be matched alone. The tool's user-installed surface is primarily the CLI and the `/fspec` slash command, not a dedicated config dir.)

### config_file
- (none unique enough to match alone — generic `.gitignore`, `package.json`, `tsconfig.json`)

### package_manifest
- npm `@sengac/fspec` (https://www.npmjs.com/package/@sengac/fspec).
- Reference patterns: `"@sengac/fspec"` in a `package.json`, or `npmjs.com/package/@sengac/fspec` URL.

### command_name
- `fspec init`, `fspec restore-checkpoint`, `fspec report-bug-to-github` — documented CLI subcommands.

### slash_command
- `/fspec` — Claude Code bootstrap slash command, documented in the README.
- `/prompts:fspec` — Codex bootstrap.

### mcp_server_name
- (none documented)

### skill_name
- (none documented)

### plugin_manifest
- (none documented)

### cli_binary
- `fspec` — global CLI binary installed by `npm install -g @sengac/fspec`. Documented in the README.

### cli_version_probe
- args: `["--version"]`
- expected output pattern: `fspec[^\d]*(\d+\.\d+)` → bucket `MAJOR.MINOR`.

## Negative-test markers (must NOT trigger this detector alone)
- Bare `fspec` token without a CLI binary, `@sengac/fspec` package reference, or `/fspec` slash command — many older unrelated projects use the name.
- Generic `spec/` directory — must not match alone.
- The word "fspec" in unrelated contexts (e.g., F# "fspec" testing libraries) — require disambiguating evidence (the `@sengac/` scope or `sengac/fspec` repo path).
- `.kittify/`, `kitty-specs/`, `.specify/`, `openspec/`, `.kiro/`, `.bmad-core/` — other SDD tools; veto.

## Confidence wiring
- **High**: `@sengac/fspec` package reference **and** any of (`/fspec` slash command, `fspec` CLI binary present).
- **Medium**: `fspec` CLI binary installed, OR `/fspec` slash command, OR `@sengac/fspec` package reference (each alone).
- **Low**: bare `fspec init` text mention.

## Source references (citations)
- https://github.com/sengac/fspec — official repository (README documents npm install, CLI binary, `/fspec` slash command).
- https://www.npmjs.com/package/@sengac/fspec — npm package.
- https://fspec.dev/ — official product home.
