# Spec: Aggregate Intelligence Privacy-Safe Analytics

| Field | Value |
| --- | --- |
| Mission slug | `aggregate-intelligence-privacy-safe-analytics-01KS214Y` |
| Mission ID | `01KS214YZRA30F90QKXSYBKGB9` |
| Mission type | software-dev |
| Target branch | `main` |
| Source brief | `/Users/robert/code-analyzer-dev/aggregate-intelligence-20260520-081911-cbwF6i/start-here.md` |
| Upstream issues | [#40](https://github.com/robertDouglass/claude-log-analyzer/issues/40), [#58](https://github.com/robertDouglass/claude-log-analyzer/issues/58), [#59](https://github.com/robertDouglass/claude-log-analyzer/issues/59), [#60](https://github.com/robertDouglass/claude-log-analyzer/issues/60), [#61](https://github.com/robertDouglass/claude-log-analyzer/issues/61), [#65](https://github.com/robertDouglass/claude-log-analyzer/issues/65) |

## Purpose

Retain useful ecosystem intelligence from sanitized Claude Analyzer reports
without retaining user content, raw report JSON, user/repo identifiers, or
stable private fingerprints. The retained analytics surface must be narrower
than `Report` and explicit enough that future report fields cannot become
analytics by accident.

## User Scenarios & Testing

### Primary scenario

A user analyzes Claude Code logs locally and uploads only a sanitized report.
The API stores the short-lived report for product display, extracts a separate
bounded analytics event, and appends that event to the analytics store. Later,
an operator runs an offline summary command over retained analytics JSONL and
gets cohort-level adoption, co-occurrence, MCP/skill bloat, and recommendation
frequency summaries. Small cohorts are suppressed by default.

### Acceptance scenarios

| # | Input | Expected outcome |
| --- | --- | --- |
| AS-01 | Client report containing allowlisted SDD/MCP/skill/recommendation fields | Analytics event retains public IDs, buckets, and counts |
| AS-02 | Client report containing private MCP/skill/plugin/tool names in known arrays | Analytics event drops the private names and preserves unknown counts only |
| AS-03 | Report with raw prompt/path/tool-output canaries in finding evidence | Analytics event contains only finding ID and severity |
| AS-04 | Free report upload through `POST /api/client-reports` | Local/AWS store receives one analytics event; report storage remains short-lived |
| AS-05 | Legacy worker path or paid bundle path completes a job | Worker appends one analytics event for the completed report |
| AS-06 | JSONL containing rare SDD co-occurrences | `analytics-summary --min-cohort 10` suppresses rows below threshold |

## Functional Requirements

| ID | Description |
| --- | --- |
| FR-001 | Define a retained analytics event narrower than `analyzer.Report` and `AggregateSafeEvent`. |
| FR-002 | Retain scan type, parser/input/turn/session/score/waste buckets, known finding severities, and redaction-family counts. |
| FR-003 | Retain only allowlisted public ecosystem IDs for coding agents, frameworks, MCPs, skills, plugins, package managers, and SDD fingerprints. |
| FR-004 | Retain MCP/skill utilization as buckets, known public IDs, and unknown counts only. |
| FR-005 | Retain recommendation analytics as enum/allowlist fields: class, tool ID, reason, signal IDs, risk, and install policy. |
| FR-006 | Append analytics events from direct sanitized-report uploads. |
| FR-007 | Append analytics events from worker-completed legacy/internal and paid jobs. Paid bundles emit one event per aggregate report, not per raw session. |
| FR-008 | Add a local JSONL analytics store and an AWS private S3 JSONL analytics store. |
| FR-009 | Add an offline summary CLI with cohort suppression for adoption, co-occurrence, bloat, and recommendation summaries. |
| FR-010 | Document the aggregate analytics threat model and the engineering checklist for new retained fields. |

## Non-Functional Requirements

| ID | Requirement | Threshold |
| --- | --- | --- |
| NFR-001 | Privacy | Analytics event JSON contains no raw prompts, paths, URLs, private names, hostnames, usernames, emails, job IDs, session IDs, tokens, raw version output, or stable private hashes. |
| NFR-002 | Bounded cardinality | Every retained string is a closed enum, public allowlist ID, or documented bucket. Unknown values become `unknown` or counts. |
| NFR-003 | Determinism | Repeated extraction from the same report produces byte-identical JSON. |
| NFR-004 | Cohort safety | Offline summaries suppress rows below the configured minimum cohort, default 10. |
| NFR-005 | Launch performance | Analytics append must not block report rendering on failure; failures log only an error category. |

## Constraints

- Do not store raw report JSON as retained analytics.
- Do not add dashboards, accounts, teams, or per-user longitudinal tracking.
- Do not implement the WASM browser-local demo in this mission.
- Do not add signed-release/Homebrew/domain-rename work in this mission.
- Do not retain exact timestamps in event JSON.
- Do not use job IDs, report tokens, upload paths, or report paths in analytics object keys or event bodies.

## Success Criteria

- `internal/analytics.Event` exists and is narrower than `Report`.
- Direct report and worker paths append analytics events.
- `cmd/analytics-summary` produces cohort-level summaries.
- Privacy tests prove private canaries do not serialize.
- Threat model docs are in repo.
- `go test ./...`, Docker smoke, and local load pass.
