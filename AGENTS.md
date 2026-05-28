## Spec Kitty SaaS Testing On This Computer

- On this computer, when running `spec-kitty` commands that use SaaS, tracker, or sync flows for testing, always set `SPEC_KITTY_ENABLE_SAAS_SYNC=1`.
-  purpose of this machine-level rule is to ensure CLI sync and tracker data flows to Spec Kitty SaaS dev deployment used for testing, currently `https://spec-kitty-dev.fly.dev/`.
- Do not assume flag is optional on this machine during dev testing. If command path touches hosted auth, tracker, or sync behavior, use env var unless user explicitly says not to.
- This is local testing rule for CLI on this computer. It does not mean tracker itself has rollout system, and it does not justify keeping rollout gating inside `spec-kitty-tracker`.

## Project Snapshot

- Product name: Agent Analyzer.
- Purpose: local-first deterministic profiling for AI coding logs. CLI reads supported local agent logs, redacts locally, writes reviewable sanitized report JSON, and uploads only that sanitized JSON after user approval.
- Public launch command: `npx --yes agent-analyzer@latest run`.
- Supported source families include Claude Code, Claude Desktop, Codex, OpenCode, Claude Desktop MCP, Cursor, Kiro, and Google Antigravity. SQLite extraction is read-only and bounded to known conversation/diagnostic rows.
- Hosted reports live at `/r/{job_id}/{report_token}` and offer free report pack plus generated Claude Code plugin/remediation artifact from same sanitized report.
-  benchmark/proof pages are part of product promise. Keep claims tied to repeated benchmark evidence and separate input/context, tool-output, visible output, reasoning, native harness cost, and published API-rate cost.

## Repo Map

- `cmd/agent-analyzer`local CLI, log discovery, source readers, redaction, report generation, and upload prompt.
- `cmd/api`Go HTTP API, static site serving, report pages, private report endpoints, email gate/unlock flow, and admin usage endpoints.
- `cmd/worker` `cmd/sweeper` `cmd/email-events`background processing and operational utilities.
- `internal/analyzer`deterministic analysis, source normalization, privacy scrubber, ecosystem/tooling detection, SDD fingerprinting, and token-saving recommendation logic.
- `internal/remediation`generated report pack and Claude Code plugin artifact generation.
- `internal/backend` `internal/localstore` `internal/awsstore`storage abstractions for local file mode and AWS mode.
- `web`static landing page, report UI assets, privacy/security pages, and benchmark proof pages under `web/proof`.
- `docs/benchmarks`permanent benchmark fixtures, methodology, primary sanitized benchmark data, and cost translation docs.
- `docs/remediation`plugin artifact, receipt email, and token-saving recommendation docs.
- `scripts`smoke tests, benchmark runners, artifact validators, screenshot helpers, npm binary packaging, and AWS deploy.
- `testdata/fixtures/reports`sanitized report JSON fixtures used for report generation tests and screenshots.
- `infra/aws`production AWS infrastructure. Use AWS profile rules below before touching it.

## Core Local Commands

```sh
go test ./...
go run ./cmd/api
go run ./cmd/agent-analyzer run
./scripts/smoke-native.sh
./scripts/smoke-local.sh
./scripts/validate-benchmark-artifacts.py
```

- Use `./scripts/smoke-native.sh` when Docker is unavailable.
- Use `docker compose up --build` for local API/worker path w/ shared local storage.
- For npm packaging checks, use `./scripts/smoke-npm-package.sh`  `./scripts/build-npm-binaries.sh`.

## Product And Privacy Invariants

- Raw logs are toxic. Do not add product flows that upload raw Claude/Codex/OpenCode logs, raw SQLite stores, prompt text, tool output, file contents, command args, secrets, repo names, usernames, hostnames, or full local paths.
- Public intake should accept sanitized report JSON only and reject reports that claim raw transcript LLM exposure.
- Operational logs must stay allowlisted to metadata, timings, status, buckets, counts, and error categories.
- Unknown private MCPs, skills, plugins, and tools stay count-only unless future explicit opt-in path is added.
- Generated artifacts must not contain raw transcript text, raw tool output, secrets, redacted secret values, absolute local paths, or unknown private tool names.
-  public flow has no browser file picker and no hidden access to local agent directories. copy/paste CLI is trust boundary.

## Benchmark And Recommendation Rules

