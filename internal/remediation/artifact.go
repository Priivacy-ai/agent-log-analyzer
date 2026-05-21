package remediation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
)

const GeneratorVersion = "0.1.0"

var safeValueRE = regexp.MustCompile(`^[a-z0-9_.:-]+$`)

const (
	sourceAgentAnalyzerMatrix  = "https://github.com/Priivacy-ai/agent-log-analyzer/blob/main/docs/remediation/token-saving-tooling-matrix.md"
	sourceAnthropicPlugins     = "https://code.claude.com/docs/en/discover-plugins"
	sourceAwesomeClaudeCode    = "https://github.com/hesreallyhim/awesome-claude-code"
	sourceClaudeContext        = "https://github.com/zilliztech/claude-context"
	sourceClaudeHooksMastery   = "https://github.com/disler/claude-code-hooks-mastery"
	sourceClaudeTokenEfficient = "https://github.com/drona23/claude-token-efficient"
	sourceContextMode          = "https://github.com/mksglu/context-mode"
	sourceCCStatusline         = "https://github.com/sirmalloc/ccstatusline"
	sourceCCUsage              = "https://github.com/ryoppippi/ccusage"
	sourceGrepAI               = "https://github.com/yoanbernabeu/grepai"
	sourceRTK                  = "https://github.com/rtk-ai/rtk"
	sourceUsageMonitor         = "https://github.com/Maciek-roboblog/Claude-Code-Usage-Monitor"
	sourceGitHubPlugin         = "https://claude.com/plugins/github"
	sourceLinearPlugin         = "https://claude.com/plugins/linear"
	sourceNotionPlugin         = "https://claude.com/plugins/notion"
	sourcePHPPlugin            = "https://claude.com/plugins/php-lsp"
	sourcePyrightPlugin        = "https://claude.com/plugins/pyright-lsp"
	sourceRustAnalyzerPlugin   = "https://claude.com/plugins/rust-analyzer-lsp"
	sourceSentryPlugin         = "https://claude.com/plugins/sentry"
	sourceSupabasePlugin       = "https://claude.com/plugins/supabase"
	sourceTypeScriptPlugin     = "https://claude.com/plugins/typescript-lsp"
	sourceGoplsPlugin          = "https://claude.com/plugins/gopls-lsp"
)

// artifactPrefixToCategory maps the public-artifact prefix space used in
// SourceSummary.KnownEcosystem to the analyzer's signature-registry
// category keys (see analyzer.KnownEcosystemIDs). Keeping a single source
// of truth — the embedded signature registries — prevents drift between
// the analyzer's allowlist and the paid-artifact allowlist.
var artifactPrefixToCategory = map[string]string{
	"agent":           "coding_agent",
	"framework":       "framework",
	"mcp":             "mcp",
	"skill":           "skill",
	"plugin":          "plugin",
	"package_manager": "package_manager",
}

type Options struct {
	ArtifactURL string
	GeneratedAt time.Time
}

type Artifact struct {
	SchemaVersion          string                      `json:"schema_version"`
	Generator              string                      `json:"generator"`
	PluginName             string                      `json:"plugin_name"`
	PluginVersion          string                      `json:"plugin_version"`
	GeneratedAt            time.Time                   `json:"generated_at"`
	Source                 SourceSummary               `json:"source"`
	Customizations         []Customization             `json:"customizations"`
	VettedRecommendations  []ToolRecommendation        `json:"vetted_recommendations"`
	Recommendation         *analyzer.RecommendationSet `json:"recommendation,omitempty"`
	RequiredAcknowledgment string                      `json:"required_acknowledgment"`
	Files                  []File                      `json:"files"`
	Install                Install                     `json:"install"`
}

type SourceSummary struct {
	AnalyzerVersion string   `json:"analyzer_version"`
	ScoreBucket     string   `json:"score_bucket"`
	WasteBucket     string   `json:"waste_bucket"`
	FindingIDs      []string `json:"finding_ids"`
	KnownEcosystem  []string `json:"known_ecosystem"`
}

type Customization struct {
	ID       string   `json:"id"`
	Reason   string   `json:"reason"`
	Evidence Evidence `json:"evidence"`
	Files    []string `json:"files"`
}

type Evidence struct {
	FindingID  string `json:"finding_id,omitempty"`
	Severity   string `json:"severity,omitempty"`
	CostImpact string `json:"cost_impact,omitempty"`
	Count      int    `json:"count,omitempty"`
	TokenShare int    `json:"token_share_pct,omitempty"`
}

type File struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Content string `json:"content"`
}

type ToolRecommendation struct {
	ID                string   `json:"id"`
	Category          string   `json:"category"`
	FailureModes      []string `json:"failure_modes,omitempty"`
	Why               string   `json:"why"`
	InstallCommand    string   `json:"install_command"`
	RequiredBinary    string   `json:"required_binary,omitempty"`
	BinaryInstallHint string   `json:"binary_install_hint,omitempty"`
	Source            string   `json:"source"`
	RiskLevel         string   `json:"risk_level,omitempty"`
	DataMovementRisk  string   `json:"data_movement_risk,omitempty"`
	InstallSurface    string   `json:"install_surface,omitempty"`
	ConflictsWith     []string `json:"conflicts_with,omitempty"`
	AmbiguityWarning  string   `json:"ambiguity_warning,omitempty"`
	VettingNotes      string   `json:"vetting_notes,omitempty"`
}

