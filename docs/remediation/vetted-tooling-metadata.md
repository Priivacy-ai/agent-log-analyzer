# Vetted Tooling Metadata (Issue #32)

This matrix is the launch-review checklist for paid remediation recommendations. It adds explicit install command, required binary, install risk, data-movement risk, and rollback guidance for each currently allowlisted recommendation family.

Rules:
- Every row must have a canonical source URL.
- `install_command` must be copyable and deterministic.
- `rollback_guidance` must be actionable in one short step.
- Unknown/private tool names from user logs are never added automatically.
- Third-party tools remain review-gated before paid artifact inclusion.

## Core Recommendations

| tool_id | source_url | install_command | required_binary | install_risk | data_movement_risk | rollback_guidance | status |
|---|---|---|---|---|---|---|---|
| ccusage | https://github.com/ryoppippi/ccusage | `npx ccusage@latest` | `ccusage` | low | low | Remove from local shell aliases/scripts and stop invoking it in workflows. | vetted |
| context-mode | https://github.com/mksglu/context-mode | `/plugin marketplace add mksglu/context-mode` then `/plugin install context-mode@context-mode` | `claude` | medium | low | Run `/plugin remove context-mode@context-mode` and `/reload-plugins`. | vetted |
| rtk | https://github.com/rtk-ai/rtk | `brew install rtk` then `rtk init -g` | `rtk` | high | high | Remove RTK hooks from `.claude/settings.json` and uninstall `rtk`. | vetted+waiver |
| claude-token-efficient | https://github.com/drona23/claude-token-efficient | Review upstream and apply a minimal `CLAUDE.md` diff only | none | low | low | Revert the added `CLAUDE.md` section or restore previous version. | vetted |
| grepai | https://github.com/yoanbernabeu/grepai | `brew install yoanbernabeu/tap/grepai` then `grepai init` | `grepai` | medium | low | Stop `grepai watch`, remove config/index directory, uninstall `grepai`. | vetted |
| claude-context | https://github.com/zilliztech/claude-context | `claude mcp add claude-context ... -- npx @zilliz/claude-context-mcp@latest` | `claude` | medium | medium | Remove the `claude-context` MCP entry and rotate external API credentials. | vetted+waiver |
| ccstatusline | https://github.com/sirmalloc/ccstatusline | Install per upstream release instructions | `ccstatusline` | low | low | Remove statusline config entry and uninstall the binary. | vetted |
| claude-code-usage-monitor | https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor | `uv tool install claude-monitor` | `claude-monitor` | low | low | `uv tool uninstall claude-monitor` (or remove pip install). | vetted |

## Official Code-Intelligence Plugins

| tool_id | source_url | install_command | required_binary | install_risk | data_movement_risk | rollback_guidance | status |
|---|---|---|---|---|---|---|---|
| typescript-lsp | https://claude.com/plugins/typescript-lsp | `/plugin install typescript-lsp@claude-plugins-official` | `typescript-language-server` | medium | low | `/plugin remove typescript-lsp@claude-plugins-official` and uninstall binary. | vetted |
| pyright-lsp | https://claude.com/plugins/pyright-lsp | `/plugin install pyright-lsp@claude-plugins-official` | `pyright-langserver` | medium | low | Remove plugin and uninstall `pyright`. | vetted |
| gopls-lsp | https://claude.com/plugins/gopls-lsp | `/plugin install gopls-lsp@claude-plugins-official` | `gopls` | medium | low | Remove plugin and uninstall `gopls`. | vetted |
| rust-analyzer-lsp | https://claude.com/plugins/rust-analyzer-lsp | `/plugin install rust-analyzer-lsp@claude-plugins-official` | `rust-analyzer` | medium | low | Remove plugin and remove `rust-analyzer` component. | vetted |
| php-lsp | https://claude.com/plugins/php-lsp | `/plugin install php-lsp@claude-plugins-official` | `intelephense` | medium | low | Remove plugin and uninstall `intelephense`. | vetted |

## Official MCP Integrations

| tool_id | source_url | install_command | required_binary | install_risk | data_movement_risk | rollback_guidance | status |
|---|---|---|---|---|---|---|---|
| github | https://claude.com/plugins/github | `/plugin install github@claude-plugins-official` | none | medium | medium | Remove plugin and revoke app token if previously granted. | vetted |
| notion | https://claude.com/plugins/notion | `/plugin install notion@claude-plugins-official` | none | medium | medium | Remove plugin and revoke Notion integration access. | vetted |
| linear | https://claude.com/plugins/linear | `/plugin install linear@claude-plugins-official` | none | medium | medium | Remove plugin and revoke Linear integration access. | vetted |
| sentry | https://claude.com/plugins/sentry | `/plugin install sentry@claude-plugins-official` | none | medium | medium | Remove plugin and revoke Sentry integration token. | vetted |
| supabase | https://claude.com/plugins/supabase | `/plugin install supabase@claude-plugins-official` | none | medium | medium | Remove plugin and rotate Supabase credentials if used. | vetted |

## Review Guardrail

Before shipping any new row to paid artifacts, verify all fields above and record reviewer/date in the PR body.