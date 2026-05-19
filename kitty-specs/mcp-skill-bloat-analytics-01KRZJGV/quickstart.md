# Quickstart: MCP and Skill Bloat Analytics

**Mission**: `mcp-skill-bloat-analytics-01KRZJGV`

## Local dev loop

```sh
# From the repository root checkout.
cd /Users/robert/code-analyzer-dev/claude-code-profiler-20260519-093035-WPIi53/claude-code-profiler

# Implementation will live on this branch (cut by /spec-kitty.implement, not now).
git switch -c codex/mcp-skill-utilization   # add timestamp suffix if branch exists

# Iterate.
gofmt -w $(find . -name '*.go' -not -path './.git/*')
go test ./internal/analyzer/...      # fast inner loop while iterating on tooling.go
go test ./...                        # full suite before commit
./scripts/smoke-local.sh             # end-to-end smoke; document blocker if it fails
```

## How to verify the feature works

After implementation, point the analyzer at one of the fixtures and inspect the JSON:

```sh
go run ./cmd/cca analyze \
  --input internal/analyzer/testdata/tooling/03-many-low-util.log \
  --out /tmp/report.json
jq '.ecosystem.tooling_utilization' /tmp/report.json
```

Expected for fixture `03-many-low-util` (paraphrased — exact bucket labels pinned in `data-model.md`):

```json
{
  "mcp": {
    "known_server_ids": ["github", "linear"],
    "unknown_server_count": 28,
    "server_count_bucket": "26-50",
    "exposed_tool_count_bucket": "51-100",
    "context_token_bucket": "15k-50k",
    "exposure_known": true,
    "inference_source": "header",
    "call_count": 2,
    "utilization_ratio_pct": 6,
    "context_efficiency_bucket": "unused",
    "warning_band": "high",
    ...
  },
  "skill": { ... }
}
```

And:

```sh
jq '.immediate_fixes' /tmp/report.json
# Expect at least one bloat-remediation string from FR-006's fixed set.
```

## Privacy spot-check

Run the analyzer against the privacy-leak fixture and grep the output for forbidden substrings — expect zero matches:

```sh
go run ./cmd/cca analyze \
  --input internal/analyzer/testdata/tooling/06-private-only.log \
  --out /tmp/private-report.json

# Every one of these must return zero matches:
grep -F 'acme_internal_mcp_server'   /tmp/private-report.json || echo "OK: no private MCP name"
grep -F 'super_secret_skill_name'    /tmp/private-report.json || echo "OK: no private skill name"
grep -F '/Users/robert/secret/path'  /tmp/private-report.json || echo "OK: no raw path"
grep -F 'AKIA'                       /tmp/private-report.json || echo "OK: no secret pattern"
```

Same expectation when running against the upload-safe shape only:

```sh
jq '.aggregate_event' /tmp/private-report.json | grep -F 'acme_internal' && echo "FAIL" || echo "OK"
```

## Where to look in the code

- `internal/analyzer/types.go` — new `ToolingUtilization`/`MCPUtilization`/`SkillUtilization` structs (data-model.md).
- `internal/analyzer/tooling.go` — bucketing helpers, band classifier, footprint estimator, exposure detector (NEW file).
- `internal/analyzer/ecosystem.go` — `DetectEcosystem` extended to populate `ToolingUtilization`.
- `internal/analyzer/analyzer.go` — `buildFindings` extended with `mcp_bloat_*` and `skill_bloat_*` finding IDs.
- `internal/analyzer/testdata/tooling/` — 7 synthetic fixtures.
- `docs/ecosystem-signatures.md` — user-facing description of the metrics and privacy stance.

## When you're confident, ship it

```sh
git add internal/analyzer/ docs/
git commit -m "Add MCP and skill utilization analytics"
git push -u origin codex/mcp-skill-utilization
```

Then comment on GitHub issues #39, #51, #52, #53, #54, #55, #56, #57 per the brief's "GitHub Issue Hygiene" section. Open a PR against `main`.
