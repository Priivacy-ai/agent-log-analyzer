# Token-Saving Tooling Matrix

This matrix is the starting allowlist for paid remediation recommendations. The plugin may recommend these tools only with explicit user approval and the waiver gate. Installation is never automatic.

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
| RTK | https://github.com/rtk-ai/rtk | Shell-output compression through command proxying | Promising for severe shell-output bloat. Treat as advanced because it rewrites command execution through hooks. |
| Caveman | https://github.com/JuliusBrussee/caveman | Response terseness/compression | Research only. Reports suggest configuration confusion and inconsistent activation. |
| memsearch | https://github.com/zilliztech/memsearch | Persistent cross-agent memory and retrieval | Research only. Promising for sparse memory, but too stateful for the initial low-risk paid pack. |

## Recommendation Mapping

| Analyzer Signal | Recommend |
| --- | --- |
| `tool_output_bloat` | Context Mode, RTK, claude-token-efficient, ccstatusline |
| `repeated_file_reads` | grepai, claude-context, language-server/code-intelligence plugins |
| `retry_loop` | hooks architecture reference, statusline awareness, session hygiene skill |
| `context_growth_spikes` | Context Mode, ccstatusline, session hygiene, claude-token-efficient review |
| Any paid scan | ccusage, awesome-claude-code monitoring note |

## Guardrails

- Prefer plugin marketplace or package-manager installs over curl scripts.
- Curl install scripts are allowed only as reviewed fallback instructions, never silent defaults.
- External hosted retrieval tools must disclose API keys, data movement, and vendor dependency.
- Local semantic search tools must disclose indexing cost, storage location, and embedding provider.
- Tools that rewrite shell commands are advanced-only and must be separately approved.
- Generated artifacts must not include unknown private tool names from logs.

## Registry cross-reference (Phase A)

The canonical machine-readable registry of token-saving tools lives in `internal/analyzer/token_saving_tools.go`. That Go file is the source of truth that the recommendation engine actually consults at runtime; the tier tables above remain the human-facing reference and product rationale.

This matrix doc and the registry are intentionally complementary: when a tool's tier, signal mapping, or product framing changes, update this document so reviewers and operators retain narrative context; when the engine's runtime behavior changes (new tool entry, signal alias, activation heuristic), update the Go registry so the binary behavior follows. Drift between the two is a planning bug, not a runtime bug — the engine will keep operating off the registry regardless of doc state.

Phase A enforces a dedupe-aware recommendation contract: for any given analyzed session, the engine emits at most one primary and at most one secondary token-saving recommendation. Tools that telemetry classifies as `active_high` for a given signal are treated as already in effect and skipped rather than re-recommended, so the user is never asked to install something they are already running successfully.

See `docs/remediation/token-saving-recommendation-engine.md` for the full Phase A state model, rule precedence, signal-to-tool mapping, and the additive contract surface that downstream artifacts may embed.

## See also

- `token-saving-recommendation-engine.md` — Phase A recommendation engine doc (state model, rule precedence, contract shape).
- `plugin-artifacts.md` — paid plugin artifact contract and how the recommendation object may optionally embed into generated artifacts.
