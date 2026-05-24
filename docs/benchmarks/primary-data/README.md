# Primary Benchmark Data

This directory contains the sanitized benchmark recordings that back the proof pages and paid plugin messaging.

Included:

- suite-level `aggregate.json`, `manifest.json`, and `suite-status.json`
- per-run `comparison.json`
- per-run task prompt, quality status, exit status, and quality-gate output
- SHA-256 manifest in `index.json`

Excluded by design:

- raw Claude Code and Codex JSONL logs
- raw assistant transcript stdout/stderr
- generated plugin zip artifacts
- copied benchmark worktrees
- private paths, secrets, and unknown private tool names

The public proof pages use the smaller published aggregates in `web/proof/reports/`. This directory keeps the fuller sanitized primary data in git so curious users can audit the repeated 3x result set without requiring access to local `.data` output.

Cost scale-up math is stored in the published aggregates and summarized in `docs/benchmarks/api-cost-translation.md`. The core Agent Analyzer claim is based on three fresh noisy-repo runs: `-$0.0592073` published Claude Sonnet 4.6 API-rate mean delta from a `$0.2468368` baseline, or `23.986%` lower API-rate cost for comparable work.
