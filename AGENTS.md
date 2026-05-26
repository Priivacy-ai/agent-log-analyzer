## Spec Kitty SaaS Testing On This Computer

- On this computer, when running `spec-kitty` commands that use SaaS, tracker, or sync flows for testing, always set `SPEC_KITTY_ENABLE_SAAS_SYNC=1`.
- The purpose of this machine-level rule is to ensure CLI sync and tracker data flows to the Spec Kitty SaaS dev deployment used for testing, currently `https://spec-kitty-dev.fly.dev/`.
- Do not assume the flag is optional on this machine during dev testing. If a command path touches hosted auth, tracker, or sync behavior, use the env var unless the user explicitly says not to.
- This is a local testing rule for the CLI on this computer. It does not mean tracker itself has a rollout system, and it does not justify keeping rollout gating inside `spec-kitty-tracker`.

## Project Snapshot

- Product name: Agent Analyzer.
- Purpose: local-first deterministic profiling for AI coding logs. The CLI reads supported local agent logs, redacts locally, writes a reviewable sanitized report JSON, and uploads only that sanitized JSON after user approval.
- Public launch command: `npx --yes agent-analyzer@latest run`.
- Supported source families include Claude Code, Claude Desktop, Codex, OpenCode, Claude Desktop MCP, Cursor, Kiro, and Google Antigravity. SQLite extraction is read-only and bounded to known conversation/diagnostic rows.
- Hosted reports live at `/r/{job_id}/{report_token}` and offer a free report pack plus a generated Claude Code plugin/remediation artifact from the same sanitized report.
- The benchmark/proof pages are part of the product promise. Keep claims tied to repeated benchmark evidence and separate input/context, tool-output, visible output, reasoning, native harness cost, and published API-rate cost.

## Repo Map

- `cmd/agent-analyzer`: local CLI, log discovery, source readers, redaction, report generation, and upload prompt.
- `cmd/api`: Go HTTP API, static site serving, report pages, private report endpoints, email gate/unlock flow, and admin usage endpoints.
- `cmd/worker`, `cmd/sweeper`, `cmd/email-events`: background processing and operational utilities.
- `internal/analyzer`: deterministic analysis, source normalization, privacy scrubber, ecosystem/tooling detection, SDD fingerprinting, and token-saving recommendation logic.
- `internal/remediation`: generated report pack and Claude Code plugin artifact generation.
- `internal/backend`, `internal/localstore`, `internal/awsstore`: storage abstractions for local file mode and AWS mode.
- `web`: static landing page, report UI assets, privacy/security pages, and benchmark proof pages under `web/proof`.
- `docs/benchmarks`: permanent benchmark fixtures, methodology, primary sanitized benchmark data, and cost translation docs.
- `docs/remediation`: plugin artifact, receipt email, and token-saving recommendation documentation.
- `scripts`: smoke tests, benchmark runners, artifact validators, screenshot helpers, npm binary packaging, and AWS deploy.
- `testdata/fixtures/reports`: sanitized report JSON fixtures used for report generation tests and screenshots.
- `infra/aws`: production AWS infrastructure. Use the AWS profile rules below before touching it.

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
- Use `docker compose up --build` for the local API/worker path with shared local storage.
- For npm packaging checks, use `./scripts/smoke-npm-package.sh` and `./scripts/build-npm-binaries.sh`.

## Product And Privacy Invariants

- Raw logs are toxic. Do not add product flows that upload raw Claude/Codex/OpenCode logs, raw SQLite stores, prompt text, tool output, file contents, command arguments, secrets, repo names, usernames, hostnames, or full local paths.
- Public intake should accept sanitized report JSON only and reject reports that claim raw transcript LLM exposure.
- Operational logs must stay allowlisted to metadata, timings, status, buckets, counts, and error categories.
- Unknown private MCPs, skills, plugins, and tools stay count-only unless a future explicit opt-in path is added.
- Generated artifacts must not contain raw transcript text, raw tool output, secrets, redacted secret values, absolute local paths, or unknown private tool names.
- The public flow has no browser file picker and no hidden access to local agent directories. The copy/paste CLI is the trust boundary.

## Benchmark And Recommendation Rules

