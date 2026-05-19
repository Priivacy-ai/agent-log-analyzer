package analyzer_test

// View-renderer input invariants for WP01 of mission
// report-intelligence-ux-01KS070G.
//
// These tests structurally backstop the privacy and contract properties the
// web/app.js renderers depend on:
//
//  1. TestRenderInputs_NoCanaryInRendererJSON — extends the existing
//     forbidden-strings leak canary (leak_test.go) to every JSON surface the
//     two new renderers consume: rep.Ecosystem.WorkflowFingerprints,
//     rep.Ecosystem.ToolingUtilization, and the full rep.Findings slice.
//  2. TestPruningAdviceRecommendations_BytewiseEqualToConstants — pins the
//     four band-keyed advice strings (mcp_bloat_severe / mcp_bloat_high /
//     skill_bloat_severe / skill_bloat_high) byte-for-byte to the literals
//     emitted by internal/analyzer/analyzer.go:381-393. Any reword forces a
//     contemporaneous review of NFR-007 (no private-name leakage in advice
//     copy).
//  3. TestBandFindingPairings_DrivenByAnalyze — pins the pairing contract end
//     to end: for each surface (MCP / Skill), exactly one *_bloat_* finding
//     is emitted when the warning band is "severe" or "high", and zero are
//     emitted when the band is in {watch, normal, unknown}. This pins INV-6
//     so a future analyzer change can't silently break the band → advice
//     lookup the UI uses.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
)

// Literal strings emitted by internal/analyzer/analyzer.go:381-393. These are
// pinned here so any reword in the analyzer fails the test loudly. Update both
// sides together after re-reviewing NFR-007 (no private-name leakage in advice
// copy).
const (
	adviceMCPSevere   = "Disable unused MCP servers by default and lazy-load heavy MCP servers only when needed."
	adviceMCPHigh     = "Scope project-specific MCPs to project config instead of global config; prefer narrower MCP servers over all-tools-enabled setups."
	adviceSkillSevere = "Move rarely used instructions out of always-loaded skill context; keep only high-signal skills in the default agent context."
	adviceSkillHigh   = "Split general skills from project-specific skills."
)

// renderCanaries returns the canary list this WP asserts is never present in
// any JSON surface the renderers consume. It is the union of the 16 canaries
// from leak_test.go (TestReportSerializationContainsNoForbiddenStrings) plus
// the three explicitly tagged for WP01 (private MCP/skill/plugin name
// canaries surfaced through the new Tooling Utilization / Workflow
// Fingerprints sections).
func renderCanaries() []string {
	return []string{
		// 1. user prompts
		"FORBIDDEN-PROMPT-CANARY",
		// 2. task descriptions
		"FORBIDDEN-TASK-CANARY",
		// 3. raw transcript excerpts
		"FORBIDDEN-TRANSCRIPT-CANARY",
		// 4. raw tool inputs
		"FORBIDDEN-TOOLIN-CANARY",
		// 5. raw tool outputs
		"FORBIDDEN-TOOLOUT-CANARY",
		// 6. raw file paths
		"/Users/robert/private-project",
		// 7. repo URLs
		"git@github.com:private-org/private-repo.git",
		// 8. branch names
		"customer-private-branch",
		// 9. usernames
		"jdoe-private",
		// 10. hostnames
		"corp-internal-host.local",
		// 11. emails
		"private.user@example.internal",
		// 12. session IDs
		"sess_PRIVATESESSION",
		// 13. transcript paths
		"/Users/robert/.claude/projects/PRIVATE/transcript.jsonl",
		// 14. private MCP / skill / plugin names
		"mcp__private__internal",
		"skill__private__internal",
		"plugin__private__internal",
		// 15. raw LookPath / which paths
		"/opt/homebrew/bin/openspec",
		"/usr/local/bin/spec-kitty",
		// 16. raw --version output / stable hashes of private strings
		"openspec 1.2.3 built /private/path",
		"sha256-of-private-name-DO-NOT-LEAK",
		// WP01 — explicit private MCP/skill/plugin canaries for the
		// renderer-input surfaces introduced by this mission.
		"mcp__renderprivate__rogue",
		"skill__renderprivate__rogue",
		"plugin__renderprivate__rogue",
	}
}

