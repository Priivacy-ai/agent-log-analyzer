# Data Model: Report Intelligence UX

**Mission**: `report-intelligence-ux-01KS070G`
**Phase**: 1 (Design)
**Date**: 2026-05-19

## Scope

This mission is a **view layer**. No new persistent entities, no new value objects, no new serialized fields. The data model documents which existing fields the new UI renders and the privacy classification of each. Authoritative source: `internal/analyzer/types.go` (do not edit).

## Entities Consumed (read-only)

### Ecosystem (existing)

`Report.Ecosystem` — already in the JSON; new UI does not modify it.

### EcosystemFingerprint (consumed by WP01)

Source: `internal/analyzer/types.go` lines 73–81.

| Field | Type | Cardinality | Rendered as | Privacy class |
|---|---|---|---|---|
| `id` | string | bounded (public allowlist) | literal text | safe-public-id |
| `confidence` | string enum | `low` / `medium` / `high` | enum label | bounded-enum |
| `sources` | []string | closed enum entries | badges row | bounded-enum |
| `evidence_count` | int | non-negative | numeric count | bounded-count |
| `active` | bool | — | indicator | safe-bool |
| `installed` | bool | — | indicator | safe-bool |
| `version_bucket` | string (optional) | bounded enum | label (when present) | bounded-enum |

**Invariants surfaced in UI**:
- `id` is always in the public allowlist — guaranteed by `safePublicID` (`internal/remediation/artifact.go`).
- `sources[]` and `confidence` are emitted by the SDD detector using closed enums; no free text.
- `version_bucket` may be empty — UI must treat absence as "do not render this cell".

### MCPUtilization (consumed by WP02)

Source: `internal/analyzer/types.go` lines 88–104.

| Field | Type | Cardinality | Rendered as | Privacy class |
|---|---|---|---|---|
| `known_server_ids` | []string | bounded (public allowlist) | (not rendered as names; **count only** in V2 — see note) | safe-public-id (count-only display) |
| `unknown_server_count` | int | non-negative | numeric count | bounded-count |
| `server_count_bucket` | string enum | bounded | bucket label | bounded-enum |
| `exposed_tool_count_bucket` | string enum | bounded | bucket label | bounded-enum |
| `context_token_bucket` | string enum | bounded | bucket label | bounded-enum |
| `exposure_known` | bool | — | controls ratio gating | safe-bool |
| `inference_source` | string enum | bounded | "inferred from: X" when `exposure_known==false` | bounded-enum |
| `call_count` | int | non-negative | numeric count | bounded-count |
| `known_call_count` | int | non-negative | numeric count | bounded-count |
| `unknown_call_count` | int | non-negative | numeric count | bounded-count |
| `unique_known_called_ids` | []string | bounded (allowlist) | count of length (not names) | safe-public-id (count-only display) |
| `unique_unknown_called_count` | int | non-negative | numeric count | bounded-count |
| `utilization_ratio_pct` | int (0..100) | clamped | percentage (only when `exposure_known==true`) | bounded-count |
| `context_efficiency_bucket` | string enum | bounded | bucket label | bounded-enum |
| `warning_band` | string enum | `severe`/`high`/`watch`/`normal`/`unknown` | band chip | bounded-enum |

> **Render-vs-data note**: Even though `known_server_ids` and `unique_known_called_ids` contain only allowlisted public IDs (and would be safe to print), the V2 UI deliberately renders **counts only** to keep the row compact and to keep the section's privacy story uniform. If a later mission decides to list IDs, the data is already available without a schema change.

### SkillUtilization (consumed by WP02)

Source: `internal/analyzer/types.go` lines 106–119.

Same field-by-field treatment as `MCPUtilization` with skill-scoped names (`known_exposed_ids`, `executed_count`, `known_executed_ids`, etc.). Privacy classification identical.

### Finding (consumed by WP02 for advice copy)

Source: `internal/analyzer/types.go` lines 34–49.

WP02 reads `findings[].id` and `findings[].recommendation` and looks for these four IDs only:

- `mcp_bloat_severe`
- `mcp_bloat_high`
- `skill_bloat_severe`
- `skill_bloat_high`

If a matching finding exists, its `recommendation` string is rendered in the advice block beneath the corresponding row. If no matching finding exists, no advice block renders (this is the structural enforcement of FR-006).

**Privacy class of `recommendation` for these four IDs**: vetted static copy in `internal/analyzer/analyzer.go` lines 380–393. Reviewed for NFR-007 (no private-name leakage). Do not generalize this lookup to other finding IDs — other findings may carry data-derived strings.

## Invariants

| ID | Invariant | Enforced by |
|---|---|---|
| INV-1 | No unknown private MCP/skill/plugin name reaches the renderer. | Server-side bounded shape; verified by hostile-fixture leak test. |
| INV-2 | No new serialized field is introduced. | Code review + V3 in research.md. |
| INV-3 | Pruning advice renders only when `warning_band ∈ {high, severe}`. | Structural — analyzer emits no finding outside those bands. |
| INV-4 | All displayed strings are either enum values, integer-derived counts/percentages, or vetted advice copy from the four allowlisted findings. | Renderer code shape (no template interpolation of raw fields). |
| INV-5 | When `exposure_known == false`, the utilization ratio is not displayed. | Renderer gate (FR-007). |

## Field Reference Index (renderer ↔ field)

| Renderer | Reads | Writes |
|---|---|---|
| `renderWorkflowFingerprints(report)` | `report.ecosystem.workflow_fingerprints[]` | DOM under `#workflow-fingerprints` |
| `renderToolingUtilization(report)` | `report.ecosystem.tooling_utilization.{mcp,skill}` + `report.findings[]` filtered to four IDs | DOM under `#tooling-utilization` |

## Out of Model (deliberately)

- `Recommendation` objects from Phase 3 (#73) — not consumed.
- `AggregateEvent.Ecosystem` — server-side aggregate, not part of the report-page view.
- `SecurityReceipt`, `Timeline`, `Metrics`, `Redactions` — already rendered today; untouched by this mission.
