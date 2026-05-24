# External Token Benchmark Comparison

Research date: 2026-05-24

This note compares public token/context optimization benchmarks against the Claude Analyzer benchmark protocol. The key lesson is that "token savings" means different things across projects: some replay transcripts through a compressor, some run live task A/B tests, some measure CLAUDE.md quality, and some measure MCP/tool overhead rather than task completion.

We classify each external claim by token category when possible: cached input, input/context, tool-output, visible output, task total, or telemetry. Most public studies do not separately report hidden reasoning tokens, so their results should not be presented as reasoning-token savings.

## External Benchmarks

| Source | Methodology | Reported result | Primary token category | What it proves | Main caveat |
| --- | --- | --- | --- | --- | --- |
| Anthropic cost guidance | Official guidance, not an A/B benchmark. Recommends `/usage`, clearing stale context, compaction instructions, fewer MCP servers, CLI tools over MCP where possible, and hooks that filter noisy test output. | No single headline benchmark in the docs. Gives example hook pattern that filters test output to failures. | Input/context, tool-output, telemetry | Establishes the vendor-endorsed mechanisms: reduce stale context, reduce MCP overhead, filter tool output. | Not a controlled task benchmark. |
| Anthropic token-saving API update | API/platform benchmark claims for prompt caching and token-efficient tool use. | Prompt caching can reduce cost up to 90% and latency up to 85% for long prompts; token-efficient tool use reduces output tokens up to 70%, with 14% average reduction among early users. | Cached input and output | Platform-level token savings can be large when repeated prompt prefixes or tool-use output dominate. | API feature benchmark, not Claude Code plugin/task workflow A/B. |
| The Distillery | Deterministic replay of eight realistic multi-turn Claude Code fixture sessions through an optimization pipeline. Uses chars/4 token estimation. | 20% reproducible reduction on 124,580 raw tokens at the smart preset; 30-60% in heavier real sessions depending on pattern. | Transcript input/context estimate | Compression/dedup pipelines reduce transcript/tool-result payloads on fixed fixtures. | Replay benchmark, not fresh Claude task execution with quality gate. |
| Tamp v0.8.0 whitepaper | 12 scenarios x 18 configs = 216 live A/B calls routed through OpenRouter, judged by Claude Sonnet Haiku 4.5. | L5 balanced default: 47.56% tokens saved; 216/216 quality retention. | Task total, category split unclear | Live compression can preserve task completion across a scenario grid when an independent judge scores outputs. | Single-turn micro-fixtures understate session-scoped stages; judge-based quality differs from repository tests. |
| TechLoom CLAUDE.md benchmark | 540 Phase 1 runs plus 648 Phase 2 runs. Three models, 12 standardized coding tasks, 10 instruction profiles. Scores test pass, lint, complexity, and LLM judgment. | CLAUDE.md compression saved only 5-13% actual API tokens; compressed instructions hurt Haiku/Sonnet in several profiles; empty profile won overall on generic tasks. | Input/context instructions and task total | Measures both token savings and output quality, and falsifies input-only compression claims. | Generic single-file tasks; does not test project-specific architecture context or long multi-file sessions. |
| Boarder copy/paste MCP benchmark | Five baseline and five MCP runs in isolated containers. Task: split a 700-line Express monolith into 11 modules. | Baseline averaged 70.9K tokens and $1.22; MCP averaged 81.0K tokens and $1.34, about 10% higher cost. | Task total and MCP input/context overhead | Adding tools/MCPs can make costs worse when the model uses them sporadically or the abstraction is wrong. | One task family; negative result may change on larger refactors or block-level abstractions. |
| Token Savior | Vendor benchmark on 96 real coding tasks with Claude Opus 4.7; includes structural navigation and memory. | Claims active tokens/task from 17,221 to 3,395 (-80%) and wall time/task from 110.6s to 18.9s (-83%). | Active context/task total | Symbol-level navigation plus memory can be a large win when used consistently. | Needs independent reproduction; profile/task details matter. |
| ComputingForGeeks tested-tool roundup | Practical tool tests across a small 52-file benchmark plus vendor data comparisons. | Reports small-repo code-review-graph savings around 5%; notes RTK showed 0% on their bash task but shines on noisy output. | Tool-output and task total | Confirms overhead can dominate on small repos and output compression is workload-dependent. | Roundup format; methods vary per tool. |
| Local-Splitter paper | Open-source shim across MCP and OpenAI-compatible HTTP. Evaluates seven tactics individually, in pairs, and greedily across edit-heavy, explanation-heavy, general chat, and RAG-heavy workloads. | Local routing plus prompt compression saves 45-79% cloud tokens on edit/explanation workloads; full tactic set saves 51% on RAG-heavy workloads. | Cloud input/output total by workload | Workload-dependent tactic selection matters more than any single universal reducer. | Cloud-token routing benchmark, not specifically Claude Code plugin install behavior. |
| StackOne MCP token optimization comparison | Compares four MCP optimization approaches: schema compression, search-first discovery, response filtering, and code-mode execution. | Reports approach-level ranges such as 70-97% schema compression and cites large reductions from code-mode execution. | MCP schema/response input context | Useful taxonomy for MCP-token bottlenecks. | Approach comparison, not a single reproducible coding-task benchmark. |

## How This Compares To Our Benchmark

Claude Analyzer's current benchmark is narrower but stricter on task execution:

- It runs real Claude Code `-p` sessions, not only replayed fixture compression.
- Baseline and optimized runs start from the same fixed commit and prompt.
- Each result requires the external quality gate to pass on both sides.
- It publishes negative findings when a promising tool adds turns or duplicate reads.

The tradeoff is that one bounded Go benchmark does not prove long-session savings, project-memory savings, or monorepo architectural-navigation savings. The external literature suggests those need separate benchmark families.

## Product Implications

- Keep the proof page explicit about benchmark type.
- Do not compare a fixture replay percentage directly to a live task A/B percentage.
- Add benchmark families for long sessions, monorepos, MCP-heavy workflows, and CLAUDE.md/profile changes before claiming those categories.
- Treat external results as candidate mechanisms, not product proof.