// buildRenderHostileInput is a local clone of leak_test.go::buildHostileInput.
// We clone instead of editing leak_test.go because that file is not part of
// WP01's owned_files boundary.
func buildRenderHostileInput(canaries []string) []byte {
	var lines []string
	for _, c := range canaries {
		escaped := renderJSONEscape(c)
		lines = append(lines, `{"type":"user","message":"`+escaped+`"}`)
		lines = append(lines, `{"type":"tool","tool_input":"`+escaped+`"}`)
		lines = append(lines, `{"type":"tool","tool_output":"`+escaped+`"}`)
		lines = append(lines, `{"type":"tool","command":"`+escaped+`"}`)
		lines = append(lines, c)
	}
	return []byte(strings.Join(lines, "\n"))
}

func renderJSONEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// TestRenderInputs_NoCanaryInRendererJSON asserts that none of the canary
// substrings appears in the JSON serialization of any field the new
// renderers consume. We cover three surfaces independently so a regression
// names the surface clearly:
//
//   - rep.Ecosystem.WorkflowFingerprints (consumed by renderWorkflowFingerprints)
//   - rep.Ecosystem.ToolingUtilization   (consumed by renderToolingUtilization)
//   - rep.Findings                        (renderer reads .find(f => f.id === …)
//     across the WHOLE findings slice when resolving advice; ANY current or
//     future Finding whose Recommendation could carry private text must be
//     caught here, not just the four *_bloat_* IDs).
func TestRenderInputs_NoCanaryInRendererJSON(t *testing.T) {
	canaries := renderCanaries()
	input := buildRenderHostileInput(canaries)
	rep, err := analyzer.Analyze("job-render-1", input)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	surfaces := []struct {
		name string
		v    any
	}{
		{"Ecosystem.WorkflowFingerprints", rep.Ecosystem.WorkflowFingerprints},
		{"Ecosystem.ToolingUtilization", rep.Ecosystem.ToolingUtilization},
		{"Findings", rep.Findings},
	}
	for _, surface := range surfaces {
		b, err := json.Marshal(surface.v)
		if err != nil {
			t.Fatalf("marshal %s: %v", surface.name, err)
		}
		s := string(b)
		for _, c := range canaries {
			if strings.Contains(s, c) {
				t.Errorf("forbidden canary %q leaked into renderer-input surface %s; if a new field was added to a type the renderer reads, update this test and audit the new field for raw user data", c, surface.name)
			}
		}
	}
}

