# Plan: Aggregate Intelligence Privacy-Safe Analytics

## Architecture

Add `internal/analytics` as a narrow extraction layer:

```text
short-lived Report
  |
  +--> web/report/product storage (TTL)
  |
  +--> analytics.FromReport(report, scanType)
         |
         +--> analytics.Event JSONL (retained)
         +--> cmd/analytics-summary (offline cohort summaries)
```

`analytics.Event` is not a renamed `Report`; it is an allowlisted retained
contract. It filters all client-provided strings through public registries or
closed enum sets.

## Decisions

| ID | Decision |
| --- | --- |
| D-01 | Retain analytics as JSONL events, not report JSON. |
| D-02 | Use existing local data dir and existing AWS private report bucket for first analytics storage. A dedicated bucket can come later. |
| D-03 | Store no exact timestamp in event JSON. AWS object partitioning may use date/hour but event body stays identifier-free. |
| D-04 | Suppress summary rows below `min_cohort`, default 10. |
| D-05 | Treat malformed client-provided IDs as unknown/private and drop them from retained ID arrays. |

## Implementation Map

| Work package | Files |
| --- | --- |
| WP01 analytics contract | `internal/analytics/event.go`, `internal/analyzer/registry.go` |
| WP02 storage wiring | `internal/app/ports.go`, `internal/localstore/store.go`, `internal/awsstore/store.go`, `cmd/api/main.go`, `cmd/worker/main.go` |
| WP03 summary CLI | `internal/analytics/summary.go`, `cmd/analytics-summary/main.go` |
| WP04 docs/tests | `internal/analytics/*_test.go`, `docs/aggregate-analytics-threat-model.md`, `docs/data-retention-and-analytics.md`, `docs/logging-policy.md` |

## Validation

Required:

```sh
go test ./...
terraform -chdir=infra/aws fmt -check -recursive
./scripts/smoke-local.sh
COMPOSE_PROJECT_NAME=agent-log-analyzer-aggregate docker compose up --build -d
./scripts/load-local.sh 25
COMPOSE_PROJECT_NAME=agent-log-analyzer-aggregate docker compose down -v
```

Optional manual summary check:

```sh
go run ./cmd/analytics-summary --input /tmp/agent-log-analyzer/analytics/events.jsonl --min-cohort 10
```
