package analyzer

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Private synthetic names used across tests to assert no leakage of unknown
// strings into returned structs. These are intentionally implausible and
// never match any real product name.
var privateLeakStrings = []string{
	"acme_internal_secret",
	"acme_internal",
	"private_corp_mcp",
	"private_corp_skill",
	"acme_secret",
}

func assertNoLeak(t *testing.T, label string, value any) {
	t.Helper()
	blob, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("%s: json.Marshal failed: %v", label, err)
	}
	got := string(blob)
	for _, leak := range privateLeakStrings {
		if strings.Contains(got, leak) {
			t.Errorf("%s: privacy leak — serialized output contains %q\n  payload: %s", label, leak, got)
		}
	}
}

func TestDetectMCPExposureFromHeaders(t *testing.T) {
	registry := ecosystemRegistry()

	t.Run("empty input returns zero value", func(t *testing.T) {
		out := detectMCPExposureFromHeaders(nil, registry)
		if !reflect.DeepEqual(out, mcpExposure{}) {
			t.Fatalf("expected zero value, got %#v", out)
		}
		if out.InferenceSource != "" {
			t.Fatalf("expected empty InferenceSource, got %q", out.InferenceSource)
		}
	})

	t.Run("no header returns zero value", func(t *testing.T) {
		input := []byte("some narrative text without any header.\nanother line.\n")
		out := detectMCPExposureFromHeaders(input, registry)
		if out.InferenceSource != "" || len(out.KnownIDs) != 0 || out.UnknownCount != 0 {
			t.Fatalf("expected zero exposure, got %#v", out)
		}
	})

	t.Run("known-only header", func(t *testing.T) {
		input := []byte("Available MCP servers:\n- github\n- linear\n\nother text\n")
		out := detectMCPExposureFromHeaders(input, registry)
		if out.InferenceSource != InferenceSourceHeader {
			t.Fatalf("expected header inference, got %q", out.InferenceSource)
		}
		want := []string{"github", "linear"}
		if !reflect.DeepEqual(out.KnownIDs, want) {
			t.Fatalf("KnownIDs = %v, want %v", out.KnownIDs, want)
		}
		if out.UnknownCount != 0 {
			t.Fatalf("UnknownCount = %d, want 0", out.UnknownCount)
		}
	})

	t.Run("unknown-only header: counts only, no names", func(t *testing.T) {
		input := []byte("Available MCP servers:\n- acme_internal_secret\n- private_corp_mcp\n\n")
		out := detectMCPExposureFromHeaders(input, registry)
		if out.UnknownCount != 2 {
			t.Fatalf("UnknownCount = %d, want 2", out.UnknownCount)
		}
		if len(out.KnownIDs) != 0 {
			t.Fatalf("expected no known IDs, got %v", out.KnownIDs)
		}
		assertNoLeak(t, "mcp unknown-only", out)
	})

	t.Run("mixed header with tool tokens", func(t *testing.T) {
		input := []byte("Following deferred tools are now available:\n- mcp__github__create_issue\n- mcp__linear__list_issues\n- mcp__acme_internal__send\n\nend\n")
		out := detectMCPExposureFromHeaders(input, registry)
		if out.InferenceSource != InferenceSourceHeader {
			t.Fatalf("expected header inference, got %q", out.InferenceSource)
		}
		if out.ExposedToolCount != 3 {
			t.Fatalf("ExposedToolCount = %d, want 3", out.ExposedToolCount)
		}
		if !out.ExposedToolKnown {
			t.Fatalf("ExposedToolKnown should be true after a header match")
		}
		if out.SchemaTextBytes == 0 {
			t.Fatalf("expected non-zero SchemaTextBytes")
		}
		assertNoLeak(t, "mcp mixed", out)
	})

	t.Run("case-insensitive header alternates", func(t *testing.T) {
		input := []byte("MCP tools available\n- github\n- acme_internal_secret\n\n")
		out := detectMCPExposureFromHeaders(input, registry)
		if out.InferenceSource != InferenceSourceHeader {
			t.Fatalf("expected header inference, got %q", out.InferenceSource)
		}
		if out.UnknownCount != 1 {
			t.Fatalf("UnknownCount = %d, want 1", out.UnknownCount)
		}
		assertNoLeak(t, "mcp alternate", out)
	})
}

