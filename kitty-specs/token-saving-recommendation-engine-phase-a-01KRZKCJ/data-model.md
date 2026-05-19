# Phase 1 Data Model — Token-Saving Recommendation Engine (Phase A)

All entities live in package `analyzer` (path
`internal/analyzer/token_saving_*.go`). Every string field whose value comes
from a closed vocabulary is declared as a named type with a fixed enum set;
runtime construction outside the registered enum constants is a test failure
(`TestEnumsAreClosed`).

## Enums

### `ToolID` — registered allowlist identifier

Declared as `type ToolID string`. The set of valid values is exactly the keys
present in the registry literal in `token_saving_tools.go`. Any `ToolID` not
present in `AllTools()` is treated as unknown by the engine (counted, never
recommended). Canonical form: lowercase + `_`-separated (e.g. `ccusage`,
`context_mode`, `claude_code_usage_monitor`).

### `ToolState`

```
ToolStateUnknown          ToolState = "unknown"
ToolStateMentionedLow     ToolState = "mentioned_low"
ToolStateInstalledMedium  ToolState = "installed_medium"
ToolStateConfiguredMedium ToolState = "configured_medium"
ToolStateActiveHigh       ToolState = "active_high"
ToolStateRejectedMedium   ToolState = "rejected_medium"
```

Conflict precedence (highest trust first), enforced by `(*ToolStateMap).Resolve`:

```
rejected_medium > active_high > configured_medium > installed_medium > mentioned_low > unknown
```

### `EvidenceSource`

```
EvidenceCLIPresence          = "cli_presence"
EvidenceCLIVersion           = "cli_version"
EvidenceLogActiveCommand     = "log_active_command"
EvidenceMCPConfigured        = "mcp_configured"
EvidenceMCPActive            = "mcp_active"
EvidencePluginConfigured     = "plugin_configured"
EvidenceSkillConfigured      = "skill_configured"
EvidenceHookConfigured       = "hook_configured"
EvidenceStatuslineConfigured = "statusline_configured"
EvidenceReportMention        = "report_mention"
EvidenceFailureOrRejection   = "failure_or_rejection"
```

### `Signal`

```
SignalNoUsageVisibility    = "no_usage_visibility"
SignalToolOutputBloat      = "tool_output_bloat"
SignalShellOutputBloat     = "shell_output_bloat"
SignalMCPToolOutputBloat   = "mcp_tool_output_bloat"
SignalRepeatedFileReads    = "repeated_file_reads"
SignalBroadRepoExploration = "broad_repo_exploration"
SignalUnchangedFileRereads = "unchanged_file_rereads"
SignalMCPSkillBloat        = "mcp_skill_bloat"
SignalOutputVerbosity      = "output_verbosity"
SignalRetryLoop            = "retry_loop"
SignalContextGrowthSpikes  = "context_growth_spikes"
```

### `RecommendationClass`

```
ClassUsageVisibility     = "usage_visibility"
ClassMCPSkillHygiene     = "mcp_skill_hygiene"
ClassMCPOutputReducer    = "mcp_output_reducer"
ClassShellOutputReducer  = "shell_output_reducer"
ClassRetrieval           = "retrieval"
ClassRereadGuard         = "reread_guard"
ClassContextHygiene      = "context_hygiene"
ClassOutputVerbosity     = "output_verbosity"
```

### `Confidence`

```
ConfidenceLow    = "low"
ConfidenceMedium = "medium"
ConfidenceHigh   = "high"
```

Derived deterministically from the underlying tool-state and evidence-source
mix (see engine spec in contracts/).

### `RiskLevel`

```
RiskLow    = "low"
RiskMedium = "medium"
RiskHigh   = "high"
```

### `InstallPolicy`

```
PolicyBundle              = "bundle"
PolicyRecommend           = "recommend"
PolicyRecommendWithWaiver = "recommend_with_waiver"
PolicyResearchOnly        = "research_only"
PolicyReferenceOnly       = "reference_only"
```

### `Reason`

```
ReasonAbsent              = "absent"
ReasonInstalledInactive   = "installed_inactive"
ReasonConfiguredInactive  = "configured_inactive"
ReasonActivePersistent    = "active_persistent"
ReasonRejectedAlternative = "rejected_alternative"
ReasonPruneFirst          = "prune_first"
ReasonAuditConfig         = "audit_config"
ReasonNoOp                = "no_op"
ReasonServerQuotaCheck    = "server_quota_check"
```

## Entities

### `TokenSavingTool`

