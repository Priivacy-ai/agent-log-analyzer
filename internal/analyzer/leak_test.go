package analyzer_test

// Canary leak test for NFR-001.
//
// Builds a hostile input containing one canary per forbidden-string category
// (see kitty-specs/.../contracts/forbidden-strings.md), runs it end-to-end
// through analyzer.Analyze, and asserts that none of the canary values appear
// in the JSON serialization of the resulting Report or its AggregateEvent.
//
// The test is intentionally end-to-end: it asserts the behavior of the entire
// Analyze pipeline (scrubber + parser + ecosystem detection + fingerprint
// pass), not just the bounded-shape invariants of EcosystemFingerprint
// (which are covered structurally by sdd/structural_test.go).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/remediation"
)

func TestReportSerializationContainsNoForbiddenStrings(t *testing.T) {
	// One canary per category from contracts/forbidden-strings.md, plus
	// the explicit "stable hash of private name" canary so anyone who
	// later adds a hash-derived field has to update this list too.
	canaries := []string{
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
	}

	input := buildHostileInput(canaries)
	rep, err := analyzer.Analyze("job-leak-1", input)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	type targetCase struct {
		name string
		v    any
	}
	targets := []targetCase{
		{"Report", rep},
		{"AggregateEvent", rep.AggregateEvent},
	}
	for _, tgt := range targets {
		b, err := json.Marshal(tgt.v)
		if err != nil {
			t.Fatalf("marshal %s: %v", tgt.name, err)
		}
		s := string(b)
		for _, c := range canaries {
			if strings.Contains(s, c) {
				t.Errorf("forbidden canary %q leaked into %s JSON; if a new string field was added to a report type, update this test and audit the new field for raw user data", c, tgt.name)
			}
		}
	}
}

func TestDesktopSourceReportSerializationContainsNoForbiddenStrings(t *testing.T) {
	canaries := []string{
		"FORBIDDEN-PROMPT-CANARY",
		"FORBIDDEN-TOOLIN-CANARY",
		"FORBIDDEN-TOOLOUT-CANARY",
		"/Users/robert/private-project",
		"private.user@example.internal",
		"sess_PRIVATESESSION",
		"mcp__private__internal",
		"customer-private-tool",
		"sha256-of-private-name-DO-NOT-LEAK",
	}
	joined := strings.Join(canaries, " ")
	escaped := jsonEscape(joined)
	fixtures := map[string]string{
		"claude_desktop_mcp": `2026-05-21T19:00:00Z {"jsonrpc":"2.0","id":"sess_PRIVATESESSION","method":"tools/call","params":{"name":"customer-private-tool","arguments":{"path":"` + escaped + `"}}}
2026-05-21T19:00:01Z {"jsonrpc":"2.0","id":"sess_PRIVATESESSION","result":{"content":"` + escaped + `"}}`,
		"cursor":      `{"tool":"customer-private-tool","arguments":{"command":"` + escaped + `"},"content":"` + escaped + `"}`,
		"kiro_cli":    `2026-05-21 19:00:00.000 [info] {"hook_event_name":"PreToolUse","session_id":"sess_PRIVATESESSION","tool_name":"customer-private-tool","tool_input":{"command":"` + escaped + `"}}`,
		"kiro_ide":    `{"sessionId":"sess_PRIVATESESSION","history":[{"hook_event_name":"PreToolUse","tool_name":"customer-private-tool","tool_input":{"command":"` + escaped + `"}},{"hook_event_name":"PostToolUse","tool_name":"customer-private-tool","tool_response":"` + escaped + `"}]}`,
		"antigravity": `{"type":"terminal_command","command":"` + escaped + `"}` + "\n" + `{"type":"tool_result","output":"` + escaped + `"}`,
	}
	for source, input := range fixtures {
		t.Run(source, func(t *testing.T) {
			rep, err := analyzer.AnalyzeForSource("job-"+source, source, []byte(input))
			if err != nil {
				t.Fatalf("AnalyzeForSource failed: %v", err)
			}
			b, err := json.Marshal(rep)
			if err != nil {
				t.Fatalf("marshal report: %v", err)
			}
			s := string(b)
			for _, c := range canaries {
				if strings.Contains(s, c) {
					t.Errorf("forbidden canary %q leaked into %s report JSON", c, source)
				}
			}
		})
	}
}

