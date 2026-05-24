# Caveman Benchmark

Research date: 2026-05-24

This benchmark tests terse-output pressure against the same fixed v3 noisy owner-breakdown task. Caveman is useful as a control because it targets visible output, while the Agent Analyzer plugin targets measured workflow and tool-output waste.

## Final 3x Result

| Harness | Intervention | Quality | Estimated tokens | Tool output | Output/reasoning signal | Cost signal | Verdict |
| --- | --- | --- | ---: | ---: | --- | --- | --- |
| Claude Code `-p` | Caveman plugin/guidance | 3/3 | `+4,355` | `+4,868` | Claude output `-370` | native `+$0.009919`; API estimate `+$0.009211` | Negative here |
| Codex `exec --json` | Caveman text guidance | 3/3 | `-9,210` | `-9,109` | output `-172`; reasoning `-2`; uncached+output `-4,739` | API estimate `-$0.033986` | Harness-specific |

## Interpretation

Caveman proves the distinction between output-token savings and full-session savings:

- On Claude Code, terse output reduced visible output tokens but increased analyzer-estimated tokens, tool-output tokens, native cost, and published-rate estimated cost.
- On Codex, terse output helped this fixture across the native usage buckets we measured.
- The plugin should therefore not treat terse-output pressure as a universal default. It should label it as harness-specific and category-specific.

## Artifacts

- `web/proof/reports/aggregate-caveman-claude.json`
- `web/proof/reports/aggregate-caveman-codex.json`
- `docs/benchmarks/repeated-benchmark-suite.md`