type Install struct {
	Command          string   `json:"command"`
	ClaudePrompt     string   `json:"claude_prompt"`
	UninstallCommand string   `json:"uninstall_command"`
	Notes            []string `json:"notes"`
}

func Generate(report analyzer.Report, options Options) Artifact {
	generatedAt := options.GeneratedAt.UTC()
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	pluginName := "agent-analyzer-optimization"
	recommendations := toolingRecommendations(report)
	acknowledgment := liabilityAcknowledgment()
	files := baseFiles(report, pluginName, recommendations, acknowledgment)
	customizations := customizationPlan(report)
	files = append(files, customizationFiles(customizations)...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	artifact := Artifact{
		SchemaVersion:          "2026-05-18",
		Generator:              "agent-log-analyzer/remediation@" + GeneratorVersion,
		PluginName:             pluginName,
		PluginVersion:          pluginVersion(report),
		GeneratedAt:            generatedAt,
		Source:                 sourceSummary(report),
		Customizations:         customizations,
		VettedRecommendations:  recommendations,
		RequiredAcknowledgment: acknowledgment,
		Files:                  files,
	}
	artifact.Recommendation = report.Recommendation
	artifact.Install = installInstructions(pluginName, options.ArtifactURL)
	return artifact
}

func baseFiles(report analyzer.Report, pluginName string, recommendations []ToolRecommendation, acknowledgment string) []File {
	manifest := map[string]any{
		"$schema":     "https://json.schemastore.org/claude-code-plugin-manifest.json",
		"name":        pluginName,
		"description": "Deterministic Claude Code codebase-navigation and tooling recommendations generated from an Agent Analyzer report.",
		"version":     pluginVersion(report),
		"author": map[string]string{
			"name": "Agent Analyzer",
		},
		"keywords": []string{"claude-code", "tokens", "context", "profiler", "code-intelligence", "mcp"},
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	files := []File{
		{
			Path:    ".claude-plugin/plugin.json",
			Mode:    "0644",
			Content: string(manifestJSON) + "\n",
		},
		{
			Path:    "README.md",
			Mode:    "0644",
			Content: readme(report),
		},
		{
			Path:    "WAIVER.md",
			Mode:    "0644",
			Content: waiverFile(acknowledgment),
		},
		{
			Path:    "commands/agent-analyzer-status.md",
			Mode:    "0644",
			Content: statusCommand(report),
		},
		{
			Path:    "commands/agent-analyzer-tooling.md",
			Mode:    "0644",
			Content: toolingCommand(recommendations),
		},
		{
			Path:    "skills/codebase-navigation/SKILL.md",
			Mode:    "0644",
			Content: codebaseNavigationSkill(report),
		},
		{
			Path:    "skills/session-hygiene/SKILL.md",
			Mode:    "0644",
			Content: sessionHygieneSkill(report),
		},
		{
			Path:    "skills/tooling-setup/SKILL.md",
			Mode:    "0644",
			Content: toolingSetupSkill(recommendations),
		},
	}
	return files
}

func customizationPlan(report analyzer.Report) []Customization {
	var out []Customization
	byID := map[string]analyzer.Finding{}
	for _, finding := range report.Findings {
		byID[finding.ID] = finding
	}
	if finding, ok := byID["repeated_file_reads"]; ok {
		out = append(out, Customization{
			ID:       "retrieval-hygiene",
			Reason:   "Repeated reads indicate the session is rereading files instead of preserving file state summaries.",
			Evidence: evidenceFor(finding),
			Files:    []string{"skills/retrieval-hygiene/SKILL.md"},
		})
	}
	if finding, ok := byID["tool_output_bloat"]; ok {
		out = append(out, Customization{
			ID:       "output-budget",
			Reason:   "Tool output consumed a high share of estimated context tokens.",
			Evidence: evidenceFor(finding),
			Files:    []string{"skills/output-budget/SKILL.md", "skills/tooling-setup/SKILL.md"},
		})
	}
	if finding, ok := byID["retry_loop"]; ok {
		out = append(out, Customization{
			ID:       "retry-breaker",
			Reason:   "Retry-loop behavior suggests the session needs an explicit stop-and-reframe routine.",
			Evidence: evidenceFor(finding),
			Files:    []string{"skills/retry-breaker/SKILL.md"},
		})
	}
	if finding, ok := byID["context_growth_spikes"]; ok {
		out = append(out, Customization{
			ID:       "context-pivot",
			Reason:   "Context growth spikes indicate task pivots should trigger compaction or session splits.",
			Evidence: evidenceFor(finding),
			Files:    []string{"skills/session-hygiene/SKILL.md"},
		})
	}
	if len(out) == 0 {
		out = append(out, Customization{
			ID:       "baseline-hygiene",
			Reason:   "No high-confidence waste pattern dominated the scan; install the baseline session hygiene workflow.",
			Evidence: Evidence{},
			Files:    []string{"skills/session-hygiene/SKILL.md"},
		})
	}
	return out
}

func toolingRecommendations(report analyzer.Report) []ToolRecommendation {
	seen := map[string]bool{}
	var out []ToolRecommendation
	add := func(rec ToolRecommendation) {
		if rec.ID == "" || seen[rec.ID] {
			return
		}
		rec = enrichToolRecommendation(rec)
		seen[rec.ID] = true
		out = append(out, rec)
	}
	findingIDs := map[string]bool{}
	for _, finding := range report.Findings {
		findingIDs[finding.ID] = true
	}

	add(ToolRecommendation{
		ID:             "ccusage",
		Category:       "metrics_telemetry",
		Why:            "Parse local Claude Code JSONL logs for independent token, cost, and burn-rate visibility before and after optimization.",
		InstallCommand: "npx ccusage@latest",
		Source:         sourceCCUsage,
	})
	add(ToolRecommendation{
		ID:             "awesome-claude-code",
		Category:       "ecosystem_index",
		Why:            "Use as a monitored discovery source for Claude Code skills, hooks, plugins, statuslines, and orchestration tools; do not install from it directly.",
		InstallCommand: "Review https://github.com/hesreallyhim/awesome-claude-code before adding any new third-party tool to the allowlist.",
		Source:         sourceAwesomeClaudeCode,
	})

	if findingIDs["tool_output_bloat"] || findingIDs["context_growth_spikes"] {
		add(ToolRecommendation{
			ID:             "context-mode",
			Category:       "context_defense",
			Why:            "Route large tool outputs through sandboxed processing and summaries instead of flooding Claude's live context.",
			InstallCommand: "/plugin marketplace add mksglu/context-mode\n/plugin install context-mode@context-mode\n/reload-plugins\n/context-mode:ctx-doctor",
			Source:         sourceContextMode,
		})
		add(ToolRecommendation{
			ID:                "rtk",
			Category:          "advanced_shell_compression",
			Why:               "RTK (Rust Token Killer, rtk-ai/rtk) compresses common shell command output before it reaches Claude; useful when terminal output is a dominant waste source.",
			InstallCommand:    "Review https://github.com/rtk-ai/rtk first. If approved on macOS: brew install rtk\nrtk init -g",
			RequiredBinary:    "rtk",
			BinaryInstallHint: "This is github.com/rtk-ai/rtk. Do not install the unrelated npm package named rtk.",
			Source:            sourceRTK,
		})
		add(ToolRecommendation{
			ID:             "claude-token-efficient",
			Category:       "claude_md_optimization",
			Why:            "Reduce accumulated assistant verbosity, but only merge the smallest useful rules because persistent CLAUDE.md text adds input tokens.",
			InstallCommand: "Ask Claude to review https://github.com/drona23/claude-token-efficient and propose a minimal CLAUDE.md diff; do not overwrite existing CLAUDE.md automatically.",
			Source:         sourceClaudeTokenEfficient,
		})
	}

	if findingIDs["repeated_file_reads"] {
		add(ToolRecommendation{
			ID:                "grepai",
			Category:          "local_semantic_retrieval",
			Why:               "Use local semantic code search and call graphs to reduce repeated grep/read loops without sending code to a hosted retrieval service.",
			InstallCommand:    "brew install yoanbernabeu/tap/grepai\ngrepai init\ngrepai watch",
			RequiredBinary:    "grepai",
			BinaryInstallHint: "Requires an embedding provider such as Ollama; install with curl script only after reviewing the GitHub source.",
			Source:            sourceGrepAI,
		})
		add(ToolRecommendation{
			ID:             "claude-context",
			Category:       "semantic_retrieval_mcp",
			Why:            "Add MCP semantic code retrieval for large repositories where brute-force file exploration causes repeated rereads.",
			InstallCommand: "claude mcp add claude-context -e OPENAI_API_KEY=<openai-key> -e MILVUS_ADDRESS=<zilliz-endpoint> -e MILVUS_TOKEN=<zilliz-token> -- npx @zilliz/claude-context-mcp@latest",
			Source:         sourceClaudeContext,
		})
	}

	if findingIDs["retry_loop"] || findingIDs["context_growth_spikes"] {
		add(ToolRecommendation{
			ID:             "claude-code-hooks-mastery",
			Category:       "implementation_reference",
			Why:            "Use as a reference for SessionStart, PostToolUse, PreCompact, Stop, and UserPromptSubmit patterns when building workflow discipline.",
			InstallCommand: "Review https://github.com/disler/claude-code-hooks-mastery before enabling any new hook behavior.",
			Source:         sourceClaudeHooksMastery,
		})
	}

	if findingIDs["tool_output_bloat"] || findingIDs["retry_loop"] || findingIDs["context_growth_spikes"] {
		add(ToolRecommendation{
			ID:                "ccstatusline",
			Category:          "statusline_telemetry",
			Why:               "Expose session state in the statusline so users notice cost, git state, and workflow drift without adding messages to context.",
			InstallCommand:    "Review https://github.com/sirmalloc/ccstatusline and install only if it does not conflict with Context Mode or the user's existing statusline.",
			RequiredBinary:    "ccstatusline",
			BinaryInstallHint: "Prefer the repository's current release/install instructions over copied commands.",
			Source:            sourceCCStatusline,
		})
	}

	add(ToolRecommendation{
		ID:                "claude-code-usage-monitor",
		Category:          "burn_rate_monitoring",
		Why:               "Optional live forecasting for users who care about session limits and burn-rate warnings outside Claude's context.",
		InstallCommand:    "uv tool install claude-monitor\nclaude-monitor",
		RequiredBinary:    "claude-monitor",
		BinaryInstallHint: "Alternative: pip install claude-monitor.",
		Source:            sourceUsageMonitor,
	})

	for _, manager := range report.Ecosystem.PackageManagers {
		switch manager {
		case "bun", "npm", "pnpm", "yarn":
			add(ToolRecommendation{
				ID:                "typescript-lsp",
				Category:          "code_intelligence",
				Why:               "Use symbol navigation and diagnostics instead of repeated grep/read loops in JavaScript and TypeScript projects.",
				InstallCommand:    "/plugin install typescript-lsp@claude-plugins-official",
				RequiredBinary:    "typescript-language-server",
				BinaryInstallHint: "npm install -g typescript typescript-language-server",
				Source:            sourceTypeScriptPlugin,
			})
		case "pip", "poetry", "uv":
			add(ToolRecommendation{
				ID:                "pyright-lsp",
				Category:          "code_intelligence",
				Why:               "Use Python symbol navigation and diagnostics instead of opening many candidate files.",
				InstallCommand:    "/plugin install pyright-lsp@claude-plugins-official",
				RequiredBinary:    "pyright-langserver",
				BinaryInstallHint: "npm install -g pyright",
				Source:            sourcePyrightPlugin,
			})
		case "go":
			add(ToolRecommendation{
				ID:                "gopls-lsp",
				Category:          "code_intelligence",
				Why:               "Use Go definitions, references, and diagnostics before running broad searches or full test suites.",
				InstallCommand:    "/plugin install gopls-lsp@claude-plugins-official",
				RequiredBinary:    "gopls",
				BinaryInstallHint: "go install golang.org/x/tools/gopls@latest",
				Source:            sourceGoplsPlugin,
			})
		case "cargo":
			add(ToolRecommendation{
				ID:                "rust-analyzer-lsp",
				Category:          "code_intelligence",
				Why:               "Use Rust symbol navigation and diagnostics to avoid context-heavy compile/search loops.",
				InstallCommand:    "/plugin install rust-analyzer-lsp@claude-plugins-official",
				RequiredBinary:    "rust-analyzer",
				BinaryInstallHint: "rustup component add rust-analyzer",
				Source:            sourceRustAnalyzerPlugin,
			})
		case "composer":
			add(ToolRecommendation{
				ID:                "php-lsp",
				Category:          "code_intelligence",
				Why:               "Use PHP symbol navigation and diagnostics before broad text search across legacy code.",
				InstallCommand:    "/plugin install php-lsp@claude-plugins-official",
				RequiredBinary:    "intelephense",
				BinaryInstallHint: "npm install -g intelephense",
				Source:            sourcePHPPlugin,
			})
		}
	}

	if report.Ecosystem.VersionControl == "git" || containsString(report.Ecosystem.MCPServersKnown, "github") {
		add(ToolRecommendation{
			ID:             "github",
			Category:       "mcp_integration",
			Why:            "Fetch structured issue and PR context without pasting browser output or long terminal dumps into Claude.",
			InstallCommand: "/plugin install github@claude-plugins-official",
			Source:         sourceGitHubPlugin,
		})
	}
	for _, plugin := range []struct {
		id     string
		why    string
		source string
	}{
		{"notion", "Pull structured project documentation directly instead of repeatedly searching or pasting docs.", sourceNotionPlugin},
		{"linear", "Pull structured ticket context directly instead of copying long issue text into the session.", sourceLinearPlugin},
		{"sentry", "Inspect structured errors and traces instead of dumping logs into context.", sourceSentryPlugin},
		{"supabase", "Use a configured infrastructure integration for project metadata instead of ad hoc shell/API output.", sourceSupabasePlugin},
	} {
		if containsString(report.Ecosystem.MCPServersKnown, plugin.id) || containsString(report.Ecosystem.KnownPlugins, plugin.id) {
			add(ToolRecommendation{
				ID:             plugin.id,
				Category:       "mcp_integration",
				Why:            plugin.why,
				InstallCommand: "/plugin install " + plugin.id + "@claude-plugins-official",
				Source:         plugin.source,
			})
		}
	}

	if len(out) == 0 {
		add(ToolRecommendation{
			ID:             "inspect-language-stack",
			Category:       "manual_review",
			Why:            "No high-confidence language-server recommendation was inferred from the sanitized aggregate report. Inspect package manifests before installing code intelligence.",
			InstallCommand: "Ask Claude to inspect package manifests and recommend only official code-intelligence plugins with matching binaries.",
			Source:         sourceAgentAnalyzerMatrix,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func enrichToolRecommendation(rec ToolRecommendation) ToolRecommendation {
	if tool, ok := analyzer.GetTool(recommendationRegistryID(rec.ID)); ok {
		if rec.RiskLevel == "" {
			rec.RiskLevel = string(tool.InstallRisk)
		}
		if rec.DataMovementRisk == "" {
			rec.DataMovementRisk = string(tool.DataMovementRisk)
		}
		if len(rec.FailureModes) == 0 {
			rec.FailureModes = remediationFailureModes(tool.RecommendationClass)
		}
		if rec.InstallSurface == "" {
			rec.InstallSurface = remediationInstallSurface(tool.ID, tool.Category)
		}
		if len(rec.ConflictsWith) == 0 {
			rec.ConflictsWith = remediationConflicts(tool.ID)
		}
		if rec.AmbiguityWarning == "" && tool.ID == "rtk" {
			rec.AmbiguityWarning = "RTK means github.com/rtk-ai/rtk. Never install the unrelated npm package named rtk."
		}
		if rec.VettingNotes == "" {
			rec.VettingNotes = tool.Notes
		}
		return rec
	}

	if rec.RiskLevel == "" {
		rec.RiskLevel = "medium"
	}
	if rec.DataMovementRisk == "" {
		rec.DataMovementRisk = "medium"
	}
	if rec.InstallSurface == "" {
		rec.InstallSurface = "reviewed_third_party_or_official_plugin"
	}
	if len(rec.FailureModes) == 0 {
		rec.FailureModes = remediationFailureModesForCategory(rec.Category)
	}
	if rec.VettingNotes == "" {
		rec.VettingNotes = "Exact source URL is included; confirm current install instructions before running commands."
	}
	return rec
}

func recommendationRegistryID(id string) analyzer.ToolID {
	switch id {
	case "context-mode":
		return "context_mode"
	case "claude-context":
		return "claude_context"
	case "claude-token-efficient":
		return "claude_token_efficient"
	case "claude-code-usage-monitor":
		return "claude_code_usage_monitor"
	default:
		return analyzer.ToolID(strings.ReplaceAll(id, "-", "_"))
	}
}

func remediationFailureModes(class analyzer.RecommendationClass) []string {
	switch class {
	case analyzer.ClassShellOutputReducer:
		return []string{string(analyzer.FailureNoisyTerminalLogs)}
	case analyzer.ClassMCPOutputReducer:
		return []string{string(analyzer.FailureToolOutputFlooding)}
	case analyzer.ClassRetrieval, analyzer.ClassRereadGuard:
		return []string{string(analyzer.FailureRepeatedNavigation)}
	case analyzer.ClassOutputVerbosity:
		return []string{string(analyzer.FailureBroadReadsOrVerbosity)}
	default:
		return []string{string(analyzer.FailureCrossCutting)}
	}
}

func remediationFailureModesForCategory(category string) []string {
	switch category {
	case "code_intelligence", "local_semantic_retrieval", "semantic_retrieval_mcp":
		return []string{string(analyzer.FailureRepeatedNavigation)}
	case "context_defense", "mcp_integration":
		return []string{string(analyzer.FailureToolOutputFlooding)}
	case "advanced_shell_compression":
		return []string{string(analyzer.FailureNoisyTerminalLogs)}
	case "claude_md_optimization":
		return []string{string(analyzer.FailureBroadReadsOrVerbosity)}
	default:
		return []string{string(analyzer.FailureCrossCutting)}
	}
}

func remediationInstallSurface(id analyzer.ToolID, category string) string {
	switch id {
	case "rtk":
		return "local_binary_plus_claude_hook"
	case "context_mode":
		return "claude_plugin_plus_mcp"
	case "claude_context":
		return "mcp_plus_external_vector_store"
	case "grepai":
		return "local_binary_plus_optional_embedding_provider"
	case "ccusage", "ccstatusline", "claude_token_efficient":
		return "local_cli_or_local_config"
	default:
		if category == "mcp" {
			return "mcp_server"
		}
		return category
	}
}

func remediationConflicts(id analyzer.ToolID) []string {
	switch id {
	case "rtk":
		return []string{"leanctx", "headroom"}
	case "context_mode":
		return []string{"token_optimizer_mcp", "headroom"}
	case "claude_context":
		return []string{"grepai", "serena", "codegraph", "semble"}
	case "grepai":
		return []string{"claude_context", "serena", "codegraph", "semble"}
	case "claude_token_efficient":
		return []string{"caveman"}
	default:
		return nil
	}
}

func customizationFiles(customizations []Customization) []File {
	needed := map[string]bool{}
	for _, customization := range customizations {
		for _, path := range customization.Files {
			needed[path] = true
		}
	}
	var files []File
	if needed["skills/retrieval-hygiene/SKILL.md"] {
		files = append(files, File{
			Path:    "skills/retrieval-hygiene/SKILL.md",
			Mode:    "0644",
			Content: retrievalHygieneSkill(),
		})
	}
	if needed["skills/output-budget/SKILL.md"] {
		files = append(files, File{
			Path:    "skills/output-budget/SKILL.md",
			Mode:    "0644",
			Content: outputBudgetSkill(),
		})
	}
	if needed["skills/retry-breaker/SKILL.md"] {
		files = append(files, File{
			Path:    "skills/retry-breaker/SKILL.md",
			Mode:    "0644",
			Content: retryBreakerSkill(),
		})
	}
	return files
}

func evidenceFor(finding analyzer.Finding) Evidence {
	return Evidence{
		FindingID:  finding.ID,
		Severity:   finding.Severity,
		CostImpact: finding.CostImpact,
		Count:      finding.Evidence.Count,
		TokenShare: finding.Evidence.TokenShare,
	}
}

func sourceSummary(report analyzer.Report) SourceSummary {
	findingIDs := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if safeIdentifier(finding.ID) {
			findingIDs = append(findingIDs, finding.ID)
		}
	}
	sort.Strings(findingIDs)
	ecosystem := safeKnownEcosystem(report.Ecosystem)
	sort.Strings(ecosystem)
	return SourceSummary{
		AnalyzerVersion: report.Version,
		ScoreBucket:     report.AggregateEvent.ScoreBucket,
		WasteBucket:     report.AggregateEvent.WasteBucket,
		FindingIDs:      findingIDs,
		KnownEcosystem:  ecosystem,
	}
}

func safeKnownEcosystem(ecosystem analyzer.Ecosystem) []string {
	seen := map[string]bool{}
	var out []string
	addID := func(token string) {
		if seen[token] {
			return
		}
		seen[token] = true
		out = append(out, token)
	}
	add := func(prefix string, values []string) {
		for _, value := range values {
			if safePublicID(prefix, value) {
				addID(prefix + ":" + value)
			}
		}
	}
	add("agent", ecosystem.CodingAgents)
	add("framework", ecosystem.WorkflowFrameworks)
	add("mcp", ecosystem.MCPServersKnown)
	add("skill", ecosystem.KnownSkills)
	add("plugin", ecosystem.KnownPlugins)
	add("package_manager", ecosystem.PackageManagers)
	for _, value := range []struct {
		prefix string
		raw    string
	}{
		{"client", ecosystem.Client},
		{"os", ecosystem.OperatingSystem},
		{"shell", ecosystem.Shell},
		{"vcs", ecosystem.VersionControl},
	} {
		if safeIdentifier(value.raw) {
			addID(value.prefix + ":" + value.raw)
		}
	}
	// FR-009: surface the merged ToolingUtilization and WorkflowFingerprints
	// IDs through the same allowlisted-prefix space so the paid artifact
	// reflects values from `AggregateReports`. Every token here passes the
	// same publicEcosystemIDs / safeValueRE gate as the existing fields, so
	// the privacy stance (NFR-002) is preserved structurally — unknown names
	// are caught by safePublicID and silently dropped.
	add("mcp", ecosystem.ToolingUtilization.MCP.KnownServerIDs)
	add("mcp", ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs)
	add("skill", ecosystem.ToolingUtilization.Skill.KnownExposedIDs)
	add("skill", ecosystem.ToolingUtilization.Skill.KnownExecutedIDs)
	for _, fp := range ecosystem.WorkflowFingerprints {
		// Fingerprint IDs come from the SDD detector registry
		// (internal/analyzer/sdd/), a separate closed allowlist from the
		// frameworks-signature registry. Gate against the SDD registry —
		// not the framework one — and emit under the existing "framework:"
		// prefix to preserve the public artifact schema.
		if safeIdentifier(fp.ID) && analyzer.ValidEcosystemID("workflow_fingerprint", fp.ID) {
			addID("framework:" + fp.ID)
		}
	}
	return out
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func safePublicID(prefix, value string) bool {
	if !safeIdentifier(value) {
		return false
	}
	category, ok := artifactPrefixToCategory[prefix]
	if !ok {
		return false
	}
	return analyzer.ValidEcosystemID(category, value)
}

func safeIdentifier(value string) bool {
	return value != "" && safeValueRE.MatchString(value)
}

func pluginVersion(report analyzer.Report) string {
	if report.Version == "" {
		return "0.1.0"
	}
	return "0.1.0+" + strings.ReplaceAll(report.Version, ".", "-")
}

func installInstructions(pluginName, artifactURL string) Install {
	if artifactURL == "" {
		artifactURL = "<private-plugin-zip-url>"
	}
	command := strings.Join([]string{
		`PLUGIN_URL="` + artifactURL + `"`,
		`PLUGIN_ZIP="$(mktemp -t agent-analyzer-plugin.XXXXXX.zip)"`,
		`curl -fsS "$PLUGIN_URL" -o "$PLUGIN_ZIP"`,
		`claude --plugin-dir "$PLUGIN_ZIP"`,
	}, "\n")
	prompt := "Install the generated Agent Analyzer optimization plugin for this session. Run the command below, explain what it installs, and ask for approval before executing it. Do not print plugin archive contents.\n\n```sh\n" + command + "\n```"
	return Install{
		Command:          command,
		ClaudePrompt:     prompt,
		UninstallCommand: "No persistent install is performed by the default command. Close the Claude Code session to unload " + pluginName + ".",
		Notes: []string{
			"The default command uses Claude Code's --plugin-dir support with a private-link zip artifact.",
			"Marketplace installation can be added later once the plugin store is live.",
		},
	}
}

func readme(report analyzer.Report) string {
	return fmt.Sprintf(`# Agent Analyzer Optimization Plugin

Generated from deterministic Agent Analyzer metrics.

- Efficiency score bucket: %s
- Waste bucket: %s
- Raw transcript included: no
- Unknown private ecosystem names included: no

Use the included skills and commands to make the codebase easier for Claude Code to navigate: lean CLAUDE.md layers, scoped skills, official code-intelligence plugins, and vetted MCP integrations. This plugin does not nag on Bash commands.
`, report.AggregateEvent.ScoreBucket, report.AggregateEvent.WasteBucket)
}

func waiverFile(acknowledgment string) string {
	return `# Required Acknowledgment

` + acknowledgment + `

This remediation pack may ask Claude Code to recommend or install third-party software, including language servers, Claude Code plugins, and MCP-backed integrations. Those tools can execute code or access local/project data according to their own permissions.

Before installing anything:

1. Review every recommended tool and its source.
2. Confirm the command matches your operating system and package manager.
3. Confirm you have backups or version control for any repository Claude may modify.
4. Approve each installation separately.
5. Stop if Claude proposes an unvetted source, a destructive command, or a credential change you do not understand.

Agent Analyzer is not responsible for damage, data loss, credential exposure, billing impact, or other consequences caused by Claude Code, recommended tools, package managers, language servers, plugins, MCP servers, or user-approved commands.
`
}

func toolingCommand(recommendations []ToolRecommendation) string {
	return `---
description: Review the generated token-saving, code-intelligence, and MCP setup recommendations.
---

# Agent Analyzer Tooling Setup

Read WAIVER.md first. Do not install anything until the user explicitly acknowledges the waiver and approves each command.

Recommended actions:

` + recommendationMarkdown(recommendations) + `

Procedure:

1. Inspect the repository language stack and confirm each recommendation still applies.
2. Prefer official Claude Code marketplace plugins and documented language-server binaries.
3. Ask before installing each binary or plugin.
4. After installing plugins, run /reload-plugins.
5. If any tool source differs from the recommendation, stop and ask the user.
`
}

func codebaseNavigationSkill(report analyzer.Report) string {
	return fmt.Sprintf(`---
description: Use when Claude needs to understand a large or unfamiliar codebase without wasting context.
---

# Codebase Navigation

This skill follows Anthropic's large-codebase guidance: make the codebase navigable before adding more automation.

Rules:

1. Keep root CLAUDE.md lean: architecture map, critical commands, and gotchas only.
2. Prefer subdirectory CLAUDE.md files for local build/test conventions.
3. Start Claude in the relevant subdirectory when the task has a clear scope.
4. Build or update a concise codebase map when top-level folders are not self-explanatory.
5. Prefer LSP/code-intelligence lookups for definitions and references before broad grep/read loops.
6. Use MCP integrations only for structured external context; do not paste large external pages or logs into the session.

Generated score bucket: %s
Generated waste bucket: %s
`, report.AggregateEvent.ScoreBucket, report.AggregateEvent.WasteBucket)
}

func toolingSetupSkill(recommendations []ToolRecommendation) string {
	return `---
description: Use when setting up vetted token-saving tools, language servers, Claude Code plugins, or MCP integrations.
---

# Tooling Setup

Install only with explicit user approval.

` + recommendationMarkdown(recommendations) + `

Installation discipline:

1. Read WAIVER.md to the user in summary form and get explicit acceptance before proceeding.
2. Verify the current OS, package manager, and existing binaries.
3. Install language-server binaries before the matching code-intelligence plugin.
4. Use official Claude Code marketplace plugins where listed.
5. Run /reload-plugins after plugin installation.
6. If a recommended binary is already installed, do not reinstall it.
7. If a repository has custom tooling, prefer its checked-in setup docs over generic install commands.
`
}

func recommendationMarkdown(recommendations []ToolRecommendation) string {
	var b strings.Builder
	for _, rec := range recommendations {
		b.WriteString("- ")
		b.WriteString(rec.ID)
		b.WriteString(" (")
		b.WriteString(rec.Category)
		b.WriteString("): ")
		b.WriteString(rec.Why)
		b.WriteString("\n")
		if len(rec.FailureModes) > 0 {
			b.WriteString("  Failure modes: `")
			b.WriteString(strings.Join(rec.FailureModes, "`, `"))
			b.WriteString("`\n")
		}
		if rec.RiskLevel != "" || rec.DataMovementRisk != "" || rec.InstallSurface != "" {
			b.WriteString("  Risk/surface: install=`")
			b.WriteString(rec.RiskLevel)
			b.WriteString("`, data=`")
			b.WriteString(rec.DataMovementRisk)
			b.WriteString("`, surface=`")
			b.WriteString(rec.InstallSurface)
			b.WriteString("`\n")
		}
		if len(rec.ConflictsWith) > 0 {
			b.WriteString("  Conflicts/overlap: `")
			b.WriteString(strings.Join(rec.ConflictsWith, "`, `"))
			b.WriteString("`; choose one tool for this failure mode unless the user explicitly approves both.\n")
		}
		if rec.AmbiguityWarning != "" {
			b.WriteString("  Ambiguity warning: ")
			b.WriteString(rec.AmbiguityWarning)
			b.WriteString("\n")
		}
		b.WriteString("  Install: `")
		b.WriteString(rec.InstallCommand)
		b.WriteString("`\n")
		if rec.RequiredBinary != "" {
			b.WriteString("  Required binary: `")
			b.WriteString(rec.RequiredBinary)
			b.WriteString("`\n")
		}
		if rec.BinaryInstallHint != "" {
			b.WriteString("  Binary install hint: `")
			b.WriteString(rec.BinaryInstallHint)
			b.WriteString("`\n")
		}
		b.WriteString("  Source: ")
		b.WriteString(rec.Source)
		b.WriteString("\n")
		if rec.VettingNotes != "" {
			b.WriteString("  Vetting notes: ")
			b.WriteString(rec.VettingNotes)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func liabilityAcknowledgment() string {
	return "I understand that Agent Analyzer provides deterministic analysis and vetted setup recommendations, but any installation or code change is executed by Claude Code, my package manager, or third-party tools with my approval and at my own risk."
}

func statusCommand(report analyzer.Report) string {
	findings := strings.Join(sourceSummary(report).FindingIDs, ", ")
	if findings == "" {
		findings = "baseline-hygiene"
	}
	return fmt.Sprintf(`---
description: Show the Agent Analyzer session hygiene summary generated from the paid scan.
---

# Agent Analyzer Status

Report the current workflow hygiene posture in one terse line:

CTX discipline: watch | findings: %s | action: compact after pivots, cap shell output, avoid repeated reads.
`, findings)
}

func sessionHygieneSkill(report analyzer.Report) string {
	return fmt.Sprintf(`---
description: Use when a Claude Code session changes task type, grows context quickly, or needs a compact/clear decision.
---

# Session Hygiene

This plugin was generated from deterministic Agent Analyzer metrics.

Rules:

1. Keep debugging, architecture, and implementation in separate sessions when the task pivots.
2. Suggest /compact before a major subsystem pivot or after a long failed debugging branch.
3. Suggest /clear when the current context is dominated by stale assumptions.
4. Before rereading files, summarize what is already known and state the missing fact.
5. Keep advice short and operational.

Generated score bucket: %s
Generated waste bucket: %s
`, report.AggregateEvent.ScoreBucket, report.AggregateEvent.WasteBucket)
}

func retrievalHygieneSkill() string {
	return `---
description: Use when Claude Code is repeatedly reading the same files or running broad searches.
---

# Retrieval Hygiene

When inspecting code:

1. Prefer rg and targeted symbol/file searches before broad reads.
2. Read the narrowest range that can answer the question.
3. After reading a file, keep a short file-state summary before deciding to reread it.
4. If the same file appears again, state what new fact is needed before reading.
5. Avoid dumping entire files unless the file is small and central to the task.
`
}

func outputBudgetSkill() string {
	return `---
description: Use when shell, test, grep, build, or tool output may become large.
---

# Output Budget

Before running commands likely to print large output:

1. Prefer quiet flags, focused tests, and specific paths.
2. Pipe noisy commands through tail, head, rg, jq, or sed -n with a clear bound.
3. Capture full logs to a file only when needed, then inspect focused excerpts.
4. Never paste unbounded command output into context.
5. For repeated failing tests, stop after the second similar failure and inspect the invariant.
`
}

func retryBreakerSkill() string {
	return `---
description: Use after repeated failed commands, failed edits, or repeated attempts that do not change the failure mode.
---

# Retry Breaker

When the same failure repeats:

1. Stop after two similar failures.
2. State the invariant: what did not change across attempts.
3. Reduce the scope to the smallest failing command or file.
4. Re-read only the evidence needed for the next hypothesis.
5. Ask whether to compact or split the task if the session has drifted.
`
}
