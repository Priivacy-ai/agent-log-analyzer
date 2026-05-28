# Generated Plugin Artifacts

The launch product is one local scan, a free report pack, and a free generated Claude Code plugin/remediation artifact delivered after the email gate.

The plugin should optimize Claude Code's harness, not nag on shell commands. The primary value is turning the analysis into a benchmark-backed setup plan: lean CLAUDE.md hierarchy, scoped skills, output-budgeted commands, retrieval hygiene, session/retry breaks, official code-intelligence plugins, and only the conditional reducers that worked in repeated runs.

## Contract

Public scan:

- analyzes target-sized recent logs per supported source
- shows deterministic problems, evidence, and generic fixes
- offers a free report pack from the same sanitized report

Email-gated plugin delivery:

- reuses the same sanitized report from the public scan
- generates a customized Claude Code plugin archive from that report
- shows copyable install commands and a Claude-native install prompt
- records the email address and marketing opt-in flag before showing download links
- emails both download links plus the Spec Kitty training voucher reminder

The public flow must not ask users to tar, gzip, or upload raw Claude Code JSONL
logs. Any raw-log bundle or email/full-scan endpoint is legacy/internal
smoke coverage only and must require an explicit internal request path.

## Plugin Shape

Claude Code plugins use a root directory with:

- `.claude-plugin/plugin.json`
- `skills/<name>/SKILL.md`
- `commands/*.md`
- optional supporting files such as `WAIVER.md`

The generator follows that structure so the generated artifact can be installed as a Claude Code plugin archive. The default command uses persistent Claude Code plugin installation, then asks the user to run `/agent-analyzer-status` for immediate verification:

```sh
PLUGIN_URL="<private-plugin-zip-url>"
PLUGIN_ZIP="$(mktemp -t agent-analyzer-plugin.XXXXXX.zip)"
curl -fsS "$PLUGIN_URL" -o "$PLUGIN_ZIP"
claude plugin install "$PLUGIN_ZIP"
```

Session-scoped preview remains available with `claude --plugin-dir "$PLUGIN_ZIP"`, but user-facing install copy should not lead with it. Marketplace installation can be added after the plugin store is live. The generated artifact format should remain the same.

## Generated Contents

Always generated:

- `.claude-plugin/plugin.json`
- `README.md`
- `START_HERE.md`
- `WAIVER.md`
- `TOOL-CATALOG.json`
- `commands/agent-analyzer-status.md`
- `commands/agent-analyzer-tooling.md`
- `skills/codebase-navigation/SKILL.md`
- `skills/session-hygiene/SKILL.md`
- `skills/tooling-setup/SKILL.md`

Conditionally generated from deterministic findings:

- `skills/retrieval-hygiene/SKILL.md` for repeated file reads
- `skills/output-budget/SKILL.md` for large shell/tool output overhead
- `skills/retry-breaker/SKILL.md` for retry-loop behavior
- session hygiene guidance is tuned for context growth spikes

Never generate a Bash nag hook as the primary product value. Pre-command hooks may be revisited later only for quiet telemetry or deterministic safety checks that do not interrupt normal Claude Code work.

## Vetted Tooling Recommendations

The generator may recommend only allowlisted, public, stable tools that survived the repeated benchmark for the matching waste category. Recommendations are advice plus Claude-native setup instructions; each install still requires explicit user approval.

Initial allowlist is maintained in `docs/remediation/token-saving-tooling-matrix.md`.

Generated recommendations now include three classes:

- built-in Agent Analyzer workflow recommendations that require no third-party install
- official Claude Code marketplace/code-intelligence recommendations
- conditional GitHub-hosted token-saving tools that are waiver-gated, mapped to analyzer signals, and labeled by token category
- measurement-only tools documented as telemetry, not as reducers

Default pack:

- Agent Analyzer workflow: `-12,370` estimated tokens, `-12,698` tool-output tokens, and `23.986%` lower published API-rate cost in three fresh runs
- output-budgeted commands
- retrieval hygiene
- session hygiene
- retry breaker

Conditional third-party reducers:

- `Semble` for repeated file reads and bounded retrieval; `41.5%` published API-rate savings in the repeated fixture
- `context-mode` for tool-output/input-context defense; `20.4%` savings, with visible output rising on average
- `RTK (Rust Token Killer, rtk-ai/rtk)` for explicit shell-output compression; `18.2%` savings; global hooks require separate approval because it rewrites shell command execution; never install npm `rtk`, which is an unrelated package
- `grepai` for path-constrained compact retrieval; `14.5%` savings

Measurement-only tools:

- `ccusage`
- `ccstatusline`
- Claude Code usage monitors or trackers

Removed from default token-saving recommendations:

