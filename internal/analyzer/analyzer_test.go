package analyzer

import (
	"strings"
	"testing"
)

func TestAnalyzeDetectsWasteAndScrubsSecrets(t *testing.T) {
	input := []byte(strings.Join([]string{
		`{"type":"user","message":"Claude Code on macOS zsh using Spec Kitty and mcp__github__create_issue"}`,
		`{"type":"tool","command":"cat src/auth.ts","output":"api_key = \"sk-ant-123456789012345678901234567890\""}`,
		`{"type":"tool","command":"cat src/auth.ts","output":"same file again"}`,
		`{"type":"tool","command":"cat src/auth.ts","output":"same file third time"}`,
		`{"type":"tool","command":"go test ./...","error":"failed"}`,
		`{"type":"tool","command":"go test ./...","error":"failed"}`,
		`{"type":"tool","command":"go test ./...","error":"failed"}`,
	}, "\n"))

	report, err := Analyze("job-test", input)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if report.Redactions["anthropic_key"] == 0 {
		t.Fatalf("expected anthropic key redaction, got %#v", report.Redactions)
	}
	body := mustJSON(t, report)
	if strings.Contains(body, "sk-ant-") {
		t.Fatalf("report leaked secret: %s", body)
	}
	if report.Metrics.Rereads < 2 {
		t.Fatalf("expected rereads, got %d", report.Metrics.Rereads)
	}
	if report.Metrics.RetryDepthMax < 3 {
		t.Fatalf("expected retry depth, got %d", report.Metrics.RetryDepthMax)
	}
	if !contains(report.Ecosystem.WorkflowFrameworks, "spec_kitty") {
		t.Fatalf("expected spec_kitty ecosystem detection: %#v", report.Ecosystem)
	}
	if !contains(report.Ecosystem.MCPServersKnown, "github") {
		t.Fatalf("expected github MCP detection: %#v", report.Ecosystem)
	}
}

func TestUnknownNamesAreCountsOnly(t *testing.T) {
	input := []byte(`{"type":"tool","name":"mcp__private_company_server__lookup","message":"/private-internal-command"}`)
	report, err := Analyze("job-test", input)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	body := mustJSON(t, report.AggregateEvent)
	if strings.Contains(body, "private_company_server") || strings.Contains(body, "private-internal-command") {
		t.Fatalf("aggregate event leaked unknown private names: %s", body)
	}
	if report.Ecosystem.UnknownMCPServerCount != 1 {
		t.Fatalf("expected unknown MCP count, got %#v", report.Ecosystem)
	}
	if report.Ecosystem.UnknownSkillCount != 1 {
		t.Fatalf("expected unknown skill count, got %#v", report.Ecosystem)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