func TestDetectSkillExposureFromHeaders(t *testing.T) {
	registry := ecosystemRegistry()

	t.Run("empty input", func(t *testing.T) {
		out := detectSkillExposureFromHeaders(nil, registry)
		if !reflect.DeepEqual(out, skillExposure{}) {
			t.Fatalf("expected zero value, got %#v", out)
		}
	})

	t.Run("known-only", func(t *testing.T) {
		input := []byte("The following skills are available for use:\n- qa\n- review\n- ship\n\n")
		out := detectSkillExposureFromHeaders(input, registry)
		if out.InferenceSource != InferenceSourceHeader {
			t.Fatalf("expected header inference, got %q", out.InferenceSource)
		}
		want := []string{"qa", "review", "ship"}
		if !reflect.DeepEqual(out.KnownIDs, want) {
			t.Fatalf("KnownIDs = %v, want %v", out.KnownIDs, want)
		}
	})

	t.Run("unknown-only: no leak", func(t *testing.T) {
		input := []byte("Available skills:\n- private_corp_skill\n- acme_internal_secret\n\n")
		out := detectSkillExposureFromHeaders(input, registry)
		if out.UnknownCount != 2 {
			t.Fatalf("UnknownCount = %d, want 2", out.UnknownCount)
		}
		if len(out.KnownIDs) != 0 {
			t.Fatalf("expected no known IDs, got %v", out.KnownIDs)
		}
		assertNoLeak(t, "skill unknown-only", out)
	})

	t.Run("mixed", func(t *testing.T) {
		input := []byte("The following skills are available:\n- qa\n- private_corp_skill\n- ship\n\n")
		out := detectSkillExposureFromHeaders(input, registry)
		if out.UnknownCount != 1 {
			t.Fatalf("UnknownCount = %d, want 1", out.UnknownCount)
		}
		want := []string{"qa", "ship"}
		if !reflect.DeepEqual(out.KnownIDs, want) {
			t.Fatalf("KnownIDs = %v, want %v", out.KnownIDs, want)
		}
		assertNoLeak(t, "skill mixed", out)
	})
}

func TestDetectMCPCallsFromToolUse(t *testing.T) {
	registry := ecosystemRegistry()

	t.Run("empty input", func(t *testing.T) {
		out := detectMCPCallsFromToolUse(nil, nil, registry)
		if out.TotalCalls != 0 || out.KnownCallCount != 0 || out.UnknownCallCount != 0 {
			t.Fatalf("expected zero counts, got %#v", out)
		}
		if out.UniqueServerCount != 0 || out.UniqueToolCount != 0 || out.UniqueUnknownCount != 0 {
			t.Fatalf("expected zero unique counts, got %#v", out)
		}
		if len(out.UniqueKnownIDs) != 0 {
			t.Fatalf("expected empty UniqueKnownIDs, got %v", out.UniqueKnownIDs)
		}
	})

	t.Run("known-only: 5 calls to one server", func(t *testing.T) {
		input := []byte(strings.Repeat("mcp__github__create_issue\n", 5))
		out := detectMCPCallsFromToolUse(input, nil, registry)
		if out.TotalCalls != 5 || out.KnownCallCount != 5 || out.UnknownCallCount != 0 {
			t.Fatalf("call counts mismatch: %#v", out)
		}
		if !reflect.DeepEqual(out.UniqueKnownIDs, []string{"github"}) {
			t.Fatalf("UniqueKnownIDs = %v, want [github]", out.UniqueKnownIDs)
		}
		if out.UniqueServerCount != 1 || out.UniqueToolCount != 1 {
			t.Fatalf("unique counts mismatch: %#v", out)
		}
	})

	t.Run("unknown-only: counts populated, no leak", func(t *testing.T) {
		input := []byte("mcp__acme_secret__send\nmcp__acme_secret__list\nmcp__private_corp_mcp__do\n")
		out := detectMCPCallsFromToolUse(input, nil, registry)
		if out.UnknownCallCount != 3 {
			t.Fatalf("UnknownCallCount = %d, want 3", out.UnknownCallCount)
		}
		if out.UniqueUnknownCount != 2 {
			t.Fatalf("UniqueUnknownCount = %d, want 2", out.UniqueUnknownCount)
		}
		if len(out.UniqueKnownIDs) != 0 {
			t.Fatalf("expected no known IDs, got %v", out.UniqueKnownIDs)
		}
		assertNoLeak(t, "mcp calls unknown-only", out)
	})

	t.Run("mixed: raw + parsed lines", func(t *testing.T) {
		input := []byte("mcp__github__create_issue\nmcp__linear__list_issues\nmcp__acme_secret__send\n")
		lines := []parsedLine{
			{IsTool: true, ToolName: "mcp__github__delete_issue"},
			{IsTool: true, ToolName: "Read"},               // non-mcp, ignored
			{IsTool: false, ToolName: "mcp__notion__page"}, // not a tool line, ignored
			{IsTool: true, ToolName: "mcp__private_corp_mcp__do"},
		}
		out := detectMCPCallsFromToolUse(input, lines, registry)
		if out.TotalCalls != 5 {
			t.Fatalf("TotalCalls = %d, want 5", out.TotalCalls)
		}
		// github + linear known. acme_secret + private_corp_mcp unknown.
		want := []string{"github", "linear"}
		if !reflect.DeepEqual(out.UniqueKnownIDs, want) {
			t.Fatalf("UniqueKnownIDs = %v, want %v", out.UniqueKnownIDs, want)
		}
		if out.UniqueUnknownCount != 2 {
			t.Fatalf("UniqueUnknownCount = %d, want 2", out.UniqueUnknownCount)
		}
		if out.UniqueServerCount != 4 {
			t.Fatalf("UniqueServerCount = %d, want 4", out.UniqueServerCount)
		}
		// 5 distinct server::tool pairs.
		if out.UniqueToolCount != 5 {
			t.Fatalf("UniqueToolCount = %d, want 5", out.UniqueToolCount)
		}
		assertNoLeak(t, "mcp calls mixed", out)
	})
}

