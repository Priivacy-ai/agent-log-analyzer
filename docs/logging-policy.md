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

## CLI presence and version probes

The SDD fingerprint registry probes a small allowlist of public CLI binary
names to record whether a tool is installed locally. The probe surface is
constrained by
[`contracts/cli-probe.md`](../kitty-specs/top-20-sdd-fingerprint-registry-01KRZEQ3/contracts/cli-probe.md);
the logging-side rules that follow from it:

- **Never log resolved executable paths.** `LookPath` returns only a boolean
  to its caller. The resolved path MUST NOT be written to any log line,
  error message, error wrap, structured field, span attribute, metric label,
  or temporary buffer that another goroutine could observe.
- **Never log raw `--version` output.** The raw stdout/stderr of a version
  probe is consumed by the in-package version-bucket normalizer and
  discarded. The normalized `version_bucket` (e.g., `"1.2"`) is the only
  derived value that may be persisted or emitted.
- **`version_args` deny-list.** The registry loader rejects any detector
  whose `version_args` contains `"--config"`, `"--registry"`, `"--token"`,
  `"--server"`, `"--login"`, or any value containing `/`. Acceptable
  arguments are limited to `--version`, `version`, or `-v`. Loader rejection
  panics at startup.
- **2-second wall-clock timeout per probe (NFR-002).** Probes run with no
  shell, no stdin, a sanitized empty `Env`, and no network access. On
  timeout the probe records `installed: true` (if `LookPath` succeeded) and
  no version bucket; no timeout-related log line may include the resolved
  path or the partial output.

