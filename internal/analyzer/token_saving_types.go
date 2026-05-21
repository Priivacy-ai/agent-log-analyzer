// Package analyzer — token-saving engine enums, structs, and helpers.
//
// WP02 of mission token-saving-recommendation-engine-phase-a-01KRZKCJ.
//
// This file declares every Phase A enum (as named string types with
// their full constant sets), the engine input shape (ToolStateEntry /
// ToolStateMap), the deterministic conflict-resolution helper
// (ToolStateMap.Resolve), the engine output structs
// (TokenSavingRecommendation, SkipNote, RecommendationSet), and the
// EngineVersion accessor. It is consumed by token_saving_tools.go
// (WP01's registry; this file replaces the placeholder type-alias
// hand-off declared there) and by token_saving_recommendations.go
// (WP03's engine logic).
//
// No business logic lives here — only declarations and small
// deterministic helpers. All map iteration in this file goes through
// sorted keys (NFR-001 determinism).
package analyzer

import "sort"

// -----------------------------------------------------------------------------
// Enum: ToolState
// -----------------------------------------------------------------------------

// ToolState is the resolved per-tool state the engine consumes. Values
// are ordered by trust via ToolStateMap.Resolve; see research.md §4.
type ToolState string

const (
	ToolStateUnknown          ToolState = "unknown"
	ToolStateMentionedLow     ToolState = "mentioned_low"
	ToolStateInstalledMedium  ToolState = "installed_medium"
	ToolStateConfiguredMedium ToolState = "configured_medium"
	ToolStateActiveHigh       ToolState = "active_high"
	ToolStateRejectedMedium   ToolState = "rejected_medium"
)

// -----------------------------------------------------------------------------
// Enum: EvidenceSource
// -----------------------------------------------------------------------------

// EvidenceSource identifies how the caller learned about a tool. The
// closed set is enumerated below; ToolStateEntry.Sources keys must be
// drawn from these values only.
type EvidenceSource string

const (
	EvidenceCLIPresence          EvidenceSource = "cli_presence"
	EvidenceCLIVersion           EvidenceSource = "cli_version"
	EvidenceLogActiveCommand     EvidenceSource = "log_active_command"
	EvidenceMCPConfigured        EvidenceSource = "mcp_configured"
	EvidenceMCPActive            EvidenceSource = "mcp_active"
	EvidencePluginConfigured     EvidenceSource = "plugin_configured"
	EvidenceSkillConfigured      EvidenceSource = "skill_configured"
	EvidenceHookConfigured       EvidenceSource = "hook_configured"
	EvidenceStatuslineConfigured EvidenceSource = "statusline_configured"
	EvidenceReportMention        EvidenceSource = "report_mention"
	EvidenceFailureOrRejection   EvidenceSource = "failure_or_rejection"
)

// -----------------------------------------------------------------------------
// Enum: Signal
// -----------------------------------------------------------------------------

// Signal is the closed set of usage-pattern signals the engine recognizes.
type Signal string

const (
	SignalNoUsageVisibility    Signal = "no_usage_visibility"
	SignalToolOutputBloat      Signal = "tool_output_bloat"
	SignalShellOutputBloat     Signal = "shell_output_bloat"
	SignalMCPToolOutputBloat   Signal = "mcp_tool_output_bloat"
	SignalRepeatedFileReads    Signal = "repeated_file_reads"
	SignalBroadRepoExploration Signal = "broad_repo_exploration"
	SignalUnchangedFileRereads Signal = "unchanged_file_rereads"
	SignalMCPSkillBloat        Signal = "mcp_skill_bloat"
	SignalOutputVerbosity      Signal = "output_verbosity"
	SignalRetryLoop            Signal = "retry_loop"
	SignalContextGrowthSpikes  Signal = "context_growth_spikes"
)

// -----------------------------------------------------------------------------
// Enum: RecommendationClass
// -----------------------------------------------------------------------------

// RecommendationClass groups tools that solve the same kind of problem.
// Used for primary/secondary dedupe and rule precedence.
type RecommendationClass string

const (
	ClassUsageVisibility    RecommendationClass = "usage_visibility"
	ClassMCPSkillHygiene    RecommendationClass = "mcp_skill_hygiene"
	ClassMCPOutputReducer   RecommendationClass = "mcp_output_reducer"
	ClassShellOutputReducer RecommendationClass = "shell_output_reducer"
	ClassRetrieval          RecommendationClass = "retrieval"
	ClassRereadGuard        RecommendationClass = "reread_guard"
	ClassContextHygiene     RecommendationClass = "context_hygiene"
	ClassOutputVerbosity    RecommendationClass = "output_verbosity"
)

