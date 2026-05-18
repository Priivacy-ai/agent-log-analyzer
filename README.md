# Claude Log Analyzer

Deterministic performance profiler for AI coding workflows.

This repo starts with a Docker-local, end-to-end implementation:

- generate a one-time upload token
- copy a Claude-native prompt or curl command
- upload one Claude Code JSONL log through the tokenized curl path
- parse and scrub it deterministically
- detect waste patterns and ecosystem fingerprints
- generate an ephemeral report JSON
- view the report in a static local web UI

The production target is CDN + tokenized curl upload + object storage + queue + isolated workers. Local development intentionally avoids cloud dependencies so the complete flow can be tested before any infrastructure is provisioned.

There is intentionally no browser upload form. Claude Code logs live under `~/.claude`, which is awkward for Finder/browser upload flows and pushes users away from the native Claude Code workflow. The public upload path is:

1. `POST /api/analysis-sessions` creates a one-time token and short-lived report URL.
2. The user pastes the generated prompt into Claude Code or runs the generated curl command.
3. The command uploads one latest JSONL log with `PUT /api/uploads/{job_id}` and finalizes it with `POST /api/uploads/{job_id}/finalize`.
4. The report is opened at `/r/{job_id}/{report_token}` and expires on the retention schedule.

The paid scan will use a separate paid token and a different command parameter set: `CLAUDE_ANALYZER_SCAN_LIMIT=100`, `limit=100`, and `X-Scan-Limit: 100`. That command uploads a tar/gzip bundle of the 100 most recent Claude Code JSONL logs after Stripe unlock.

## Local Runthrough

```bash
docker compose up --build
```

Open `http://localhost:8080`, click `Generate Claude Prompt`, and use the generated prompt/curl flow. The smoke scripts exercise the same path with `testdata/fixtures/sample-claude.jsonl`.

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
COMPOSE_PROJECT_NAME=claude-log-analyzer-load docker compose up --build -d
./scripts/load-local.sh 25
COMPOSE_PROJECT_NAME=claude-log-analyzer-load docker compose down -v
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

- `CLAUDE_ANALYZER_DATA_DIR`, default `/tmp/claude-log-analyzer`
- `CLAUDE_ANALYZER_ADDR`, default `:8080`
- `CLAUDE_ANALYZER_WORKER_INTERVAL`, default `2s`

## Privacy Posture

Raw uploads are treated as toxic. The analyzer redacts secrets before reports are written, emits aggregate-safe ecosystem IDs only, and forbids raw prompt/tool text in operational logs.

See [docs/data-retention-and-analytics.md](docs/data-retention-and-analytics.md).

Cloud launch checklist: [docs/cloud-launch-todo.md](docs/cloud-launch-todo.md).
