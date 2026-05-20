package analytics

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSummarizeJSONLSuppressesSmallCohorts(t *testing.T) {
	var buf bytes.Buffer
	for i := 0; i < 12; i++ {
		event := Event{
			SchemaVersion: SchemaVersion,
			Event:         "analytics.report",
			ScanType:      "free",
			Score:         "40_60",
			Waste:         "20_40",
			Ecosystem: EcosystemEvent{
				Client:          "claude_code",
				OperatingSystem: "macos",
				Shell:           "zsh",
				PackageManagers: []string{"go"},
				WorkflowFingerprints: []FingerprintEvent{
					{ID: "spec_kitty", Confidence: "high"},
				},
				ToolingUtilization: ToolingEvent{
					MCP:   MCPEvent{KnownServerIDs: []string{"github"}, UniqueKnownCalledIDs: []string{"github"}, WarningBand: "normal"},
					Skill: SkillEvent{KnownExposedIDs: []string{"qa"}, KnownExecutedIDs: []string{"qa"}, WarningBand: "normal"},
				},
			},
			Recommendation: RecommendationEvent{
				Primary: &RecommendationSlot{Class: "usage_visibility", ToolID: "ccusage", Reason: "absent"},
			},
			Findings: map[string]string{"repeated_file_reads": "high"},
		}
		if i == 0 {
			event.Ecosystem.PackageManagers = append(event.Ecosystem.PackageManagers, "pnpm")
			event.Ecosystem.WorkflowFingerprints = append(event.Ecosystem.WorkflowFingerprints, FingerprintEvent{ID: "openspec"})
		}
		data, err := MarshalJSONLine(event)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		buf.Write(data)
	}

	summary, err := SummarizeJSONL(&buf, 10)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if summary.EventCount != 12 {
		t.Fatalf("EventCount: got %d, want 12", summary.EventCount)
	}
	if got := summary.SDDTools["spec_kitty"]; got != 12 {
		t.Fatalf("spec_kitty count: got %d, want 12", got)
	}
	if _, ok := summary.SDDTools["openspec"]; ok {
		t.Fatalf("openspec cohort should be suppressed: %#v", summary.SDDTools)
	}
	if _, ok := summary.PackageManagers["pnpm"]; ok {
		t.Fatalf("pnpm cohort should be suppressed: %#v", summary.PackageManagers)
	}
	if summary.SuppressedBelowCohortCount == 0 {
		t.Fatalf("expected at least one suppressed cohort")
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if strings.Contains(string(data), "private_") || strings.Contains(string(data), "job-") {
		t.Fatalf("summary leaked forbidden content: %s", data)
	}
}