func TestDetectSkillExecutionsFromLines(t *testing.T) {
	registry := ecosystemRegistry()

	t.Run("empty input", func(t *testing.T) {
		out := detectSkillExecutionsFromLines(nil, registry)
		if out.ExecutedCount != 0 || out.UnknownExecuted != 0 || len(out.KnownExecutedIDs) != 0 {
			t.Fatalf("expected zero value, got %#v", out)
		}
	})

	t.Run("path avoidance: /etc/passwd", func(t *testing.T) {
		lines := []parsedLine{
			{Text: "Inspect /etc/passwd and /home/user/file before running."},
			{Text: "Path: /var/log/system.log"},
		}
		out := detectSkillExecutionsFromLines(lines, registry)
		if out.ExecutedCount != 0 {
			t.Fatalf("expected zero executions for paths, got %d (%#v)", out.ExecutedCount, out)
		}
		if out.UnknownExecuted != 0 {
			t.Fatalf("expected no unknown executions, got %d", out.UnknownExecuted)
		}
	})

	t.Run("known skill, single invocation", func(t *testing.T) {
		lines := []parsedLine{
			{Text: "Please run /review now."},
		}
		out := detectSkillExecutionsFromLines(lines, registry)
		if out.ExecutedCount != 1 {
			t.Fatalf("ExecutedCount = %d, want 1", out.ExecutedCount)
		}
		if !reflect.DeepEqual(out.KnownExecutedIDs, []string{"review"}) {
			t.Fatalf("KnownExecutedIDs = %v, want [review]", out.KnownExecutedIDs)
		}
	})

	t.Run("known skill, multiple invocations: ExecutedCount sums, IDs dedup", func(t *testing.T) {
		lines := []parsedLine{
			{Text: "First /review run."},
			{Text: "Second /review run."},
			{Text: "Now /ship it."},
		}
		out := detectSkillExecutionsFromLines(lines, registry)
		if out.ExecutedCount != 3 {
			t.Fatalf("ExecutedCount = %d, want 3", out.ExecutedCount)
		}
		want := []string{"review", "ship"}
		if !reflect.DeepEqual(out.KnownExecutedIDs, want) {
			t.Fatalf("KnownExecutedIDs = %v, want %v", out.KnownExecutedIDs, want)
		}
	})

	t.Run("unknown skills: counts only, no leak", func(t *testing.T) {
		lines := []parsedLine{
			{Text: "Run /private_corp_skill once."},
			{Text: "Then /acme_internal_secret twice."},
			{Text: "And /acme_internal_secret again."},
		}
		out := detectSkillExecutionsFromLines(lines, registry)
		if out.ExecutedCount != 3 {
			t.Fatalf("ExecutedCount = %d, want 3", out.ExecutedCount)
		}
		if out.UnknownExecuted != 2 {
			t.Fatalf("UnknownExecuted = %d, want 2", out.UnknownExecuted)
		}
		if len(out.KnownExecutedIDs) != 0 {
			t.Fatalf("expected no known IDs, got %v", out.KnownExecutedIDs)
		}
		assertNoLeak(t, "skill executions unknown", out)
	})

	t.Run("IsTool lines are skipped", func(t *testing.T) {
		lines := []parsedLine{
			{Text: "/review here", IsTool: true, ToolName: "Bash"},
		}
		out := detectSkillExecutionsFromLines(lines, registry)
		if out.ExecutedCount != 0 {
			t.Fatalf("expected tool lines to be skipped, got %#v", out)
		}
	})

	t.Run("mixed known and path-like", func(t *testing.T) {
		lines := []parsedLine{
			{Text: "Run /review after looking at /etc/passwd."},
		}
		out := detectSkillExecutionsFromLines(lines, registry)
		if out.ExecutedCount != 1 {
			t.Fatalf("ExecutedCount = %d, want 1 (only /review, not /etc/passwd)", out.ExecutedCount)
		}
		if !reflect.DeepEqual(out.KnownExecutedIDs, []string{"review"}) {
			t.Fatalf("KnownExecutedIDs = %v, want [review]", out.KnownExecutedIDs)
		}
	})
}

