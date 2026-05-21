package analyzer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalyzeForSource_ClaudeCodeSignalsArePrivacySafe(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"assistant","parentUuid":"parent-1","message":{"usage":{"input_tokens":100000,"cache_read_input_tokens":45000,"cache_creation_input_tokens":12000,"output_tokens":500},"content":[{"type":"tool_use","id":"call-1","name":"Bash","input":{"command":"cat /Users/private/repo/.env && echo sk-test-secret"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"call-1","is_error":true,"content":"failure with sk-test-secret"}]}}`,
		`{"type":"assistant","parentUuid":"parent-1","message":{"content":[{"type":"tool_use","id":"call-2","name":"Bash","input":{"command":"cat /Users/private/repo/.env && echo sk-test-secret"}}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"call-2","is_error":true,"content":"failure with sk-test-secret"}]}}`,
		`{"type":"assistant","parentUuid":"parent-1","message":{"content":[{"type":"tool_use","id":"call-3","name":"Bash","input":{"command":"cat /Users/private/repo/.env && echo sk-test-secret"}}]}}`,
	}, "\n")

	report, err := AnalyzeForSource("claude-test", "claude_code", []byte(input))
	if err != nil {
		t.Fatalf("AnalyzeForSource: %v", err)
	}
	if report.AnalysisSignals.InputTokens != 100000 {
		t.Fatalf("expected Claude input tokens, got %#v", report.AnalysisSignals)
	}
	if report.AnalysisSignals.CacheReadTokens != 45000 || report.AnalysisSignals.CacheCreationTokens != 12000 {
		t.Fatalf("expected Claude cache signals, got %#v", report.AnalysisSignals)
	}
	if report.AnalysisSignals.ArgsHashedRetryLoops < 2 {
		t.Fatalf("expected args-hashed retry signal, got %#v", report.AnalysisSignals)
	}
	if !hasFinding(report.Findings, "args_hashed_retry_loop") {
		t.Fatalf("expected args_hashed_retry_loop finding, got %#v", report.Findings)
	}
	assertReportDoesNotContain(t, report, "sk-test-secret", "/Users/private/repo/.env", "cat /Users/private")
}

func TestAnalyzeForSource_CodexTokenDeltasAndPatchStats(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"response_item","item":{"type":"function_call","call_id":"call-a","name":"exec_command","arguments":{"cmd":"go test ./..."}}}`,
		`{"type":"response_item","item":{"type":"function_call_output","call_id":"call-a","output":"ok"}}`,
		`{"type":"event_msg","msg":{"type":"token_count","last_token_usage":{"input_tokens":9000,"cached_input_tokens":3000,"output_tokens":700}}}`,
		`{"type":"event_msg","msg":{"type":"patch_apply_end","additions":12,"deletions":4}}`,
	}, "\n")

	report, err := AnalyzeForSource("codex-test", "codex", []byte(input))
	if err != nil {
		t.Fatalf("AnalyzeForSource: %v", err)
	}
	if report.AnalysisSignals.ToolCallCount != 1 || report.AnalysisSignals.ToolResultCount != 1 {
		t.Fatalf("expected paired Codex tool events, got %#v", report.AnalysisSignals)
	}
	if report.AnalysisSignals.InputTokens != 9000 || report.AnalysisSignals.CacheReadTokens != 3000 || report.AnalysisSignals.OutputTokens != 700 {
		t.Fatalf("expected Codex token deltas, got %#v", report.AnalysisSignals)
	}
	if report.AnalysisSignals.PatchLinesAdded != 12 || report.AnalysisSignals.PatchLinesRemoved != 4 {
		t.Fatalf("expected patch stats, got %#v", report.AnalysisSignals)
	}
	assertReportDoesNotContain(t, report, "go test ./...")
}

