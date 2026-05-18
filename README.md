# Claude Log Analyzer

Deterministic performance profiler for AI coding workflows.

This repo starts with a Docker-local, end-to-end implementation:

- upload one Claude/Codex-style log
- parse and scrub it deterministically
- detect waste patterns and ecosystem fingerprints
- generate an ephemeral report JSON
- view the report in a static local web UI

The production target is CDN + signed uploads + object storage + queue + isolated workers. Local development intentionally avoids cloud dependencies so the complete flow can be tested before any infrastructure is provisioned.

## Local Runthrough

```bash
docker compose up --build
```

Open `http://localhost:8080`, upload `testdata/fixtures/sample-claude.jsonl`, and wait for the report.

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
