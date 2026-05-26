package analytics

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
)

const SchemaVersion = "2026-05-26"

type Event struct {
	SchemaVersion     string              `json:"schema_version"`
	Event             string              `json:"event"`
	Analyzer          string              `json:"analyzer_version"`
	ScanType          string              `json:"scan_type"`
	ParserType        string              `json:"parser_type"`
	InputSize         string              `json:"input_size_bucket"`
	Turns             string              `json:"turn_bucket"`
	Sessions          string              `json:"session_count_bucket"`
	Score             string              `json:"score_bucket"`
	Waste             string              `json:"waste_bucket"`
	AnalyzedLogHashes []AnalyzedLogHash   `json:"analyzed_log_hashes,omitempty"`
	Findings          map[string]string   `json:"findings,omitempty"`
	Redactions        map[string]int      `json:"redactions,omitempty"`
	Ecosystem         EcosystemEvent      `json:"ecosystem"`
	Recommendation    RecommendationEvent `json:"recommendation"`
}

type AnalyzedLogHash struct {
	SourceID          string `json:"source_id"`
	ContentHashSHA256 string `json:"content_hash_sha256"`
	SizeBucket        string `json:"size_bucket,omitempty"`
}

type EcosystemEvent struct {
	Client                string             `json:"client"`
	CodingAgents          []string           `json:"coding_agents,omitempty"`
	OperatingSystem       string             `json:"operating_system"`
	Shell                 string             `json:"shell"`
	VersionControl        string             `json:"version_control"`
	WorkflowFrameworks    []string           `json:"workflow_frameworks,omitempty"`
	MCPServersKnown       []string           `json:"mcp_servers_known,omitempty"`
	UnknownMCPServerCount int                `json:"unknown_mcp_server_count,omitempty"`
	KnownSkills           []string           `json:"known_skills,omitempty"`
	UnknownSkillCount     int                `json:"unknown_skill_count,omitempty"`
	KnownPlugins          []string           `json:"known_plugins,omitempty"`
	UnknownPluginCount    int                `json:"unknown_plugin_count,omitempty"`
	PackageManagers       []string           `json:"package_managers,omitempty"`
	WorkflowFingerprints  []FingerprintEvent `json:"workflow_fingerprints,omitempty"`
	ToolingUtilization    ToolingEvent       `json:"tooling_utilization"`
}

type FingerprintEvent struct {
	ID            string   `json:"id"`
	Confidence    string   `json:"confidence"`
	Sources       []string `json:"sources,omitempty"`
	EvidenceCount int      `json:"evidence_count,omitempty"`
	Active        bool     `json:"active,omitempty"`
	Installed     bool     `json:"installed,omitempty"`
	VersionBucket string   `json:"version_bucket,omitempty"`
}

type ToolingEvent struct {
	MCP   MCPEvent   `json:"mcp"`
	Skill SkillEvent `json:"skill"`
}

type MCPEvent struct {
	KnownServerIDs           []string `json:"known_server_ids,omitempty"`
	UnknownServerCount       int      `json:"unknown_server_count,omitempty"`
	ServerCountBucket        string   `json:"server_count_bucket"`
	ExposedToolCountBucket   string   `json:"exposed_tool_count_bucket"`
	ContextTokenBucket       string   `json:"context_token_bucket"`
	ExposureKnown            bool     `json:"exposure_known"`
	InferenceSource          string   `json:"inference_source"`
	CallCountBucket          string   `json:"call_count_bucket"`
	KnownCallCountBucket     string   `json:"known_call_count_bucket"`
	UnknownCallCountBucket   string   `json:"unknown_call_count_bucket"`
	UniqueKnownCalledIDs     []string `json:"unique_known_called_ids,omitempty"`
	UniqueUnknownCalledCount int      `json:"unique_unknown_called_count,omitempty"`
	UtilizationRatioBucket   string   `json:"utilization_ratio_bucket"`
	ContextEfficiencyBucket  string   `json:"context_efficiency_bucket"`
	WarningBand              string   `json:"warning_band"`
}

