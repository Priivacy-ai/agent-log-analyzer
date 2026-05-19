# Operational Logging Policy

Operational logs are for running the service, not debugging user content.

Allowed fields:

- request method
- route template, not raw dynamic path
- job ID only where needed for operations
- status
- duration bucket
- queue wait bucket
- score bucket
- redaction counts
- error category

Forbidden fields:

- raw transcript text
- prompt text
- tool output
- command arguments
- upload storage path
- report storage path
- full URL with user-controlled path/query
- raw secret values
- unknown private MCP/plugin/skill names

Current enforcement:

- API request logging uses route-level path sanitization.
- Job status responses strip upload and report storage paths.
- Worker logs job ID and score bucket only.
- Aggregate ecosystem telemetry stores known public IDs and unknown counts only.

## MCP and Skill Utilization

Analyzer aggregate events include privacy-safe utilization metrics for MCP servers and skills. See [ecosystem-signatures.md](./ecosystem-signatures.md) for what's measured and [data-retention-and-analytics.md](./data-retention-and-analytics.md) for the upload contract. Private MCP/skill names are counted only; nothing identifying is logged, stored, or uploaded.

