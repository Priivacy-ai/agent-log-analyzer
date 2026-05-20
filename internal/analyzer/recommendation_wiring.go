// Package analyzer — Phase B wiring between report data and the
// frozen Phase A token-saving recommendation engine.
//
// AttachRecommendation derives engine signals and tool state from a
// fully-constructed Report and assigns the engine output to
// report.Recommendation. The function is deterministic, side-effect-
// free beyond the report mutation, and never returns an error.
package analyzer

import "slices"

// sourceCountCap enforces the privacy budget on bounded integer counts
// in ToolStateEntry.Sources. No natural report can reach this in
// practice; the cap is documented in the tool-state derivation contract.
const sourceCountCap = 100

// AttachRecommendation derives engine inputs from the provided report,
// invokes the frozen Phase A engine, and assigns the result to
// report.Recommendation. Safe to call with a nil pointer (no-op).
func AttachRecommendation(report *Report) {
	if report == nil {
		return
	}
	signals := deriveSignals(report)
	state := deriveToolStateMap(report)
	set := Recommend(signals, state)
	report.Recommendation = &set
}

// deriveSignals applies rules S-01..S-07 from
// contracts/signal-derivation-map.md to the report and returns the
// deduplicated, sorted slice of engine signals.
func deriveSignals(report *Report) []Signal {
	if report == nil {
		return nil
	}
	var signals []Signal

	// S-01..S-04: derive from finding IDs.
	for _, f := range report.Findings {
		switch f.ID {
		case "tool_output_bloat":
			signals = append(signals, SignalToolOutputBloat)
		case "repeated_file_reads":
			signals = append(signals, SignalRepeatedFileReads)
		case "retry_loop":
			signals = append(signals, SignalRetryLoop)
		case "context_growth_spikes":
			signals = append(signals, SignalContextGrowthSpikes)
		}
	}

	// S-05: MCP warning band.
	if isBloatBand(report.Ecosystem.ToolingUtilization.MCP.WarningBand) {
		signals = append(signals, SignalMCPSkillBloat)
	}
	// S-06: Skill warning band.
	if isBloatBand(report.Ecosystem.ToolingUtilization.Skill.WarningBand) {
		signals = append(signals, SignalMCPSkillBloat)
	}

	// S-07: no active usage-visibility fingerprint.
	hasActiveUsageTracker := false
	for _, fp := range report.Ecosystem.WorkflowFingerprints {
		if !fp.Active {
			continue
		}
		if tool, ok := GetTool(ToolID(fp.ID)); ok && tool.RecommendationClass == ClassUsageVisibility {
			hasActiveUsageTracker = true
			break
		}
	}
	if !hasActiveUsageTracker {
		signals = append(signals, SignalNoUsageVisibility)
	}

	return sortedSignalIDs(signals)
}

// isBloatBand reports whether the band value triggers
// SignalMCPSkillBloat (S-05/S-06).
func isBloatBand(band string) bool {
	return band == "high" || band == "severe"
}