- Product-facing savings claims require repeated A/B evidence: at least three fresh baseline/optimized pairs with the same prompt, same commit, same quality gate, and passing quality on both sides.
- The permanent suite entry point is `REPEATS=3 ./scripts/benchmark-suite.sh`; selected suites can be run with `ONLY=...`.
- Before publishing proof-page changes, run `./scripts/validate-benchmark-artifacts.py`.
- Primary sanitized recordings live under `docs/benchmarks/primary-data/`; public aggregates live under `web/proof/reports/aggregate-*.json`.
- Do not promote telemetry-only tools such as `ccusage` or `ccstatusline` as token reducers.
- Current default recommendation posture: Agent Analyzer guidance is positive; Semble is positive for bounded retrieval; context-mode, RTK, grepai, and Squeez are conditional; claude-context, Probe, Claude Code Caveman, claude-rlm, and claude-token-efficient are removed or downgraded for default token-saving claims based on the current repeated suite.
- Cost copy should scale repeated percentage deltas, not one-task cents, and must state the basis when extrapolating monthly/team savings.

## Web And Report UI Rules

- The landing page is intentionally structured around the copy/paste `npx --yes agent-analyzer@latest run` CTA, local-first trust proof, benchmark proof, report preview, and email-gated report/plugin download flow. Do not replace the structure without an explicit request.
- Static proof, security, and privacy pages share the `web/site-header.js` component and `site-header` CSS. Keep those pages visually consistent.
- Report page rendering lives mostly in `cmd/api/report_html.go`; static report behavior/assets live in `web/app.js`, `web/report-actions.js`, `web/tooltips.js`, and `web/styles.css`.
- When testing report generation or screenshots, prefer sanitized fixtures in `testdata/fixtures/reports/`.

## Git Workflow

- Unless the user explicitly requests a different branch or workflow, all agents must work directly on `main`.
- For all completed work, agents must commit their changes and push to `origin/main`.
- Do not leave completed work only in the local working tree or on a feature branch unless the user explicitly requested that.

## Web Page Preview Rule

- Do not show or hand off a local or deployed web page to the user until you have loaded it yourself with Playwright.
- For local static pages, serve the relevant directory over HTTP before checking the page so root-relative CSS, JavaScript, and image assets load correctly.
- Verify the requested page renders with its CSS and images before giving the user the URL.

## AWS Deployment Profile

- Use the `claude-analyzer-prod` AWS profile for production infrastructure work.
- Default deployment region is `us-east-1`.
- Prefer setting the environment before Terraform/AWS commands:

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
- The local `.env` may contain profile/region selectors only. It must not contain credentials.
- The profile may exist before it has sufficient IAM permissions. Verify identity and permissions before applying infrastructure.

## Production Usage Stats Access

- Production usage stats are exposed through a bearer-authenticated admin endpoint.
- Do not document credential locations, service names, secret IDs, token hashes, or raw tokens in public repo files.
- Retrieve the admin token only from the operator's private credential store, keep it out of shell history where practical, and never paste it into chat, docs, commits, or logs.
- Use macOS Keychain from the shell when the user asks for production usage stats or email-form data. Prefer command substitution so the token never appears in terminal output:

```sh
TOKEN="$(security find-generic-password -s '<keychain-service>' -a '<keychain-account>' -w)"
curl -fsS 'https://analyzer.spec-kitty.ai/api/admin/usage-stats?days=1' \
  -H "Authorization: Bearer ${TOKEN}"
```

- If the Keychain service/account is unknown, first do a metadata-only search and do not print passwords:

```sh
security dump-keychain 2>/dev/null | rg -i -C 3 'claude|analyzer|agent|usage|admin'
```

- Use `GET /api/admin/usage-stats?days=N` for aggregate traffic analytics. Useful fields include `event_count`, `requests`, `unique_client_hashes`, `by_path`, `by_browser`, `by_operating_system`, `by_device_class`, `by_language`, `by_referrer_host`, and UTM aggregates.
- Use `GET /api/admin/email-unlocks?days=N&limit=1000` for email-form/export records. Treat current report-pack deliveries as records with `status: "used"` and a `source_report_job_id`; older `pending`/`confirmed` records can be legacy full-scan unlock cruft and should not be described as current email-confirmation behavior.
- Current no-confirmation report-pack email form posts to `/api/report-deliveries`. There is no user-facing email confirmation step for that flow.
- If the admin endpoint is unavailable, read the same usage events from the encrypted production report bucket with the `claude-analyzer-prod` AWS profile, but keep raw event dumps out of chat unless the user explicitly asks. Summarize counts and trends instead.
- For future website analytics + email reports, use `scripts/production-analytics-report.py` unless there is a reason to inspect raw data manually. The default owner filter treats every `@robshouse.net` address as Robert's and excludes it from "not my own" email counts; keep email addresses masked in chat unless explicitly asked otherwise.
