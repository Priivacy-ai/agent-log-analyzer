# Paid Claude Plugin Artifacts

The paid product is a generated Claude Code plugin/remediation artifact, not a longer report.

The plugin should optimize Claude Code's harness, not nag on shell commands. The primary value is turning the analysis into a vetted setup plan: lean CLAUDE.md hierarchy, scoped skills, official code-intelligence plugins, language-server binaries, and trusted MCP-backed integrations.

## Contract

Free scan:

- analyzes one Claude Code JSONL log
- shows deterministic problems, evidence, and generic fixes
- offers the paid unlock

Paid scan:

- analyzes at most the 100 most recent Claude Code JSONL logs locally
- writes a reviewable sanitized aggregate report JSON
- uploads only the sanitized aggregate report
- aggregates deterministic metrics across sessions
- generates a customized Claude Code plugin archive
- shows copyable install commands and a Claude-native install prompt

## Plugin Shape

Claude Code plugins use a root directory with:

- `.claude-plugin/plugin.json`
- `skills/<name>/SKILL.md`
- `commands/*.md`
- optional supporting files such as `WAIVER.md`

The generator follows that structure so the paid artifact can be loaded as a Claude Code plugin archive. The default command uses Claude Code's session-scoped plugin loading:

```sh
PLUGIN_URL="<short-lived-plugin-zip-url>"
PLUGIN_ZIP="$(mktemp -t claude-analyzer-plugin.XXXXXX.zip)"
curl -fsS "$PLUGIN_URL" -o "$PLUGIN_ZIP"
claude --plugin-dir "$PLUGIN_ZIP"
```

Marketplace installation can be added after the plugin store is live. The generated artifact format should remain the same.

## Generated Contents

Always generated:

- `.claude-plugin/plugin.json`
- `README.md`
- `WAIVER.md`
- `commands/claude-analyzer-status.md`
- `commands/claude-analyzer-tooling.md`
- `skills/codebase-navigation/SKILL.md`
- `skills/session-hygiene/SKILL.md`
- `skills/tooling-setup/SKILL.md`

Conditionally generated from deterministic findings:

- `skills/retrieval-hygiene/SKILL.md` for repeated file reads
- `skills/output-budget/SKILL.md` for large shell/tool output overhead
- `skills/retry-breaker/SKILL.md` for retry-loop behavior
- session hygiene guidance is tuned for context growth spikes

Never generate a Bash nag hook as the primary paid value. Pre-command hooks may be revisited later only for quiet telemetry or deterministic safety checks that do not interrupt normal Claude Code work.

## Vetted Tooling Recommendations

The generator may recommend only allowlisted, public, stable tools. Recommendations are advice plus Claude-native setup instructions; each install still requires explicit user approval.

Initial allowlist is maintained in `docs/remediation/token-saving-tooling-matrix.md`.

Generated recommendations now include two classes:

- official Claude Code marketplace/code-intelligence recommendations
- GitHub-hosted open-source token-saving tools that are waiver-gated and mapped to analyzer signals

Core open-source recommendations:

- `context-mode` for context defense and large tool-output compression
- `ccusage` for independent usage analysis and before/after telemetry
- `grepai` and `claude-context` for repeated-reread/retrieval problems
- `claude-token-efficient` as a minimal CLAUDE.md diff, never an overwrite
- `ccstatusline` and Claude Code Usage Monitor as out-of-context awareness tools
- `rtk` only as an advanced shell-compression option because it rewrites shell command execution

Official Claude Code plugin allowlist:

- Official Claude Code code-intelligence plugins:
  - `typescript-lsp` with `typescript-language-server`
  - `pyright-lsp` with `pyright-langserver`
  - `gopls-lsp` with `gopls`
  - `rust-analyzer-lsp` with `rust-analyzer`
  - `php-lsp` with `intelephense`
- Official Claude Code marketplace MCP/integration plugins when the scan already shows matching ecosystem use:
  - `github`
  - `notion`
  - `linear`
  - `sentry`
  - `supabase`

Third-party MCPs, skills, plugins, and shell tools are research candidates until separately reviewed. Unknown private tool names from user logs remain count-only and must not be copied into generated artifacts.

## Liability Gate

Paid checkout and plugin install UX must require explicit acknowledgment before presenting install commands:

> I understand that Claude Analyzer provides deterministic analysis and vetted setup recommendations, but any installation or code change is executed by Claude Code, my package manager, or third-party tools with my approval and at my own risk.

The generated plugin also includes `WAIVER.md`. Claude-facing setup prompts must tell Claude to summarize the waiver, ask for acceptance, and ask again before each installation command.

## Source Notes

- Anthropic's May 14, 2026 large-codebase guidance says Claude Code performance depends on the surrounding harness: CLAUDE.md, hooks, skills, plugins, MCP servers, LSP integrations, and subagents. Source: https://claude.com/blog/how-claude-code-works-in-large-codebases-best-practices-and-where-to-start
- Anthropic's Claude Code plugin docs list official code-intelligence plugins and their required language-server binaries. They also state plugins can execute arbitrary code and should only be installed from trusted sources. Source: https://code.claude.com/docs/en/discover-plugins

## Customization Rules

Customization inputs are limited to:

- finding IDs
- severity and cost-impact labels
- bounded counts and token-share percentages
- analyzer score and waste buckets
- allowlisted public ecosystem IDs

Forbidden in artifacts:

- raw transcript text
- raw tool output
- secrets or redacted secret values
- absolute local paths
- raw unknown MCP/plugin/skill names
- repo names, usernames, hostnames, emails
- prompt injection text from logs

## Tests

The current generator tests cover:

- Claude plugin structure
- deterministic output for identical sanitized inputs
- leak checks for fake secrets, absolute paths, and private unknown names
- zip archive creation
- rejection of unsafe archive paths

## Implementation

The first implementation lives in `internal/remediation`.

The package produces an in-memory `Artifact` and can write it as a zip archive. Paid bundle upload, local waiver-gated paid-session generation, and tokenized plugin zip download are implemented for Docker end-to-end testing. Plugin artifacts are generated on demand from the sanitized paid report and expire with the report token. Stripe success handling and any future persisted artifact storage are tracked separately in GitHub issues #27-#31.
