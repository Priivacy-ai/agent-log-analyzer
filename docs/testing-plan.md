# Testing Plan

## Quality Gates

```text
go fmt / go vet
unit tests
analyzer golden tests
secret leak tests
prompt injection tests
Docker build
Docker Compose smoke test
```

## Load Test Plan

Use `k6` for HTTP load and synthetic fixture generation for worker pressure.

Target launch model:

```text
500k landing/report page views in 24h
50k analyze clicks
20k upload-init requests
10k completed uploads
1k queued analyses at once
100-300 concurrent worker jobs in production
```

Local acceptance target for this repo before cloud work:

```text
100 sequential uploads through Docker Compose
25 concurrent uploads through Docker Compose
0 raw secret leaks in reports
0 worker crashes
all jobs finish or fail cleanly
```

Current local command:

```bash
./scripts/load-local.sh 25
```

Real local Claude log smoke:

```bash
go run ./cmd/local-log-smoke -limit 10
```

This command discovers `~/.claude/projects/**/*.jsonl`, analyzes the largest logs locally, and prints only aggregate-safe output: buckets, scores, finding IDs, redaction counts, and known ecosystem IDs. It must not print raw transcript text, raw tool output, file contents, or private unknown tool names.

Production acceptance target before launch:

```text
static landing p95: <300ms from CDN
upload-init p95: <250ms
job creation p95: <300ms
report shell p95: <500ms from CDN
normal analysis p95: <3 min
burst queue wait p95: <20 min
API 5xx rate: <0.1%
worker failure rate: <1%
```

## Hostile Upload Tests

- malformed JSONL
- zip/archive bomb once archives are supported
- huge single-line logs
- high-entropy fake secrets
- prompt injection text
- repeated tool output
- worker timeout
- worker memory pressure
