# Spec Kitty High-Context Fixtures

Research date: 2026-05-24

Fresh source workspace used for discovery:

```text
/var/folders/gj/bxx0438j003b20kn5b6s7bsh0000gn/T/spec-kitty-20260524-154607-XDlS5K
```

The permanent benchmark source is the public `Priivacy-ai/spec-kitty` repository at commit:

```text
38abeebf6fab2215fb52a099bfad707a7a503ad7
```

Prepare the source clone with:

```sh
./scripts/prepare-spec-kitty-benchmark-target.sh
```

Run the high-context suite with:

```sh
SUITE_FILE=docs/benchmarks/fixtures/tool-suite-spec-kitty-high-context.json \
REPEATS=3 \
./scripts/benchmark-suite.sh
```

Run one suite while iterating:

```sh
SUITE_FILE=docs/benchmarks/fixtures/tool-suite-spec-kitty-high-context.json \
ONLY=spec-kitty-agent-analyzer-guided \
REPEATS=1 \
./scripts/benchmark-suite.sh
```

## Why This Qualifies

Spec Kitty gives us three real high-context pressures in one permanent fixture:

| Pressure | Evidence |
| --- | --- |
| Large repo navigation | Discovery clone had 6,883 files, 2,227 Python files, 3,408 Markdown files, 391 YAML files, and about 610k counted lines. |
| Noisy CI/log task | Saved two public failed GitHub Actions job logs totaling 320,160 bytes: one diff-coverage failure and one SonarCloud quality-gate failure. |
| Retrieval amortization | The repo contains 788 mission/spec/status artifacts under `kitty-specs` plus large architecture, doctrine, command, workflow, and test surfaces. |

It also contains in-repo evidence logs that are large enough to stress shell-output handling:

- `kitty-specs/unblock-sync-identity-boundary-canary-01KRZJ07/canary-evidence/full-pytest.txt` is 619,090 bytes.
- `kitty-specs/charter-contract-cleanup-tranche-1-01KQATS4/research/_artifacts/test-gate-5-ruff.txt` is 323,451 bytes.

## CI Fixtures

The committed CI inputs are under `docs/benchmarks/fixtures/spec-kitty/ci-logs/`:

- `run-26354749577-job-77579702051-diff-coverage.txt`: PR run from 2026-05-24, failed in `diff-coverage`. The direct failure is 0% diff coverage for `src/charter/compiler.py` line 505 on the enforced critical-path check, with the advisory full-diff output also naming `src/specify_cli/cli/commands/_auth_doctor.py` line 236.
- `run-26353695768-job-77577160391-sonarcloud.txt`: scheduled main run from 2026-05-24, failed in `sonarcloud`. The run downloaded 34 report artifacts, parsed many coverage XMLs, reported ambiguous `__init__.py` coverage paths, then failed the SonarCloud Quality Gate.

The benchmark setup command copies these logs into each fresh worktree at `.benchmark-fixtures/spec-kitty-ci/`, so baseline and optimized runs see the same deterministic files.

## What This Tests

This fixture is designed to separate three token-saving mechanisms:

- Retrieval and path planning: `claude-context`, `grepai`, Semble, and Probe-like tools should only help if their search/index overhead is amortized by fewer repeated broad reads.
- Tool-output/log compression: RTK, Squeez, and context-mode should help most on the CI and evidence-log parts, especially if the agent avoids dumping huge logs verbatim.
- Visible-output terseness: `claude-token-efficient`, Caveman, and Agent Analyzer guidance can reduce final response/output tokens, but they do not automatically reduce input/context tokens unless paired with better search discipline.

Telemetry-only tools such as ccusage and ccstatusline remain excluded from this suite as direct reducers. They can measure or surface cost, but they do not change task behavior by themselves.

## Limits

This is not a long-running multi-hour session replay. Local `.claude/projects` logs show many Spec Kitty sessions, but raw transcripts may contain private paths and conversation data, so they are not committed as fixtures.

This is not intrinsically MCP-heavy. MCP overhead is introduced by the harness when a suite enables `claude-context`; that lets us measure MCP schema/context overhead separately from the repo/log task.

## Smoke Validation

A one-repeat smoke run on 2026-05-24 validated that the fixture copies into both worktrees and that both baseline and optimized agents can produce `benchmark-answer.md`.

The same smoke also exposed a quality-gate weakness: the optimized run used fewer analyzer-estimated tokens, less tool output, fewer Claude output tokens, and lower native Claude cost, but its answer incorrectly said the enforced diff-coverage step found no coverage reports. The log actually shows three coverage reports:

- `coverage-fast-cli.xml`
- `coverage-fast-charter.xml`
- `coverage-integration-cli.xml`

That run is therefore a harness smoke, not a valid savings claim. The task prompt and quality command now require the answer to state the three-report fact so future high-context results cannot pass while compressing away the root diagnostic detail.

A follow-up smoke produced a correct optimized answer, but the first tightened quality regex was overly literal about where the number `3` appeared relative to "coverage reports." The quality command now checks the diagnostic concept rather than one exact sentence.
