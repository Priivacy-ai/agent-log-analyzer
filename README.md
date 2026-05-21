# Agent Analyzer

Deterministic performance profiler for AI coding workflows.

This repo starts with a Docker-local, end-to-end implementation:

- run the analyzer locally against one supported agent log per source
- write a sanitized report JSON that the user can inspect before upload
- upload only the sanitized report JSON
- detect waste patterns and ecosystem fingerprints
- generate a private-link report JSON
- email-confirm a free full-scan token for plugin generation
- view the report in a static local web UI

The production target is CDN + local deterministic CLI + report-only upload + durable private-link report storage. Local development intentionally avoids cloud dependencies so the complete flow can be tested before any infrastructure is provisioned.

## Launch Command

The public launch path is one copy/paste command:

```sh
npx --yes agent-analyzer@latest run
```

That command fetches a scriptless npm package, runs the bundled native Go binary,
analyzes one newest log per supported agent source locally, writes `agent-analyzer-report.json`,
shows the upload boundary, asks for confirmation, uploads only sanitized report
JSON, and opens the private report page.

For users who do not want npm/NPX, versioned GitHub Release archives with
`checksums.txt` remain available. See [docs/distribution.md](docs/distribution.md).

There is intentionally no browser upload form. Agent logs live in hidden tool-specific directories, which are awkward for Finder/browser upload flows. The public launch path is local-first:

1. `npx --yes agent-analyzer@latest run` starts the local native analyzer.
2. The analyzer finds one latest bounded-size log per supported source, currently Claude Code, Codex, and OpenCode, parses and redacts them locally, and writes `agent-analyzer-report.json`.
3. The CLI prints the upload boundary and asks for confirmation.
4. After confirmation, it sends only the sanitized report to `POST /api/client-reports`.
5. The private report opens at `/r/{job_id}/{report_token}` and remains available for later review.

The full optimization scan is email-confirmed during launch testing: the report page asks for an email, sends a confirmation link, then sends a one-line `npx --yes agent-analyzer@latest full-scan --token ...` command. That command analyzes up to 100 recent logs per supported source locally and uploads only sanitized aggregate JSON for report/plugin generation.

Legacy raw-log token upload endpoints still exist for internal Docker smoke coverage while the full scan is moved to the same local-first model. They are not the public onboarding path.

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

If neither form is supplied, the CLI auto-discovers one newest log per supported source, skipping files over 2 MiB in the free first pass so the one-line launch command stays responsive. The email-confirmed full scan uses the same discovery model with up to 100 logs per source.

```bash
docker compose up --build
```

Open `http://localhost:8080` and use the displayed one-line local analyze/review/upload flow. The smoke scripts also exercise the email-confirmed full-scan/plugin lifecycle and the legacy token path with `testdata/fixtures/sample-claude.jsonl` for backend compatibility.

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