// TestPruningAdviceRecommendations_BytewiseEqualToConstants drives
// analyzer.Analyze over fixtures that produce each of the four *_bloat_*
// findings and asserts the Recommendation field is byte-for-byte equal to the
// literal string at internal/analyzer/analyzer.go:381-393.
//
// The fixture coverage:
//   - mcp_bloat_severe   ← testdata/tooling/04-many-low-util-degraded.log
//   - mcp_bloat_high     ← testdata/tooling/03-many-low-util.log
//   - skill_bloat_high   ← testdata/tooling/05-skill-bloat.log
//   - skill_bloat_severe ← 05-skill-bloat.log + appended degradation triggers
//     (consecutive error/traceback lines drive RetryDepthMax ≥ 3, which is
//     all the severe gate adds on top of the existing high gate per
//     tooling_classify.go:208).
//
// If any reword lands in analyzer.go without also updating the constants
// above, this test fails and forces a contemporaneous NFR-007 re-review.
func TestPruningAdviceRecommendations_BytewiseEqualToConstants(t *testing.T) {
	cases := []struct {
		name        string
		fixturePath string
		appendBytes []byte
		findingID   string
		wantText    string
	}{
		{
			name:        "mcp_bloat_severe",
			fixturePath: filepath.Join("testdata", "tooling", "04-many-low-util-degraded.log"),
			findingID:   "mcp_bloat_severe",
			wantText:    adviceMCPSevere,
		},
		{
			name:        "mcp_bloat_high",
			fixturePath: filepath.Join("testdata", "tooling", "03-many-low-util.log"),
			findingID:   "mcp_bloat_high",
			wantText:    adviceMCPHigh,
		},
		{
			name:        "skill_bloat_high",
			fixturePath: filepath.Join("testdata", "tooling", "05-skill-bloat.log"),
			findingID:   "skill_bloat_high",
			wantText:    adviceSkillHigh,
		},
		{
			name:        "skill_bloat_severe",
			fixturePath: filepath.Join("testdata", "tooling", "05-skill-bloat.log"),
			appendBytes: degradationSuffix(),
			findingID:   "skill_bloat_severe",
			wantText:    adviceSkillSevere,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.fixturePath)
			if err != nil {
				t.Fatalf("read fixture %s: %v", tc.fixturePath, err)
			}
			if len(tc.appendBytes) > 0 {
				data = append(data, '\n')
				data = append(data, tc.appendBytes...)
			}
			rep, err := analyzer.Analyze("advice-"+tc.name, data)
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			var got *analyzer.Finding
			for i := range rep.Findings {
				if rep.Findings[i].ID == tc.findingID {
					got = &rep.Findings[i]
					break
				}
			}
			if got == nil {
				t.Fatalf("fixture did not emit finding %s; ids=%v", tc.findingID, findingIDs(rep))
			}
			if got.Recommendation != tc.wantText {
				t.Errorf("Recommendation for %s drifted from the constant; if you intentionally rewrote the advice copy in analyzer.go, update the matching const at the top of this file and re-audit for NFR-007. got=%q want=%q", tc.findingID, got.Recommendation, tc.wantText)
			}
		})
	}
}

// TestBandFindingPairings_DrivenByAnalyze pins the band→finding-ID contract
// the UI lookup depends on. For each fixture we run Analyze and then assert
// the per-surface invariants:
//
//   - band ∈ {severe, high} → exactly one *_bloat_* finding for that surface
//     with the band-matching ID (INV-6: each ID emitted at most once).
//   - band ∈ {watch, normal, unknown} → zero *_bloat_* findings for that
//     surface.
//
// "Exactly one" matters because the UI uses .find() (first match); a future
// duplicate emission would silently hide a row.
func TestBandFindingPairings_DrivenByAnalyze(t *testing.T) {
	cases := []struct {
		name        string
		fixturePath string
		appendBytes []byte
	}{
		{name: "00-empty", fixturePath: filepath.Join("testdata", "tooling", "00-empty.log")},
		{name: "01-healthy-small", fixturePath: filepath.Join("testdata", "tooling", "01-healthy-small.log")},
		{name: "03-many-low-util", fixturePath: filepath.Join("testdata", "tooling", "03-many-low-util.log")},
		{name: "04-many-low-util-degraded", fixturePath: filepath.Join("testdata", "tooling", "04-many-low-util-degraded.log")},
		{name: "05-skill-bloat", fixturePath: filepath.Join("testdata", "tooling", "05-skill-bloat.log")},
		{name: "05-skill-bloat+degraded", fixturePath: filepath.Join("testdata", "tooling", "05-skill-bloat.log"), appendBytes: degradationSuffix()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.fixturePath)
			if err != nil {
				t.Fatalf("read fixture %s: %v", tc.fixturePath, err)
			}
			if len(tc.appendBytes) > 0 {
				data = append(data, '\n')
				data = append(data, tc.appendBytes...)
			}
			rep, err := analyzer.Analyze("pair-"+tc.name, data)
			if err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			assertBandPairing(t, rep, "mcp", rep.Ecosystem.ToolingUtilization.MCP.WarningBand)
			assertBandPairing(t, rep, "skill", rep.Ecosystem.ToolingUtilization.Skill.WarningBand)
		})
	}
}

