package analyzer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoldenSampleReport(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "sample-claude.jsonl")
	golden := filepath.Join("..", "..", "testdata", "golden", "sample-report.json")
	input, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	report, err := Analyze("job-golden-sample", input)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Validate the bounded shape of any emitted fingerprints (NFR-003)
	// before normalization — if any entry is malformed, the test should
	// fail loudly rather than silently passing after the slice is wiped.
	// The seven-field cap on EcosystemFingerprint is enforced structurally
	// by internal/analyzer/sdd/structural_test.go.
	for i, fp := range report.Ecosystem.WorkflowFingerprints {
		if fp.ID == "" {
			t.Errorf("fingerprint %d: empty ID", i)
		}
		if fp.Confidence == "" {
			t.Errorf("fingerprint %d (%q): empty Confidence", i, fp.ID)
		}
		if len(fp.Sources) == 0 {
			t.Errorf("fingerprint %d (%q): empty Sources", i, fp.ID)
		}
	}

	// Normalize away the entire WorkflowFingerprints slice before golden
	// comparison. The SDD evaluator's emission is gated on which CLI
	// binaries the host happens to have installed (sdd.NewRealProbe walks
	// $PATH for spec-kitty / openspec / etc.), so neither the slice
	// contents nor its presence is reproducible across environments —
	// the same fixture yields zero fingerprints inside a clean container
	// and two fingerprints on a developer laptop with spec-kitty installed.
	// Per-field scrubbing isn't enough: Sources and EvidenceCount also
	// depend on which markers fired, which is environment-dependent here.
	// Setting the slice to nil makes the json `omitempty` tag drop the
	// field entirely, yielding a deterministic golden artifact. The SDD
	// evaluator's behavior is covered exhaustively by unit tests in
	// internal/analyzer/sdd/.
	//
	// Both Ecosystem.WorkflowFingerprints (the report-level copy) and
	// AggregateEvent.Ecosystem.WorkflowFingerprints (the aggregate copy)
	// must be cleared; the aggregator deep-copies from the same source.
	report.Ecosystem.WorkflowFingerprints = nil
	report.AggregateEvent.Ecosystem.WorkflowFingerprints = nil

	actual, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	actual = append(actual, '\n')

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(golden, actual, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}

	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("golden report mismatch; run UPDATE_GOLDEN=1 go test ./internal/analyzer -run TestGoldenSampleReport")
	}
}

func TestAnalyzeRejectsEmptyUpload(t *testing.T) {
	if _, err := Analyze("job-empty", []byte(" \n\t ")); err == nil {
		t.Fatal("expected empty upload error")
	}
}

func TestAnalyzeHandlesMalformedJSONLAsText(t *testing.T) {
	report, err := Analyze("job-malformed", []byte("{not-json\ncommand: cat src/auth.ts\ncommand: cat src/auth.ts"))
	if err != nil {
		t.Fatalf("Analyze should fall back to text parsing: %v", err)
	}
	if report.Metrics.Turns != 3 {
		t.Fatalf("unexpected turn count: %#v", report.Metrics)
	}
	if report.Metrics.Rereads == 0 {
		t.Fatalf("expected repeated file read detection: %#v", report.Metrics)
	}
}

