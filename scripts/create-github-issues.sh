#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-robertDouglass/claude-log-analyzer}"

gh label create epic --repo "$REPO" --color 5319e7 --description "Large workstream" >/dev/null 2>&1 || true
gh label create subissue --repo "$REPO" --color 1d76db --description "Planned child issue" >/dev/null 2>&1 || true
gh label create security --repo "$REPO" --color b60205 --description "Security/privacy critical" >/dev/null 2>&1 || true
gh label create ci --repo "$REPO" --color 0e8a16 --description "CI and quality gates" >/dev/null 2>&1 || true
gh label create local-docker --repo "$REPO" --color fbca04 --description "Docker-local runthrough" >/dev/null 2>&1 || true

create_issue() {
  local title="$1"
  local labels="$2"
  local body="$3"
  gh issue create --repo "$REPO" --title "$title" --label "$labels" --body "$body"
}

EPIC_CORE=$(create_issue "Epic: Deterministic analyzer core" "epic" "Build the parser, scrubber, metrics, findings, ecosystem detectors, and report schema. No raw logs may reach reports or aggregate analytics.")
EPIC_SECURITY=$(create_issue "Epic: Security, retention, and privacy boundary" "epic,security" "Keep raw Claude Code logs on the user's machine in the public flow. Use local parsing/redaction, show-before-send sanitized report JSON, strict logging allowlists, prompt-injection tests, and aggregate-only analytics.")
EPIC_LOCAL=$(create_issue "Epic: 100% Docker-local runthrough" "epic,local-docker" "Everything must run locally before cloud infrastructure: static UI, API, local CLI analysis, sanitized report upload, legacy token compatibility path, report generation, smoke tests, and load tests.")
EPIC_SCALE=$(create_issue "Epic: Launch-scale production architecture" "epic" "Prepare CDN/static hosting, local-first report uploads, object storage, metadata TTL, rate limits, WAF body-size handling, and load-shedding.")
EPIC_CI=$(create_issue "Epic: GitHub CI quality gates" "epic,ci" "Use GitHub Actions for formatting, vetting, tests, Docker build, Docker Compose smoke test, and later load/security gates.")
EPIC_ECO=$(create_issue "Epic: Ecosystem signature research sprint" "epic" "Build the comprehensive known-name and fingerprint registry for Claude Code workflows, MCPs, plugins, skills, OS, shells, package managers, and coding agents.")

create_issue "Implement parser fixtures and golden reports" "subissue" "Parent: $EPIC_CORE\n\nAdd fixtures for Claude JSONL, Codex transcripts, malformed logs, retry loops, repeated reads, and prompt injection. Golden reports must be deterministic."
create_issue "Ship signed local analyzer CLI releases" "subissue,security" "Parent: $EPIC_SECURITY\n\nPublish versioned GitHub releases with checksums, then add Homebrew and Scoop install paths. The public flow should not depend on pasted heredoc scripts."
create_issue "Move paid 100-log scan to local-first upload" "subissue,security" "Parent: $EPIC_SECURITY\n\nAnalyze the 100 most recent Claude Code logs locally, write a reviewable sanitized aggregate report, and upload only that report after Stripe unlock."
create_issue "Rename launch host away from Claude-branded subdomain" "subissue,security" "Parent: $EPIC_SECURITY\n\nMove from claude-code.spec-kitty.ai to a host that does not imply Anthropic affiliation unless brand review explicitly approves the current name."
create_issue "Expand secret scrubber detectors" "subissue,security" "Parent: $EPIC_SECURITY\n\nAdd detectors for API keys, JWTs, private keys, URL credentials, cookies, DB URLs, emails, entropy, and known provider keys."
create_issue "Add strict operational logging allowlist" "subissue,security" "Parent: $EPIC_SECURITY\n\nEnsure logs contain only job IDs, status, buckets, durations, error categories, and redaction counts. No raw transcript strings."
create_issue "Add local load test harness" "subissue,local-docker" "Parent: $EPIC_LOCAL\n\nExtend scripts/load-local.sh into a full concurrent sanitized-report upload test with pass/fail thresholds."
create_issue "Map local storage/queue to production adapters" "subissue" "Parent: $EPIC_SCALE\n\nIntroduce interfaces for upload storage, queue, metadata store, and report storage. Add S3/SQS/DynamoDB implementations after local path is stable."
create_issue "Add production load-shedding design" "subissue" "Parent: $EPIC_SCALE\n\nDefine queue-depth thresholds, worker autoscaling rules, LLM-disable mode, and user-facing wait estimates."
create_issue "Harden GitHub Actions smoke test" "subissue,ci" "Parent: $EPIC_CI\n\nKeep Docker Compose smoke test in CI and add artifact upload for logs on failure."
create_issue "Build ecosystem signature registry v1" "subissue" "Parent: $EPIC_ECO\n\nCreate YAML registries for frameworks, MCP servers, plugins, skills, agents, hooks, slash commands, coding agents, package managers, OS, and clients."
create_issue "Build signature research crawler" "subissue" "Parent: $EPIC_ECO\n\nResearch GitHub, npm, PyPI, MCP registries, official docs, and community lists. Output candidate signatures for review."
create_issue "Add privacy tests for ecosystem telemetry" "subissue,security" "Parent: $EPIC_ECO\n\nUnknown MCP/plugin/skill/private command names must never appear in aggregate events by default."

printf 'Created epics:\n%s\n%s\n%s\n%s\n%s\n%s\n' "$EPIC_CORE" "$EPIC_SECURITY" "$EPIC_LOCAL" "$EPIC_SCALE" "$EPIC_CI" "$EPIC_ECO"