type SkillEvent struct {
	KnownExposedIDs         []string `json:"known_exposed_ids,omitempty"`
	UnknownExposedCount     int      `json:"unknown_exposed_count,omitempty"`
	ExposedCountBucket      string   `json:"exposed_count_bucket"`
	ContextTokenBucket      string   `json:"context_token_bucket"`
	ExposureKnown           bool     `json:"exposure_known"`
	InferenceSource         string   `json:"inference_source"`
	ExecutedCountBucket     string   `json:"executed_count_bucket"`
	KnownExecutedIDs        []string `json:"known_executed_ids,omitempty"`
	UnknownExecutedCount    int      `json:"unknown_executed_count,omitempty"`
	UtilizationRatioBucket  string   `json:"utilization_ratio_bucket"`
	ContextEfficiencyBucket string   `json:"context_efficiency_bucket"`
	WarningBand             string   `json:"warning_band"`
}

type RecommendationEvent struct {
	Primary   *RecommendationSlot `json:"primary,omitempty"`
	Secondary *RecommendationSlot `json:"secondary,omitempty"`
	Signals   []string            `json:"signals,omitempty"`
	Skipped   string              `json:"skipped_count_bucket"`
}

type RecommendationSlot struct {
	Class         string   `json:"class"`
	ToolID        string   `json:"tool_id,omitempty"`
	Reason        string   `json:"reason"`
	Signals       []string `json:"signals,omitempty"`
	RiskLevel     string   `json:"risk_level"`
	InstallPolicy string   `json:"install_policy"`
}

func FromReport(report analyzer.Report, scanType string) Event {
	agg := report.AggregateEvent
	event := Event{
		SchemaVersion:     SchemaVersion,
		Event:             "analytics.report",
		Analyzer:          report.Version,
		ScanType:          normalizeScanType(scanType),
		ParserType:        enumOrUnknown(agg.ParserType, parserTypes),
		InputSize:         enumOrUnknown(agg.InputSizeBucket, inputSizeBuckets),
		Turns:             enumOrCalculated(agg.TurnBucket, rangeBucket(report.Metrics.Turns, []int{10, 50, 100, 200}), turnBuckets),
		Sessions:          intBucket(sessionCount(report, scanType), []int{2, 10, 25, 50, 100}),
		Score:             enumOrCalculated(agg.ScoreBucket, rangeBucket(report.Score, []int{20, 40, 60, 80}), scoreBuckets),
		Waste:             enumOrCalculated(agg.WasteBucket, rangeBucket(report.EstimatedWaste.High, []int{10, 20, 40, 60}), wasteBuckets),
		AnalyzedLogHashes: analyzedLogHashesFromReport(report),
		Findings:          filterFindingSeverities(report),
		Redactions:        filterRedactions(report.Redactions),
		Ecosystem:         ecosystemFromReport(report.Ecosystem),
		Recommendation:    recommendationFromReport(report.Recommendation),
	}
	if event.Analyzer == "" {
		event.Analyzer = "unknown"
	}
	return event
}

