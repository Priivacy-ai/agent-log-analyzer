package analyzer

import (
	"encoding/json"
	"reflect"
	"testing"
)

// -----------------------------------------------------------------------------
// deriveSignals — table-driven coverage of rules S-01..S-07.
// -----------------------------------------------------------------------------

func TestDeriveSignals(t *testing.T) {
	cases := []struct {
		name   string
		report Report
		want   []Signal
	}{
		{
			name: "S-01 tool_output_bloat",
			report: reportWithFindingsAndFingerprint(
				[]string{"tool_output_bloat"},
				activeUsageTrackerFingerprint(),
			),
			want: []Signal{SignalToolOutputBloat},
		},
		{
			name: "S-02 repeated_file_reads",
			report: reportWithFindingsAndFingerprint(
				[]string{"repeated_file_reads"},
				activeUsageTrackerFingerprint(),
			),
			want: []Signal{SignalRepeatedFileReads},
		},
		{
			name: "S-03 retry_loop",
			report: reportWithFindingsAndFingerprint(
				[]string{"retry_loop"},
				activeUsageTrackerFingerprint(),
			),
			want: []Signal{SignalRetryLoop},
		},
		{
			name: "S-04 context_growth_spikes",
			report: reportWithFindingsAndFingerprint(
				[]string{"context_growth_spikes"},
				activeUsageTrackerFingerprint(),
			),
			want: []Signal{SignalContextGrowthSpikes},
		},
		{
			name: "S-05 mcp band severe",
			report: func() Report {
				r := reportWithFindingsAndFingerprint(nil, activeUsageTrackerFingerprint())
				r.Ecosystem.ToolingUtilization.MCP.WarningBand = "severe"
				return r
			}(),
			want: []Signal{SignalMCPSkillBloat},
		},
		{
			name: "S-06 skill band high",
			report: func() Report {
				r := reportWithFindingsAndFingerprint(nil, activeUsageTrackerFingerprint())
				r.Ecosystem.ToolingUtilization.Skill.WarningBand = "high"
				return r
			}(),
			want: []Signal{SignalMCPSkillBloat},
		},
		{
			name:   "S-07 no active usage-visibility fingerprint",
			report: Report{},
			want:   []Signal{SignalNoUsageVisibility},
		},
		{
			name: "dedupe: MCP severe AND Skill high → single mcp_skill_bloat",
			report: func() Report {
				r := reportWithFindingsAndFingerprint(nil, activeUsageTrackerFingerprint())
				r.Ecosystem.ToolingUtilization.MCP.WarningBand = "severe"
				r.Ecosystem.ToolingUtilization.Skill.WarningBand = "high"
				return r
			}(),
			want: []Signal{SignalMCPSkillBloat},
		},
		{
			name: "dedupe: duplicate findings → single signal",
			report: reportWithFindingsAndFingerprint(
				[]string{"tool_output_bloat", "tool_output_bloat"},
				activeUsageTrackerFingerprint(),
			),
			want: []Signal{SignalToolOutputBloat},
		},
		{
			name: "active usage-visibility tracker suppresses S-07",
			report: reportWithFindingsAndFingerprint(
				nil,
				activeUsageTrackerFingerprint(),
			),
			want: nil,
		},
		{
			name: "unknown band does not fire S-05/S-06",
			report: func() Report {
				r := reportWithFindingsAndFingerprint(nil, activeUsageTrackerFingerprint())
				r.Ecosystem.ToolingUtilization.MCP.WarningBand = "watch"
				r.Ecosystem.ToolingUtilization.Skill.WarningBand = "normal"
				return r
			}(),
			want: nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := deriveSignals(&tc.report)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("deriveSignals: got %v, want %v", got, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// deriveToolStateMap — table-driven coverage of rules T-F-01..T-S-02.
// -----------------------------------------------------------------------------

func TestDeriveToolStateMap(t *testing.T) {
	type wantEntry struct {
		state   ToolState
		sources map[EvidenceSource]int
	}

	cases := []struct {
		name      string
		report    Report
		want      map[ToolID]wantEntry
		mustEmpty bool
	}{
		{
			name: "T-F-01 active fingerprint → active_high",
			report: Report{Ecosystem: Ecosystem{WorkflowFingerprints: []EcosystemFingerprint{
				// "cli_binary" is the canonical source enum emitted by the
				// SDD evaluator (internal/analyzer/sdd). Using the real
				// value catches the prior regression where the wiring
				// checked for "cli_probe" — a string the evaluator never
				// emits — and EvidenceCLIPresence was silently always 0.
				{ID: "ccusage", Active: true, Sources: []string{"cli_binary"}, VersionBucket: "recent"},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateActiveHigh,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
						EvidenceCLIPresence:   1,
						EvidenceCLIVersion:    1,
					},
				},
			},
		},
		{
			name: "T-F-01 cli_version_probe also counts as CLI presence",
			report: Report{Ecosystem: Ecosystem{WorkflowFingerprints: []EcosystemFingerprint{
				{ID: "ccusage", Active: true, Sources: []string{"cli_version_probe"}},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateActiveHigh,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
						EvidenceCLIPresence:   1,
					},
				},
			},
		},
		{
			name: "T-F-02 installed-inactive fingerprint → installed_medium",
			report: Report{Ecosystem: Ecosystem{WorkflowFingerprints: []EcosystemFingerprint{
				{ID: "ccusage", Active: false, Installed: true, Sources: []string{"cli_binary"}},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateInstalledMedium,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
						EvidenceCLIPresence:   1,
					},
				},
			},
		},
		{
			name: "T-F-03 mentioned-only fingerprint → mentioned_low",
			report: Report{Ecosystem: Ecosystem{WorkflowFingerprints: []EcosystemFingerprint{
				{ID: "ccusage", Active: false, Installed: false},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateMentionedLow,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
					},
				},
			},
		},
		{
			// T-W-01: ccusage detected via frameworks.json signature only
			// (no SDD fingerprint, no MCP, no skill record). Must surface
			// as mentioned_low so the engine can attribute report-mention
			// evidence rather than treating ccusage as totally absent.
			name: "T-W-01 WorkflowFrameworks mention → mentioned_low",
			report: Report{Ecosystem: Ecosystem{
				WorkflowFrameworks: []string{"ccusage"},
			}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateMentionedLow,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
					},
				},
			},
		},
		{
			// T-W-01 must not promote state when a stronger downstream
			// signal exists. Mention + active MCP → active_high.
			name: "T-W-01 framework mention loses to active MCP",
			report: Report{Ecosystem: Ecosystem{
				WorkflowFrameworks: []string{"ccusage"},
				ToolingUtilization: ToolingUtilization{
					MCP: MCPUtilization{
						KnownServerIDs:       []string{"ccusage"},
						UniqueKnownCalledIDs: []string{"ccusage"},
					},
				},
			}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateActiveHigh,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
						EvidenceMCPActive:     1,
					},
				},
			},
		},
		{
			// T-W-01 must ignore framework IDs not in the token-saving
			// registry (e.g. spec_kitty exists in frameworks.json but is
			// not a token-saving tool). No state entry should appear.
			name: "T-W-01 non-token-saving framework ignored",
			report: Report{Ecosystem: Ecosystem{
				WorkflowFrameworks: []string{"spec_kitty"},
			}},
			mustEmpty: true,
		},
		{
			name: "T-M-01 MCP active server",
			report: Report{Ecosystem: Ecosystem{ToolingUtilization: ToolingUtilization{
				MCP: MCPUtilization{
					KnownServerIDs:       []string{"ccusage"},
					UniqueKnownCalledIDs: []string{"ccusage"},
				},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateActiveHigh,
					sources: map[EvidenceSource]int{
						EvidenceMCPActive: 1,
					},
				},
			},
		},
		{
			name: "T-M-02 MCP configured-only server",
			report: Report{Ecosystem: Ecosystem{ToolingUtilization: ToolingUtilization{
				MCP: MCPUtilization{
					KnownServerIDs: []string{"ccusage"},
				},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateConfiguredMedium,
					sources: map[EvidenceSource]int{
						EvidenceMCPConfigured: 1,
					},
				},
			},
		},
		{
			name: "T-S-01 skill executed",
			report: Report{Ecosystem: Ecosystem{ToolingUtilization: ToolingUtilization{
				Skill: SkillUtilization{
					KnownExposedIDs:  []string{"ccusage"},
					KnownExecutedIDs: []string{"ccusage"},
				},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateActiveHigh,
					sources: map[EvidenceSource]int{
						EvidenceSkillConfigured: 1,
						EvidenceReportMention:   1,
					},
				},
			},
		},
		{
			name: "T-S-02 skill exposed but not executed",
			report: Report{Ecosystem: Ecosystem{ToolingUtilization: ToolingUtilization{
				Skill: SkillUtilization{
					KnownExposedIDs: []string{"ccusage"},
				},
			}}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateConfiguredMedium,
					sources: map[EvidenceSource]int{
						EvidenceSkillConfigured: 1,
					},
				},
			},
		},
		{
			name: "multi-source conflict: installed fingerprint + configured MCP → active_high precedence? No, configured_medium > installed_medium",
			report: Report{Ecosystem: Ecosystem{
				WorkflowFingerprints: []EcosystemFingerprint{
					{ID: "ccusage", Installed: true},
				},
				ToolingUtilization: ToolingUtilization{
					MCP: MCPUtilization{
						KnownServerIDs: []string{"ccusage"},
					},
				},
			}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateConfiguredMedium,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
						EvidenceMCPConfigured: 1,
					},
				},
			},
		},
		{
			name: "multi-source: active fingerprint + active MCP sums sources, state stays active_high",
			report: Report{Ecosystem: Ecosystem{
				WorkflowFingerprints: []EcosystemFingerprint{
					{ID: "ccusage", Active: true},
				},
				ToolingUtilization: ToolingUtilization{
					MCP: MCPUtilization{
						KnownServerIDs:       []string{"ccusage"},
						UniqueKnownCalledIDs: []string{"ccusage"},
					},
				},
			}},
			want: map[ToolID]wantEntry{
				"ccusage": {
					state: ToolStateActiveHigh,
					sources: map[EvidenceSource]int{
						EvidenceReportMention: 1,
						EvidenceMCPActive:     1,
					},
				},
			},
		},
		{
			name: "privacy: unknown MCP and skill IDs absent from map",
			report: Report{Ecosystem: Ecosystem{
				WorkflowFingerprints: []EcosystemFingerprint{
					{ID: "definitely_not_a_real_tool", Active: true},
				},
				ToolingUtilization: ToolingUtilization{
					MCP: MCPUtilization{
						KnownServerIDs: []string{"some-private-server"},
					},
					Skill: SkillUtilization{
						KnownExposedIDs: []string{"some-private-skill"},
					},
				},
				UnknownMCPServerCount: 3,
				UnknownSkillCount:     2,
			}},
			mustEmpty: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := deriveToolStateMap(&tc.report)
			if tc.mustEmpty {
				if len(got) != 0 {
					t.Fatalf("expected empty ToolStateMap, got %v", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("entry count mismatch: got %d (%v), want %d (%v)", len(got), got, len(tc.want), tc.want)
			}
			for id, w := range tc.want {
				e, ok := got[id]
				if !ok {
					t.Fatalf("missing entry for %s", id)
				}
				if e.State != w.state {
					t.Errorf("%s state: got %q, want %q", id, e.State, w.state)
				}
				if !reflect.DeepEqual(e.Sources, w.sources) {
					t.Errorf("%s sources: got %v, want %v", id, e.Sources, w.sources)
				}
				if e.Tool != id {
					t.Errorf("%s Tool field: got %q, want %q", id, e.Tool, id)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// AttachRecommendation — wiring behavior.
// -----------------------------------------------------------------------------

func TestAttachRecommendation(t *testing.T) {
	t.Run("nil pointer is no-op", func(t *testing.T) {
		AttachRecommendation(nil) // must not panic.
	})

	t.Run("empty Report attaches non-nil RecommendationSet", func(t *testing.T) {
		var r Report
		AttachRecommendation(&r)
		if r.Recommendation == nil {
			t.Fatalf("expected non-nil Recommendation")
		}
		if r.Recommendation.EngineVersion != EngineVersion() {
			t.Errorf("EngineVersion: got %q, want %q", r.Recommendation.EngineVersion, EngineVersion())
		}
	})

	t.Run("canonical fixture: tool_output_bloat + no usage tracker → Primary != nil", func(t *testing.T) {
		r := canonicalPrimaryFixture()
		AttachRecommendation(&r)
		if r.Recommendation == nil {
			t.Fatalf("expected non-nil Recommendation")
		}
		if r.Recommendation.Primary == nil {
			t.Fatalf("expected non-nil Primary; got set=%+v", r.Recommendation)
		}
	})
}

// TestAttachRecommendationDeterminism enforces NFR-001: byte-identical
// marshaled JSON across 100 iterations on identical input.
func TestAttachRecommendationDeterminism(t *testing.T) {
	var first []byte
	for i := 0; i < 100; i++ {
		r := canonicalPrimaryFixture()
		AttachRecommendation(&r)
		if r.Recommendation == nil {
			t.Fatalf("iter %d: Recommendation is nil", i)
		}
		buf, err := json.Marshal(r.Recommendation)
		if err != nil {
			t.Fatalf("iter %d: marshal error: %v", i, err)
		}
		if i == 0 {
			first = buf
			continue
		}
		if string(buf) != string(first) {
			t.Fatalf("iter %d: JSON differs from iter 0\n  iter 0: %s\n  iter %d: %s", i, first, i, buf)
		}
	}
}

// -----------------------------------------------------------------------------
// Test helpers (file-local).
// -----------------------------------------------------------------------------

// activeUsageTrackerFingerprint returns a single fingerprint that
// satisfies S-07's "active usage-visibility tracker" check, so callers
// that want to test other rules in isolation can suppress S-07 emission.
func activeUsageTrackerFingerprint() EcosystemFingerprint {
	return EcosystemFingerprint{ID: "ccusage", Active: true}
}

// reportWithFindingsAndFingerprint constructs a minimal Report with the
// given Finding IDs and a single fingerprint. Used by deriveSignals
// table cases.
func reportWithFindingsAndFingerprint(findingIDs []string, fps ...EcosystemFingerprint) Report {
	r := Report{}
	for _, id := range findingIDs {
		r.Findings = append(r.Findings, Finding{ID: id})
	}
	r.Ecosystem.WorkflowFingerprints = append(r.Ecosystem.WorkflowFingerprints, fps...)
	return r
}

// canonicalPrimaryFixture builds the shared fixture used by both
// TestAttachRecommendation's Primary case and the determinism test.
// It produces S-01 (tool_output_bloat) + S-07 (no usage tracker), which
// fires both ClassShellOutputReducer and ClassUsageVisibility rules.
func canonicalPrimaryFixture() Report {
	return Report{
		Findings: []Finding{
			{ID: "tool_output_bloat", Title: "Tool output bloat", Severity: "high"},
		},
		Ecosystem: Ecosystem{
			ToolingUtilization: ToolingUtilization{
				MCP:   MCPUtilization{WarningBand: "normal"},
				Skill: SkillUtilization{WarningBand: "normal"},
			},
		},
	}
}
