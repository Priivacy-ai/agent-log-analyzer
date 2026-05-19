# GSD (gsd)

- Status: verified
- Category: spec-driven-development meta-prompting and context-engineering system for Claude Code (Get Shit Done / GSD by TÂCHES)
- Competitor priority: 6
- Official repository: https://github.com/gsd-build/get-shit-done
- Official docs: https://github.com/gsd-build/get-shit-done/blob/main/docs/USER-GUIDE.md and https://gsd.build/
- Release / package source: https://www.npmjs.com/package/get-shit-done-cc (installed via `npx get-shit-done-cc@latest`)
- Aliases: ["GSD", "Get Shit Done", "get-shit-done", "get-shit-done-cc", "gsd-build"]

## Markers (public-source only)

### config_dir
- `.planning/` — root directory created by GSD to hold session memory, decisions, blockers, phase artifacts, research, spikes, and sketches. Documented at https://github.com/gsd-build/get-shit-done/blob/main/docs/USER-GUIDE.md.
- `.planning/phases/` — per-phase artifact subtrees (e.g., `phases/01-foundation/`).

### config_file
- `PROJECT.md`, `REQUIREMENTS.md`, `ROADMAP.md`, `STATE.md`, `CONTEXT.md`, `REVIEW.md` — top-level markdown projections. **Never match these names alone** (FR-012): generic file names. Only count when paired with `.planning/`.

### package_manifest
- npm `get-shit-done-cc` (https://www.npmjs.com/package/get-shit-done-cc).

### command_name
- `npx get-shit-done-cc` invocation in scrubbed transcripts. Documented in README quick-start.

### slash_command
- `/gsd-new-project`, `/gsd-discuss-phase`, `/gsd-plan-phase`, `/gsd-execute-phase`, `/gsd-verify-work`, `/gsd-ship`, `/gsd-progress`, `/gsd-quick`, `/gsd-ui-phase`, `/gsd-spike`, `/gsd-complete-milestone`, `/gsd-new-milestone`, `/gsd-map-codebase`, `/gsd-settings`, `/gsd-code-review` — Claude Code slash commands. Documented in the USER-GUIDE.

### mcp_server_name
- (none documented)

### skill_name
- (Skills install to `~/.claude/skills/gsd-*/` per the USER-GUIDE, but the path itself is private; skill names alone are not enumerated as a stable public surface.)

### plugin_manifest
- (none documented as a stand-alone Claude Code plugin manifest)

### cli_binary
- (no globally installed binary documented; invocation is via `npx get-shit-done-cc`)

### cli_version_probe
- (not applicable; no global binary)

## Negative-test markers (must NOT trigger this detector alone)
- Bare three-letter token `GSD` — extremely common acronym ("Get Shit Done" productivity framework, unrelated tools). Must NOT match on the token alone.
- Generic `STATE.md` / `PROJECT.md` / `ROADMAP.md` / `REQUIREMENTS.md` filenames alone — extremely generic; require pairing with `.planning/` or `get-shit-done-cc` or a `/gsd-*` slash command.
- The unrelated `gsd` npm package (file-system watcher) — different product.
- `.kittify/`, `kitty-specs/`, `.specify/`, `openspec/`, `.kiro/`, `.bmad-core/` — other SDD tools; veto.

## Confidence wiring
- **High**: `.planning/` directory present **and** any of (`/gsd-*` slash command, `get-shit-done-cc` npm reference).
- **Medium**: any single `/gsd-*` slash command OR `get-shit-done-cc` package reference, with no other corroboration.
- **Low**: bare `npx get-shit-done-cc` text mention only.

## Source references (citations)
- https://github.com/gsd-build/get-shit-done — official repository (README + USER-GUIDE document `.planning/`, slash commands, npm package).
- https://github.com/gsd-build/get-shit-done/blob/main/docs/USER-GUIDE.md — user guide with the canonical command list.
- https://www.npmjs.com/package/get-shit-done-cc — npm package.
- https://gsd.build/ — official product home.
