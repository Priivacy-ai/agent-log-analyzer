# Tessl (tessl)

- Status: verified
- Category: spec-driven-development platform; CLI + MCP server with tiles (composable spec/skill packages) and a hosted registry
- Competitor priority: 18
- Official repository: https://github.com/tesslio (organization; see e.g. https://github.com/tesslio/spec-driven-development-tile)
- Official docs: https://docs.tessl.io/
- Release / package source: Tessl CLI is distributed from https://tessl.io/ (and via `tessl cli update`). Tile registry at https://tessl.io/registry/.
- Aliases: ["Tessl", "tessl", "Tessl.io"]

## Markers (public-source only)

### config_dir
- `.tessl/` — main configuration and cache directory. Documented at https://docs.tessl.io/reference/configuration.
- `.tessl/tiles/` — downloaded tile cache organized as `workspace/tile-name/`.

### config_file
- `tessl.json` — repository tile-dependency manifest. Documented at https://docs.tessl.io/reference/configuration.
- `tile.json` — per-tile metadata file (in tile authoring repos).
- `.tessl/RULES.md` — auto-generated agent rules file.
- `.tileignore` — file-exclusion patterns for tiles.

### package_manifest
- (Tessl distributes via its own CLI and registry, not npm/PyPI; `tessl.json` is the equivalent manifest)

### command_name
- `tessl init`, `tessl install`, `tessl mcp start`, `tessl project create`, `tessl skill new`, `tessl tile new`, `tessl workspace create`, `tessl eval run`, etc. — documented at https://docs.tessl.io/reference/cli-commands.

### slash_command
- (Tessl integrates with agents via MCP and per-agent rule files, not Claude Code slash commands.)

### mcp_server_name
- `tessl` — the MCP server is started via `tessl mcp start`. Documented at https://docs.tessl.io/reference/cli-commands. The Claude Code MCP registration appears under the `mcp__tessl__*` namespace (per standard Claude Code MCP-name → tool-name convention).

### skill_name
- (Tessl supports `tessl skill new` / `tessl skill publish` for authoring skills, but the skill names themselves are user-defined.)

### plugin_manifest
- (none documented as a Claude Code plugin manifest)

### cli_binary
- `tessl` — primary CLI binary. Documented at https://docs.tessl.io/reference/cli-commands.

### cli_version_probe
- args: `["--version"]`
- expected output pattern: `tessl[^\d]*(\d+\.\d+)` → bucket `MAJOR.MINOR`.

## Negative-test markers (must NOT trigger this detector alone)
- The English word "tessellate" / "tessellation" — different stem but partial overlap; the pattern must anchor on `\btessl\b` (not `tessl` as a substring).
- Random `tile.json` files in unrelated repos — only count when paired with `.tessl/` or `tessl.json`.
- `.kittify/`, `kitty-specs/`, `.specify/`, `openspec/`, `.kiro/`, `.bmad-core/` — other SDD tools; veto.

## Confidence wiring
- **High**: `.tessl/` directory present **and** any of (`tessl.json`, `tessl` CLI binary present, `tessl mcp start` text).
- **Medium**: `tessl` CLI binary installed, OR `tessl.json` file, OR `mcp__tessl__*` MCP namespace match.
- **Low**: bare `tessl` text mention or `docs.tessl.io` URL.

## Source references (citations)
- https://docs.tessl.io/ — official documentation home.
- https://docs.tessl.io/reference/configuration — documents `.tessl/`, `tessl.json`, `tile.json`, `.tileignore`.
- https://docs.tessl.io/reference/cli-commands — documents the `tessl` CLI surface including `tessl mcp start`.
- https://docs.tessl.io/use/spec-driven-development-with-tessl — documents the SDD workflow with Tessl.
- https://github.com/tesslio/spec-driven-development-tile — official tile published by Tessl.
