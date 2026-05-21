# Data Retention And Analytics

## Retention Classes

```text
raw uploaded logs:
  launch CLI flow: never received by the server
  legacy internal token flow: deleted by agent-analyzer-sweeper
  analytics: never

intermediate parsed transcript:
  launch CLI flow: local process memory only
  legacy internal token flow: worker memory or short-lived encrypted temp
  analytics: never

sanitized report JSON:
  local MVP: stored under /data/reports after explicit upload
  production: 15 minutes free, 24 hours full-scan/plugin artifact
  analytics: never as raw JSON

email unlock records:
  local MVP: stored under /data/email_unlocks for Docker lifecycle testing
  production: retained only as needed for confirmation, transactional delivery, consent proof, and abuse prevention
  analytics: email addresses are never copied into aggregate analytics

job metadata:
  local MVP: job JSON files
  production: short-lived metadata with TTL
  analytics: status/timing/error category only

aggregate analytics:
  local MVP: not exported
  production: retained as `analytics.Event` JSONL
  analytics: yes, no raw strings
```

## Operational Logging Allowlist

Allowed:

- `job_id`
- `request_id`
- `file_size_bucket`
- `parser_type`
- `analyzer_version`
- `status`
- `duration_ms`
- `queue_wait_ms`
- `worker_exit_reason`
- `error_category`
- `redaction_counts_by_type`

Forbidden:

- raw transcript text
- prompts
- tool output
- file contents
- secret values
- command arguments
- raw unknown MCP/plugin/skill names
- repo names
- usernames
- hostnames
- full file paths

## Aggregate Ecosystem Intelligence

Collect by default through the retained `analytics.Event` shape:

- known public workflow framework IDs
- known public MCP server IDs
- known public plugin IDs
- known public skill IDs
- OS category
- shell category
- package manager category
- counts and buckets
- SDD fingerprint confidence/source buckets
- MCP and skill utilization warning bands
- recommendation classes, known tool IDs, reasons, risk levels, install policies, and signal IDs

Unknown private names are counted, not stored:

```json
{
  "unknown_mcp_server_count": 3,
  "unknown_skill_count": 4,
  "unknown_plugin_count": 1
}
```

Exact unknown names require explicit opt-in.

The retained event is intentionally narrower than `Report` and
`AggregateSafeEvent`. Report JSON remains a short-lived product artifact; it is
not the analytics storage format. See
[`aggregate-analytics-threat-model.md`](aggregate-analytics-threat-model.md).

## Upload Scope

Free scan analyzes one newest bounded-size session per auto-discovered supported source selected by the local CLI. The current auto-discovered sources are Claude Code, Codex, and OpenCode. The free first pass skips files over 2 MiB to keep the one-line launch command responsive; full scans remove that first-pass cap. The server receives only the generated sanitized report JSON after the user has had a chance to inspect it.

Email-confirmed full scan must use the same local-first model for at most the 100 most recent sessions per supported source. Aggregate analytics from full scans must still use the same allowlist: known public ecosystem IDs, counts, buckets, timing, parser status, and redaction totals. Raw logs, raw paths, unknown private names, emails, and report JSON are not retained as analytics.

Full-scan analytics emit one retained event for the aggregate report. They do
not emit one retained event per raw session inside the full local scan.

## Retained Analytics Storage

Local backend:

- appends retained analytics events to `/data/analytics/events.jsonl`
- stores only `internal/analytics.Event` JSONL
- does not store `Report` JSON as analytics

AWS backend:

- writes encrypted private S3 JSONL objects under
  `analytics/events/date=YYYY-MM-DD/hour=HH/`
- object keys use random server-generated names, not job IDs
- event JSON contains no exact timestamp, token, job ID, session ID, path, or
  stable private hash

Offline analysis:

- `cmd/analytics-summary` reads analytics JSONL and emits cohort summaries
- `--min-cohort` defaults to 10
- rows below the cohort threshold are suppressed

## Tooling Utilization Block