func TestEstimateFootprint(t *testing.T) {
	cases := []struct {
		name       string
		fn         func() (int, bool)
		wantTokens int
		wantKnown  bool
	}{
		{
			name:       "MCP: schema bytes win when present",
			fn:         func() (int, bool) { return estimateMCPFootprintTokens(4000, -1, -1) },
			wantTokens: 1000,
			wantKnown:  true,
		},
		{
			name:       "MCP: fallback to per-server + per-tool",
			fn:         func() (int, bool) { return estimateMCPFootprintTokens(0, 10, 30) },
			wantTokens: 10*mcpServerOverheadTokens + 30*mcpToolTokens,
			wantKnown:  true,
		},
		{
			name:       "MCP: zero servers still known when count >= 0",
			fn:         func() (int, bool) { return estimateMCPFootprintTokens(0, 0, 0) },
			wantTokens: 0,
			wantKnown:  true,
		},
		{
			name:       "MCP: no signal returns unknown",
			fn:         func() (int, bool) { return estimateMCPFootprintTokens(0, -1, -1) },
			wantTokens: 0,
			wantKnown:  false,
		},
		{
			name:       "MCP: negative toolCount clamped to zero",
			fn:         func() (int, bool) { return estimateMCPFootprintTokens(0, 4, -1) },
			wantTokens: 4 * mcpServerOverheadTokens,
			wantKnown:  true,
		},
		{
			name:       "Skill: schema bytes win",
			fn:         func() (int, bool) { return estimateSkillFootprintTokens(8000, -1) },
			wantTokens: 2000,
			wantKnown:  true,
		},
		{
			name:       "Skill: fallback to per-skill",
			fn:         func() (int, bool) { return estimateSkillFootprintTokens(0, 5) },
			wantTokens: 5 * skillTokens,
			wantKnown:  true,
		},
		{
			name:       "Skill: no signal returns unknown",
			fn:         func() (int, bool) { return estimateSkillFootprintTokens(0, -1) },
			wantTokens: 0,
			wantKnown:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTokens, gotKnown := tc.fn()
			if gotTokens != tc.wantTokens || gotKnown != tc.wantKnown {
				t.Fatalf("got (%d, %v), want (%d, %v)", gotTokens, gotKnown, tc.wantTokens, tc.wantKnown)
			}
		})
	}

	// Privacy assertion on a struct using estimator outputs as fields. The
	// estimator itself only emits integers, but we round out the privacy
	// posture by serialising a wrapper holding the result alongside synthetic
	// names and confirming the estimator outputs don't leak them.
	t.Run("estimator output never leaks names", func(t *testing.T) {
		type holder struct {
			Tokens int  `json:"tokens"`
			Known  bool `json:"known"`
		}
		tokens, known := estimateMCPFootprintTokens(4000, 1, 2)
		assertNoLeak(t, "footprint", holder{Tokens: tokens, Known: known})
	})
}