func assertBandPairing(t *testing.T, rep analyzer.Report, surface, band string) {
	t.Helper()
	prefix := surface + "_bloat_"
	// Count findings whose ID starts with the surface prefix.
	hits := map[string]int{}
	byID := map[string]analyzer.Finding{}
	for _, f := range rep.Findings {
		if strings.HasPrefix(f.ID, prefix) {
			hits[f.ID]++
			byID[f.ID] = f
		}
	}
	totalHits := 0
	for _, n := range hits {
		totalHits += n
	}
	switch band {
	case analyzer.WarningBandSevere:
		wantID := prefix + "severe"
		if hits[wantID] != 1 {
			t.Errorf("%s band=severe: want exactly one %s finding, got %d (all %s_bloat_* hits=%v)", surface, wantID, hits[wantID], surface, hits)
		}
		if totalHits != 1 {
			t.Errorf("%s band=severe: want exactly one %s_bloat_* finding total, got %d (hits=%v)", surface, surface, totalHits, hits)
		}
		assertBloatEvidenceBand(t, byID[wantID], analyzer.WarningBandSevere)
	case analyzer.WarningBandHigh:
		wantID := prefix + "high"
		if hits[wantID] != 1 {
			t.Errorf("%s band=high: want exactly one %s finding, got %d (all %s_bloat_* hits=%v)", surface, wantID, hits[wantID], surface, hits)
		}
		if totalHits != 1 {
			t.Errorf("%s band=high: want exactly one %s_bloat_* finding total, got %d (hits=%v)", surface, surface, totalHits, hits)
		}
		assertBloatEvidenceBand(t, byID[wantID], analyzer.WarningBandHigh)
	case analyzer.WarningBandWatch, analyzer.WarningBandNormal, analyzer.WarningBandUnknown:
		if totalHits != 0 {
			t.Errorf("%s band=%s: want zero %s_bloat_* findings, got %d (hits=%v)", surface, band, surface, totalHits, hits)
		}
	default:
		t.Errorf("%s: unexpected warning band %q (not one of severe/high/watch/normal/unknown)", surface, band)
	}
}

func assertBloatEvidenceBand(t *testing.T, finding analyzer.Finding, band string) {
	t.Helper()
	want := "Bloat band: " + band
	if finding.Evidence.Description != want {
		t.Errorf("%s evidence description = %q, want %q", finding.ID, finding.Evidence.Description, want)
	}
}

func findingIDs(rep analyzer.Report) []string {
	ids := make([]string, 0, len(rep.Findings))
	for _, f := range rep.Findings {
		ids = append(ids, f.ID)
	}
	return ids
}

// degradationSuffix returns a synthetic transcript fragment that drives the
// degradation signal (RetryDepthMax ≥ 3) used by the analyzer's classifier
// to pick severe over high. We emit a sequence of tool error lines so
// consecutiveErrors crosses the threshold; see analyzer.go:257-263 for the
// retry-depth accumulator and tooling_classify.go:195/208 for the gate.
//
// We do NOT include any forbidden canary content here — these are bland
// error markers only.
func degradationSuffix() []byte {
	lines := []string{
		`{"type":"tool","tool_output":"error: synthetic degradation marker 1","is_error":true}`,
		`{"type":"tool","tool_output":"error: synthetic degradation marker 2","is_error":true}`,
		`{"type":"tool","tool_output":"error: synthetic degradation marker 3","is_error":true}`,
		`{"type":"tool","tool_output":"error: synthetic degradation marker 4","is_error":true}`,
	}
	return []byte(strings.Join(lines, "\n"))
}