// TestGoldenToolingFixtures pins the deterministic MCP/skill bloat pipeline
// against the seven synthetic fixtures under testdata/tooling/. Each fixture
// exercises one row of the data-model.md §Synthetic Fixtures matrix. The
// classifier from WP03 is deterministic — when an assertion changes, fix the
// fixture, not the assertion.
func TestGoldenToolingFixtures(t *testing.T) {
	cases := []struct {
		name                     string
		path                     string
		wantMCPBand              string // "" = don't assert
		wantSkillBand            string
		assertMCPExposure        bool
		wantMCPExposure          bool
		assertSkillExposure      bool
		wantSkillExposure        bool
		wantImmediateFixContains []string // substrings expected somewhere in ImmediateFixes
	}{
		{
			name:                "00-empty",
			path:                "testdata/tooling/00-empty.log",
			wantMCPBand:         WarningBandUnknown,
			wantSkillBand:       WarningBandUnknown,
			assertMCPExposure:   true,
			wantMCPExposure:     false,
			assertSkillExposure: true,
			wantSkillExposure:   false,
		},
		{
			name:                "01-healthy-small",
			path:                "testdata/tooling/01-healthy-small.log",
			wantMCPBand:         WarningBandNormal,
			wantSkillBand:       WarningBandNormal,
			assertMCPExposure:   true,
			wantMCPExposure:     true,
			assertSkillExposure: true,
			wantSkillExposure:   true,
		},
		{
			name:        "02-many-high-util",
			path:        "testdata/tooling/02-many-high-util.log",
			wantMCPBand: WarningBandNormal,
		},
		{
			name:        "03-many-low-util",
			path:        "testdata/tooling/03-many-low-util.log",
			wantMCPBand: WarningBandHigh,
			wantImmediateFixContains: []string{
				"Scope project-specific MCPs",
			},
		},
		{
			name:        "04-many-low-util-degraded",
			path:        "testdata/tooling/04-many-low-util-degraded.log",
			wantMCPBand: WarningBandSevere,
			wantImmediateFixContains: []string{
				"Disable unused MCP servers",
				"lazy-load",
			},
		},
		{
			name:          "05-skill-bloat",
			path:          "testdata/tooling/05-skill-bloat.log",
			wantSkillBand: WarningBandHigh,
			wantImmediateFixContains: []string{
				"general skills from project-specific",
			},
		},
		{
			name:                "06-private-only",
			path:                "testdata/tooling/06-private-only.log",
			wantMCPBand:         WarningBandHigh,
			wantSkillBand:       WarningBandHigh,
			assertMCPExposure:   true,
			wantMCPExposure:     true,
			assertSkillExposure: true,
			wantSkillExposure:   true,
		},
		{
			name: "07-mixed-known-unknown",
			path: "testdata/tooling/07-mixed-known-unknown.log",
			// Bands vary based on count distribution. The load-bearing
			// assertion here is that path-shaped tokens (/etc/passwd,
			// /var/log/syslog, /home/user/file.txt) are NOT counted as
			// skill executions — see in-body check below.
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			report, err := Analyze("test-"+tc.name, data)
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			mcp := report.Ecosystem.ToolingUtilization.MCP
			skill := report.Ecosystem.ToolingUtilization.Skill
			if tc.wantMCPBand != "" && mcp.WarningBand != tc.wantMCPBand {
				t.Errorf("mcp warning_band: got %q want %q (ratio=%d server_bucket=%s tool_bucket=%s token_bucket=%s)",
					mcp.WarningBand, tc.wantMCPBand, mcp.UtilizationRatioPct,
					mcp.ServerCountBucket, mcp.ExposedToolCountBucket, mcp.ContextTokenBucket)
			}
			if tc.wantSkillBand != "" && skill.WarningBand != tc.wantSkillBand {
				t.Errorf("skill warning_band: got %q want %q (ratio=%d count_bucket=%s token_bucket=%s)",
					skill.WarningBand, tc.wantSkillBand, skill.UtilizationRatioPct,
					skill.ExposedCountBucket, skill.ContextTokenBucket)
			}
			if tc.assertMCPExposure && mcp.ExposureKnown != tc.wantMCPExposure {
				t.Errorf("mcp exposure_known: got %v want %v", mcp.ExposureKnown, tc.wantMCPExposure)
			}
			if tc.assertSkillExposure && skill.ExposureKnown != tc.wantSkillExposure {
				t.Errorf("skill exposure_known: got %v want %v", skill.ExposureKnown, tc.wantSkillExposure)
			}
			for _, want := range tc.wantImmediateFixContains {
				found := false
				for _, fix := range report.ImmediateFixes {
					if strings.Contains(fix, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected immediate_fixes to contain %q; got %v", want, report.ImmediateFixes)
				}
			}
			// Fixture-specific structural assertions.
			switch tc.name {
			case "01-healthy-small":
				// The two known MCP IDs and one known skill must be surfaced
				// by lowercase canonical form.
				if !containsAll(mcp.KnownServerIDs, []string{"github", "linear"}) {
					t.Errorf("known mcp server IDs missing github/linear: %v", mcp.KnownServerIDs)
				}
				if !containsAll(skill.KnownExposedIDs, []string{"review", "qa"}) {
					t.Errorf("known skill IDs missing review/qa: %v", skill.KnownExposedIDs)
				}
				if len(skill.KnownExecutedIDs) == 0 {
					t.Errorf("expected at least one executed skill, got none")
				}
			case "07-mixed-known-unknown":
				// Allowlist IDs are emitted by their canonical names.
				if !containsAll(mcp.KnownServerIDs, []string{"github", "notion"}) {
					t.Errorf("known mcp server IDs missing github/notion: %v", mcp.KnownServerIDs)
				}
				// Path-shaped slash tokens must NOT count as skill executions.
				// The fixture mentions /etc/passwd, /var/log/syslog, and
				// /home/user/file.txt — all should be filtered out by the
				// path-avoidance branch in detectSkillExecutionsFromLines.
				// Only "/review" (if present) would qualify. The fixture has
				// no /skill invocations, so executed_count must stay at zero.
				if skill.ExecutedCount != 0 {
					t.Errorf("path-shaped tokens leaked into skill executions: %d", skill.ExecutedCount)
				}
			}
		})
	}
}