The `Ecosystem.tooling_utilization` block (introduced by Epic #39) is included verbatim inside `AggregateSafeEvent.Ecosystem`. Because `AggregateSafeEvent` embeds `Ecosystem` directly, this new field is part of the upload contract by construction.

Every string-valued field in `tooling_utilization` comes from a fixed closed enumeration or from the public MCP/skill allowlist in `internal/analyzer/signatures/`. The enumerations are:

- **Count buckets**: `none`, `1-3`, `4-10`, `11-25`, `26-50`, `51-100`, `100+`, `unknown`.
- **Context-token buckets**: `none`, `<1k`, `1k-5k`, `5k-15k`, `15k-50k`, `50k+`, `unknown`.
- **Context-efficiency buckets**: `unused`, `underutilized`, `moderate`, `well-utilized`, `unknown`.
- **Warning bands**: `normal`, `watch`, `high`, `severe`, `unknown`.
- **Inference sources**: `header`, `calls`, `none`.
- **Known IDs** (`known_server_ids`, `known_exposed_ids`, `unique_known_called_ids`, `known_executed_ids`): drawn from the public allowlist only.

**No free-form strings are ever included.** Unknown — that is, non-allowlist — MCP server names, MCP tool names, and skill names are emitted as counts only (`unknown_server_count`, `unknown_exposed_count`, `unique_unknown_called_count`, `unknown_executed_count`).

The full JSON Schema is at [`kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json`](../kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/contracts/tooling-utilization.json). The field-by-field reference is at [`kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md`](../kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/data-model.md); the rationale and band thresholds are at [`kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md`](../kitty-specs/mcp-skill-bloat-analytics-01KRZJGV/plan.md).

### What we never collect

Aggregate output (report JSON and `AggregateSafeEvent`) must never contain any of the following (spec C-001):

- user prompts
- task descriptions
- raw transcript excerpts
- raw tool inputs/outputs
- raw MCP schemas/descriptions/arguments
- MCP server URLs
- auth scopes
- private MCP/tool names
- private skill names
- skill instruction text
- skill examples
- user-authored skill docs
- raw file paths
- repo URLs
- branch names
- usernames
- hostnames
- emails
- session IDs
- transcript paths
- stable hashes of any private string

This list is exhaustive: anything in those categories is excluded from every aggregate output, full stop. Allowlist IDs and counts are the only signal that crosses the privacy boundary.

## SDD Fingerprint Privacy

Spec-driven-development (SDD) tool detection follows the same retention and
allowlist discipline as the rest of ecosystem analytics. See
[`sdd-fingerprint-registry.md`](sdd-fingerprint-registry.md) for the
maintainer-facing overview.

- **Bounded record shape.** Each entry in `Ecosystem.WorkflowFingerprints`
  is an `EcosystemFingerprint` carrying only seven primitive fields: `id`,
  `confidence`, `sources`, `evidence_count`, `active`, `installed`,
  `version_bucket`. No `map[string]string`, no free-text `note`, no raw
  path, no raw `--version` output, no `evidence_locations` slice. See
  [data-model.md](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/data-model.md#analyzerecosystemfingerprint)
  for the exact type and invariants.
- **`id` is allowlisted.** It is exactly the detector id from the registry
  (lowercase snake-case, public, never derived from user input).
- **Unknown names remain counts only (FR-011).** Unknown or private MCP /
  skill / plugin / tool names are counted in
  `Ecosystem.UnknownMCPServerCount`, `UnknownSkillCount`, and
  `UnknownPluginCount`. Their names, identifiers, and any derivable hashes
  do not appear in aggregate output or aggregate events. Tools not in the
  registry are simply absent from `WorkflowFingerprints`.
- **Version buckets, not version strings.** `version_bucket` is normalized to
  coarse buckets only (e.g., `"1.x"`, `"4_plus"`, `"unknown"`, or absent).
  The raw `--version` stdout/stderr is consumed in-package and discarded
  before serialization.
- **Build-time enforcement (NFR-001).** A serialization-leak test asserts
  that none of the 16 forbidden raw-string categories appears in any fully
  populated `Report` or `AggregateEvent` payload. Adding a new field to
  `EcosystemFingerprint` requires extending the leak test with a
  corresponding canary in the same PR.

### Forbidden raw-string categories (high level)

The 16 categories the leak test covers (full canary fixtures live in
[`contracts/forbidden-strings.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/forbidden-strings.md)):

1. user prompts
2. task descriptions
3. raw transcript excerpts
4. raw tool inputs
5. raw tool outputs
6. raw file paths
7. repo URLs
8. branch names
9. usernames
10. hostnames
11. emails
12. session IDs
13. transcript paths
14. private MCP / skill / plugin names
15. raw `LookPath` / `which` paths
16. raw `--version` output / stable hashes of private strings
