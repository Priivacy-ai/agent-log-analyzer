package analytics

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
)

func TestFromReportFiltersPrivateAndHighCardinalityFields(t *testing.T) {
	report := analyzer.Report{
		JobID:   "job-private-session-id",
		Version: "0.1.0",
		Score:   42,
		EstimatedWaste: analyzer.WasteRange{
			High: 35,
		},
		Metrics: analyzer.Metrics{Turns: 88, SessionCount: 3},
		Findings: []analyzer.Finding{
			{
				ID:       "repeated_file_reads",
				Severity: "high",
				Evidence: analyzer.FindingEvidence{
					TopFiles:    []string{"/Users/alice/private/repo/auth.go"},
					Description: "raw prompt text and tool output must not cross",
				},
			},
			{ID: "private_finding_from_prompt", Severity: "private_severity"},
		},
		Redactions: map[string]int{
			"aws_access_key":        2,
			"private_secret_family": 99,
		},
		SourceReports: []analyzer.SourceReport{
			{
				SourceID: "codex",
				LogRefs: []analyzer.AnalyzedLogRef{
					{
						SizeBucket:        "10-100 KB",
						ContentHashSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
					},
					{
						ContentHashSHA256: "not-a-valid-hash-private_thing",
					},
				},
			},
			{
				SourceID: "private_agent",
				LogRefs: []analyzer.AnalyzedLogRef{
					{
						SizeBucket:        "not-a-real-bucket",
						ContentHashSHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
					},
				},
			},
		},
		Ecosystem: analyzer.Ecosystem{
			Client:                "claude_code",
			CodingAgents:          []string{"claude_code", "private_agent"},
			OperatingSystem:       "macos",
			Shell:                 "zsh",
			VersionControl:        "git",
			WorkflowFrameworks:    []string{"spec_kitty", "private_framework"},
			MCPServersKnown:       []string{"github", "private_mcp_acme"},
			UnknownMCPServerCount: 7,
			KnownSkills:           []string{"qa", "private_skill_acme"},
			UnknownSkillCount:     5,
			KnownPlugins:          []string{"notion", "private_plugin_acme"},
			UnknownPluginCount:    3,
			PackageManagers:       []string{"go", "private_pm"},
			WorkflowFingerprints: []analyzer.EcosystemFingerprint{
				{
					ID:            "spec_kitty",
					Confidence:    "high",
					Sources:       []string{"config_dir", "private_source"},
					EvidenceCount: 4,
					Active:        true,
					Installed:     true,
					VersionBucket: "1.2",
				},
				{ID: "private_workflow_tool", Confidence: "high", Sources: []string{"config_dir"}},
			},
			ToolingUtilization: analyzer.ToolingUtilization{
				MCP: analyzer.MCPUtilization{
					KnownServerIDs:           []string{"github", "private_mcp_acme"},
					UnknownServerCount:       7,
					ServerCountBucket:        "26-50",
					ExposedToolCountBucket:   "26-50",
					ContextTokenBucket:       "1k-5k",
					ExposureKnown:            true,
					InferenceSource:          "header",
					CallCount:                11,
					KnownCallCount:           2,
					UnknownCallCount:         9,
					UniqueKnownCalledIDs:     []string{"github", "private_mcp_acme"},
					UniqueUnknownCalledCount: 2,
					UtilizationRatioPct:      6,
					ContextEfficiencyBucket:  "underutilized",
					WarningBand:              "severe",
				},
				Skill: analyzer.SkillUtilization{
					KnownExposedIDs:         []string{"qa", "private_skill_acme"},
					UnknownExposedCount:     5,
					ExposedCountBucket:      "11-25",
					ContextTokenBucket:      "<1k",
					ExposureKnown:           true,
					InferenceSource:         "calls",
					ExecutedCount:           1,
					KnownExecutedIDs:        []string{"qa", "private_skill_acme"},
					UnknownExecutedCount:    1,
					UtilizationRatioPct:     10,
					ContextEfficiencyBucket: "moderate",
					WarningBand:             "high",
				},
			},
		},
		Recommendation: &analyzer.RecommendationSet{
			Primary: &analyzer.TokenSavingRecommendation{
				PrimaryToolID:  "ccusage",
				Reason:         analyzer.ReasonAbsent,
				SignalIDs:      []analyzer.Signal{analyzer.SignalNoUsageVisibility},
				RiskLevel:      analyzer.RiskLow,
				InstallPolicy:  analyzer.PolicyRecommend,
				EvidenceCounts: map[analyzer.EvidenceSource]int{analyzer.EvidenceReportMention: 1},
			},
			Secondary: &analyzer.TokenSavingRecommendation{
				PrimaryToolID:  "private_tool_acme",
				Reason:         analyzer.ReasonAbsent,
				SignalIDs:      []analyzer.Signal{analyzer.SignalRepeatedFileReads},
				RiskLevel:      analyzer.RiskLow,
				InstallPolicy:  analyzer.PolicyRecommend,
				EvidenceCounts: map[analyzer.EvidenceSource]int{analyzer.EvidenceReportMention: 1},
			},
			Signals: []analyzer.Signal{analyzer.SignalNoUsageVisibility, analyzer.SignalRepeatedFileReads},
		},
	}
	report.AggregateEvent = analyzer.AggregateSafeEvent{
		ParserType:      "jsonl",
		InputSizeBucket: "malicious_bucket_/Users/alice",
		TurnBucket:      "malicious_bucket_turn",
		ScoreBucket:     "malicious_bucket_score",
		WasteBucket:     "malicious_bucket_waste",
	}

	event := FromReport(report, "paid_bundle")
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	body := string(data)
	for _, forbidden := range []string{
		"job-private-session-id",
		"/Users/alice",
		"private_",
		"raw prompt text",
		"tool output",
		"private_agent",
		"private_mcp_acme",
		"private_skill_acme",
		"private_plugin_acme",
		"private_tool_acme",
		"private_source",
		"private_secret_family",
		"malicious_bucket",
		`"1.2"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("analytics event leaked forbidden substring %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"spec_kitty"`) {
		t.Fatalf("expected allowlisted SDD fingerprint to remain: %s", body)
	}
	if !strings.Contains(body, `"github"`) || !strings.Contains(body, `"qa"`) {
		t.Fatalf("expected allowlisted MCP/skill IDs to remain: %s", body)
	}
	if strings.Contains(body, `"job_id"`) {
		t.Fatalf("analytics event must not include job_id: %s", body)
	}
	if !strings.Contains(body, `"1.x"`) {
		t.Fatalf("expected coarse major version bucket to remain: %s", body)
	}
	if !strings.Contains(body, `"analyzed_log_hashes"`) || !strings.Contains(body, `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`) {
		t.Fatalf("expected valid analyzed log hash to remain: %s", body)
	}
	if strings.Contains(body, "not-a-valid-hash") || strings.Contains(body, "not-a-real-bucket") {
		t.Fatalf("analytics event leaked invalid hash metadata: %s", body)
	}
}

