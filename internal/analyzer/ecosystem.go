package analyzer

import (
	"bytes"
	"regexp"
	"sort"
	"strings"
)

type signature struct {
	id       string
	patterns []*regexp.Regexp
}

func DetectEcosystem(input []byte, lines []parsedLine) Ecosystem {
	text := string(input)
	registry := ecosystemRegistry()
	codingAgents := detectMany(text, registry.CodingAgents)
	eco := Ecosystem{
		Client:             primaryClient(codingAgents),
		CodingAgents:       codingAgents,
		OperatingSystem:    detectOS(text),
		Shell:              detectShell(text),
		WorkflowFrameworks: detectMany(text, registry.Frameworks),
		MCPServersKnown:    detectMany(text, registry.MCPServers),
		KnownSkills:        detectMany(text, registry.Skills),
		KnownPlugins:       detectMany(text, registry.Plugins),
		PackageManagers:    detectMany(text, registry.PackageManagers),
		VersionControl:     detectVCS(text),
	}
	eco.UnknownMCPServerCount = countUnknownMCP(input, eco.MCPServersKnown)
	eco.UnknownSkillCount = countUnknownSlashCommands(lines, registry.KnownSlashCommandIDs())
	return eco
}

func detectMany(text string, signatures []signature) []string {
	seen := map[string]bool{}
	for _, signature := range signatures {
		for _, pattern := range signature.patterns {
			if pattern.MatchString(text) {
				seen[signature.id] = true
				break
			}
		}
	}
	return sortedKeys(seen)
}

func primaryClient(codingAgents []string) string {
	if len(codingAgents) == 0 {
		return "unknown"
	}
	return codingAgents[0]
}

func detectOS(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "darwin") || strings.Contains(lower, "macos"):
		return "macos"
	case strings.Contains(lower, "wsl"):
		return "wsl"
	case strings.Contains(lower, "windows") || strings.Contains(lower, "powershell"):
		return "windows"
	case strings.Contains(lower, "linux") || strings.Contains(lower, "/home/"):
		return "linux"
	default:
		return "unknown"
	}
}

func detectShell(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "zsh"):
		return "zsh"
	case strings.Contains(lower, "bash"):
		return "bash"
	case strings.Contains(lower, "fish"):
		return "fish"
	case strings.Contains(lower, "powershell") || strings.Contains(lower, "pwsh"):
		return "powershell"
	default:
		return "unknown"
	}
}

func detectVCS(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "jj "):
		return "jj"
	case strings.Contains(lower, "git "):
		return "git"
	default:
		return "unknown"
	}
}

func countUnknownMCP(input []byte, known []string) int {
	re := regexp.MustCompile(`mcp__([A-Za-z0-9_-]+)__`)
	knownSet := map[string]bool{}
	for _, item := range known {
		knownSet[normalizeID(item)] = true
	}
	unknown := map[string]bool{}
	for _, match := range re.FindAllSubmatch(input, -1) {
		if len(match) < 2 {
			continue
		}
		name := normalizeID(string(bytes.ToLower(match[1])))
		if !knownSet[name] {
			unknown[name] = true
		}
	}
	return len(unknown)
}

func countUnknownSlashCommands(lines []parsedLine, known []string) int {
	re := regexp.MustCompile(`(?:^|[\s"'(:])/(?:[A-Za-z][A-Za-z0-9_-]{2,})`)
	knownSet := map[string]bool{}
	for _, item := range known {
		knownSet[normalizeID(item)] = true
	}
	unknown := map[string]bool{}
	for _, line := range lines {
		if line.IsTool {
			continue
		}
		for _, raw := range re.FindAllString(line.Text, -1) {
			raw = strings.TrimLeft(raw, " \t\n\r\"'(:")
			matchEnd := strings.Index(line.Text, raw) + len(raw)
			if matchEnd > len(raw)-1 && matchEnd < len(line.Text) && line.Text[matchEnd] == '/' {
				continue
			}
			name := strings.TrimPrefix(strings.ToLower(raw), "/")
			name = strings.TrimPrefix(name, "gstack-")
			if !knownSet[normalizeID(name)] {
				unknown[normalizeID(name)] = true
			}
		}
	}
	return len(unknown)
}

