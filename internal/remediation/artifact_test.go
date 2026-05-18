package remediation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
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

	if artifact.PluginName != "claude-analyzer-optimization" {
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
		"commands/claude-analyzer-status.md",
		"commands/claude-analyzer-tooling.md",
	} {
		if !hasFile(artifact, path) {
			t.Fatalf("expected artifact file %s in %#v", path, artifact.Files)
		}
	}
	for _, path := range []string{"hooks/hooks.json", "scripts/claude-analyzer-hook.py"} {
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
	if !strings.Contains(artifact.RequiredAcknowledgment, "at my own risk") {
		t.Fatalf("expected liability acknowledgment, got %q", artifact.RequiredAcknowledgment)
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

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