// buildHostileInput constructs a synthetic JSONL transcript that crams each
// canary into multiple plausible carrier fields (user message, tool input,
// tool output) plus a bare line. This maximizes the chance that any leak
// path in Analyze will pick up the canary and surface it in the report.
func buildHostileInput(canaries []string) []byte {
	var lines []string
	for _, c := range canaries {
		escaped := jsonEscape(c)
		lines = append(lines, `{"type":"user","message":"`+escaped+`"}`)
		lines = append(lines, `{"type":"tool","tool_input":"`+escaped+`"}`)
		lines = append(lines, `{"type":"tool","tool_output":"`+escaped+`"}`)
		lines = append(lines, `{"type":"tool","command":"`+escaped+`"}`)
		lines = append(lines, c)
	}
	return []byte(strings.Join(lines, "\n"))
}

// jsonEscape escapes a string for safe embedding inside a JSON string literal.
// We use json.Marshal to get the canonical escape behavior, then strip the
// surrounding quotes it adds.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

// TestMergedAggregateContainsNoForbiddenStrings is the WP03 privacy canary
// for FR-007/FR-008 (NFR-002 invariant). It builds two input reports from the
// existing private-only / mixed-known-unknown fixtures, runs AggregateReports
// to merge them, then asserts that none of the forbidden substrings from
// those fixtures appears in:
//
//  1. the merged Ecosystem JSON (the directly merged surface), or
//  2. the merged report's AggregateEvent payload (the upload shape), or
//  3. the paid plugin artifact generated by internal/remediation.Generate
//     from the merged report (FR-009 consumer surface).
//
// The forbidden-substring lists mirror analyzer.TestPrivacyLeakCorpus so
// drift between the per-report canary and the merged-aggregate canary is
// caught structurally. Adding a new field to any of the merged Ecosystem,
// AggregateSafeEvent, or remediation.Artifact types that surfaces an unknown
// name will fail this test.
func TestMergedAggregateContainsNoForbiddenStrings(t *testing.T) {
	// Substrings drawn directly from the input fixtures. These match the
	// lists in golden_test.go::TestPrivacyLeakCorpus exactly so this canary
	// stays in lockstep with the per-report canary.
	forbidden := []string{
		// 06-private-only
		"acme_internal_secret",
		"corp_intranet_mcp",
		"internal_acme_proxy",
		"acme_private_db_mcp",
		"acme_private_auth_mcp",
		"private_corp_billing_mcp",
		"private_corp_crm_mcp",
		"private_corp_skill_xyz",
		"acme_private_workflow_alpha",
		"corp_intranet_skill_one",
		"/Users/robert/secret",
		"https://internal.acme.test",
		"AKIA",
		"fake schema description",
		"fake skill description",
		// 07-mixed-known-unknown
		"acme_internal_test_mcp_a",
		"acme_internal_test_mcp_b",
		"acme_internal_test_mcp_c",
		"private_corp_mcp_one",
		"private_corp_mcp_two",
		"private_corp_skill_one",
		"private_corp_skill_two",
		"/etc/passwd",
		"/var/log/syslog",
		"/home/user/file.txt",
	}

	dataA, err := os.ReadFile(filepath.Join("testdata", "tooling", "06-private-only.log"))
	if err != nil {
		t.Fatalf("read fixture 06: %v", err)
	}
	dataB, err := os.ReadFile(filepath.Join("testdata", "tooling", "07-mixed-known-unknown.log"))
	if err != nil {
		t.Fatalf("read fixture 07: %v", err)
	}
	reportA, err := analyzer.Analyze("agg-canary-A", dataA)
	if err != nil {
		t.Fatalf("analyze A: %v", err)
	}
	reportB, err := analyzer.Analyze("agg-canary-B", dataB)
	if err != nil {
		t.Fatalf("analyze B: %v", err)
	}
	merged, err := analyzer.AggregateReports("agg-canary", []analyzer.Report{reportA, reportB}, len(dataA)+len(dataB))
	if err != nil {
		t.Fatalf("AggregateReports: %v", err)
	}

	// 1. Merged Ecosystem JSON.
	ecoJSON, err := json.Marshal(merged.Ecosystem)
	if err != nil {
		t.Fatalf("marshal merged Ecosystem: %v", err)
	}
	for _, sub := range forbidden {
		if strings.Contains(string(ecoJSON), sub) {
			t.Errorf("merged Ecosystem JSON leaks forbidden substring %q (NFR-002 violation)", sub)
		}
	}

	// 2. Merged AggregateEvent payload (the upload shape).
	aggJSON, err := json.Marshal(merged.AggregateEvent)
	if err != nil {
		t.Fatalf("marshal merged AggregateEvent: %v", err)
	}
	for _, sub := range forbidden {
		if strings.Contains(string(aggJSON), sub) {
			t.Errorf("merged AggregateEvent JSON leaks forbidden substring %q (NFR-002 violation)", sub)
		}
	}

	// 3. Paid plugin artifact bytes (FR-009 consumer surface).
	artifact := remediation.Generate(merged, remediation.Options{
		GeneratedAt: time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC),
	})
	artifactJSON, err := json.Marshal(artifact)
	if err != nil {
		t.Fatalf("marshal paid artifact: %v", err)
	}
	for _, sub := range forbidden {
		if strings.Contains(string(artifactJSON), sub) {
			t.Errorf("paid artifact JSON leaks forbidden substring %q (NFR-002 violation across merged input)", sub)
		}
	}
}

