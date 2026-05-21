# Token-Saving Tooling Matrix

This matrix is the starting allowlist for paid remediation recommendations. The plugin may recommend these tools only with explicit user approval and the waiver gate. Installation is never automatic.

Every installable recommendation must carry a precise source URL. Do not recommend a bare package name when that name can resolve to a different package in another ecosystem.

## Recommendation Taxonomy

Recommendations are grouped by bounded context-defocus failure modes:

- `noisy_terminal_logs`: shell/build/test/CLI output that pollutes context.
- `tool_output_flooding`: MCP, browser, or tool result payloads that enter the session as large raw blocks.
- `repeated_codebase_navigation`: repeated grep/read/list exploration that should be replaced by targeted retrieval or code intelligence.
- `broad_file_reads_or_verbose_output`: whole-file reads or assistant verbosity that bloat future turns.
- `memory_gaps`: project decisions or debugging state repeatedly reintroduced across sessions.
- `cross_cutting_telemetry`: usage visibility, MCP/skill hygiene, retry loops, and session-boundary discipline.

Every recommendation must carry install risk, data-movement risk, install surface, known overlaps, and any package-name ambiguity warning. Unknown private MCP, plugin, and skill names remain count-only and must never be copied into reports or plugin artifacts.

## Tier 1: Bundle Or Strongly Recommend

| Tool | Source | Role | Product Decision |
| --- | --- | --- | --- |
| Context Mode | https://github.com/mksglu/context-mode | Context defense, sandboxed tool-output compression, context telemetry, statusline support | Recommend when analysis shows tool-output bloat or context growth spikes. Candidate backbone of the optimization pack. |
| ccusage | https://github.com/ryoppippi/ccusage | Claude Code JSONL usage parsing, token/cost accounting, burn-rate visibility | Always recommend as the independent metrics layer and comparison source for our analyzer. Also evaluate as backend parser input. |
| claude-context | https://github.com/zilliztech/claude-context | MCP semantic retrieval for large codebases | Recommend for repeated file reads or large-repo workflows, but flag external API/vector DB requirements. |
| grepai | https://github.com/yoanbernabeu/grepai | Local semantic search and call-graph retrieval | Recommend for repeated file reads when the user wants local-first retrieval. Requires embedding provider setup. |
| Claude Code Hooks Mastery | https://github.com/disler/claude-code-hooks-mastery | Hook architecture reference | Reference only. Use to guide our own hook design; do not ask users to install as a runtime dependency. |
| claude-token-efficient | https://github.com/drona23/claude-token-efficient | Small CLAUDE.md verbosity rules | Recommend as a diff, not overwrite. Useful only when output verbosity dominates enough to offset persistent instruction cost. |

## Tier 2: Supporting Recommendations

| Tool | Source | Role | Product Decision |
| --- | --- | --- | --- |
| ccstatusline | https://github.com/sirmalloc/ccstatusline | Claude Code statusline telemetry | Recommend only if it does not conflict with Context Mode or existing statusline config. |
| Claude Code Usage Monitor | https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor | Live burn-rate monitoring and forecasting | Optional external monitor for power users. Keep outside the plugin runtime. |
| Claude Code Usage Tracker | https://github.com/LyndonWangWork/Claude-Code-Usage-Tracker | Lightweight desktop usage tracking | Mention as an alternative monitor, not a default recommendation. |
| awesome-claude-code | https://github.com/hesreallyhim/awesome-claude-code | Ecosystem discovery index | Monitor continuously for candidate tools; never install from it directly. |

## Tier 3: Research Candidates

| Tool | Source | Role | Product Decision |
| --- | --- | --- | --- |
| RTK (Rust Token Killer, `rtk-ai/rtk`) | https://github.com/rtk-ai/rtk | Shell-output compression through command proxying | Advanced/waiver-gated only because it rewrites command execution through hooks. Never describe this as the npm package `rtk`; that package is an unrelated release/changelog tool. Homebrew `rtk` currently points at `rtk-ai/rtk`. |
| Caveman | https://github.com/JuliusBrussee/caveman | Response terseness/compression | Research only. Reports suggest configuration confusion and inconsistent activation. |
| memsearch | https://github.com/zilliztech/memsearch | Persistent cross-agent memory and retrieval | Research only. Promising for sparse memory, but too stateful for the initial low-risk paid pack. |

## Recommendation Mapping