// containsAll reports whether every want value appears in have.
func containsAll(have []string, want []string) bool {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// TestPrivacyLeakCorpus is the load-bearing privacy enforcement for the
// mission: it proves that the worst-case private/synthetic inputs flow
// through Analyze() and emerge as a Report (and an AggregateSafeEvent)
// whose serialized JSON contains zero substring matches for any private
// name, path, schema fragment, or fake credential token used in the input.
//
// Forbidden-substring lists are kept identical to the synthetic names in
// the corresponding fixtures from T019. If a future contributor adds a
// field that leaks an unknown name, this test fails loudly.
func TestPrivacyLeakCorpus(t *testing.T) {
	cases := []struct {
		name          string
		path          string
		forbiddenSubs []string
	}{
		{
			name: "06-private-only",
			path: "testdata/tooling/06-private-only.log",
			forbiddenSubs: []string{
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
				"AKIA", // scrubber replaces matching tokens with [REDACTED-AWS-ACCESS-KEY]
				"fake schema description",
				"fake skill description",
			},
		},
		{
			name: "07-mixed-known-unknown",
			path: "testdata/tooling/07-mixed-known-unknown.log",
			forbiddenSubs: []string{
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
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.forbiddenSubs) < 5 {
				t.Fatalf("test contract: forbidden-substring list must have >= 5 entries (got %d)", len(tc.forbiddenSubs))
			}
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			report, err := Analyze("privacy-"+tc.name, data)
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			reportJSON, err := json.Marshal(report)
			if err != nil {
				t.Fatalf("marshal report: %v", err)
			}
			for _, sub := range tc.forbiddenSubs {
				if bytes.Contains(reportJSON, []byte(sub)) {
					t.Errorf("Report JSON leaks forbidden substring %q (privacy violation)", sub)
				}
			}
			aggJSON, err := json.Marshal(report.AggregateEvent)
			if err != nil {
				t.Fatalf("marshal aggregate: %v", err)
			}
			for _, sub := range tc.forbiddenSubs {
				if bytes.Contains(aggJSON, []byte(sub)) {
					t.Errorf("AggregateSafeEvent JSON leaks forbidden substring %q (privacy violation)", sub)
				}
			}
		})
	}
}