// -----------------------------------------------------------------------------
// Enum: Confidence
// -----------------------------------------------------------------------------

// Confidence is derived deterministically from the underlying tool-state
// and evidence-source mix; never user-supplied.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// -----------------------------------------------------------------------------
// Enum: RiskLevel
// -----------------------------------------------------------------------------

// RiskLevel is the bucketed install / data-movement risk surface used by
// both TokenSavingTool and TokenSavingRecommendation.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// -----------------------------------------------------------------------------
// Enum: InstallPolicy
// -----------------------------------------------------------------------------

// InstallPolicy gates whether and how a tool may be emitted as a
// recommendation.
type InstallPolicy string

const (
	PolicyBundle              InstallPolicy = "bundle"
	PolicyRecommend           InstallPolicy = "recommend"
	PolicyRecommendWithWaiver InstallPolicy = "recommend_with_waiver"
	PolicyResearchOnly        InstallPolicy = "research_only"
	PolicyReferenceOnly       InstallPolicy = "reference_only"
)

// -----------------------------------------------------------------------------
// Enum: Reason
// -----------------------------------------------------------------------------

// Reason is the closed set of justifications attached to a
// TokenSavingRecommendation or SkipNote.
type Reason string

const (
	ReasonAbsent              Reason = "absent"
	ReasonInstalledInactive   Reason = "installed_inactive"
	ReasonConfiguredInactive  Reason = "configured_inactive"
	ReasonActivePersistent    Reason = "active_persistent"
	ReasonRejectedAlternative Reason = "rejected_alternative"
	ReasonPruneFirst          Reason = "prune_first"
	ReasonAuditConfig         Reason = "audit_config"
	ReasonNoOp                Reason = "no_op"
	ReasonServerQuotaCheck    Reason = "server_quota_check"
)

// DefocusFailureMode is the bounded taxonomy used to explain why a
// recommendation exists. It is adapted from the context-defocus tooling
// audit and intentionally names problem classes, not private user data.
type DefocusFailureMode string

const (
	FailureNoisyTerminalLogs     DefocusFailureMode = "noisy_terminal_logs"
	FailureToolOutputFlooding    DefocusFailureMode = "tool_output_flooding"
	FailureRepeatedNavigation    DefocusFailureMode = "repeated_codebase_navigation"
	FailureBroadReadsOrVerbosity DefocusFailureMode = "broad_file_reads_or_verbose_output"
	FailureMemoryGaps            DefocusFailureMode = "memory_gaps"
	FailureCrossCutting          DefocusFailureMode = "cross_cutting_telemetry"
)

// -----------------------------------------------------------------------------
// Engine input shape
// -----------------------------------------------------------------------------

// ToolStateEntry is one row of caller-provided evidence about a single
// tool. The State field is the resolved (post-conflict) state; the
// Sources map carries bounded integer counts of how many times each
// EvidenceSource fired for this tool.
//
// Callers must populate Sources with bounded integer counts only —
// never with raw user data, free-form strings, or unbounded values. The
// privacy budget asserted by WP04's TestRecommendPrivacyBudget depends
// on this invariant.
type ToolStateEntry struct {
	Tool    ToolID                 `json:"tool"`
	State   ToolState              `json:"state"`
	Sources map[EvidenceSource]int `json:"sources"`
}

// ToolStateMap is the engine's input keyed by ToolID. Iteration in
// engine paths must go through SortedTools() to preserve determinism;
// raw range over this map is forbidden in WP03's engine code.
type ToolStateMap map[ToolID]ToolStateEntry

// Resolve combines two ToolState values according to the conflict
// precedence documented in research.md §4 and spec.md "Edge Cases":
//
//	rejected_medium > active_high > configured_medium >
//	installed_medium > mentioned_low > unknown
//
// Rationale: an explicit rejection is the strongest signal (the user
// actively opted out); an active observation outranks mere config
// presence; mere mentions are the weakest. The receiver is on
// ToolStateMap for discoverability — the function itself is a pure
// two-input combiner with no map access.
func (ToolStateMap) Resolve(a, b ToolState) ToolState {
	order := map[ToolState]int{
		ToolStateRejectedMedium:   5,
		ToolStateActiveHigh:       4,
		ToolStateConfiguredMedium: 3,
		ToolStateInstalledMedium:  2,
		ToolStateMentionedLow:     1,
		ToolStateUnknown:          0,
	}
	if order[a] >= order[b] {
		return a
	}
	return b
}

