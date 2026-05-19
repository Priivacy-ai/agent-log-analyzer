# Plan: Token-Saving Recommendation Engine (Phase B Wiring)

| Field | Value |
| --- | --- |
| Mission slug | `token-saving-recommendation-phase-b-01KS0JZ4` |
| Mission ID | `01KS0JZ495XV0PCKSVBNDVAY16` |
| Mission type | software-dev |
| Target branch | `main` |
| Planning base branch | `main` |
| Merge target branch | `main` |
| Branch matches target | true |
| Spec | [spec.md](./spec.md) |
| Research | [research.md](./research.md) |
| Data model | [data-model.md](./data-model.md) |
| Contracts | [contracts/](./contracts/) |
| Quickstart | [quickstart.md](./quickstart.md) |

## Technical Context

**Language/Version**: Go 1.25 (per `go.mod`)

**Primary Dependencies**: `internal/analyzer` (existing), `internal/remediation` (existing), Go standard library. **No new external modules.**

**Frontend**: Vanilla HTML/CSS/JS under `web/` (rendered by `web/app.js`); no build step, no framework.

**Storage**: JSON `Report` documents (filesystem, S3-backed via existing `awsstore`); no schema-level migration is required.

**Identity / Auth**: N/A — local-first analyzer; paid path is waiver-gated session tokens (untouched by this mission).

**Testing**: `go test ./...` (table-driven, golden, leak); existing `internal/analyzer/leak_test.go` pattern is the canonical privacy gate; browser QA against the existing severe-MCP fixture for DOM privacy and render p95.

**Validation commands**: `go test ./...` · `go vet ./...` · `terraform -chdir=infra/aws fmt -check -recursive` · `./scripts/smoke-local.sh` · `./scripts/load-local.sh 25` (after web changes).

**Performance target**: Free-report render p95 < 500ms on the existing severe-MCP fixture; engine call adds ≪ 1ms per `Report` (deterministic, in-memory).

**Deterministic build**: All Go iteration over the engine state map and signal slice goes through sorted keys (engine NFR-001); Phase B's signal and tool-state derivation observe the same invariant.

**Distribution**: No CLI/binary distribution changes; new code rides the existing `claude-analyzer` binary and the existing static `web/` bundle.

## Charter Check

Charter context for `plan` action returned `compact` mode with no directive
text and no referenced docs at `.kittify/charter/charter.md`. The charter file
is absent in this repo. Charter Check is therefore skipped.

Two directive IDs (`DIR-001`, `DIR-002`) appeared in the compact context but
have no referenced bodies; they cannot be evaluated. If a charter is added
later, the next plan-phase mission must re-evaluate.

## Engineering Alignment

This section is the contract between specify-phase decisions and the work
packages `tasks.md` will produce. Everything below is settled and recorded as
resolved decisions in `decisions/`.

### Decisions (resolved)

| Decision ID | Slot | Outcome |
| --- | --- | --- |
| `01KS0K7JR5J25BJ4HERMJ0P913` | `plan.ui.section-placement` | Render the new recommendation panel **above Workflow Fingerprints** in the report intelligence area. |
| `01KS0K7NEM1FNE38KKSRWGVBK5` | `plan.architecture.engine-call-site` | Single helper `analyzer.AttachRecommendation` called from both `Analyze` and `AggregateReports`. |
| `01KS0NFDAQQ27YZ7R3JTY13R57` | `plan.signals.no-usage-visibility-trigger` | **Always emit** `Signal=no_usage_visibility` when no usage-tracking tool is detected, independent of other waste signals. |
| `01KS0NFFPCDHF5YYVPRXF2QDW2` | `plan.ui.savings-estimate` | UI-layer **bounded savings bucket** derived from existing `Report.EstimatedWaste.High`. **No engine change.** |
| `01KS0K7GC9GH94QCMZ4SHJEBJP` | `plan.signals.no-usage-visibility` (superseded) | Superseded by `01KS0NFDAQQ27YZ7R3JTY13R57`. |

