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
20k local CLI report uploads
10k completed report views
```

Local acceptance target for this repo before cloud work:

```text
100 sequential sanitized report uploads through Docker Compose
25 concurrent sanitized report uploads through Docker Compose
0 raw secret leaks in reports
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

Cloud report-upload load smoke:

```bash
CLAUDE_ANALYZER_URL=http://<alb-dns> ./scripts/load-local.sh 25
```

The load command must use fake-secret fixtures by default and exercise local analysis, sanitized report upload, and tokenized report fetch. It prints only aggregate pass/fail status and checks that raw fake secrets do not leak into reports.

Full Docker smoke:

```bash
./scripts/smoke-local.sh
```

This covers the local sanitized-report upload path plus the legacy free one-log token path and local waiver-gated paid bundle path kept for compatibility. It verifies aggregate report fetch, tokenized plugin zip download, and raw-transcript leak checks.

Production acceptance target before launch:

```text
static landing p95: <300ms from CDN
sanitized report upload p95: <500ms
report shell p95: <500ms from CDN
API 5xx rate: <0.1%
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
- paid-scan tar/gzip bundle with 100 JSONL files
