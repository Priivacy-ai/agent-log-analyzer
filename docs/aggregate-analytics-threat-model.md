# Aggregate Analytics Threat Model

This document defines the retained analytics boundary for Agent Analyzer.
It covers only aggregate ecosystem intelligence derived from sanitized
reports. It does not weaken the launch rule that raw agent logs stay on
the user's machine.

## Assets

- User source code, prompts, tool arguments, tool outputs, paths, repository
  names, branch names, hostnames, usernames, emails, and private tool names.
- Short-lived report JSON and paid plugin artifacts.
- Retained analytics JSONL events.
- Offline aggregate summaries generated from analytics events.

## Retained Event Boundary

The retained event is `analytics.Event` in `internal/analytics`. It is
intentionally narrower than `analyzer.Report` and `AggregateSafeEvent`.

Allowed retained fields:

- analyzer version and schema version
- scan type bucket: `free`, `paid`, or `legacy_internal`
- parser/input/turn/session/score/waste buckets
- finding IDs from the analyzer allowlist with severity buckets
- redaction family counts from the scrubber allowlist
- known public ecosystem IDs from embedded registries
- SDD fingerprint IDs, confidence/source buckets, active/installed booleans,
  evidence counts, and coarse major-version buckets
- MCP and skill utilization buckets, known public IDs, and unknown counts
- recommendation class/tool/reason/risk/policy/signal enums

Forbidden retained fields:

- job IDs, upload tokens, report tokens, upload paths, report paths
- exact timestamps in event JSON
- prompts, task descriptions, transcript excerpts
- raw tool inputs, raw tool outputs, command arguments
- raw MCP schemas, descriptions, URLs, auth scopes
- private MCP/tool/skill/plugin names
- skill text, examples, or user-authored skill docs
- raw file paths, repo URLs, branch names
- usernames, hostnames, emails
- raw version output or stable hashes of private strings

## Threats And Controls

| Threat | Risk | Control |
| --- | --- | --- |
| Raw report retention becomes analytics by accident | Report grows later with high-cardinality fields | Analytics storage uses `analytics.FromReport`, not raw report JSON or full `AggregateSafeEvent` |
| Private MCP/skill names leak through "known" arrays | Malicious or buggy client submits arbitrary strings | `analytics.FromReport` revalidates IDs against embedded public allowlists before retention |
| Rare tool combinations reidentify a user | A unique SDD/MCP/OS combination appears in summaries | `analytics-summary` suppresses rows/cells below `--min-cohort`, default 10 |
| Stable fingerprints across uploads | Job ID, session ID, path, timestamp, or hash links events | Event JSON excludes identifiers, exact timestamps, paths, URLs, and hashes |
| Prompt/path leakage through evidence fields | Finding evidence contains top file names or descriptions | Analytics retains only finding ID and severity, not `FindingEvidence` |
| Raw `--version` output leaks environment details | CLI version output contains paths/usernames | Analyzer stores only normalized version buckets; analytics collapses detected versions to coarse major buckets |
| Operational logs duplicate analytics payloads | Logs become a shadow retention layer | Logs should record only append success/failure categories, not event bodies |

## Storage

Local backend:

- retained events append to `/data/analytics/events.jsonl`
- file contains `analytics.Event` JSONL only
- raw reports remain under the private report path and are not reused as analytics

AWS backend:

- retained events are written as private S3 JSONL objects under
  `analytics/events/date=YYYY-MM-DD/hour=HH/`
- object keys use server-generated randomness, never job IDs or user strings
- event JSON does not include exact timestamps or identifiers
- events use the existing encrypted private report bucket unless/until a
  dedicated analytics bucket is introduced

## Offline Summaries

`cmd/analytics-summary` reads retained JSONL events and emits cohort-level
summary JSON. It must never output per-event rows.

Default suppression:

- `--min-cohort` defaults to 10
- any row or cell below the threshold is omitted
- suppressed rows are counted in `suppressed_below_cohort_count`

The default summary can answer adoption, co-occurrence, MCP/skill bloat, and
recommendation-frequency questions without identifying a person, repository, or
private tool.

## Engineering Checklist For New Analytics Fields

Before adding any retained analytics field:

- Is the field derived from an allowlist or closed enum?
- If it is numeric, is it a count or bucket?
- Could it identify a repo, company, person, host, path, or private tool?
- Could the field become stable across uploads?
- Does `internal/analytics` filter malicious client-provided strings?
- Does a privacy test include a canary for this field?
- Does the summary command suppress small cohorts using the field?

If any answer is unclear, do not retain the field.