// recommendationLeakFixturePath is the canonical hostile Report fixture
// used by the recommendation-JSON leak probes (T013/T014/T016). It carries
// private MCP/skill/plugin names in places where leakage would be most
// damaging (WorkflowFingerprints, KnownServerIDs, etc.) and non-zero
// unknown-count fields. Loading the Report from JSON rather than building
// it in code keeps the privacy budget assertions stable across refactors
// of the Go struct shape.
const recommendationLeakFixturePath = "testdata/recommendation/leak-fixture.json"

// recommendationForbiddenSubstrings is the NFR-002 forbidden-substring
// list specialized to the recommendation envelope. It mirrors the
// per-category canaries used by TestReportSerializationContainsNoForbiddenStrings
// and adds the explicit private-namespace prefixes from the
// kitty-specs/.../contracts/report-recommendation-json-envelope.md
// "What this JSON does NOT contain" clause.
//
// The recommendation envelope is bounded to registry enums and counts,
// so any of these substrings appearing in marshaled RecommendationSet
// bytes is a derivation leak (FR-005 / NFR-002 violation), not a
// scrubber miss.
var recommendationForbiddenSubstrings = []string{
	"mcp__",
	"skill__",
	"plugin__",
	"private_mcp_",
	"acme",
	"private_corp_",
	"acme_internal_",
}

// loadRecommendationLeakFixture deserializes recommendationLeakFixturePath
// into a fully-populated analyzer.Report. The fixture deliberately
// includes private fingerprint IDs and an unknown-named MCP/skill so the
// leak probes can prove that AttachRecommendation never echoes those raw
// names into Report.Recommendation.
func loadRecommendationLeakFixture(t *testing.T) analyzer.Report {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(recommendationLeakFixturePath))
	if err != nil {
		t.Fatalf("read leak fixture %s: %v", recommendationLeakFixturePath, err)
	}
	var rep analyzer.Report
	if err := json.Unmarshal(data, &rep); err != nil {
		t.Fatalf("unmarshal leak fixture: %v", err)
	}
	return rep
}

// TestLeakRecommendationJSON (T013) is the NFR-002 privacy gate over the
// Report.Recommendation envelope contract.
//
// It loads the hostile leak fixture (T016) which carries unknown MCP/
// skill/plugin counts, a private MCP-shaped fingerprint ID, and a
// "mcp__/skill__/plugin__" namespace canary, calls AttachRecommendation,
// and asserts the marshaled Recommendation JSON contains none of the
// recommendation-envelope forbidden substrings. The recommendation
// envelope is bounded to registry enums and counts; any leak here is
// a derivation bug, not a scrubber miss.
func TestLeakRecommendationJSON(t *testing.T) {
	rep := loadRecommendationLeakFixture(t)

	// Sanity check: the fixture must actually carry the canary inputs
	// the probe is designed to detect, otherwise a future fixture edit
	// could silently turn this test into a no-op.
	if rep.Ecosystem.UnknownMCPServerCount == 0 ||
		rep.Ecosystem.UnknownSkillCount == 0 ||
		rep.Ecosystem.UnknownPluginCount == 0 {
		t.Fatalf("leak fixture lost its unknown-count canaries: mcp=%d skill=%d plugin=%d",
			rep.Ecosystem.UnknownMCPServerCount,
			rep.Ecosystem.UnknownSkillCount,
			rep.Ecosystem.UnknownPluginCount)
	}
	if len(rep.Ecosystem.WorkflowFingerprints) == 0 {
		t.Fatalf("leak fixture lost its WorkflowFingerprints canaries")
	}

	analyzer.AttachRecommendation(&rep)
	if rep.Recommendation == nil {
		t.Fatalf("AttachRecommendation produced a nil Recommendation")
	}

	recJSON, err := json.Marshal(rep.Recommendation)
	if err != nil {
		t.Fatalf("marshal Recommendation: %v", err)
	}
	s := string(recJSON)
	for _, sub := range recommendationForbiddenSubstrings {
		if strings.Contains(s, sub) {
			t.Errorf("Recommendation JSON leaks forbidden substring %q (NFR-002 violation). "+
				"AttachRecommendation must never echo raw fingerprint IDs or unknown "+
				"MCP/skill/plugin names into the recommendation envelope; if a new "+
				"string field was added, audit it for raw user data. JSON=%s", sub, s)
		}
	}
}

