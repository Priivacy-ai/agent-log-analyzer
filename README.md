# Agent Analyzer

Deterministic performance profiler for AI coding workflows.

This repo starts with a Docker-local, end-to-end implementation:

- run the analyzer locally against up to three largest-recent supported agent logs per source
- write a sanitized report JSON that the user can inspect before upload
- upload only the sanitized report JSON
- detect waste patterns and ecosystem fingerprints
- generate a private-link report JSON
- download a free extended report and generate a plugin artifact from the same scan
- view the report in a static local web UI

The production target is CDN + local deterministic CLI + report-only upload + durable private-link report storage. Local development intentionally avoids cloud dependencies so the complete flow can be tested before any infrastructure is provisioned.

## Launch Command

The public launch path is one copy/paste command:

```sh
npx --yes agent-analyzer@latest run
```

That command fetches a scriptless npm package, runs the bundled native Go binary,
analyzes up to three largest-recent logs per supported agent source locally, writes `agent-analyzer-report.json`,
shows the upload boundary, asks for confirmation, uploads only sanitized report
JSON, and opens the private report page.

For users who do not want npm/NPX, versioned GitHub Release archives with
`checksums.txt` remain available. See [docs/distribution.md](docs/distribution.md).

There is intentionally no browser upload form. Agent logs live in hidden tool-specific directories, which are awkward for Finder/browser upload flows. The public launch path is local-first:

1. `npx --yes agent-analyzer@latest run` starts the local native analyzer.
2. The analyzer finds up to three largest-recent logs per supported source, currently Claude Code, Codex, and OpenCode, parses and redacts them locally, and writes `agent-analyzer-report.json`.
3. The CLI prints the upload boundary and asks for confirmation.
4. After confirmation, it sends only the sanitized report to `POST /api/client-reports`.
5. The private report opens at `/r/{job_id}/{report_token}` and remains available for later review.
6. The report page offers a free extended Markdown download from the same sanitized report. The custom plugin artifact is generated from that same scan; there is no second local scan in the public flow.

Legacy raw-log token upload and email/full-scan endpoints still exist for internal compatibility tests. They are not the public onboarding path.

Paid delivery contract: [docs/remediation/plugin-artifacts.md](docs/remediation/plugin-artifacts.md).

## Local Runthrough

The `analyze` subcommand accepts a log path either as a positional argument or
via the `--log` flag. The two forms are mutually exclusive; passing both, or
passing more than one positional, fails fast with a non-zero exit:

```bash
# positional form (equivalent to using --log):
agent-analyzer analyze ~/.claude/projects/some-session.jsonl --out ./report.json

# explicit --log form:
agent-analyzer analyze --log ~/.claude/projects/some-session.jsonl --out ./report.json
```

If neither form is supplied, the CLI auto-discovers up to three largest-recent logs per supported source with a size-and-recency ranking.

```bash
docker compose up --build
```

Open `http://localhost:8080` and use the displayed one-line local analyze/review/upload flow. The smoke scripts also exercise the free extended report, plugin artifact download, and legacy token path with `testdata/fixtures/sample-claude.jsonl` for backend compatibility.

Smoke test:

```bash
./scripts/smoke-local.sh
```

If Docker Desktop is unavailable, the same API/worker path can be checked with:

```bash
./scripts/smoke-native.sh
```

Local load gate:

```bash
COMPOSE_PROJECT_NAME=agent-log-analyzer-load docker compose up --build -d
./scripts/load-local.sh 25
COMPOSE_PROJECT_NAME=agent-log-analyzer-load docker compose down -v
```

Aggregate analytics summary, for retained `analytics.Event` JSONL only:

```bash
go run ./cmd/analytics-summary --input /tmp/agent-log-analyzer/analytics/events.jsonl --min-cohort 10
```

AWS-backend local smoke with LocalStack:

```bash
./scripts/smoke-aws-local.sh
```

## Development

```bash
go test ./...
go run ./cmd/api
go run ./cmd/worker
```

Useful local env vars:

- `CLAUDE_ANALYZER_DATA_DIR`, default `/tmp/agent-log-analyzer`
- `CLAUDE_ANALYZER_ADDR`, default `:8080`
- `CLAUDE_ANALYZER_WORKER_INTERVAL`, default `2s`

## Privacy Posture

Raw logs are treated as toxic. The launch UX parses and redacts locally, emits aggregate-safe ecosystem IDs only, and uploads only sanitized report JSON. Operational logs forbid raw prompt/tool text.

See [docs/data-retention-and-analytics.md](docs/data-retention-and-analytics.md).

Cloud launch checklist: [docs/cloud-launch-todo.md](docs/cloud-launch-todo.md).

## License

Agent Analyzer is source-available, not open source. You may inspect,
clone, and run the software for personal/internal evaluation and development
testing, but production, hosted, commercial, redistribution, and managed-service
uses require a separate written license.

See [LICENSE](LICENSE).