| Analyzer Signal | Recommend |
| --- | --- |
| `tool_output_bloat` | Context Mode, RTK (`rtk-ai/rtk`, not npm `rtk`), claude-token-efficient, ccstatusline |
| `repeated_file_reads` | grepai, claude-context, language-server/code-intelligence plugins |
| `retry_loop` | hooks architecture reference, statusline awareness, session hygiene skill |
| `context_growth_spikes` | Context Mode, ccstatusline, session hygiene, claude-token-efficient review |
| Any paid scan | ccusage, awesome-claude-code monitoring note |

## Guardrails

- Prefer plugin marketplace or package-manager installs over curl scripts.
- Curl install scripts are allowed only as reviewed fallback instructions, never silent defaults.
- Any generated recommendation must include a canonical source URL. Generic strings like "official marketplace docs" are not sufficient when a concrete plugin page exists.
- RTK must always be written as `RTK (Rust Token Killer, rtk-ai/rtk)` or include `https://github.com/rtk-ai/rtk`; never tell users to install npm `rtk`.
- External hosted retrieval tools must disclose API keys, data movement, and vendor dependency.
- Local semantic search tools must disclose indexing cost, storage location, and embedding provider.
- Tools that rewrite shell commands are advanced-only and must be separately approved.
- Generated artifacts must not include unknown private tool names from logs.

## Registry cross-reference (Phase A)

The canonical machine-readable registry of token-saving tools lives in `internal/analyzer/token_saving_tools.go`. That Go file is the source of truth that the recommendation engine actually consults at runtime; the tier tables above remain the human-facing reference and product rationale.

This matrix doc and the registry are intentionally complementary: when a tool's tier, signal mapping, or product framing changes, update this document so reviewers and operators retain narrative context; when the engine's runtime behavior changes (new tool entry, signal alias, activation heuristic), update the Go registry so the binary behavior follows. Drift between the two is a planning bug, not a runtime bug — the engine will keep operating off the registry regardless of doc state.

Phase A enforces a dedupe-aware recommendation contract: for any given analyzed session, the engine emits at most one primary and at most one secondary token-saving recommendation. Tools that telemetry classifies as `active_high` for a given signal are treated as already in effect and skipped rather than re-recommended, so the user is never asked to install something they are already running successfully.

See `docs/remediation/token-saving-recommendation-engine.md` for the full Phase A state model, rule precedence, signal-to-tool mapping, and the additive contract surface that downstream artifacts may embed.

## Source URL Audit (2026-05-20)

Verified repository URLs with `git ls-remote` for the GitHub-hosted recommendations:

- `https://github.com/ryoppippi/ccusage`
- `https://github.com/sirmalloc/ccstatusline`
- `https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor`
- `https://github.com/LyndonWangWork/Claude-Code-Usage-Tracker`
- `https://github.com/mksglu/context-mode`
- `https://github.com/rtk-ai/rtk`
- `https://github.com/zilliztech/claude-context`
- `https://github.com/yoanbernabeu/grepai`
- `https://github.com/zilliztech/memsearch`
- `https://github.com/drona23/claude-token-efficient`
- `https://github.com/JuliusBrussee/caveman`
- `https://github.com/disler/claude-code-hooks-mastery`
- `https://github.com/hesreallyhim/awesome-claude-code`
- `https://github.com/anthropics/claude-plugins-official`

Verified current official Claude plugin pages used by paid artifacts:

- `https://claude.com/plugins/typescript-lsp`
- `https://claude.com/plugins/pyright-lsp`
- `https://claude.com/plugins/gopls-lsp`
- `https://claude.com/plugins/rust-analyzer-lsp`
- `https://claude.com/plugins/php-lsp`
- `https://claude.com/plugins/github`
- `https://claude.com/plugins/notion`
- `https://claude.com/plugins/linear`
- `https://claude.com/plugins/sentry`
- `https://claude.com/plugins/supabase`

RTK disambiguation check: `npm view rtk` resolves to `github.com/cliffano/rtk` and describes a release/version/changelog tool. `@rtk-ai/rtk` is not published on npm. Therefore Agent Analyzer must never recommend npm installation for RTK.

## See also

- `token-saving-recommendation-engine.md` — Phase A recommendation engine doc (state model, rule precedence, contract shape).
- `plugin-artifacts.md` — paid plugin artifact contract and how the recommendation object may optionally embed into generated artifacts.
