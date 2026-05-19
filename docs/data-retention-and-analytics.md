# Data Retention And Analytics

## Retention Classes

```text
raw uploaded logs:
  launch CLI flow: never received by the server
  legacy internal token flow: deleted by claude-analyzer-sweeper
  analytics: never

intermediate parsed transcript:
  launch CLI flow: local process memory only
  legacy internal token flow: worker memory or short-lived encrypted temp
  analytics: never

sanitized report JSON:
  local MVP: stored under /data/reports after explicit upload
  production: 15 minutes free, 24 hours paid artifact
  analytics: never as raw JSON

job metadata:
  local MVP: job JSON files
  production: short-lived metadata with TTL
  analytics: status/timing/error category only

aggregate analytics:
  local MVP: not exported
  production: allowlisted numeric/categorical events
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

Collect by default:

- known public workflow framework IDs
- known public MCP server IDs
- known public plugin IDs
- known public skill IDs
- OS category
- shell category
- package manager category
- counts and buckets

Unknown private names are counted, not stored:

```json
{
  "unknown_mcp_server_count": 3,
  "unknown_skill_count": 4,
  "unknown_plugin_count": 1
}
```

Exact unknown names require explicit opt-in.

## Upload Scope

Free scan analyzes exactly one Claude Code JSONL session selected by the local CLI. The server receives only the generated sanitized report JSON after the user has had a chance to inspect it.

Paid scan must use the same local-first model for at most the 100 most recent Claude Code JSONL sessions. Aggregate analytics from paid scans must still use the same allowlist: known public ecosystem IDs, counts, buckets, timing, parser status, and redaction totals. Raw logs, raw paths, unknown private names, and report JSON are not retained as analytics.

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