- `claude-context`: increased published API-rate cost by `26.0%` in this fixture
- `Probe`: increased published API-rate cost by `16.6%`
- `Caveman` for Claude Code: reduced visible output but increased estimated/tool-output tokens and cost
- `claude-rlm`: increased analyzer-estimated tokens and tool output in this medium-context fixture; root stdout cost was incomplete for sub-agent usage
- `claude-token-efficient`: only `1.8%` repeated API-rate savings, too small for default install advice
- `Squeez`: conflicts with Spec Kitty workflows; keep the old benchmark result visible as historical evidence, but do not recommend or emit it in generated artifacts

Generated copy must not blur output tokens and reasoning tokens. Output tokens are visible assistant text/tool calls. Reasoning tokens are hidden model work reported by some harnesses and should only be claimed when measured. Terse-response tools can reduce output tokens while still increasing full-session cost.

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

## Machine-Readable Install Catalog

Live install testing showed that prose-only setup guidance made agents infer too much: slash commands were mistaken for shell commands, marketplace CLI setup had to be discovered manually, and verification steps were inconsistent. Every generated plugin now includes `TOOL-CATALOG.json` so agents can execute a deterministic detect -> install -> verify loop.

The catalog contains:

- `install_order`: explicit phases: waiver, platform detection, binary checks, marketplace setup, plugin install, reload/restart, verification.
- `install_interactive`: Claude Code slash-command form for a human inside the interactive UI.
- `install_cli`: Bash-safe `claude plugin install ...` form for agents and scripts.
- `marketplace_cli`: Bash-safe `claude plugin marketplace add ...` form when a third-party marketplace is required.
- `binary`: `name`, `check`, `expect_pattern`, `install`, and `verify_after` fields for language-server and local binary prerequisites.
- `post_install`: restart/reload requirements and CLI or interactive verification commands.
- `platform_installs`: OS-specific install hints, with Linux falling back to source review when we do not have a verified package-manager command.
- `idempotent`: whether the command is safe to rerun without pre-checking.
- `conflicts_with`: overlap declarations so agents avoid installing redundant reducers for the same failure mode.

Human-facing skills and commands must tell agents to prefer `install_cli` from Bash. `install_interactive` exists for the Claude Code slash-command UI only.

## Liability Gate

Plugin install UX must require explicit acknowledgment before presenting install commands:

> I understand that Agent Analyzer provides deterministic analysis and vetted setup recommendations, but any installation or code change is executed by Claude Code, my package manager, or third-party tools with my approval and at my own risk.

The generated plugin also includes `WAIVER.md`. Claude-facing setup prompts must tell Claude to summarize the waiver, ask for acceptance, and ask again before each installation command.

Receipt/support email copy is specified in `docs/remediation/receipt-email.md` so email delivery repeats the same analysis-first, benchmark-backed, no-unproven-tools message when that surface is used.

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

API route tests also cover the local-first trust boundary:

- the public report can download a report pack and tokenized plugin zip from
  the same private report token
- the default email-gated report/plugin response does not mint a raw upload token or expose
  `/api/paid-uploads/`, tar/gzip commands, or raw bundle copy
- `POST /api/paid-client-reports` accepts only waiver-gated sanitized aggregate
  reports and creates a private report job without storing an upload path
- tokenized plugin zip generation works from that sanitized aggregate report
- the legacy raw bundle command is available only through
  `/api/paid-sessions?legacy_raw_bundle=1` for internal smoke compatibility

## Implementation

The first implementation lives in `internal/remediation`.

The package produces an in-memory `Artifact` and can write it as a zip archive. Tokenized plugin zip download is implemented for Docker end-to-end testing. Plugin artifacts are generated on demand from the sanitized report and are scoped by the private report token.

The legacy raw bundle upload route remains only for internal Docker smoke
coverage while the CLI aggregate command is integrated. It is not the
default `/api/paid-sessions` response and must not be linked from public launch
UX. Any future persisted artifact storage is tracked separately.

## Token-saving recommendation embedding (Phase A, additive)

Phase A of the token-saving recommendation engine introduces a `TokenSavingRecommendation` contract object (see `docs/remediation/token-saving-recommendation-engine.md` for the canonical schema and rule precedence). This object captures at most one primary and at most one secondary tool suggestion per analyzed session, plus the signals and dedupe state that justified each suggestion.

The recommendation object is **optional** in generated plugin artifacts. Generators may attach it to the existing artifact payload without altering the established artifact shape, and the current generator tests (plugin structure, deterministic output for identical sanitized inputs, leak checks, zip archive creation, unsafe-archive-path rejection) continue to hold because none of them require the field to be present or absent. New tests covering the embedded recommendation are introduced alongside the engine and are gated on the field actually being populated.

Refer to `docs/remediation/token-saving-recommendation-engine.md` for the full Phase A contract: input signals, dedupe-aware emission rules, `active_high` skip behavior, and the precise field layout that downstream artifacts may serialize.

## See also

- `token-saving-recommendation-engine.md` — Phase A recommendation engine doc (state model, rule precedence, contract shape).
- `token-saving-tooling-matrix.md` — human-facing tier matrix and product framing for the allowlisted tools the engine may recommend.
