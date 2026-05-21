package remediation

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
)

func TestGenerateCreatesClaudePluginArtifact(t *testing.T) {
	report := analyzer.Report{
		Version: "0.1.0",
		Score:   42,
		EstimatedWaste: analyzer.WasteRange{
			Low:  29,
			High: 41,
		},
		AggregateEvent: analyzer.AggregateSafeEvent{
			ScoreBucket: "40_60",
			WasteBucket: "40_60",
		},
		Findings: []analyzer.Finding{
			{
				ID:         "repeated_file_reads",
				Severity:   "high",
				CostImpact: "medium-high",
				Evidence: analyzer.FindingEvidence{
					Count:    17,
					TopFiles: []string{"auth.ts", "routes.py"},
				},
			},
			{
				ID:         "tool_output_bloat",
				Severity:   "high",
				CostImpact: "high",
				Evidence: analyzer.FindingEvidence{
					TokenShare: 63,
				},
			},
			{
				ID:         "retry_loop",
				Severity:   "medium",
				CostImpact: "medium",
				Evidence: analyzer.FindingEvidence{
					Count: 4,
				},
			},
		},
		Ecosystem: analyzer.Ecosystem{
			Client:             "claude_code",
			OperatingSystem:    "macos",
			Shell:              "zsh",
			WorkflowFrameworks: []string{"spec_kitty", "bmad"},
			MCPServersKnown:    []string{"github"},
			KnownPlugins:       []string{"notion"},
			KnownSkills:        []string{"qa"},
			PackageManagers:    []string{"pnpm"},
			VersionControl:     "git",
		},
	}

	artifact := Generate(report, Options{
		ArtifactURL: "https://example.test/plugin.zip",
		GeneratedAt: time.Date(2026, 5, 18, 14, 0, 0, 0, time.UTC),
	})

	if artifact.PluginName != "agent-analyzer-optimization" {
		t.Fatalf("unexpected plugin name: %s", artifact.PluginName)
	}
	for _, path := range []string{
		".claude-plugin/plugin.json",
		"README.md",
		"WAIVER.md",
		"skills/session-hygiene/SKILL.md",
		"skills/codebase-navigation/SKILL.md",
		"skills/tooling-setup/SKILL.md",
		"skills/retrieval-hygiene/SKILL.md",
		"skills/output-budget/SKILL.md",
		"skills/retry-breaker/SKILL.md",
		"commands/agent-analyzer-status.md",
		"commands/agent-analyzer-tooling.md",
	} {
		if !hasFile(artifact, path) {
			t.Fatalf("expected artifact file %s in %#v", path, artifact.Files)
		}
	}
	for _, path := range []string{"hooks/hooks.json", "scripts/agent-analyzer-hook.py"} {
		if hasFile(artifact, path) {
			t.Fatalf("did not expect bash nag hook file %s in %#v", path, artifact.Files)
		}
	}
	if !strings.Contains(artifact.Install.Command, "claude --plugin-dir") {
		t.Fatalf("expected plugin-dir install command, got %s", artifact.Install.Command)
	}
	if !containsCustomization(artifact, "retrieval-hygiene") || !containsCustomization(artifact, "output-budget") || !containsCustomization(artifact, "retry-breaker") {
		t.Fatalf("missing expected customizations: %#v", artifact.Customizations)
	}
	for _, want := range []string{"ccusage", "context-mode", "grepai", "claude-context", "typescript-lsp", "github"} {
		if !containsRecommendation(artifact, want) {
			t.Fatalf("missing vetted recommendation %s: %#v", want, artifact.VettedRecommendations)
		}
	}
	for _, recommendation := range artifact.VettedRecommendations {
		if !isHTTPSURL(recommendation.Source) {
			t.Fatalf("recommendation %s has non-URL source %q", recommendation.ID, recommendation.Source)
		}
		if recommendation.RiskLevel == "" || recommendation.DataMovementRisk == "" || recommendation.InstallSurface == "" {
			t.Fatalf("recommendation %s missing risk/surface metadata: %#v", recommendation.ID, recommendation)
		}
		if len(recommendation.FailureModes) == 0 {
			t.Fatalf("recommendation %s missing failure mode metadata: %#v", recommendation.ID, recommendation)
		}
	}
	if !strings.Contains(artifact.RequiredAcknowledgment, "at my own risk") {
		t.Fatalf("expected liability acknowledgment, got %q", artifact.RequiredAcknowledgment)
	}
}