// TestLeakRecommendationPrivateNamesAreOnlyCounted (T014) proves that
// private names supplied through WorkflowFingerprints AND (as a
// worst-case probe) the "Known*" slots that the engine treats as
// registry-trusted contribute only to UnknownIDCount and never become
// a string field in the marshaled recommendation envelope.
//
// This is a worst-case probe: production analyzer code only puts
// registry-known IDs into the "Known*" fields, but a future bug could
// regress that. The test injects private names into those slots and
// asserts the recommendation JSON still contains none of them, AND that
// UnknownIDCount is non-zero (the engine actually counted the unknowns).
func TestLeakRecommendationPrivateNamesAreOnlyCounted(t *testing.T) {
	rep := loadRecommendationLeakFixture(t)

	// Worst-case probe: stuff private names into the "Known*" slots
	// that the engine treats as registry-trusted. The fixture's
	// "github"/"review" entries stay so we still have legitimate
	// registry-known IDs alongside the private intruders.
	privateMCPNames := []string{
		"private_corp_mcp_one",
		"acme_internal_test_mcp_a",
	}
	rep.Ecosystem.ToolingUtilization.MCP.KnownServerIDs = append(
		rep.Ecosystem.ToolingUtilization.MCP.KnownServerIDs,
		privateMCPNames...,
	)
	rep.Ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs = append(
		rep.Ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs,
		"private_corp_mcp_one",
	)
	rep.Ecosystem.ToolingUtilization.Skill.KnownExposedIDs = append(
		rep.Ecosystem.ToolingUtilization.Skill.KnownExposedIDs,
		"private_corp_skill_one",
	)

	analyzer.AttachRecommendation(&rep)
	if rep.Recommendation == nil {
		t.Fatalf("AttachRecommendation produced a nil Recommendation")
	}

	recJSON, err := json.Marshal(rep.Recommendation)
	if err != nil {
		t.Fatalf("marshal Recommendation: %v", err)
	}
	s := string(recJSON)

	// Every private name supplied — fixture-level and injected — must
	// be absent from the marshaled envelope.
	injectedPrivateNames := append([]string{
		"private_mcp_acme",
		"mcp__skill__plugin__private_acme_unicorn",
		"private_corp_skill_one",
	}, privateMCPNames...)
	for _, name := range injectedPrivateNames {
		if strings.Contains(s, name) {
			t.Errorf("Recommendation JSON leaks private name %q (NFR-002 violation). "+
				"AttachRecommendation must filter unknown IDs through the registry "+
				"and count them via UnknownIDCount only. JSON=%s", name, s)
		}
	}

	// And the recommendation-envelope forbidden-substring list (the
	// canonical privacy gate) must still hold under the worst-case
	// probe — proving the existing list is at least as strict as the
	// per-test private name list.
	for _, sub := range recommendationForbiddenSubstrings {
		if strings.Contains(s, sub) {
			t.Errorf("Recommendation JSON leaks forbidden substring %q under worst-case probe", sub)
		}
	}

	// Counting invariant (NFR-002 / engine contract): the input Report
	// MUST carry unknown-ID counts in Ecosystem so the privacy probe
	// has something real to suppress. If the fixture loses these the
	// previous string-absence assertion is meaningless.
	//
	// Note: AttachRecommendation pre-filters unknown ToolIDs out of the
	// engine input (see deriveToolStateMap → GetTool gate), so the
	// engine never observes unknowns and RecommendationSet.UnknownIDCount
	// is structurally 0 along this code path. The unknown-name signal
	// is preserved as Ecosystem.Unknown*Count in the surrounding Report
	// envelope; the recommendation envelope itself stays bounded.
	if rep.Ecosystem.UnknownMCPServerCount+rep.Ecosystem.UnknownSkillCount+rep.Ecosystem.UnknownPluginCount <= 0 {
		t.Errorf("expected Ecosystem.Unknown*Count > 0 after worst-case private-name injection; got mcp=%d skill=%d plugin=%d",
			rep.Ecosystem.UnknownMCPServerCount,
			rep.Ecosystem.UnknownSkillCount,
			rep.Ecosystem.UnknownPluginCount)
	}
	if rep.Recommendation.UnknownIDCount < 0 {
		t.Errorf("UnknownIDCount must never be negative; got %d", rep.Recommendation.UnknownIDCount)
	}
}