// computeToolingUtilization orchestrates the WP02 detection layer and WP03
// classifier into the ToolingUtilization payload that hangs off Ecosystem.
// Pure function: no I/O, no globals beyond the package-level registry cache.
// Unknown names are surfaced as counts only — never as strings — preserving
// the privacy invariant inherited from DetectEcosystem.
func computeToolingUtilization(input []byte, lines []parsedLine, metrics Metrics) ToolingUtilization {
	registry := ecosystemRegistry()

	// --- MCP ---
	mcpExp := detectMCPExposureFromHeaders(input, registry)
	mcpCalls := detectMCPCallsFromToolUse(input, lines, registry)
	mcpExposureKnown := mcpExp.InferenceSource == InferenceSourceHeader
	mcpInferenceSource := InferenceSourceNone
	serverCount := -1
	toolCount := -1
	if mcpExposureKnown {
		mcpInferenceSource = InferenceSourceHeader
		serverCount = len(mcpExp.KnownIDs) + mcpExp.UnknownCount
		if mcpExp.ExposedToolKnown {
			toolCount = mcpExp.ExposedToolCount
		}
	} else if mcpCalls.UniqueServerCount > 0 {
		mcpExposureKnown = true
		mcpInferenceSource = InferenceSourceCalls
		serverCount = mcpCalls.UniqueServerCount
		toolCount = mcpCalls.UniqueToolCount
	}
	mcpTokens, mcpTokensKnown := estimateMCPFootprintTokens(mcpExp.SchemaTextBytes, serverCount, toolCount)

	// Utilization ratio: callers as proxy for utilized exposure. Guard
	// against division-by-zero and clamp to [0,100].
	mcpRatio := 0
	if mcpExposureKnown {
		denom := toolCount
		if denom <= 0 {
			denom = serverCount
		}
		if denom > 0 {
			numer := mcpCalls.UniqueServerCount
			if numer > denom {
				numer = denom
			}
			mcpRatio = numer * 100 / denom
		}
	}

	mcpTokensField := tokenBucket(mcpTokens, mcpExposureKnown && mcpTokensKnown)
	mcp := MCPUtilization{
		KnownServerIDs:           dedupeSorted(mcpExp.KnownIDs),
		UnknownServerCount:       mcpExp.UnknownCount,
		ServerCountBucket:        countBucket(maxIntTU(serverCount, 0), mcpExposureKnown),
		ExposedToolCountBucket:   countBucket(maxIntTU(toolCount, 0), mcpExposureKnown && toolCount >= 0),
		ContextTokenBucket:       mcpTokensField,
		ExposureKnown:            mcpExposureKnown,
		InferenceSource:          mcpInferenceSource,
		CallCount:                mcpCalls.TotalCalls,
		KnownCallCount:           mcpCalls.KnownCallCount,
		UnknownCallCount:         mcpCalls.UnknownCallCount,
		UniqueKnownCalledIDs:     mcpCalls.UniqueKnownIDs,
		UniqueUnknownCalledCount: mcpCalls.UniqueUnknownCount,
		UtilizationRatioPct:      mcpRatio,
		ContextEfficiencyBucket:  efficiencyBucket(mcpRatio, mcpTokensField, mcpExposureKnown),
		WarningBand: classifyMCPBand(mcpBandInput{
			ServerCountBucket:      countBucket(maxIntTU(serverCount, 0), mcpExposureKnown),
			ExposedToolCountBucket: countBucket(maxIntTU(toolCount, 0), mcpExposureKnown && toolCount >= 0),
			ContextTokenBucket:     mcpTokensField,
			UtilizationRatioPct:    mcpRatio,
			ExposureKnown:          mcpExposureKnown,
			Rereads:                metrics.Rereads,
			RetryDepthMax:          metrics.RetryDepthMax,
			ContextGrowthEvents:    metrics.ContextGrowthEvents,
		}),
	}

	// --- Skill ---
	skillExp := detectSkillExposureFromHeaders(input, registry)
	skillExec := detectSkillExecutionsFromLines(lines, registry)
	skillExposureKnown := skillExp.InferenceSource == InferenceSourceHeader
	skillInferenceSource := InferenceSourceNone
	skillCount := -1
	if skillExposureKnown {
		skillInferenceSource = InferenceSourceHeader
		skillCount = len(skillExp.KnownIDs) + skillExp.UnknownCount
	}
	skillTokens, skillTokensKnown := estimateSkillFootprintTokens(skillExp.SchemaTextBytes, skillCount)
	skillRatio := 0
	if skillExposureKnown && skillCount > 0 {
		used := len(skillExec.KnownExecutedIDs) + skillExec.UnknownExecuted
		if used > skillCount {
			used = skillCount
		}
		skillRatio = used * 100 / skillCount
	}
	skillTokensField := tokenBucket(skillTokens, skillExposureKnown && skillTokensKnown)
	skill := SkillUtilization{
		KnownExposedIDs:         dedupeSorted(skillExp.KnownIDs),
		UnknownExposedCount:     skillExp.UnknownCount,
		ExposedCountBucket:      countBucket(maxIntTU(skillCount, 0), skillExposureKnown),
		ContextTokenBucket:      skillTokensField,
		ExposureKnown:           skillExposureKnown,
		InferenceSource:         skillInferenceSource,
		ExecutedCount:           skillExec.ExecutedCount,
		KnownExecutedIDs:        skillExec.KnownExecutedIDs,
		UnknownExecutedCount:    skillExec.UnknownExecuted,
		UtilizationRatioPct:     skillRatio,
		ContextEfficiencyBucket: efficiencyBucket(skillRatio, skillTokensField, skillExposureKnown),
		WarningBand: classifySkillBand(skillBandInput{
			ExposedCountBucket:  countBucket(maxIntTU(skillCount, 0), skillExposureKnown),
			ContextTokenBucket:  skillTokensField,
			UtilizationRatioPct: skillRatio,
			ExposureKnown:       skillExposureKnown,
			Rereads:             metrics.Rereads,
			RetryDepthMax:       metrics.RetryDepthMax,
			ContextGrowthEvents: metrics.ContextGrowthEvents,
		}),
	}

	return ToolingUtilization{MCP: mcp, Skill: skill}
}

// maxIntTU returns the larger of two ints. Named to avoid colliding with the
// existing max() helper in analyzer.go (kept stylistically local to WP04).
func maxIntTU(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// dedupeSorted returns a sorted, de-duplicated copy of xs. Returns a non-nil
// empty slice when xs is empty so the JSON contract never emits null.
func dedupeSorted(xs []string) []string {
	if len(xs) == 0 {
		return []string{}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func normalizeID(value string) string {
	return strings.ReplaceAll(strings.ToLower(value), "-", "_")
}
