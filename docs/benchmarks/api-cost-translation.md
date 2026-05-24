# Published API Cost Translation

Research date: 2026-05-24

This note converts repeated benchmark token deltas into dollar estimates using published API inference rates. It does not replace harness-native cost fields. Claude Code and Codex can apply product-specific routing, credits, subscriptions, or internal model work that differs from direct API inference.

## Published Rates Used

Claude Code was run with `--model sonnet`. The exposed aggregate usage fields are repriced with Claude Sonnet 4.6 API rates:

- input: `$3.00` / MTok
- 1-hour cache write: `$6.00` / MTok
- cache read: `$0.30` / MTok
- output: `$15.00` / MTok

Source: https://platform.claude.com/docs/en/about-claude/pricing

The raw Claude stdout also reports internal model usage such as Haiku in some runs, but the comparison JSON exposes only aggregate usage buckets. Published API estimates therefore use Sonnet rates as an approximation and are reported separately from Claude Code native `total_cost_usd`.

Codex JSON usage is repriced under the published Standard rate for `gpt-5.3-codex`:

- input: `$1.75` / MTok
- cached input: `$0.175` / MTok
- output: `$14.00` / MTok

Sources:

- https://developers.openai.com/api/docs/pricing
- https://help.openai.com/en/articles/20001106-codex-rate-card

Because Codex exposes `reasoning_output_tokens` separately, this estimate bills reasoning tokens at the output-token rate. If a future exporter includes reasoning inside `output_tokens`, do not add it twice.

## Formula

Claude Sonnet estimate:

```text
cost =
  input_tokens * 3.00 / 1,000,000
+ cache_creation_input_tokens * 6.00 / 1,000,000
+ cache_read_input_tokens * 0.30 / 1,000,000
+ output_tokens * 15.00 / 1,000,000
```

Codex estimate:

```text
uncached_input_tokens = input_tokens - cached_input_tokens

cost =
  uncached_input_tokens * 1.75 / 1,000,000
+ cached_input_tokens * 0.175 / 1,000,000
+ output_tokens * 14.00 / 1,000,000
+ reasoning_output_tokens * 14.00 / 1,000,000
```

## Final 3x Mean Cost Results

All rows are means across three fresh baseline/optimized pairs.

| Suite | Harness/rate assumption | Baseline mean | Optimized mean | Published API delta mean | Percent | Native cost signal |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| Agent Analyzer guided | Claude Sonnet 4.6 | `$0.246837` | `$0.187629` | `-$0.059207` | `-24.0%` | Claude Code native `-$0.044219` |
| claude-context limit 3 | Claude Sonnet 4.6 | `$0.222950` | `$0.280989` | `+$0.058038` | `+26.0%` | Claude Code native `+$0.048434` |
| claude-rlm discovery | Claude Sonnet 4.6 | n/a | n/a | n/a | n/a | root-session-only native `-$0.075322`; optimized sub-agent usage not exposed in root stdout |
| context-mode batch | Claude Sonnet 4.6 | `$0.255157` | `$0.202981` | `-$0.052175` | `-20.4%` | Claude Code native `-$0.036390` |
| grepai path-constrained | Claude Sonnet 4.6 | `$0.259462` | `$0.221805` | `-$0.037657` | `-14.5%` | Claude Code native `-$0.017598` |
| claude-token-efficient | Claude Sonnet 4.6 | `$0.230198` | `$0.225991` | `-$0.004208` | `-1.8%` | Claude Code native `-$0.003828` |
| RTK explicit | Claude Sonnet 4.6 | `$0.244114` | `$0.199798` | `-$0.044316` | `-18.2%` | Claude Code native `-$0.031479` |
| Probe | Claude Sonnet 4.6 | `$0.230660` | `$0.268999` | `+$0.038340` | `+16.6%` | Claude Code native `+$0.038069` |
| Semble | Claude Sonnet 4.6 | `$0.275398` | `$0.161205` | `-$0.114194` | `-41.5%` | Claude Code native `-$0.089147` |
| Squeez | Claude Sonnet 4.6 | `$0.232566` | `$0.204342` | `-$0.028224` | `-12.1%` | Claude Code native `-$0.014049` |
| Agent Analyzer text guidance | GPT-5.3-Codex Standard | `$0.196060` | `$0.133667` | `-$0.062392` | `-31.8%` | uncached+output `-24,369`; reasoning `-45` |
| Caveman Claude | Claude Sonnet 4.6 | `$0.236776` | `$0.245987` | `+$0.009211` | `+3.9%` | Claude Code native `+$0.009919` |
| Caveman Codex | GPT-5.3-Codex Standard | `$0.186165` | `$0.152179` | `-$0.033986` | `-18.3%` | uncached+output `-4,739`; reasoning `-2` |

## Scale-Up Math

The single-run dollar deltas are small because the fixture is one coding task. Product copy should lead with the percent reduction and scale only against comparable recurring spend:

```text
savings_percent = (baseline_mean - optimized_mean) / baseline_mean
monthly_savings = comparable_monthly_baseline_spend * savings_percent
```

For Agent Analyzer guided runs:

- Baseline mean: `$0.2468368`
- Optimized mean: `$0.1876295`
- Delta: `-$0.0592073`
- Savings percent: `0.0592073 / 0.2468368 = 23.986%`

Examples for comparable Claude Sonnet API-equivalent coding work:

| Baseline spend | Agent Analyzer savings at 23.986% |
| --- | ---: |
| `$100/week` | `$23.99/week` |
| `$500/week` | `$119.93/week` |
| `$2,000/month` | `$479.73/month` |
| `$5,000/month` | `$1,199.32/month` |
| `$10,000/month` | `$2,398.64/month` |

Tooltip/basis copy: "Based on three fresh noisy-repo benchmark pairs. Agent Analyzer reduced the published Claude Sonnet 4.6 API-rate estimate from `$0.2468368` to `$0.1876295`, a `23.986%` reduction. Scaled examples assume your future workload has a similar token mix and quality requirements."

## Product Rule

Cost claims must name both the token category and the pricing surface:

- "Reduced tool-output tokens" is an Analyzer workflow claim.
- "Reduced visible output cost" is an output-token pricing claim.
- "Reduced published API cost" requires input/cache/output/reasoning repricing.
- "Reduced Claude Code/Codex cost" requires native harness billing/credit evidence.

Do not collapse those into one generic savings claim.

For multi-session tools such as claude-rlm, do not publish root stdout usage as full cost. The analyzer can aggregate root and sub-agent logs for estimated/tool-output metrics, but Claude Code `-p` stdout only exposes the root session's native usage fields in this harness.
