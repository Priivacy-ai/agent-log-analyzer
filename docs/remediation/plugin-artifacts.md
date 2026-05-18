# Paid Claude Plugin Artifacts

The paid product is a generated Claude Code plugin/remediation artifact, not a longer report.

## Contract

Free scan:

- analyzes one Claude Code JSONL log
- shows deterministic problems, evidence, and generic fixes
- offers the paid unlock

Paid scan:

- uses a separate paid token
- uploads at most the 100 most recent Claude Code JSONL logs
- sends `limit=100` and `X-Scan-Limit: 100`
- finalizes with `POST /api/paid-uploads/{job_id}/finalize`
- aggregates deterministic metrics across sessions
- generates a customized Claude Code plugin archive
- shows copyable install commands and a Claude-native install prompt

## Plugin Shape

Claude Code plugins use a root directory with:

- `.claude-plugin/plugin.json`
- `skills/<name>/SKILL.md`
- `commands/*.md`
- `hooks/hooks.json`
- supporting scripts under `scripts/`

The generator follows that structure so the paid artifact can be loaded as a Claude Code plugin archive. The default command uses Claude Code's session-scoped plugin loading:

```sh
PLUGIN_URL="<short-lived-plugin-zip-url>"
PLUGIN_ZIP="$(mktemp -t claude-analyzer-plugin.XXXXXX.zip)"
curl -fsS "$PLUGIN_URL" -o "$PLUGIN_ZIP"
claude --plugin-dir "$PLUGIN_ZIP"
```

Marketplace installation can be added after the plugin store is live. The generated artifact format should remain the same.

## Generated Contents

Always generated:

- `.claude-plugin/plugin.json`
- `README.md`
- `hooks/hooks.json`
- `scripts/claude-analyzer-hook.py`
- `commands/claude-analyzer-status.md`
- `skills/session-hygiene/SKILL.md`

Conditionally generated from deterministic findings:

- `skills/retrieval-hygiene/SKILL.md` for repeated file reads
- `skills/output-budget/SKILL.md` for large shell/tool output overhead
- `skills/retry-breaker/SKILL.md` for retry-loop behavior
- session hygiene guidance is tuned for context growth spikes

## Customization Rules

Customization inputs are limited to:

- finding IDs
- severity and cost-impact labels
- bounded counts and token-share percentages
- analyzer score and waste buckets
- allowlisted public ecosystem IDs

Forbidden in artifacts:

- raw transcript text
- raw tool output
- secrets or redacted secret values
- absolute local paths
- raw unknown MCP/plugin/skill names
- repo names, usernames, hostnames, emails
- prompt injection text from logs

## Tests

The current generator tests cover:

- Claude plugin structure
- deterministic output for identical sanitized inputs
- leak checks for fake secrets, absolute paths, and private unknown names
- zip archive creation
- rejection of unsafe archive paths

## Implementation

The first implementation lives in `internal/remediation`.

The package produces an in-memory `Artifact` and can write it as a zip archive. API wiring, Stripe success handling, paid bundle upload, and artifact TTL storage are tracked separately in GitHub issues #27-#31.