func TestToolRecommendationsUsePreciseSources(t *testing.T) {
	report := analyzer.Report{
		Findings: []analyzer.Finding{
			{ID: "tool_output_bloat"},
			{ID: "repeated_file_reads"},
			{ID: "retry_loop"},
		},
		Ecosystem: analyzer.Ecosystem{
			PackageManagers: []string{"npm", "pip", "go", "cargo", "composer"},
			VersionControl:  "git",
			MCPServersKnown: []string{"notion", "linear", "sentry", "supabase"},
		},
	}

	recommendations := toolingRecommendations(report)
	if len(recommendations) == 0 {
		t.Fatal("expected recommendations")
	}

	for _, recommendation := range recommendations {
		if !isHTTPSURL(recommendation.Source) {
			t.Errorf("recommendation %s source = %q, want precise HTTPS URL", recommendation.ID, recommendation.Source)
		}
		if strings.Contains(recommendation.Source, "official marketplace") {
			t.Errorf("recommendation %s still uses generic source text: %q", recommendation.ID, recommendation.Source)
		}
	}

	rtk := recommendationByID(recommendations, "rtk")
	if rtk == nil {
		t.Fatal("expected RTK recommendation for tool_output_bloat")
	}
	if rtk.Source != sourceRTK {
		t.Fatalf("rtk source = %q, want %q", rtk.Source, sourceRTK)
	}
	if !strings.Contains(rtk.Why, "rtk-ai/rtk") || !strings.Contains(rtk.BinaryInstallHint, "unrelated npm package named rtk") {
		t.Fatalf("RTK recommendation must disambiguate from npm rtk: %#v", *rtk)
	}
	if !strings.Contains(rtk.AmbiguityWarning, "unrelated npm package named rtk") {
		t.Fatalf("RTK recommendation must carry an ambiguity warning: %#v", *rtk)
	}
	if len(rtk.ConflictsWith) == 0 {
		t.Fatalf("RTK recommendation must include conflict metadata: %#v", *rtk)
	}
}

func TestToolRecommendationsRespectEngineSelectionAndSkipNotes(t *testing.T) {
	report := analyzer.Report{
		Findings: []analyzer.Finding{
			{ID: "repeated_file_reads"},
		},
		Recommendation: &analyzer.RecommendationSet{
			Primary: &analyzer.TokenSavingRecommendation{
				PrimaryToolID: "claude_context",
				FailureModes:  []analyzer.DefocusFailureMode{analyzer.FailureRepeatedNavigation},
				Reason:        analyzer.ReasonAbsent,
				SkippedToolIDs: []analyzer.ToolID{
					"grepai",
				},
			},
		},
	}

	recommendations := toolingRecommendations(report)
	if recommendationByID(recommendations, "claude-context") == nil {
		t.Fatalf("expected claude-context to be selected")
	}
	if recommendationByID(recommendations, "grepai") != nil {
		t.Fatalf("expected grepai to be suppressed by engine selection")
	}
	primary := recommendationByID(recommendations, "claude-context")
	if primary == nil || primary.SelectionReason == "" {
		t.Fatalf("expected selection reason on selected recommendation: %#v", primary)
	}
	md := recommendationMarkdown(report, recommendations)
	if !strings.Contains(md, "`grepai` skipped as an alternative") {
		t.Fatalf("expected markdown to explain skipped tool: %s", md)
	}
}

