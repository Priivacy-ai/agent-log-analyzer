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

func TestScrubberCoversCommonSecretFamilies(t *testing.T) {
	input := []byte(strings.Join([]string{
		`github=ghp_123456789012345678901234567890123456`,
		`npm=npm_123456789012345678901234567890123456`,
		`aws=AKIA1234567890ABCDEF`,
		`google=AIza12345678901234567890123456789012345`,
		`db=postgres://user:pass@example.com/prod`,
		`cookie=session=supersecret`,
		`jwt=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signaturevalue`,
		`-----BEGIN OPENSSH PRIVATE KEY-----`,
		`private material`,
		`-----END OPENSSH PRIVATE KEY-----`,
	}, "\n"))

	scrubbed, counts := Scrub(input)
	output := string(scrubbed)
	for _, leaked := range []string{
		"ghp_",
		"npm_",
		"AKIA",
		"AIza",
		"postgres://user:pass",
		"session=supersecret",
		"eyJhbGci",
		"private material",
	} {
		if strings.Contains(output, leaked) {
			t.Fatalf("scrubbed output leaked %q: %s", leaked, output)
		}
	}
	for _, family := range []string{"github_token", "npm_token", "aws_access_key", "google_api_key", "database_url", "cookie", "jwt", "ssh_private_key"} {
		if counts[family] == 0 {
			t.Fatalf("expected redaction count for %s, got %#v", family, counts)
		}
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