func TestAnalyzeForSource_PatchSignalsCanBeDisabled(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_DISABLE_PATCH_SIGNALS", "1")
	input := strings.Join([]string{
		`{"type":"event_msg","msg":{"type":"token_count","last_token_usage":{"input_tokens":9000,"cached_input_tokens":3000,"output_tokens":700}}}`,
		`{"type":"event_msg","msg":{"type":"patch_apply_end","additions":12,"deletions":4}}`,
	}, "\n")

	report, err := AnalyzeForSource("codex-test-disable-patch", "codex", []byte(input))
	if err != nil {
		t.Fatalf("AnalyzeForSource: %v", err)
	}
	if report.AnalysisSignals.PatchLinesAdded != 0 || report.AnalysisSignals.PatchLinesRemoved != 0 {
		t.Fatalf("expected patch stats to be disabled, got %#v", report.AnalysisSignals)
	}
	if report.AnalysisSignals.InputTokens != 9000 || report.AnalysisSignals.CacheReadTokens != 3000 || report.AnalysisSignals.OutputTokens != 700 {
		t.Fatalf("expected token signals to remain enabled, got %#v", report.AnalysisSignals)
	}
}

func TestAnalyzeForSource_OpenCodeToolPartSignals(t *testing.T) {
	input := strings.Join([]string{
		`{"id":"msg-1","sessionID":"ses-1","role":"assistant","modelID":"qwen/qwen3-coder"}`,
		`{"id":"part-1","sessionID":"ses-1","messageID":"msg-1","type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"grep -R privateCustomerName ."},"output":"line 1\nline 2\nline 3","metadata":{"exit":0}}}`,
	}, "\n")

	report, err := AnalyzeForSource("opencode-test", "opencode", []byte(input))
	if err != nil {
		t.Fatalf("AnalyzeForSource: %v", err)
	}
	if report.AnalysisSignals.ToolCallCount != 1 {
		t.Fatalf("expected OpenCode tool call signal, got %#v", report.AnalysisSignals)
	}
	if report.AnalysisSignals.ToolOutputBytes == 0 {
		t.Fatalf("expected OpenCode output byte attribution, got %#v", report.AnalysisSignals)
	}
	assertReportDoesNotContain(t, report, "privateCustomerName", "grep -R")
}

func TestAggregateReports_MergesAnalysisSignals(t *testing.T) {
	a, err := AnalyzeForSource("a", "codex", []byte(`{"type":"event_msg","msg":{"type":"token_count","last_token_usage":{"input_tokens":1000,"cached_input_tokens":250,"output_tokens":50}}}`))
	if err != nil {
		t.Fatalf("AnalyzeForSource a: %v", err)
	}
	b, err := AnalyzeForSource("b", "claude_code", []byte(`{"type":"assistant","message":{"usage":{"input_tokens":2000,"cache_read_input_tokens":1000,"cache_creation_input_tokens":9000,"output_tokens":100}}}`))
	if err != nil {
		t.Fatalf("AnalyzeForSource b: %v", err)
	}
	merged, err := AggregateReportsWithParserType("merged", []Report{a, b}, 2, "multi_source")
	if err != nil {
		t.Fatalf("AggregateReportsWithParserType: %v", err)
	}
	if merged.AnalysisSignals.InputTokens != 3000 || merged.AnalysisSignals.CacheReadTokens != 1250 {
		t.Fatalf("expected merged token signals, got %#v", merged.AnalysisSignals)
	}
	if merged.AnalysisSignals.CacheInvalidationEvents != 1 {
		t.Fatalf("expected merged cache invalidation, got %#v", merged.AnalysisSignals)
	}
	if merged.AnalysisSignals.SampleConfidence != "low" {
		t.Fatalf("expected low sample confidence for two sessions, got %#v", merged.AnalysisSignals)
	}
}

func hasFinding(findings []Finding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func assertReportDoesNotContain(t *testing.T, report Report, forbidden ...string) {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	serialized := string(data)
	for _, value := range forbidden {
		if strings.Contains(serialized, value) {
			t.Fatalf("report leaked %q in %s", value, serialized)
		}
	}
}