// SortedTools returns the map's keys lexicographically sorted. All
// engine iteration over a ToolStateMap must go through this helper to
// satisfy NFR-001 (byte-identical JSON for identical inputs).
func (m ToolStateMap) SortedTools() []ToolID {
	keys := make([]ToolID, 0, len(m))
	for id := range m {
		keys = append(keys, id)
	}
	sort.Slice(keys, func(i, j int) bool {
		return string(keys[i]) < string(keys[j])
	})
	return keys
}

// sortedSignalIDs returns a copy of in sorted ascending by string value
// with duplicates removed. Package-private — used by the engine when
// echoing the caller's signals slice into RecommendationSet.Signals and
// when composing TokenSavingRecommendation.SignalIDs.
func sortedSignalIDs(in []Signal) []Signal {
	if len(in) == 0 {
		return nil
	}
	cp := make([]Signal, len(in))
	copy(cp, in)
	sort.Slice(cp, func(i, j int) bool {
		return string(cp[i]) < string(cp[j])
	})
	// Dedupe in place. cp is sorted, so duplicates are adjacent.
	out := cp[:0]
	for i, s := range cp {
		if i == 0 || s != cp[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// Engine output shapes
// -----------------------------------------------------------------------------

// TokenSavingRecommendation is one emitted recommendation. JSON tags
// match contracts/token_saving_engine_go_api.md exactly; reordering or
// renaming a field is a contract break.
type TokenSavingRecommendation struct {
	RecommendationID string                 `json:"recommendation_id"`
	PrimaryToolID    ToolID                 `json:"primary_tool_id"`
	PrimaryToolName  string                 `json:"primary_tool_name,omitempty"`
	PrimaryToolURL   string                 `json:"primary_tool_url,omitempty"`
	FailureModes     []DefocusFailureMode   `json:"failure_modes,omitempty"`
	SkippedToolIDs   []ToolID               `json:"skipped_tool_ids,omitempty"`
	Reason           Reason                 `json:"reason"`
	SignalIDs        []Signal               `json:"signal_ids"`
	Confidence       Confidence             `json:"confidence"`
	RiskLevel        RiskLevel              `json:"risk_level"`
	DataMovementRisk RiskLevel              `json:"data_movement_risk,omitempty"`
	InstallPolicy    InstallPolicy          `json:"install_policy"`
	InstallSurface   string                 `json:"install_surface,omitempty"`
	ConflictsWith    []ToolID               `json:"conflicts_with,omitempty"`
	VettingNotes     string                 `json:"vetting_notes,omitempty"`
	AmbiguityWarning string                 `json:"ambiguity_warning,omitempty"`
	EvidenceCounts   map[EvidenceSource]int `json:"evidence_counts"`
}

// SkipNote records that a candidate tool was *not* recommended for a
// particular signal, with the reason it was skipped.
type SkipNote struct {
	ToolID    ToolID `json:"tool_id"`
	Reason    Reason `json:"reason"`
	ForSignal Signal `json:"for_signal"`
}

// RecommendationSet is the entire Phase A engine contract returned to
// callers. Primary == nil && Secondary == nil with an empty Skipped is
// the no-op case (Reason = ReasonNoOp is recorded on the unset
// Primary's absence).
type RecommendationSet struct {
	Primary         *TokenSavingRecommendation `json:"primary,omitempty"`
	Secondary       *TokenSavingRecommendation `json:"secondary,omitempty"`
	Skipped         []SkipNote                 `json:"skipped,omitempty"`
	RegistryVersion string                     `json:"registry_version"`
	EngineVersion   string                     `json:"engine_version"`
	Signals         []Signal                   `json:"signals"`
	UnknownIDCount  int                        `json:"unknown_id_count"`
}

// -----------------------------------------------------------------------------
// Engine version
// -----------------------------------------------------------------------------

// engineVersionString is bumped on every change to rule precedence,
// conflict resolution, or recommendation-ID composition. It is
// exposed only via EngineVersion().
const engineVersionString = "v0.1-phase-a"

// EngineVersion returns the engine's policy version. Pure; safe to
// call concurrently.
func EngineVersion() string {
	return engineVersionString
}
