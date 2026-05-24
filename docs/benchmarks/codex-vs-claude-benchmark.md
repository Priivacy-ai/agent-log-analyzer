# Codex vs Claude Code Benchmark

Research date: 2026-05-24

This benchmark runs the same fixed v3 noisy owner-breakdown task through Claude Code `-p` and Codex `exec --json`.

## Fixture

- target repo: `/tmp/agent-analyzer-benchmark-target-v3`
- fixed commit: `b96b8a7f5cc57c4335bc7bc85ec726c836ed0996`
- task prompt: `docs/benchmarks/fixtures/tasks/owner-breakdown-v3-noisy.txt`
- quality gate: `go test ./...`
- repeat count: `3` fresh baseline/optimized pairs per harness

Claude Code used the generated Agent Analyzer plugin. Codex used equivalent text guidance because the generated artifact is a Claude Code plugin, not a Codex plugin.

## Final 3x Result

| Harness | Intervention | Quality | Analyzer estimated tokens | Analyzer tool output | Native token/cost signal | Published API estimate | Verdict |
| --- | --- | --- | ---: | ---: | --- | ---: | --- |
| Claude Code `-p` | Generated Agent Analyzer plugin | 3/3 | `-12,370` | `-12,698` | native cost `-$0.044219`; output `-504` | `-$0.059207` | Positive |
| Codex `exec --json` | Agent Analyzer text guidance | 3/3 | `-14,520` | `-14,527` | uncached+output `-24,369`; output `-483`; reasoning `-45` | `-$0.062392` | Positive on this fixture |

The earlier one-off Codex run showed `+17,949` uncached-plus-output tokens. The repeated suite reversed that finding: three fresh Codex pairs averaged `-24,369` uncached-plus-output tokens and `-45` reasoning tokens. This is the reason the product should use the repeated aggregate, not the first single-run result, for final copy.

## Token Category Interpretation

- Analyzer estimated tokens and tool-output tokens measure event-stream workflow waste.
- Codex `uncached_plus_output_tokens` is a native usage proxy: input tokens minus cached input tokens, plus output tokens.
- Codex reasoning tokens are hidden model work reported separately by Codex usage metadata.
- Published API estimates bill Codex reasoning tokens at the output-token rate in this note.

## Artifacts

- `web/proof/reports/aggregate-agent-analyzer-guided-v3.json`
- `web/proof/reports/aggregate-codex-guided.json`
- `web/proof/reports/results.json`
- `docs/benchmarks/api-cost-translation.md`
