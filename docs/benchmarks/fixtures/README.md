# Benchmark Fixtures

This directory contains the permanent, auditable benchmark inputs used by the proof pages.

## Files

- `tool-suite.json`: canonical list of benchmark suites, harnesses, guidance files, local requirements, and default repeat count.
- `tasks/owner-breakdown-v3-noisy.txt`: fixed task prompt for the noisy Go benchmark.
- `guidance/*.txt`: optimized-run guidance for each intervention.
- `guidance/claude-token-efficient-profile.md`: the minimal CLAUDE.md profile used for the claude-token-efficient trial.
- `mcp/claude-context-local.json`: local claude-context MCP configuration used when Ollama and Milvus are available.

## Running

Run one suite:

```sh
ONLY=rtk-explicit REPEATS=3 ./scripts/benchmark-suite.sh
```

Run all locally available suites:

```sh
REPEATS=3 ./scripts/benchmark-suite.sh
```

The suite runner writes local raw artifacts under `.data/benchmarks/suites/<suite-id>/`. These raw local directories are not intended for publication. Public artifacts should be generated from `aggregate.json` and sanitized comparison JSON only.

## Evidence Standard

A tool recommendation is not considered repeated evidence until:

- `REPEATS >= 3`
- all repeats create fresh baseline and optimized sessions
- baseline and optimized quality gates both pass
- `aggregate.json` is saved
- the public proof page references the aggregate rather than only a single run

Telemetry-only tools such as ccusage and ccstatusline do not have a task intervention delta. They should be validated separately and labeled as telemetry.
