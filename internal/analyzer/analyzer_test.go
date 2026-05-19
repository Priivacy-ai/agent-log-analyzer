package analyzer

import (
	"encoding/json"
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
	input := []byte(strings.Join([]string{
		`{"type":"tool","name":"mcp__private_company_server__lookup"}`,
		`{"type":"user","message":"/private-internal-command"}`,
	}, "\n"))
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

func TestAnalyzeEmitsEmptyCollectionsInsteadOfNull(t *testing.T) {
	input := []byte(strings.Join([]string{
		`{"type":"user","message":"Please inspect this small focused task."}`,
		`{"type":"assistant","message":"Done."}`,
	}, "\n"))
	report, err := Analyze("job-clean", input)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	body := mustJSON(t, report)
	for _, leakedNull := range []string{`"findings":null`, `"timeline":null`, `"immediate_fixes":null`} {
		if strings.Contains(body, leakedNull) {
			t.Fatalf("report emitted null collection %s: %s", leakedNull, body)
		}
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings for clean input, got %#v", report.Findings)
	}
}

func TestSlashCommandDetectionDoesNotCountPaths(t *testing.T) {
	input := []byte(strings.Join([]string{
		`{"type":"user","message":"Inspect /Users/example/project and src/auth.ts before running tests."}`,
		`{"type":"tool","command":"cat src/routes.ts","output":"ok"}`,
	}, "\n"))
	report, err := Analyze("job-test", input)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if report.Ecosystem.UnknownSkillCount != 0 {
		t.Fatalf("expected file paths not to count as slash commands: %#v", report.Ecosystem)
	}
}

func TestEcosystemRegistryDetectsKnownPublicTools(t *testing.T) {
	input := []byte(strings.Join([]string{
		`{"type":"user","message":"Claude Code, Cursor IDE, OpenCode, BMAD, OpenSpec, Spec Kit, Spec Kitty, ccusage"}`,
		`{"type":"tool","name":"mcp__context7__resolve-library-id","message":"mcp__playwright__browser_navigate mcp__sentry__find_issues mcp__google-drive__search"}`,
		`{"type":"assistant","message":"Using @notion plugin, @github plugin, /plan-eng-review, /gstack-qa, pnpm-lock.yaml, uv.lock"}`,
	}, "\n"))
	report, err := Analyze("job-test", input)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	for _, want := range []string{"claude_code", "cursor", "opencode"} {
		if !contains(report.Ecosystem.CodingAgents, want) {
			t.Fatalf("expected coding agent %s in %#v", want, report.Ecosystem.CodingAgents)
		}
	}
	for _, want := range []string{"bmad", "openspec", "spec_kit", "spec_kitty", "ccusage"} {
		if !contains(report.Ecosystem.WorkflowFrameworks, want) {
			t.Fatalf("expected framework %s in %#v", want, report.Ecosystem.WorkflowFrameworks)
		}
	}
	for _, want := range []string{"context7", "playwright", "sentry", "google_drive"} {
		if !contains(report.Ecosystem.MCPServersKnown, want) {
			t.Fatalf("expected MCP %s in %#v", want, report.Ecosystem.MCPServersKnown)
		}
	}
	for _, want := range []string{"github", "notion"} {
		if !contains(report.Ecosystem.KnownPlugins, want) {
			t.Fatalf("expected plugin %s in %#v", want, report.Ecosystem.KnownPlugins)
		}
	}
	for _, want := range []string{"plan_eng_review", "qa"} {
		if !contains(report.Ecosystem.KnownSkills, want) {
			t.Fatalf("expected skill %s in %#v", want, report.Ecosystem.KnownSkills)
		}
	}
	for _, want := range []string{"pnpm", "uv"} {
		if !contains(report.Ecosystem.PackageManagers, want) {
			t.Fatalf("expected package manager %s in %#v", want, report.Ecosystem.PackageManagers)
		}
	}
	if report.Ecosystem.UnknownMCPServerCount != 0 {
		t.Fatalf("expected known MCPs not to count as unknown: %#v", report.Ecosystem)
	}
}

func TestAnalyzeDetectsNestedClaudeToolUseAndResults(t *testing.T) {
	input := []byte(strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"cat src/auth.ts"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"first output"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"cat src/auth.ts"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true,"content":"failed"}]}}`,
	}, "\n"))

	report, err := Analyze("job-test", input)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if report.Metrics.ToolOutputTokens == 0 {
		t.Fatalf("expected nested tool_use/tool_result to count as tool tokens: %#v", report.Metrics)
	}
	if report.Metrics.Rereads == 0 {
		t.Fatalf("expected nested Bash command reread detection: %#v", report.Metrics)
	}
	if report.Metrics.FailedCommands == 0 {
		t.Fatalf("expected nested is_error detection: %#v", report.Metrics)
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

// TestUnknownMCPRemainsCountOnlyAfterFingerprintPass regression-tests the
// NFR-001 invariant: when a transcript references an unknown (non-allowlisted)
// MCP server, the analyzer must record only its count, never the name itself,
// and the fingerprint pass must not resurrect the name into
// Ecosystem.WorkflowFingerprints.
func TestUnknownMCPRemainsCountOnlyAfterFingerprintPass(t *testing.T) {
	input := []byte(`{"type":"tool","name":"mcp__company_private_42__do_thing"}` + "\n")
	rep, err := Analyze("job-unknown-mcp", input)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if rep.Ecosystem.UnknownMCPServerCount != 1 {
		t.Errorf("expected UnknownMCPServerCount=1, got %d", rep.Ecosystem.UnknownMCPServerCount)
	}

	// No fingerprint should match the unknown private MCP name.
	for _, fp := range rep.Ecosystem.WorkflowFingerprints {
		if strings.Contains(fp.ID, "company_private_42") {
			t.Errorf("private MCP name leaked into fingerprint ID %q", fp.ID)
		}
	}

	// The serialized report must contain the count but NOT the name.
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `"unknown_mcp_server_count":1`) {
		t.Errorf("expected serialized report to include unknown_mcp_server_count:1; got %s", s)
	}
	if strings.Contains(s, "company_private_42") {
		t.Errorf("private MCP name leaked into serialized report: %s", s)
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