| Field | Type | Notes |
| --- | --- | --- |
| `ID` | `ToolID` | unique within registry |
| `DisplayName` | `string` | human-readable, never returned in engine output |
| `SourceURL` | `string` | public URL; empty for `research_only` entries until Phase B verification |
| `Category` | `string` | free-text grouping (e.g. `"observability"`, `"shell"`) — registry-internal, not emitted |
| `RecommendationClass` | `RecommendationClass` | drives precedence + secondary dedupe |
| `ClassRank` | `int` | ordering within class; lower = preferred |
| `DetectorSources` | `[]EvidenceSource` | which evidence sources can plausibly identify this tool |
| `InstallRisk` | `RiskLevel` | install-time risk surface |
| `DataMovementRisk` | `RiskLevel` | runtime data-egress / cloud-call risk |
| `RollbackGuidance` | `string` | non-empty when `InstallPolicy = recommend_with_waiver` |
| `FreeReportAllowed` | `bool` | may appear in the free analyzer report |
| `PaidPackAllowed` | `bool` | may appear in the paid plugin pack |
| `ResearchOnly` | `bool` | equivalent shorthand for `InstallPolicy == research_only`; both must agree |
| `InstallPolicy` | `InstallPolicy` | gate for emission |
| `Notes` | `string` | maintainer notes; never emitted |

**Invariants** (asserted by `TestRegistryInvariants`):

- `ID` is non-empty, lowercase + `_`-only, unique across the registry.
- `InstallPolicy != ""` for every entry.
- `ResearchOnly == (InstallPolicy == PolicyResearchOnly)`.
- `InstallPolicy == PolicyRecommendWithWaiver` ⇒ `RollbackGuidance != ""`.
- `RecommendationClass` is one of the eight defined classes.
- `(RecommendationClass, ClassRank)` pairs are unique.
- Either `SourceURL != ""` or `ResearchOnly == true`.

### `ToolStateEntry`

| Field | Type | Notes |
| --- | --- | --- |
| `Tool` | `ToolID` | matches registry; unknown IDs are counted in `unknown_id_count` and dropped |
| `State` | `ToolState` | resolved state (post-conflict-precedence) |
| `Sources` | `map[EvidenceSource]int` | bounded count per evidence source; map keys are static enum values |

### `ToolStateMap`

```
type ToolStateMap map[ToolID]ToolStateEntry
```

Iteration in engine paths goes through `(ToolStateMap).SortedTools()`, which
returns a lexicographic `[]ToolID`. The map itself is never marshalled.

### `TokenSavingRecommendation`

| JSON field | Type | Required |
| --- | --- | --- |
| `recommendation_id` | string (composed enum) | yes |
| `primary_tool_id` | `ToolID` | yes |
| `skipped_tool_ids` | `[]ToolID` | omitempty |
| `reason` | `Reason` | yes |
| `signal_ids` | `[]Signal` (sorted) | yes |
| `confidence` | `Confidence` | yes |
| `risk_level` | `RiskLevel` | yes |
| `install_policy` | `InstallPolicy` | yes |
| `evidence_counts` | `map[EvidenceSource]int` | yes (may be empty `{}`) |

Marshalling is via `encoding/json`; map keys for `evidence_counts` are sorted
by Go's standard library, satisfying determinism.

### `SkipNote`

| JSON field | Type | Notes |
| --- | --- | --- |
| `tool_id` | `ToolID` | the tool that was *not* recommended |
| `reason` | `Reason` | typically `ReasonActivePersistent`, `ReasonRejectedAlternative`, `ReasonInstalledInactive`, or `ReasonConfiguredInactive` |
| `for_signal` | `Signal` | signal that would otherwise have promoted this tool |

### `RecommendationSet`

```
type RecommendationSet struct {
    Primary         *TokenSavingRecommendation `json:"primary,omitempty"`
    Secondary       *TokenSavingRecommendation `json:"secondary,omitempty"`
    Skipped         []SkipNote                 `json:"skipped,omitempty"`
    RegistryVersion string                     `json:"registry_version"`
    EngineVersion   string                     `json:"engine_version"`
    Signals         []Signal                   `json:"signals"`           // sorted echo of input
    UnknownIDCount  int                        `json:"unknown_id_count"`
}
```

The `RecommendationSet` is the entire contract returned to Phase B callers.
`Primary == nil` and `Secondary == nil` is the no-op case (`Reason =
ReasonNoOp` is recorded via an empty `Skipped` slice and an unset Primary).

## State transitions

The only state the engine maintains is the input `ToolStateMap`. The decision
flow is:

```
input(signals, state)
  → conflict-resolve per-tool state
  → for each rule in fixed precedence list:
      if rule fires:
        candidate = first eligible tool in registry for rule.class
        if state[candidate] == ActiveHigh:
          append SkipNote{candidate, ActivePersistent, signal}; advance to next rule
        else if state[candidate] in {InstalledMedium, ConfiguredMedium}:
          emit AuditConfig recommendation (still points at candidate)
        else if state[candidate] == RejectedMedium:
          append SkipNote{candidate, RejectedAlternative, signal}; advance to next eligible tool
        else:
          emit Absent / Active-persistent recommendation
        record the class as "primary-claimed"
        if Primary already set and class not yet claimed: emit Secondary; break loop
  → assemble RecommendationSet
```

No timers, no I/O, no globals besides the immutable registry literal.
