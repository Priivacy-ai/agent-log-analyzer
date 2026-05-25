package analyzer

import (
	"encoding/json"
	"os"
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

var specKittyDoctrineSkillIDs = []string{
	"ad_hoc_profile_load",
	"debugger_debbie",
	"spec_kitty_bulk_edit_classification",
	"spec_kitty_charter_doctrine",
	"spec_kitty_git_workflow",
	"spec_kitty_glossary_context",
	"spec_kitty_implement_review",
	"spec_kitty_mission_review",
	"spec_kitty_mission_system",
	"spec_kitty_orchestrator_api_operator",
	"spec_kitty_program_orchestrate",
	"spec_kitty_runtime_next",
	"spec_kitty_runtime_review",
	"spec_kitty_setup_doctor",
	"spec_kitty_spdd_reasons",
}

var specKittyDoctrineSkillNames = []string{
	"ad-hoc-profile-load",
	"debugger-debbie",
	"spec-kitty-bulk-edit-classification",
	"spec-kitty-charter-doctrine",
	"spec-kitty-git-workflow",
	"spec-kitty-glossary-context",
	"spec-kitty-implement-review",
	"spec-kitty-mission-review",
	"spec-kitty-mission-system",
	"spec-kitty-orchestrator-api-operator",
	"spec-kitty-program-orchestrate",
	"spec-kitty-runtime-next",
	"spec-kitty-runtime-review",
	"spec-kitty-setup-doctor",
	"spec-kitty-spdd-reasons",
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

	t.Run("spec kitty doctrine inventory is known", func(t *testing.T) {
		input := []byte("The following skills are available:\n- " + strings.Join(specKittyDoctrineSkillNames, "\n- ") + "\n\n")
		out := detectSkillExposureFromHeaders(input, registry)
		if out.InferenceSource != InferenceSourceHeader {
			t.Fatalf("expected header inference, got %q", out.InferenceSource)
		}
		if out.UnknownCount != 0 {
			t.Fatalf("UnknownCount = %d, want 0 for public Spec Kitty skills: %#v", out.UnknownCount, out)
		}
		if !reflect.DeepEqual(out.KnownIDs, specKittyDoctrineSkillIDs) {
			t.Fatalf("KnownIDs = %v, want %v", out.KnownIDs, specKittyDoctrineSkillIDs)
		}
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

	t.Run("spec kitty doctrine slash skills are known", func(t *testing.T) {
		lines := make([]parsedLine, 0, len(specKittyDoctrineSkillNames))
		for _, name := range specKittyDoctrineSkillNames {
			lines = append(lines, parsedLine{Text: "Use /" + name + " for this workflow."})
		}
		out := detectSkillExecutionsFromLines(lines, registry)
		if out.ExecutedCount != len(specKittyDoctrineSkillNames) {
			t.Fatalf("ExecutedCount = %d, want %d", out.ExecutedCount, len(specKittyDoctrineSkillNames))
		}
		if out.UnknownExecuted != 0 {
			t.Fatalf("UnknownExecuted = %d, want 0 for public Spec Kitty skills: %#v", out.UnknownExecuted, out)
		}
		if !reflect.DeepEqual(out.KnownExecutedIDs, specKittyDoctrineSkillIDs) {
			t.Fatalf("KnownExecutedIDs = %v, want %v", out.KnownExecutedIDs, specKittyDoctrineSkillIDs)
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

// TestInsideAny exercises the masking primitive used by
// detectMCPCallsFromToolUse to skip raw-byte matches that fall inside any
// exposure-header byte range. Boundary behavior matters: the range is
// half-open [Start, End), so Start is inside and End is outside.
func TestInsideAny(t *testing.T) {
	t.Run("empty ranges: always false", func(t *testing.T) {
		for _, off := range []int{-1, 0, 1, 100, 1 << 20} {
			if insideAny(off, nil) {
				t.Errorf("insideAny(%d, nil) = true, want false", off)
			}
			if insideAny(off, []byteRange{}) {
				t.Errorf("insideAny(%d, empty) = true, want false", off)
			}
		}
	})

	t.Run("single range [10,20): half-open boundary", func(t *testing.T) {
		ranges := []byteRange{{Start: 10, End: 20}}
		cases := []struct {
			off  int
			want bool
		}{
			{9, false},  // just before Start: outside
			{10, true},  // at Start: inside (half-open low)
			{15, true},  // middle: inside
			{19, true},  // just before End: inside
			{20, false}, // at End: outside (half-open high)
			{21, false}, // just after End: outside
		}
		for _, tc := range cases {
			if got := insideAny(tc.off, ranges); got != tc.want {
				t.Errorf("insideAny(%d, [10,20)) = %v, want %v", tc.off, got, tc.want)
			}
		}
	})

	t.Run("multiple ranges: any-match short-circuits to true", func(t *testing.T) {
		ranges := []byteRange{
			{Start: 0, End: 5},
			{Start: 100, End: 110},
			{Start: 200, End: 210},
		}
		// Inside the third range — must still return true even though earlier
		// ranges don't contain the offset.
		if !insideAny(205, ranges) {
			t.Errorf("insideAny(205, multi) = false, want true (offset is in third range)")
		}
		// Inside the first.
		if !insideAny(3, ranges) {
			t.Errorf("insideAny(3, multi) = false, want true (offset is in first range)")
		}
		// Between ranges: not inside any.
		if insideAny(50, ranges) {
			t.Errorf("insideAny(50, multi) = true, want false (offset is in a gap)")
		}
		if insideAny(150, ranges) {
			t.Errorf("insideAny(150, multi) = true, want false (offset is in a gap)")
		}
	})
}

// TestInsideAnyCombinedMCPAndSkillRanges is the defensive-symmetry coverage
// for the skill side of the mask. It constructs a combined ranges slice
// carrying BOTH MCP and skill exposure-header byte ranges, then asserts that
// candidate offsets falling inside either kind of range are filtered out.
// This guards against a future skill exposure-header schema change reintroducing
// the issue #70 class of bug on the skill side without test detection.
func TestInsideAnyCombinedMCPAndSkillRanges(t *testing.T) {
	// Imagine the MCP header occupies bytes [10, 50) and the skill header
	// occupies bytes [80, 120). The combined set is the union.
	mcpRanges := []byteRange{{Start: 10, End: 50}}
	skillRanges := []byteRange{{Start: 80, End: 120}}
	combined := append([]byteRange{}, mcpRanges...)
	combined = append(combined, skillRanges...)

	type candidate struct {
		off    int
		masked bool // true => insideAny must return true (offset filtered out)
		why    string
	}
	cases := []candidate{
		{off: 5, masked: false, why: "before any header"},
		{off: 25, masked: true, why: "inside MCP header"},
		{off: 60, masked: false, why: "between MCP and skill headers"},
		{off: 100, masked: true, why: "inside skill header"},
		{off: 150, masked: false, why: "after all headers"},
		{off: 50, masked: false, why: "at MCP End (half-open)"},
		{off: 80, masked: true, why: "at skill Start (half-open)"},
	}
	for _, tc := range cases {
		got := insideAny(tc.off, combined)
		if got != tc.masked {
			t.Errorf("insideAny(%d, combined) = %v, want %v (%s)", tc.off, got, tc.masked, tc.why)
		}
	}
}

// TestDetectMCPCallsFromToolUseHeaderMask is the load-bearing fixture-based
// assertion for FR-005 / FR-006 / NFR-003: a log that contains an MCP
// exposure header advertising several `mcp__server__tool` identifiers but
// zero actual tool_use records must produce CallCount == 0,
// KnownCallCount == 0, and an empty UniqueKnownCalledIDs after the mask is
// applied.
func TestDetectMCPCallsFromToolUseHeaderMask(t *testing.T) {
	registry := ecosystemRegistry()
	data, err := os.ReadFile("testdata/tooling/08-header-only-zero-calls.log")
	if err != nil {
		t.Fatalf("read fixture 08: %v", err)
	}

	// Sanity-check: the header detector itself sees the header and records
	// a non-empty byte range. Without this, the mask test below would be
	// vacuously green if the header parser stopped detecting the block.
	exp := detectMCPExposureFromHeaders(data, registry)
	if exp.InferenceSource != InferenceSourceHeader {
		t.Fatalf("fixture 08 must produce a header inference, got %q", exp.InferenceSource)
	}
	if len(exp.HeaderRanges) == 0 {
		t.Fatalf("fixture 08 must produce at least one HeaderRanges entry")
	}
	if exp.ExposedToolCount < 5 {
		t.Fatalf("fixture 08 must advertise at least 5 mcp__server__tool tokens, got %d", exp.ExposedToolCount)
	}

	// Now the actual assertion: the call detector must mask all of those
	// header tokens and report zero calls.
	calls := detectMCPCallsFromToolUse(data, nil, registry)
	if calls.TotalCalls != 0 {
		t.Errorf("CallCount = %d, want 0 (header tokens must not count as calls)", calls.TotalCalls)
	}
	if calls.KnownCallCount != 0 {
		t.Errorf("KnownCallCount = %d, want 0", calls.KnownCallCount)
	}
	if calls.UnknownCallCount != 0 {
		t.Errorf("UnknownCallCount = %d, want 0", calls.UnknownCallCount)
	}
	if len(calls.UniqueKnownIDs) != 0 {
		t.Errorf("UniqueKnownCalledIDs = %v, want empty", calls.UniqueKnownIDs)
	}
	if calls.UniqueServerCount != 0 {
		t.Errorf("UniqueServerCount = %d, want 0", calls.UniqueServerCount)
	}
	if calls.UniqueToolCount != 0 {
		t.Errorf("UniqueToolCount = %d, want 0", calls.UniqueToolCount)
	}
}
