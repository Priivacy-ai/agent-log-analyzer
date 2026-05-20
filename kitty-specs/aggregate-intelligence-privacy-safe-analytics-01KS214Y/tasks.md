# Tasks: Aggregate Intelligence Privacy-Safe Analytics

## WP01: Analytics Contract

- T001 Define `internal/analytics.Event`.
- T002 Extract event from `analyzer.Report` without retaining raw report JSON.
- T003 Revalidate ecosystem IDs against public allowlists.
- T004 Retain recommendation analytics as enum/allowlist fields only.
- T005 Add deterministic and forbidden-canary tests.

## WP02: Analytics Storage

- T006 Add optional `app.AnalyticsStore`.
- T007 Add local JSONL append storage under `/data/analytics/events.jsonl`.
- T008 Add AWS S3 JSONL append/put storage under `analytics/events/date=.../hour=...`.
- T009 Wire direct sanitized-report upload path.
- T010 Wire worker-completed single/paid job path.
- T011 Ensure append failures log only `error_category=analytics_append`.

## WP03: Offline Summary

- T012 Add `analytics.SummarizeJSONL`.
- T013 Add `cmd/analytics-summary`.
- T014 Summarize adoption, co-occurrence, bloat, recommendation, finding, and score/waste cohorts.
- T015 Suppress cohorts below `--min-cohort`, default 10.

## WP04: Privacy Docs And Gates

- T016 Add aggregate analytics threat model.
- T017 Update retention and logging docs.
- T018 Add localstore analytics append test.
- T019 Run full Go tests.
- T020 Run Docker smoke and local load.