func TestToolRecommendationsDeferNewMCPPluginsOnLowUtilization(t *testing.T) {
	report := analyzer.Report{
		Ecosystem: analyzer.Ecosystem{
			VersionControl: "git",
			KnownPlugins:   []string{"notion"},
			ToolingUtilization: analyzer.ToolingUtilization{
				MCP: analyzer.MCPUtilization{
					UtilizationRatioPct: 10,
					WarningBand:         "watch",
				},
				Skill: analyzer.SkillUtilization{
					UtilizationRatioPct: 15,
					WarningBand:         "watch",
				},
			},
		},
	}

	recommendations := toolingRecommendations(report)
	for _, rec := range recommendations {
		if isPluginOrMCPRecommendation(rec) {
			t.Fatalf("expected plugin/mcp recommendation to be deferred on low utilization: %#v", rec)
		}
	}
	md := recommendationMarkdown(report, recommendations)
	if !strings.Contains(md, "No new MCP/plugin additions while utilization is low") {
		t.Fatalf("expected low-utilization defer note in markdown: %s", md)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	report := analyzer.Report{
		Version: "0.1.0",
		AggregateEvent: analyzer.AggregateSafeEvent{
			ScoreBucket: "60_80",
			WasteBucket: "20_40",
		},
		Findings: []analyzer.Finding{
			{ID: "retry_loop", Severity: "medium", CostImpact: "medium", Evidence: analyzer.FindingEvidence{Count: 3}},
			{ID: "repeated_file_reads", Severity: "high", CostImpact: "medium-high", Evidence: analyzer.FindingEvidence{Count: 10}},
		},
	}
	options := Options{GeneratedAt: time.Date(2026, 5, 18, 15, 0, 0, 0, time.UTC)}

	first := mustJSON(t, Generate(report, options))
	second := mustJSON(t, Generate(report, options))
	if first != second {
		t.Fatalf("expected deterministic artifact\nfirst=%s\nsecond=%s", first, second)
	}
}

func TestGenerateDoesNotLeakPrivateNamesOrSecrets(t *testing.T) {
	report := analyzer.Report{
		Version: "0.1.0",
		AggregateEvent: analyzer.AggregateSafeEvent{
			ScoreBucket: "0_20",
			WasteBucket: "40_60",
		},
		Findings: []analyzer.Finding{
			{
				ID:         "tool_output_bloat",
				Severity:   "high",
				CostImpact: "high",
				Evidence: analyzer.FindingEvidence{
					TokenShare: 72,
					TopFiles:   []string{"/Users/alice/private/repo/.env"},
				},
			},
		},
		Ecosystem: analyzer.Ecosystem{
			MCPServersKnown:       []string{"github"},
			UnknownMCPServerCount: 1,
			KnownPlugins:          []string{"private_company_plugin", "notion"},
			UnknownPluginCount:    1,
			KnownSkills:           []string{"qa", "customer_secret_skill"},
		},
		Redactions: map[string]int{"anthropic_key": 1},
	}

	body := mustJSON(t, Generate(report, Options{GeneratedAt: time.Date(2026, 5, 18, 16, 0, 0, 0, time.UTC)}))
	for _, forbidden := range []string{
		"sk-ant-",
		"/Users/alice",
		"private/repo",
		"private_company",
		"customer_secret",
		".env",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("artifact leaked %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "plugin:notion") || !strings.Contains(body, "mcp:github") || !strings.Contains(body, "skill:qa") {
		t.Fatalf("expected known safe ecosystem IDs in artifact: %s", body)
	}
}

// TestGenerate_MergedAggregate_FlowsToArtifact locks FR-009: the paid plugin
// artifact must consume merged ToolingUtilization and WorkflowFingerprints
// values produced by analyzer.AggregateReports, not pre-merge values from any
// single input report.
//
// Strategy: build two synthetic input reports whose ecosystems each carry
// distinct allowlisted IDs in ToolingUtilization.MCP.KnownServerIDs and
// WorkflowFingerprints. Aggregate them, generate the artifact from the merged
// report, and assert the artifact's KnownEcosystem (and the same data surfaced
// through the artifact's JSON) reflects the union of both inputs — not just
// one of them.
func TestGenerate_MergedAggregate_FlowsToArtifact(t *testing.T) {
	// Report A: github MCP server, spec_kitty framework fingerprint, call
	// count 5.
	reportA := analyzer.Report{
		Version: "0.1.0",
		Ecosystem: analyzer.Ecosystem{
			ToolingUtilization: analyzer.ToolingUtilization{
				MCP: analyzer.MCPUtilization{
					KnownServerIDs:       []string{"github"},
					UniqueKnownCalledIDs: []string{"github"},
					CallCount:            5,
					KnownCallCount:       5,
					WarningBand:          analyzer.WarningBandNormal,
				},
				Skill: analyzer.SkillUtilization{
					KnownExposedIDs:  []string{"qa"},
					KnownExecutedIDs: []string{"qa"},
					ExecutedCount:    1,
					WarningBand:      analyzer.WarningBandNormal,
				},
			},
			WorkflowFingerprints: []analyzer.EcosystemFingerprint{
				{
					ID:            "spec_kitty",
					Confidence:    "high",
					Sources:       []string{"binary_present"},
					EvidenceCount: 3,
					Installed:     true,
				},
			},
		},
	}
	// Report B: linear MCP server, openspec framework fingerprint,
	// call count 7. No overlap on the unique IDs with A — so the merged
	// artifact must include BOTH or the test fails (proving the artifact
	// reads from the merged result, not from a single input).
	reportB := analyzer.Report{
		Version: "0.1.0",
		Ecosystem: analyzer.Ecosystem{
			ToolingUtilization: analyzer.ToolingUtilization{
				MCP: analyzer.MCPUtilization{
					KnownServerIDs:       []string{"linear"},
					UniqueKnownCalledIDs: []string{"linear"},
					CallCount:            7,
					KnownCallCount:       7,
					WarningBand:          analyzer.WarningBandHigh,
				},
				Skill: analyzer.SkillUtilization{
					KnownExposedIDs:  []string{"review"},
					KnownExecutedIDs: []string{"review"},
					ExecutedCount:    2,
					WarningBand:      analyzer.WarningBandWatch,
				},
			},
			WorkflowFingerprints: []analyzer.EcosystemFingerprint{
				{
					ID:            "openspec",
					Confidence:    "medium",
					Sources:       []string{"transcript_command"},
					EvidenceCount: 4,
					Active:        true,
				},
			},
		},
	}

	merged, err := analyzer.AggregateReports("artifact-merge-test", []analyzer.Report{reportA, reportB}, 2048)
	if err != nil {
		t.Fatalf("AggregateReports: %v", err)
	}
	// Sanity-check that the merge populated the fields (otherwise the
	// downstream artifact assertion below would silently degrade to checking
	// nothing useful).
	if merged.Ecosystem.ToolingUtilization.MCP.CallCount != 12 {
		t.Fatalf("merge precondition failed: CallCount=%d want 12 (5+7)",
			merged.Ecosystem.ToolingUtilization.MCP.CallCount)
	}
	if len(merged.Ecosystem.WorkflowFingerprints) != 2 {
		t.Fatalf("merge precondition failed: WorkflowFingerprints len=%d want 2 (%v)",
			len(merged.Ecosystem.WorkflowFingerprints), merged.Ecosystem.WorkflowFingerprints)
	}

	// Generate the paid artifact from the merged report.
	artifact := Generate(merged, Options{
		GeneratedAt: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC),
	})
	known := artifact.Source.KnownEcosystem
	knownSet := map[string]bool{}
	for _, id := range known {
		knownSet[id] = true
	}

	// FR-009 assertions: the artifact's KnownEcosystem must include the
	// merged-ID tokens from BOTH input reports. If safeKnownEcosystem
	// dropped ToolingUtilization or WorkflowFingerprints, these fail.
	wantTokens := []string{
		"mcp:github",           // from A.ToolingUtilization.MCP.KnownServerIDs
		"mcp:linear",           // from B.ToolingUtilization.MCP.KnownServerIDs
		"skill:qa",             // from A.ToolingUtilization.Skill.KnownExposedIDs
		"skill:review",         // from B.ToolingUtilization.Skill.KnownExposedIDs
		"framework:spec_kitty", // from A.WorkflowFingerprints
		"framework:openspec",   // from B.WorkflowFingerprints
	}
	for _, tok := range wantTokens {
		if !knownSet[tok] {
			t.Errorf("FR-009: artifact KnownEcosystem missing merged token %q (got %v)", tok, known)
		}
	}

	// Whole-artifact bytes assertion: serialize the entire artifact and
	// confirm both fingerprint IDs and both MCP IDs are reachable in
	// content the consumer reads (not just the Source struct).
	body, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	for _, tok := range wantTokens {
		if !strings.Contains(string(body), tok) {
			t.Errorf("FR-009: artifact JSON missing merged token %q (artifact does not consume merged data)", tok)
		}
	}
}

// TestGenerate_KnownEcosystem_PreservesAllSDDFingerprints regression-locks
// the bug where a stale, hand-maintained framework allowlist in the
// remediation package silently dropped WorkflowFingerprint IDs that existed
// only in the SDD detector registry (github_spec_kit, kiro, gsd, and later
// additions) and not in the frameworks-signature registry.
//
// The test deliberately derives its fixture from the analyzer registry. That
// makes future SDD detector additions fail here automatically if paid artifact
// export drifts away from the single source of truth.
func TestGenerate_KnownEcosystem_PreservesAllSDDFingerprints(t *testing.T) {
	sddIDs := analyzer.KnownEcosystemIDs("workflow_fingerprint")
	if len(sddIDs) == 0 {
		t.Fatalf("workflow_fingerprint registry is empty")
	}
	fps := make([]analyzer.EcosystemFingerprint, 0, len(sddIDs))
	for id := range sddIDs {
		fps = append(fps, analyzer.EcosystemFingerprint{
			ID:         id,
			Confidence: "high",
			Sources:    []string{"cli_binary"},
			Installed:  true,
		})
	}
	report := analyzer.Report{
		Version:   "0.1.0",
		Ecosystem: analyzer.Ecosystem{WorkflowFingerprints: fps},
	}
	artifact := Generate(report, Options{
		GeneratedAt: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC),
	})
	known := map[string]bool{}
	for _, tok := range artifact.Source.KnownEcosystem {
		known[tok] = true
	}
	for id := range sddIDs {
		tok := "framework:" + id
		if !known[tok] {
			t.Errorf("KnownEcosystem dropped SDD fingerprint %q (got %v)", tok, artifact.Source.KnownEcosystem)
		}
	}
}

