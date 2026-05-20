# Contract: Token-Saving Engine Public Go API (Phase A)

The Phase A package exposes exactly the symbols documented here. Anything not
in this contract is package-private and may be reshaped without notice.

Package: `github.com/priivacy-ai/agent-log-analyzer/internal/analyzer`
Module: `github.com/priivacy-ai/agent-log-analyzer` (Go 1.25)

## Stable public surface

### Constants & types

All enum types and constants from `data-model.md` are exported:

```go
type ToolID string
type ToolState string
type EvidenceSource string
type Signal string
type RecommendationClass string
type Confidence string
type RiskLevel string
type InstallPolicy string
type Reason string
```

Every constant listed in `data-model.md` is exported under the `analyzer`
package (e.g. `analyzer.SignalShellOutputBloat`,
`analyzer.PolicyRecommendWithWaiver`).

### Structs

```go
type TokenSavingTool struct {
    ID                  ToolID              `json:"id"`
    DisplayName         string              `json:"display_name"`
    SourceURL           string              `json:"source_url"`
    Category            string              `json:"category"`
    RecommendationClass RecommendationClass `json:"recommendation_class"`
    ClassRank           int                 `json:"class_rank"`
    DetectorSources     []EvidenceSource    `json:"detector_sources"`
    InstallRisk         RiskLevel           `json:"install_risk"`
    DataMovementRisk    RiskLevel           `json:"data_movement_risk"`
    RollbackGuidance    string              `json:"rollback_guidance,omitempty"`
    FreeReportAllowed   bool                `json:"free_report_allowed"`
    PaidPackAllowed     bool                `json:"paid_pack_allowed"`
    ResearchOnly        bool                `json:"research_only"`
    InstallPolicy       InstallPolicy       `json:"install_policy"`
    Notes               string              `json:"notes,omitempty"`
}

type ToolStateEntry struct {
    Tool    ToolID                 `json:"tool"`
    State   ToolState              `json:"state"`
    Sources map[EvidenceSource]int `json:"sources"`
}

type ToolStateMap map[ToolID]ToolStateEntry

type TokenSavingRecommendation struct {
    RecommendationID string                 `json:"recommendation_id"`
    PrimaryToolID    ToolID                 `json:"primary_tool_id"`
    SkippedToolIDs   []ToolID               `json:"skipped_tool_ids,omitempty"`
    Reason           Reason                 `json:"reason"`
    SignalIDs        []Signal               `json:"signal_ids"`
    Confidence       Confidence             `json:"confidence"`
    RiskLevel        RiskLevel              `json:"risk_level"`
    InstallPolicy    InstallPolicy          `json:"install_policy"`
    EvidenceCounts   map[EvidenceSource]int `json:"evidence_counts"`
}

type SkipNote struct {
    ToolID    ToolID `json:"tool_id"`
    Reason    Reason `json:"reason"`
    ForSignal Signal `json:"for_signal"`
}

type RecommendationSet struct {
    Primary         *TokenSavingRecommendation `json:"primary,omitempty"`
    Secondary       *TokenSavingRecommendation `json:"secondary,omitempty"`
    Skipped         []SkipNote                 `json:"skipped,omitempty"`
    RegistryVersion string                     `json:"registry_version"`
    EngineVersion   string                     `json:"engine_version"`
    Signals         []Signal                   `json:"signals"`
    UnknownIDCount  int                        `json:"unknown_id_count"`
}
```

### Functions

```go
// GetTool returns the registry entry for id, or (zero, false) if id is not
// in the allowlist. Pure; safe to call concurrently.
func GetTool(id ToolID) (TokenSavingTool, bool)

// AllTools returns a defensive copy of the registry, sorted by (Class, Rank).
// Pure; safe to call concurrently.
func AllTools() []TokenSavingTool

// RegistryVersion returns the registry's stable identifier (e.g.
// "phase-a-2026-05-19"). The value changes only when the registry literal
// changes; a CI test guards this invariant.
func RegistryVersion() string

// EngineVersion returns the engine's policy version (e.g. "v0.1-phase-a").
// Bumped on every change to rule precedence, conflict resolution, or
// recommendation-ID composition.
func EngineVersion() string

// Recommend is the entry point. It is a pure deterministic function of its
// inputs: identical (signals, state) yields byte-identical JSON when the
// result is marshalled with encoding/json.
//
// Inputs:
//   signals — must contain only registered Signal values; unknown values are
//             ignored and counted toward UnknownIDCount.
//   state   — ToolStateEntry rows for any tools the caller has evidence
//             about. Unknown ToolIDs are counted via UnknownIDCount and
//             dropped from the decision.
//
// Output:
//   RecommendationSet with at most one Primary and one Secondary
//   recommendation. Primary and Secondary always belong to different
//   RecommendationClass values when both are present.
func Recommend(signals []Signal, state ToolStateMap) RecommendationSet
```

## Invariants the engine enforces

1. **Determinism.** For identical `(signals, state)` inputs, two calls to
   `Recommend` produce `RecommendationSet` values that marshal to
   byte-identical JSON. Verified by `TestRecommendDeterminism`.

2. **Allowlist closure.** Every `ToolID` that appears in any output field
   (including `SkipNote.ToolID`) is present in `AllTools()`. Verified by
   `TestRecommendOutputAllowlist`.

3. **At most one primary + one secondary.** `len([]{Primary, Secondary}) ≤ 2`
   and, when both are set, their `RecommendationClass` differ. Verified by
   `TestRecommendOnePlusOne`.

4. **Active tools are skipped, not recommended.** If `state[tool].State ==
   ToolStateActiveHigh` for the tool a rule would otherwise pick, the engine
   emits a `SkipNote` and advances to the next eligible tool — never re-emits
   the active tool as a recommendation. Verified by
   `TestRecommendSkipsActiveTool`.

5. **No another-MCP for skill bloat.** When `SignalMCPSkillBloat` is among
   the firing signals, the resulting Primary's `RecommendationClass` is
   `ClassMCPSkillHygiene` (prune/lazy-load/scope), never `ClassRetrieval`,
   `ClassMCPOutputReducer`, or any other MCP-introducing class. Verified by
   `TestRecommendMCPSkillBloatNeverAddsMCP`.

6. **Privacy budget.** The marshalled JSON contains only allowlisted enum
   strings, registered `ToolID` values, structural JSON characters, and
   integer counts. Verified by `TestRecommendPrivacyBudget`.

7. **Waiver gate.** Any recommendation whose primary tool has
   `InstallPolicy == PolicyRecommendWithWaiver` carries that policy in the
   recommendation's `InstallPolicy` field. Callers must enforce the waiver
   UI; the engine does not auto-suppress these. Verified by
   `TestRecommendCarriesWaiverPolicy`.

8. **Registry version stability.** `RegistryVersion()` is bumped whenever a
   tool is added, removed, or any of its fields change. A test compares the
   live value to a checked-in golden constant; failure forces the maintainer
   to acknowledge the change.

## Stability commitment

- New constants may be added to any enum type without bumping a major
  version. Existing constants will not be renamed or removed in Phase A.
- New optional fields may be added to `TokenSavingRecommendation` and
  `RecommendationSet` (additive, JSON-omitempty) without bumping a major
  version.
- The function signatures of `Recommend`, `GetTool`, `AllTools`,
  `RegistryVersion`, and `EngineVersion` are frozen for Phase A.

## Out of scope (Phase B / future)

- A `Recommend` overload that takes per-signal severity numbers.
- A streaming or batch API.
- Caller-supplied policy injection.
- Any function that reads files, networks, or environment variables.
