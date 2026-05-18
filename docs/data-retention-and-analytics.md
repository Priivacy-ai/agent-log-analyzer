# Data Retention And Analytics

## Retention Classes

```text
raw uploaded logs:
  local MVP: deleted by sweeper script / test path
  production: 15 minutes max
  analytics: never

intermediate parsed transcript:
  local MVP: memory only
  production: worker memory or short-lived encrypted temp
  analytics: never

sanitized report JSON:
  local MVP: stored under /data/reports
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