// TestGenerateAttachesRecommendation locks the WP02 carry-through: the paid
// artifact must surface report.Recommendation verbatim under the
// `recommendation` JSON key, and the field must be omitted when the report
// has no recommendation (omitempty contract).
func TestGenerateAttachesRecommendation(t *testing.T) {
	t.Run("populated round-trips and preserves VettedRecommendations", func(t *testing.T) {
		recSet := &analyzer.RecommendationSet{
			Primary: &analyzer.TokenSavingRecommendation{
				RecommendationID: "ccusage::no_usage_visibility",
				PrimaryToolID:    "ccusage",
				Reason:           "absent",
				SignalIDs:        []analyzer.Signal{analyzer.SignalNoUsageVisibility},
				Confidence:       "high",
				RiskLevel:        "low",
				InstallPolicy:    "recommend",
				EvidenceCounts:   map[analyzer.EvidenceSource]int{},
			},
			RegistryVersion: "v0-test",
			EngineVersion:   analyzer.EngineVersion(),
			Signals:         []analyzer.Signal{analyzer.SignalNoUsageVisibility},
		}
		report := analyzer.Report{
			Version: "0.1.0",
			AggregateEvent: analyzer.AggregateSafeEvent{
				ScoreBucket: "40_60",
				WasteBucket: "40_60",
			},
			Findings: []analyzer.Finding{
				{
					ID:         "repeated_file_reads",
					Severity:   "high",
					CostImpact: "medium-high",
					Evidence:   analyzer.FindingEvidence{Count: 5},
				},
			},
			Recommendation: recSet,
		}

		artifact := Generate(report, Options{
			GeneratedAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		})

		// Pointer-assignment contract: artifact carries the same pointer.
		if artifact.Recommendation != report.Recommendation {
			t.Fatalf("expected artifact.Recommendation == report.Recommendation (pointer alias), got %p vs %p",
				artifact.Recommendation, report.Recommendation)
		}

		body, err := json.Marshal(artifact)
		if err != nil {
			t.Fatalf("marshal artifact: %v", err)
		}
		if !strings.Contains(string(body), `"recommendation":`) {
			t.Fatalf("expected JSON to include \"recommendation\" key, got %s", body)
		}

		var roundTripped Artifact
		if err := json.Unmarshal(body, &roundTripped); err != nil {
			t.Fatalf("unmarshal artifact: %v", err)
		}
		if roundTripped.Recommendation == nil {
			t.Fatalf("expected round-tripped Recommendation to be non-nil")
		}
		if roundTripped.Recommendation.Primary == nil {
			t.Fatalf("expected round-tripped Recommendation.Primary to be non-nil")
		}
		if got, want := roundTripped.Recommendation.Primary.PrimaryToolID, recSet.Primary.PrimaryToolID; got != want {
			t.Errorf("Primary.PrimaryToolID: got %q want %q", got, want)
		}

		// C-004 regression guard: VettedRecommendations stays populated.
		if len(artifact.VettedRecommendations) == 0 {
			t.Errorf("expected VettedRecommendations to remain populated alongside Recommendation")
		}
	})

	t.Run("nil recommendation omits the key", func(t *testing.T) {
		report := analyzer.Report{
			Version: "0.1.0",
			AggregateEvent: analyzer.AggregateSafeEvent{
				ScoreBucket: "60_80",
				WasteBucket: "20_40",
			},
			// Recommendation deliberately left nil.
		}
		artifact := Generate(report, Options{
			GeneratedAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		})
		if artifact.Recommendation != nil {
			t.Fatalf("expected artifact.Recommendation to be nil when report.Recommendation is nil, got %#v", artifact.Recommendation)
		}
		body, err := json.Marshal(artifact)
		if err != nil {
			t.Fatalf("marshal artifact: %v", err)
		}
		if strings.Contains(string(body), `"recommendation":`) {
			t.Fatalf("omitempty contract violated: JSON contains \"recommendation\" key when input was nil: %s", body)
		}
	})
}

func hasFile(artifact Artifact, path string) bool {
	for _, file := range artifact.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func containsCustomization(artifact Artifact, id string) bool {
	for _, customization := range artifact.Customizations {
		if customization.ID == id {
			return true
		}
	}
	return false
}

func containsRecommendation(artifact Artifact, id string) bool {
	for _, recommendation := range artifact.VettedRecommendations {
		if recommendation.ID == id {
			return true
		}
	}
	return false
}

func recommendationByID(recommendations []ToolRecommendation, id string) *ToolRecommendation {
	for i := range recommendations {
		if recommendations[i].ID == id {
			return &recommendations[i]
		}
	}
	return nil
}

func isHTTPSURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