- Product-facing savings claims require repeated A/B evidence: at least three fresh baseline/optimized pairs w/ same prompt, same commit, same quality gate, and passing quality on both sides.
-  permanent suite entry point is `REPEATS=3 ./scripts/benchmark-suite.sh`selected suites can be run w/ `ONLY=...`.
- Before publishing proof-page changes, run `./scripts/validate-benchmark-artifacts.py`.
- Primary sanitized recordings live under `docs/benchmarks/primary-data/`public aggregates live under `web/proof/reports/aggregate-*.json`.
- Do not promote telemetry-only tools such as `ccusage`  `ccstatusline` as token reducers.
- Current default recommendation posture: Agent Analyzer guidance is positive; Semble is positive for bounded retrieval; context-mode, RTK, and grepai are conditional; Squeez is removed because it conflicts with Spec Kitty workflows; claude-context, Probe, Claude Code Caveman, claude-rlm, and claude-token-efficient are removed or downgraded for default token-saving claims based on current repeated suite.
- Cost copy should scale repeated percentage deltas, not one-task cents, and must state basis when extrapolating monthly/team savings.

## Web And Report UI Rules

-  landing page is intentionally structured around copy/paste `npx --yes agent-analyzer@latest run` CTA, local-first trust proof, benchmark proof, report preview, and email-gated report/plugin download flow. Do not replace structure w/o explicit request.
- Static proof, security, and privacy pages share `web/site-header.js` component `site-header` CSS. Keep those pages visually consistent.
- Report page rendering lives mostly in `cmd/api/report_html.go`static report behavior/assets live in `web/app.js` `web/report-actions.js` `web/tooltips.js` `web/styles.css`.
- When testing report generation or screenshots, prefer sanitized fixtures in `testdata/fixtures/reports/`.

## Git Workflow

- Unless user explicitly requests different branch or workflow, all agents must work directly on `main`.
- For all completed work, agents must commit their changes and push to `origin/main`.
- Do not leave completed work only in local working tree or on feature branch unless user explicitly requested that.

## Web Page Preview Rule

- Do not show or hand off local or deployed web page to user until you have loaded it yourself w/ Playwright.
- For local static pages, serve relevant dir over HTTP before checking page so root-relative CSS, JavaScript, and image assets load correctly.
- Verify requested page renders w/ its CSS and images before giving user URL.

## AWS Deployment Profile

- Use `claude-analyzer-prod` AWS profile for production infrastructure work.
- Default deployment region is `us-east-1`.
- Prefer setting environment before Terraform/AWS commands:

```sh
export AWS_PROFILE=claude-analyzer-prod
export AWS_REGION=us-east-1
terraform -chdir=infra/aws plan
```

- One-off equivalent:

```sh
AWS_PROFILE=claude-analyzer-prod terraform -chdir=infra/aws plan
```

- Do not paste AWS access keys or secret access keys into chat, docs, commits, or logs.
-  local `.env` may contain profile/region selectors only. It must not contain credentials.
-  profile may exist before it has sufficient IAM permissions. Verify identity and permissions before applying infrastructure.

## Production Usage Stats Access

- Production usage stats are exposed through bearer-authenticated admin endpoint.
- Do not document credential locations, service names, secret IDs, token hashes, or raw tokens in public repo files.
- Retrieve admin token only from operator's private credential store, keep it out of shell history where practical, and never paste it into chat, docs, commits, or logs.
- Use macOS Keychain from shell when user asks for production usage stats or email-form data. Prefer command substitution so token never appears in terminal output:

```sh
TOKEN="$(security find-generic-password -s '<keychain-service>' -a '<keychain-account>' -w)"
curl -fsS 'https://analyzer.spec-kitty.ai/api/admin/usage-stats?days=1' \
  -H "Authorization: Bearer ${TOKEN}"
```

- If Keychain service/account is unknown, first do metadata-only search and do not print passwords:

```sh
security dump-keychain 2>/dev/null | rg -i -C 3 'claude|analyzer|agent|usage|admin'
```

- Use `GET /api/admin/usage-stats?days=N` for aggregate traffic analytics. Useful fields include `event_count` `requests` `unique_client_hashes` `by_path` `by_browser` `by_operating_system` `by_device_class` `by_language` `by_referrer_host`and UTM aggregates.
- Use `GET /api/admin/email-unlocks?days=N&limit=1000` for email-form/export records. Treat current report-pack deliveries as records w/ `status: "used"`  `source_report_job_id`older `pending`/`confirmed` records can be legacy full-scan unlock cruft and should not be described as current email-confirmation behavior.
- Current no-confirmation report-pack email form posts to `/api/report-deliveries`. There is no user-facing email confirmation step for that flow.
- If admin endpoint is unavailable, read same usage events from encrypted production report bucket w/ `claude-analyzer-prod` AWS profile, but keep raw event dumps out of chat unless user explicitly asks. Summarize counts and trends instead.
- For future website analytics + email reports, use `scripts/production-analytics-report.py` unless there is reason to inspect raw data manually. default owner filter treats every `@robshouse.net` address as Robert's and excludes it from "not my own" email counts; keep email addresses masked in chat unless explicitly asked otherwise.
