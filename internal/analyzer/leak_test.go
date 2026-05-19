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
	"strings"
	"testing"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
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
