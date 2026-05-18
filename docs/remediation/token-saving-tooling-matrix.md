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