// deriveToolStateMap applies rules T-F-01..T-S-02 from
// contracts/tool-state-derivation-map.md and returns the engine input
// keyed by ToolID. Only registry-known tool IDs may appear as keys.
func deriveToolStateMap(report *Report) ToolStateMap {
	state := ToolStateMap{}
	if report == nil {
		return state
	}

	// T-W-01: allowlisted WorkflowFrameworks entries that are also
	// token-saving registry tools contribute mentioned_low evidence so the
	// engine can attribute "we saw this tool referenced in your logs" even
	// when no SDD fingerprint or MCP/skill record exists. Stronger
	// downstream signals (active fingerprint, configured MCP, etc.) win
	// via mergeStateEntry's ToolStateMap.Resolve.
	for _, framework := range report.Ecosystem.WorkflowFrameworks {
		id := ToolID(framework)
		if _, ok := GetTool(id); !ok {
			continue
		}
		mergeStateEntry(state, id, ToolStateMentionedLow, map[EvidenceSource]int{
			EvidenceReportMention: 1,
		})
	}

	// Fingerprints (T-F-01..T-F-03).
	for _, fp := range report.Ecosystem.WorkflowFingerprints {
		id := ToolID(fp.ID)
		if _, ok := GetTool(id); !ok {
			continue
		}
		var ruleState ToolState
		switch {
		case fp.Active:
			ruleState = ToolStateActiveHigh
		case fp.Installed:
			ruleState = ToolStateInstalledMedium
		default:
			ruleState = ToolStateMentionedLow
		}
		sources := map[EvidenceSource]int{
			EvidenceReportMention: 1,
		}
		// Source enum values come from internal/analyzer/sdd: cli_binary is
		// "binary present on PATH"; cli_version_probe is "--version probe
		// matched" and only fires alongside a cli_binary peer. Either is
		// evidence the CLI is present locally.
		if slices.Contains(fp.Sources, "cli_binary") || slices.Contains(fp.Sources, "cli_version_probe") {
			sources[EvidenceCLIPresence] = 1
		}
		if fp.VersionBucket != "" {
			sources[EvidenceCLIVersion] = 1
		}
		mergeStateEntry(state, id, ruleState, sources)
	}

	// MCP (T-M-01/T-M-02).
	mcp := report.Ecosystem.ToolingUtilization.MCP
	for _, raw := range mcp.KnownServerIDs {
		id := ToolID(raw)
		if _, ok := GetTool(id); !ok {
			continue
		}
		var ruleState ToolState
		sources := map[EvidenceSource]int{}
		if slices.Contains(mcp.UniqueKnownCalledIDs, raw) {
			ruleState = ToolStateActiveHigh
			sources[EvidenceMCPActive] = 1
		} else {
			ruleState = ToolStateConfiguredMedium
			sources[EvidenceMCPConfigured] = 1
		}
		mergeStateEntry(state, id, ruleState, sources)
	}

	// Skill (T-S-01/T-S-02).
	skill := report.Ecosystem.ToolingUtilization.Skill
	for _, raw := range skill.KnownExposedIDs {
		id := ToolID(raw)
		if _, ok := GetTool(id); !ok {
			continue
		}
		var ruleState ToolState
		sources := map[EvidenceSource]int{}
		if slices.Contains(skill.KnownExecutedIDs, raw) {
			ruleState = ToolStateActiveHigh
			sources[EvidenceSkillConfigured] = 1
			sources[EvidenceReportMention] = 1
		} else {
			ruleState = ToolStateConfiguredMedium
			sources[EvidenceSkillConfigured] = 1
		}
		mergeStateEntry(state, id, ruleState, sources)
	}

	return state
}

// mergeStateEntry merges a single rule contribution for tool id into
// state, resolving the state via ToolStateMap.Resolve and summing
// source counts (capped at sourceCountCap per source key).
func mergeStateEntry(state ToolStateMap, id ToolID, ruleState ToolState, sources map[EvidenceSource]int) {
	existing, ok := state[id]
	if !ok {
		// Fresh entry: copy sources, clamp.
		fresh := ToolStateEntry{
			Tool:    id,
			State:   ruleState,
			Sources: make(map[EvidenceSource]int, len(sources)),
		}
		for src, n := range sources {
			fresh.Sources[src] = clampSourceCount(n)
		}
		state[id] = fresh
		return
	}
	// Existing: resolve state and sum sources, clamping at the cap.
	existing.State = state.Resolve(existing.State, ruleState)
	for src, n := range sources {
		existing.Sources[src] = clampSourceCount(existing.Sources[src] + n)
	}
	state[id] = existing
}

// clampSourceCount caps a bounded integer evidence count at the
// privacy-budget ceiling. Negative values are coerced to zero.
func clampSourceCount(n int) int {
	if n < 0 {
		return 0
	}
	if n > sourceCountCap {
		return sourceCountCap
	}
	return n
}