func TestFromReportIncludesDeterministicAnalyzedLogHashes(t *testing.T) {
	report := analyzer.Report{
		Version:        "0.1.0",
		Metrics:        analyzer.Metrics{Turns: 20},
		EstimatedWaste: analyzer.WasteRange{High: 15},
		SourceReports: []analyzer.SourceReport{
			{
				SourceID: "codex",
				LogRefs: []analyzer.AnalyzedLogRef{
					{SizeBucket: "10-100 KB", ContentHashSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
					{SizeBucket: "10-100 KB", ContentHashSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
					{SizeBucket: "<10 KB", ContentHashSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				},
			},
			{
				SourceID: "claude_code",
				LogRefs: []analyzer.AnalyzedLogRef{
					{SizeBucket: ">5 MB", ContentHashSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
				},
			},
		},
	}

	event := FromReport(report, "single")
	if len(event.AnalyzedLogHashes) != 3 {
		t.Fatalf("expected three unique hashes, got %#v", event.AnalyzedLogHashes)
	}
	got := []string{
		event.AnalyzedLogHashes[0].SourceID + ":" + event.AnalyzedLogHashes[0].ContentHashSHA256 + ":" + event.AnalyzedLogHashes[0].SizeBucket,
		event.AnalyzedLogHashes[1].SourceID + ":" + event.AnalyzedLogHashes[1].ContentHashSHA256 + ":" + event.AnalyzedLogHashes[1].SizeBucket,
		event.AnalyzedLogHashes[2].SourceID + ":" + event.AnalyzedLogHashes[2].ContentHashSHA256 + ":" + event.AnalyzedLogHashes[2].SizeBucket,
	}
	want := []string{
		"claude_code:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc:>5 MB",
		"codex:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:<10 KB",
		"codex:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb:10-100 KB",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected hash order at %d: got %q want %q; all=%#v", i, got[i], want[i], event.AnalyzedLogHashes)
		}
	}
}

func TestFromReportDeterministicJSON(t *testing.T) {
	report := analyzer.Report{
		Version: "0.1.0",
		Metrics: analyzer.Metrics{Turns: 20},
		Ecosystem: analyzer.Ecosystem{
			CodingAgents:       []string{"codex", "claude_code"},
			WorkflowFrameworks: []string{"openspec", "spec_kitty"},
			MCPServersKnown:    []string{"linear", "github"},
			KnownSkills:        []string{"review", "qa"},
			PackageManagers:    []string{"pnpm", "go"},
		},
	}
	first, err := json.Marshal(FromReport(report, "single"))
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	for i := 0; i < 100; i++ {
		next, err := json.Marshal(FromReport(report, "single"))
		if err != nil {
			t.Fatalf("marshal iter %d: %v", i, err)
		}
		if string(next) != string(first) {
			t.Fatalf("non-deterministic event JSON:\nfirst=%s\nnext=%s", first, next)
		}
	}
}