func MarshalJSONLine(event Event) ([]byte, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func normalizeScanType(scanType string) string {
	switch scanType {
	case "single", "free", "":
		return "free"
	case "paid_bundle", "paid":
		return "paid"
	default:
		return "legacy_internal"
	}
}

func sessionCount(report analyzer.Report, scanType string) int {
	if report.Metrics.SessionCount > 0 {
		return report.Metrics.SessionCount
	}
	if normalizeScanType(scanType) == "free" {
		return 1
	}
	return 0
}

func analyzedLogHashesFromReport(report analyzer.Report) []AnalyzedLogHash {
	if len(report.SourceReports) == 0 {
		return nil
	}
	allowedSources := knownStringSet(analyzer.KnownEcosystemIDs("coding_agent"))
	seen := map[string]bool{}
	out := []AnalyzedLogHash{}
	for _, source := range report.SourceReports {
		sourceID := enumOrUnknown(source.SourceID, allowedSources)
		for _, ref := range source.LogRefs {
			hash := strings.ToLower(strings.TrimSpace(ref.ContentHashSHA256))
			if !sha256HexPattern.MatchString(hash) {
				continue
			}
			key := sourceID + "\x00" + hash
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, AnalyzedLogHash{
				SourceID:          sourceID,
				ContentHashSHA256: hash,
				SizeBucket:        enumOrEmpty(ref.SizeBucket, logSizeBuckets),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceID != out[j].SourceID {
			return out[i].SourceID < out[j].SourceID
		}
		return out[i].ContentHashSHA256 < out[j].ContentHashSHA256
	})
	return out
}

func ecosystemFromReport(ecosystem analyzer.Ecosystem) EcosystemEvent {
	return EcosystemEvent{
		Client:                enumOrUnknown(ecosystem.Client, knownStringSet(analyzer.KnownEcosystemIDs("coding_agent"))),
		CodingAgents:          filterIDs(ecosystem.CodingAgents, "coding_agent"),
		OperatingSystem:       enumOrUnknown(ecosystem.OperatingSystem, operatingSystems),
		Shell:                 enumOrUnknown(ecosystem.Shell, shells),
		VersionControl:        enumOrUnknown(ecosystem.VersionControl, versionControls),
		WorkflowFrameworks:    filterIDs(ecosystem.WorkflowFrameworks, "framework"),
		MCPServersKnown:       filterIDs(ecosystem.MCPServersKnown, "mcp"),
		UnknownMCPServerCount: clampCount(ecosystem.UnknownMCPServerCount),
		KnownSkills:           filterIDs(ecosystem.KnownSkills, "skill"),
		UnknownSkillCount:     clampCount(ecosystem.UnknownSkillCount),
		KnownPlugins:          filterIDs(ecosystem.KnownPlugins, "plugin"),
		UnknownPluginCount:    clampCount(ecosystem.UnknownPluginCount),
		PackageManagers:       filterIDs(ecosystem.PackageManagers, "package_manager"),
		WorkflowFingerprints:  filterFingerprints(ecosystem.WorkflowFingerprints),
		ToolingUtilization:    toolingFromReport(ecosystem.ToolingUtilization),
	}
}

func toolingFromReport(tooling analyzer.ToolingUtilization) ToolingEvent {
	mcp := tooling.MCP
	skill := tooling.Skill
	return ToolingEvent{
		MCP: MCPEvent{
			KnownServerIDs:           filterIDs(mcp.KnownServerIDs, "mcp"),
			UnknownServerCount:       clampCount(mcp.UnknownServerCount),
			ServerCountBucket:        enumOrUnknown(mcp.ServerCountBucket, countBuckets),
			ExposedToolCountBucket:   enumOrUnknown(mcp.ExposedToolCountBucket, countBuckets),
			ContextTokenBucket:       enumOrUnknown(mcp.ContextTokenBucket, tokenBuckets),
			ExposureKnown:            mcp.ExposureKnown,
			InferenceSource:          enumOrUnknown(mcp.InferenceSource, inferenceSources),
			CallCountBucket:          intBucket(mcp.CallCount, []int{1, 3, 10, 25, 50, 100}),
			KnownCallCountBucket:     intBucket(mcp.KnownCallCount, []int{1, 3, 10, 25, 50, 100}),
			UnknownCallCountBucket:   intBucket(mcp.UnknownCallCount, []int{1, 3, 10, 25, 50, 100}),
			UniqueKnownCalledIDs:     filterIDs(mcp.UniqueKnownCalledIDs, "mcp"),
			UniqueUnknownCalledCount: clampCount(mcp.UniqueUnknownCalledCount),
			UtilizationRatioBucket:   ratioBucket(mcp.UtilizationRatioPct),
			ContextEfficiencyBucket:  enumOrUnknown(mcp.ContextEfficiencyBucket, efficiencyBuckets),
			WarningBand:              enumOrUnknown(mcp.WarningBand, warningBands),
		},
		Skill: SkillEvent{
			KnownExposedIDs:         filterIDs(skill.KnownExposedIDs, "skill"),
			UnknownExposedCount:     clampCount(skill.UnknownExposedCount),
			ExposedCountBucket:      enumOrUnknown(skill.ExposedCountBucket, countBuckets),
			ContextTokenBucket:      enumOrUnknown(skill.ContextTokenBucket, tokenBuckets),
			ExposureKnown:           skill.ExposureKnown,
			InferenceSource:         enumOrUnknown(skill.InferenceSource, inferenceSources),
			ExecutedCountBucket:     intBucket(skill.ExecutedCount, []int{1, 3, 10, 25, 50, 100}),
			KnownExecutedIDs:        filterIDs(skill.KnownExecutedIDs, "skill"),
			UnknownExecutedCount:    clampCount(skill.UnknownExecutedCount),
			UtilizationRatioBucket:  ratioBucket(skill.UtilizationRatioPct),
			ContextEfficiencyBucket: enumOrUnknown(skill.ContextEfficiencyBucket, efficiencyBuckets),
			WarningBand:             enumOrUnknown(skill.WarningBand, warningBands),
		},
	}
}

func filterFingerprints(in []analyzer.EcosystemFingerprint) []FingerprintEvent {
	out := make([]FingerprintEvent, 0, len(in))
	for _, fp := range in {
		if !analyzer.ValidEcosystemID("workflow_fingerprint", fp.ID) {
			continue
		}
		out = append(out, FingerprintEvent{
			ID:            fp.ID,
			Confidence:    enumOrUnknown(fp.Confidence, confidenceBuckets),
			Sources:       filterStrings(fp.Sources, fingerprintSources),
			EvidenceCount: clampCount(fp.EvidenceCount),
			Active:        fp.Active,
			Installed:     fp.Installed,
			VersionBucket: versionBucket(fp.VersionBucket),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func recommendationFromReport(set *analyzer.RecommendationSet) RecommendationEvent {
	if set == nil {
		return RecommendationEvent{Skipped: "0_1"}
	}
	return RecommendationEvent{
		Primary:   recommendationSlot(set.Primary),
		Secondary: recommendationSlot(set.Secondary),
		Signals:   filterSignalIDs(set.Signals),
		Skipped:   intBucket(len(set.Skipped), []int{1, 3, 10, 25, 50}),
	}
}

func recommendationSlot(rec *analyzer.TokenSavingRecommendation) *RecommendationSlot {
	if rec == nil {
		return nil
	}
	class := ""
	if rec.PrimaryToolID != "" {
		if tool, ok := analyzer.GetTool(rec.PrimaryToolID); ok {
			class = string(tool.RecommendationClass)
		} else {
			return nil
		}
	} else {
		class = classFromSignals(rec.SignalIDs)
	}
	return &RecommendationSlot{
		Class:         enumOrUnknown(class, recommendationClasses),
		ToolID:        knownToolID(rec.PrimaryToolID),
		Reason:        enumOrUnknown(string(rec.Reason), recommendationReasons),
		Signals:       filterSignalIDs(rec.SignalIDs),
		RiskLevel:     enumOrUnknown(string(rec.RiskLevel), riskLevels),
		InstallPolicy: enumOrUnknown(string(rec.InstallPolicy), installPolicies),
	}
}

func classFromSignals(signals []analyzer.Signal) string {
	for _, signal := range signals {
		switch signal {
		case analyzer.SignalMCPSkillBloat:
			return "mcp_skill_hygiene"
		case analyzer.SignalRetryLoop, analyzer.SignalContextGrowthSpikes:
			return "context_hygiene"
		}
	}
	return "unknown"
}

func filterSignalIDs(signals []analyzer.Signal) []string {
	out := make([]string, 0, len(signals))
	for _, signal := range signals {
		if enumAllowed(string(signal), signalsAllowed) {
			out = append(out, string(signal))
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

func knownToolID(id analyzer.ToolID) string {
	if id == "" {
		return ""
	}
	if _, ok := analyzer.GetTool(id); !ok {
		return ""
	}
	return string(id)
}

func filterFindingSeverities(report analyzer.Report) map[string]string {
	out := map[string]string{}
	for _, finding := range report.Findings {
		if !enumAllowed(finding.ID, findingIDs) {
			continue
		}
		out[finding.ID] = enumOrUnknown(finding.Severity, severities)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterRedactions(in map[string]int) map[string]int {
	out := map[string]int{}
	for key, value := range in {
		if enumAllowed(key, redactionFamilies) {
			out[key] = clampCount(value)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func filterIDs(in []string, category string) []string {
	out := make([]string, 0, len(in))
	for _, id := range in {
		if analyzer.ValidEcosystemID(category, id) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

func filterStrings(in []string, allowed map[string]bool) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if allowed[s] {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := in[:0]
	var prev string
	for i, item := range in {
		if i > 0 && item == prev {
			continue
		}
		out = append(out, item)
		prev = item
	}
	return out
}

func intBucket(value int, thresholds []int) string {
	if value <= 0 {
		return "0_1"
	}
	prev := 0
	for _, threshold := range thresholds {
		if value < threshold {
			return formatBucket(prev, threshold)
		}
		prev = threshold
	}
	return formatBucket(prev, 0)
}

func rangeBucket(value int, thresholds []int) string {
	prev := 0
	for _, threshold := range thresholds {
		if value < threshold {
			return formatBucket(prev, threshold)
		}
		prev = threshold
	}
	return formatBucket(prev, 0)
}

func ratioBucket(value int) string {
	if value <= 0 {
		return "0"
	}
	if value < 10 {
		return "1_10"
	}
	if value < 25 {
		return "10_25"
	}
	if value < 50 {
		return "25_50"
	}
	if value < 75 {
		return "50_75"
	}
	if value < 100 {
		return "75_100"
	}
	return "100"
}

func formatBucket(low, high int) string {
	if high == 0 {
		return itoa(low) + "_plus"
	}
	return itoa(low) + "_" + itoa(high)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func enumOrEmpty(value string, allowed map[string]bool) string {
	if value == "" {
		return ""
	}
	return enumOrUnknown(value, allowed)
}

func enumOrUnknown(value string, allowed map[string]bool) string {
	if allowed[value] {
		return value
	}
	return "unknown"
}

func enumOrCalculated(value, calculated string, allowed map[string]bool) string {
	if allowed[value] {
		return value
	}
	if allowed[calculated] {
		return calculated
	}
	return "unknown"
}

func enumAllowed(value string, allowed map[string]bool) bool {
	return allowed[value]
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func clampCount(value int) int {
	if value < 0 {
		return 0
	}
	if value > 1000 {
		return 1000
	}
	return value
}

func knownStringSet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in)+1)
	for key := range in {
		out[key] = true
	}
	out["unknown"] = true
	return out
}

var parserTypes = map[string]bool{"jsonl": true, "text": true, "multi_source": true, "paid_bundle": true, "unknown": true}
var inputSizeBuckets = map[string]bool{"0_1024": true, "1024_1048576": true, "1048576_10485760": true, "10485760_52428800": true, "52428800_plus": true, "unknown": true}
var logSizeBuckets = map[string]bool{"<10 KB": true, "10-100 KB": true, "100 KB-1 MB": true, "1-5 MB": true, ">5 MB": true, "unknown": true}
var turnBuckets = map[string]bool{"0_10": true, "10_50": true, "50_100": true, "100_200": true, "200_plus": true, "unknown": true}
var scoreBuckets = map[string]bool{"0_20": true, "20_40": true, "40_60": true, "60_80": true, "80_plus": true, "unknown": true}
var wasteBuckets = map[string]bool{"0_10": true, "10_20": true, "20_40": true, "40_60": true, "60_plus": true, "unknown": true}
var operatingSystems = map[string]bool{"macos": true, "linux": true, "windows": true, "wsl": true, "unknown": true}
var shells = map[string]bool{"zsh": true, "bash": true, "fish": true, "powershell": true, "unknown": true}
var versionControls = map[string]bool{"git": true, "jj": true, "unknown": true}
var countBuckets = map[string]bool{"none": true, "1-3": true, "4-10": true, "11-25": true, "26-50": true, "51-100": true, "100+": true, "unknown": true}
var tokenBuckets = map[string]bool{"none": true, "<1k": true, "1k-5k": true, "5k-15k": true, "15k-50k": true, "50k+": true, "unknown": true}
var efficiencyBuckets = map[string]bool{"unused": true, "underutilized": true, "moderate": true, "well-utilized": true, "unknown": true}
var warningBands = map[string]bool{"normal": true, "watch": true, "high": true, "severe": true, "unknown": true}
var inferenceSources = map[string]bool{"header": true, "calls": true, "none": true, "unknown": true}
var confidenceBuckets = map[string]bool{"low": true, "medium": true, "high": true, "unknown": true}
var versionBuckets = map[string]bool{"unknown": true, "recent": true, "older": true, "0.x": true, "1.x": true, "2.x": true, "3.x": true, "4_plus": true}
var severities = map[string]bool{"low": true, "medium": true, "high": true, "unknown": true}
var findingIDs = map[string]bool{
	"repeated_file_reads": true, "tool_output_bloat": true, "retry_loop": true,
	"context_growth_spikes": true, "mcp_bloat": true, "skill_bloat": true,
}
var redactionFamilies = map[string]bool{
	"anthropic_key": true, "openai_key": true, "github_token": true, "npm_token": true,
	"aws_access_key": true, "google_api_key": true, "database_url": true, "cookie": true,
	"jwt": true, "ssh_private_key": true, "email": true, "generic_secret": true,
}
var fingerprintSources = map[string]bool{
	"config_dir": true, "config_file": true, "package_manifest": true, "command_name": true,
	"slash_command": true, "mcp_server_name": true, "skill_name": true, "plugin_manifest": true,
	"cli_binary": true, "cli_probe": true, "cli_version_probe": true,
}
var signalsAllowed = map[string]bool{
	"no_usage_visibility": true, "tool_output_bloat": true, "shell_output_bloat": true,
	"mcp_tool_output_bloat": true, "repeated_file_reads": true, "broad_repo_exploration": true,
	"unchanged_file_rereads": true, "mcp_skill_bloat": true, "output_verbosity": true,
	"retry_loop": true, "context_growth_spikes": true,
}
var recommendationClasses = map[string]bool{
	"usage_visibility": true, "mcp_skill_hygiene": true, "mcp_output_reducer": true,
	"shell_output_reducer": true, "retrieval": true, "reread_guard": true,
	"context_hygiene": true, "output_verbosity": true, "unknown": true,
}
var recommendationReasons = map[string]bool{
	"absent": true, "installed_inactive": true, "configured_inactive": true,
	"active_persistent": true, "rejected_alternative": true, "prune_first": true,
	"audit_config": true, "no_op": true, "server_quota_check": true, "unknown": true,
}
var riskLevels = map[string]bool{"low": true, "medium": true, "high": true, "unknown": true}
var installPolicies = map[string]bool{
	"bundle": true, "recommend": true, "recommend_with_waiver": true,
	"research_only": true, "reference_only": true, "unknown": true,
}

var majorMinorVersionRE = regexp.MustCompile(`^(\d+)\.\d+$`)
var sha256HexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func versionBucket(value string) string {
	if value == "" {
		return ""
	}
	if matches := majorMinorVersionRE.FindStringSubmatch(value); len(matches) == 2 {
		switch matches[1] {
		case "0", "1", "2", "3":
			return matches[1] + ".x"
		default:
			return "4_plus"
		}
	}
	return enumOrUnknown(value, versionBuckets)
}