### Architecture (single-helper call site)

A new helper lives in a new file `internal/analyzer/recommendation_wiring.go`:

```
func AttachRecommendation(report *Report)
```

The helper:

1. Derives `[]Signal` from `report` (finding IDs, utilization bands, fingerprint absence).
2. Derives `ToolStateMap` from `report.Ecosystem` (fingerprints, MCP/skill utilization, known IDs, fingerprint CLI-probe metadata).
3. Calls `Recommend(signals, state)` (frozen Phase A contract).
4. Assigns the returned `RecommendationSet` to `report.Recommendation`.

Call sites:

- `internal/analyzer/analyzer.go::Analyze` calls `AttachRecommendation` after the
  per-report `Report` is fully constructed (after Findings, Ecosystem, and
  `AggregateEvent` are populated).
- `internal/analyzer/aggregate.go::AggregateReports` calls
  `AttachRecommendation` **once on the merged Report**, after all merge logic
  has settled. The aggregate path does not field-merge two `RecommendationSet`
  objects; it re-runs the engine on merged inputs (spec FR-008).

### Report shape change

`Report` gains an optional pointer field:

```go
Recommendation *RecommendationSet `json:"recommendation,omitempty"`
```

`omitempty` ensures legacy report JSON (pre-Phase-B writers) round-trips cleanly
(FR-012, NFR-004). After Phase B lands, `AttachRecommendation` always assigns a
non-nil pointer, even when the engine returns no primary and no secondary —
the pointer carries `RegistryVersion`, `EngineVersion`, signals evaluated, and
`UnknownIDCount`.

### Signal derivation (FR-001, FR-002, no-usage-visibility policy)

| Source | Condition | Emitted Signal |
| --- | --- | --- |
| `report.Findings[].ID == "tool_output_bloat"` | finding present | `SignalToolOutputBloat` |
| `report.Findings[].ID == "repeated_file_reads"` | finding present | `SignalRepeatedFileReads` |
| `report.Findings[].ID == "retry_loop"` | finding present | `SignalRetryLoop` |
| `report.Findings[].ID == "context_growth_spikes"` | finding present | `SignalContextGrowthSpikes` |
| `report.Ecosystem.ToolingUtilization.MCP.WarningBand` | ∈ `{"high","severe"}` | `SignalMCPSkillBloat` |
| `report.Ecosystem.ToolingUtilization.Skill.WarningBand` | ∈ `{"high","severe"}` | `SignalMCPSkillBloat` |
| Absence of any active usage-tracker fingerprint | no `EcosystemFingerprint` with `Active==true` and ID in the usage-visibility allowlist (engine class `usage_visibility` tool IDs) | `SignalNoUsageVisibility` |

Notes:

- The same `Signal` value may be triggered by multiple sources; the
  derivation helper deduplicates via `sortedSignalIDs` (already exported in
  Phase A's `token_saving_types.go`).
- No other Phase A signal values are derived by this mission. Adding more
  (e.g. `SignalShellOutputBloat`, `SignalOutputVerbosity`) is a future
  phase and is explicitly out of scope.

### Tool-state derivation (FR-003, FR-004, FR-005)

For each known public tool ID, the helper assembles a `ToolStateEntry` from
three evidence streams. When two streams disagree, the helper calls
`ToolStateMap.Resolve` (engine's documented precedence).

**From `report.Ecosystem.WorkflowFingerprints`** (per entry where `ID` is in the
engine's public tool ID allowlist):

| Fingerprint state | Resolved `ToolState` | EvidenceSource increment |
| --- | --- | --- |
| `Active == true` | `ToolStateActiveHigh` | `EvidenceReportMention` += 1 |
| `Active == false && Installed == true` | `ToolStateInstalledMedium` | `EvidenceReportMention` += 1 |
| `Active == false && Installed == false` | `ToolStateMentionedLow` | `EvidenceReportMention` += 1 |

If the fingerprint records a CLI probe (presence/version detected via the SDD
fingerprint pipeline, where that metadata is already bounded), the helper
adds `EvidenceCLIPresence` += 1 (and `EvidenceCLIVersion` += 1 when the
fingerprint exposes a `VersionBucket` value). **No raw version string is
read or stored** — only the bounded `VersionBucket` enum's presence acts as
the trigger.

**From `report.Ecosystem.ToolingUtilization.MCP`** (per known MCP ID):

| Condition | Resolved `ToolState` | EvidenceSource increment |
| --- | --- | --- |
| ID ∈ `UniqueKnownCalledIDs` (executed) | `ToolStateActiveHigh` | `EvidenceMCPActive` += 1 |
| ID ∈ `KnownServerIDs` but **not** in `UniqueKnownCalledIDs` | `ToolStateConfiguredMedium` | `EvidenceMCPConfigured` += 1 |

**From `report.Ecosystem.ToolingUtilization.Skill`** (per known skill ID):

| Condition | Resolved `ToolState` | EvidenceSource increment |
| --- | --- | --- |
| ID ∈ `KnownExecutedIDs` | `ToolStateActiveHigh` | `EvidenceSkillConfigured` += 1 (plus `EvidenceReportMention` += 1; no skill-active enum exists) |
| ID ∈ `KnownExposedIDs` but **not** in `KnownExecutedIDs` | `ToolStateConfiguredMedium` | `EvidenceSkillConfigured` += 1 |

**Unknown counts** (`UnknownMCPServerCount`, `UnknownSkillCount`,
`UnknownPluginCount`): contribute only to the engine's `UnknownIDCount` via
`Recommend`'s signature (the engine increments that itself when given
unknown IDs in its registry lookup). Phase B does **not** invent
`ToolStateEntry` rows for unknown names.

### Aggregate merge (FR-008)

`AggregateReports` already merges classic ecosystem fields, fingerprint metadata,
and tooling utilization (post PR #75). After the merged `Report` is fully
built (including merged `Findings`, merged `Ecosystem`, and the merged
`AggregateEvent`), the aggregate path calls `AttachRecommendation(merged)`
**once**. Signal derivation runs on the merged findings + merged utilization
bands. Tool-state derivation runs on the merged fingerprint slice and merged
utilization tables. The engine sees a single coherent input set.

### Web UI (FR-006, FR-009)

In `web/app.js` / `web/index.html`:

- Add a section `#recommendation-section` rendered **immediately above** the
  Workflow Fingerprints section in the full-width report intelligence band.
- Render up to two cards: **Primary** and (optional) **Secondary**.
- Each card composes display text from the recommendation's enum values
  (`PrimaryToolID`, `Reason`, `Confidence`, `RiskLevel`, `InstallPolicy`)
  through an in-source allowlist map. **No `innerHTML`.** All string nodes
  use `textContent`.
- When `report.recommendation == null` (legacy report) the section does not
  render and produces no console error.
- When `Primary == null && Secondary == null`, render a single short
  "no action needed" note containing only `UnknownIDCount` and the
  count of skipped candidates (no IDs).
- **Savings bucket**: derived inline in JS from `report.estimated_waste_pct.high`:
  - `< 10` → `low`
  - `10–29` → `medium`
  - `≥ 30` → `high`
  - render only for the Primary card; absent (or omitted) when there is no Primary.

The recommendation panel's CSS lives in `web/styles.css` alongside the
existing intelligence sections.

### Paid artifact (FR-007, C-004)

`internal/remediation/artifact.go::Generate` already constructs a
`PluginArtifact` from a `Report`. Phase B adds one new optional field on the
`PluginArtifact` struct:

```go
Recommendation *analyzer.RecommendationSet `json:"recommendation,omitempty"`
```

It is populated from `report.Recommendation` (which is now always non-nil for
fresh paid aggregates) and embedded verbatim. The existing
`VettedRecommendations` slice and the existing artifact files are unchanged.

### Privacy and leak tests (NFR-002, AS-07)

Existing `internal/analyzer/leak_test.go` already asserts the report-JSON
privacy budget. Phase B extends it to also assert:

- `report.recommendation` (when present) contains zero `mcp__*`, `skill__*`,
  `plugin__*` token substrings.
- Marshalled `RecommendationSet` JSON for synthetic input with private/unknown
  tool names produces zero high-cardinality strings (only enums + bounded
  integer maps).
- DOM probe (existing browser QA harness against the severe-MCP fixture)
  reports zero forbidden substrings in the rendered recommendation panel.

### Determinism (NFR-001, C-005)

Phase B obeys two invariants:

1. `AttachRecommendation` iterates the `Findings` slice, the
   `MCP.KnownServerIDs` slice, the `Skill.KnownExposedIDs` slice, and the
   `WorkflowFingerprints` slice in deterministic order (their incoming order
   is already deterministic per PR #75/76; the helper preserves it).
2. The `ToolStateMap` is populated keyed by `ToolID`; the helper relies on
   `sortedSignalIDs` and the engine's `SortedTools()` for iteration. No
   `range map` is used in derivation paths.

A new test fixture in `internal/analyzer/testdata/` exercises the helper 100×
against a stable input and asserts byte-identical `report.recommendation`
JSON output.

## Gates

| Gate | Status | Notes |
| --- | --- | --- |
| Specify phase | ✅ Passed | Spec is committed with substantive FR rows (verified by `setup-plan`). |
| Plan exit gate | ✅ Will pass after this commit | Technical Context is populated with `Language/Version=Go 1.25` plus 4 peer fields (Primary Dependencies, Frontend, Storage, Testing). |
| Charter | ⏭️ Skipped | No charter file present. Compact charter context loaded; no overrides to apply. |
| Open decisions | ✅ All resolved | 5 decisions opened, all `resolved`. `decision verify --mission` will be re-run before tasks. |
| Bulk edit | ✅ Not applicable | New code only; no global rename. |
| Engine signature | ✅ Frozen | C-002 preserved. |
| Paid artifact backwards compat | ✅ Preserved | C-004 preserved (`VettedRecommendations` untouched, new field is additive and `omitempty`). |

## Phase 0 — Research

Captured in [`research.md`](./research.md). Each decision summarized there
maps to a settled decision ID above.

## Phase 1 — Design & Contracts

- [`data-model.md`](./data-model.md) — entity diffs and invariants.
- [`contracts/`](./contracts/):
  - `attach-recommendation-go-api.md` — Go API contract for the helper.
  - `signal-derivation-map.md` — exhaustive finding-ID/utilization-band → Signal mapping.
  - `tool-state-derivation-map.md` — fingerprint and utilization → ToolStateEntry mapping.
  - `report-recommendation-json-envelope.md` — JSON shape rendered into `Report.Recommendation` and into the paid artifact `recommendation` field.
- [`quickstart.md`](./quickstart.md) — validation scenarios (engineer-facing).

## Out of Scope (reaffirmed)

- Modifying the Phase A engine (`Recommend`, registry, rule precedence,
  `RecommendationSet` shape) — frozen by C-002.
- Consolidating the legacy `VettedRecommendations` hand-curated list with the
  engine output.
- Stripe checkout, paid-session entitlement, or other monetization paths
  (covered in later launch-completion PRs).
- CLI text rendering of the recommendation (web UI only).
- WASM/browser local-analysis demo (issue #37).
- Adding new Signals beyond the seven listed above
  (`SignalNoUsageVisibility`, `SignalToolOutputBloat`,
  `SignalMCPSkillBloat`, `SignalRepeatedFileReads`, `SignalRetryLoop`,
  `SignalContextGrowthSpikes`, plus possible deduped duplicates).

## Branch contract (final restate before `/spec-kitty.tasks`)

- Current branch at plan start: **main**
- Intended planning/base branch: **main**
- Final merge target: **main**
- `branch_matches_target`: **true**

Lane branches for work packages are created off `main` at implement time, per
the standard spec-kitty git workflow.
