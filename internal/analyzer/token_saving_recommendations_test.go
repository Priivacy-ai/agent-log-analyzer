// Package analyzer — WP04 acceptance, invariant, and privacy tests for
// the Phase A token-saving recommendation engine. See
// kitty-specs/token-saving-recommendation-engine-phase-a-01KRZKCJ/
// tasks/WP04-engine-acceptance-privacy-tests.md.
package analyzer

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Shared helpers ------------------------------------------------------------

// buildState collects ToolStateEntry rows into a ToolStateMap keyed by Tool.
func buildState(entries ...ToolStateEntry) ToolStateMap {
	m := ToolStateMap{}
	for _, e := range entries {
		m[e.Tool] = e
	}
	return m
}

// marshalSet marshals a RecommendationSet to compact JSON or fails the test.
func marshalSet(t *testing.T, set RecommendationSet) []byte {
	t.Helper()
	b, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return b
}

// classFromRecommendationID parses the <class> segment from a
// rec.<class>.<tool_id>.<signals> recommendation_id.
func classFromRecommendationID(id string) RecommendationClass {
	parts := strings.SplitN(id, ".", 4)
	if len(parts) < 4 || parts[0] != "rec" {
		return ""
	}
	return RecommendationClass(parts[1])
}

// all*() helpers — single source of truth for the privacy allowlist.

func allToolStates() []ToolState {
	return []ToolState{ToolStateUnknown, ToolStateMentionedLow, ToolStateInstalledMedium,
		ToolStateConfiguredMedium, ToolStateActiveHigh, ToolStateRejectedMedium}
}

func allEvidenceSources() []EvidenceSource {
	return []EvidenceSource{EvidenceCLIPresence, EvidenceCLIVersion, EvidenceLogActiveCommand,
		EvidenceMCPConfigured, EvidenceMCPActive, EvidencePluginConfigured, EvidenceSkillConfigured,
		EvidenceHookConfigured, EvidenceStatuslineConfigured, EvidenceReportMention, EvidenceFailureOrRejection}
}

func allSignals() []Signal {
	return []Signal{SignalNoUsageVisibility, SignalToolOutputBloat, SignalShellOutputBloat,
		SignalMCPToolOutputBloat, SignalRepeatedFileReads, SignalBroadRepoExploration,
		SignalUnchangedFileRereads, SignalMCPSkillBloat, SignalOutputVerbosity,
		SignalRetryLoop, SignalContextGrowthSpikes}
}

func allRecommendationClasses() []RecommendationClass {
	return []RecommendationClass{ClassUsageVisibility, ClassMCPSkillHygiene, ClassMCPOutputReducer,
		ClassShellOutputReducer, ClassRetrieval, ClassRereadGuard, ClassContextHygiene, ClassOutputVerbosity}
}

func allConfidences() []Confidence {
	return []Confidence{ConfidenceLow, ConfidenceMedium, ConfidenceHigh}
}

func allRiskLevels() []RiskLevel {
	return []RiskLevel{RiskLow, RiskMedium, RiskHigh}
}

func allInstallPolicies() []InstallPolicy {
	return []InstallPolicy{PolicyBundle, PolicyRecommend, PolicyRecommendWithWaiver,
		PolicyResearchOnly, PolicyReferenceOnly}
}

func allReasons() []Reason {
	return []Reason{ReasonAbsent, ReasonInstalledInactive, ReasonConfiguredInactive,
		ReasonActivePersistent, ReasonRejectedAlternative, ReasonPruneFirst,
		ReasonAuditConfig, ReasonNoOp, ReasonServerQuotaCheck}
}

// Positive-list privacy scanner (NFR-002 / AS-14) --------------------------

// recommendationAllowlist is the closed set of tokens permitted in
// Recommend's marshalled output. Anything else (and not a bare integer)
// is treated as a potential privacy leak.
func recommendationAllowlist() map[string]bool {
	allow := map[string]bool{}
	for _, t := range AllTools() {
		allow[string(t.ID)] = true
	}
	for _, v := range allToolStates() {
		allow[string(v)] = true
	}
	for _, v := range allEvidenceSources() {
		allow[string(v)] = true
	}
	for _, v := range allSignals() {
		allow[string(v)] = true
	}
	for _, v := range allRecommendationClasses() {
		allow[string(v)] = true
	}
	for _, v := range allConfidences() {
		allow[string(v)] = true
	}
	for _, v := range allRiskLevels() {
		allow[string(v)] = true
	}
	for _, v := range allInstallPolicies() {
		allow[string(v)] = true
	}
	for _, v := range allReasons() {
		allow[string(v)] = true
	}
	for _, v := range []string{
		"recommendation_id", "primary_tool_id", "skipped_tool_ids",
		"reason", "signal_ids", "confidence", "risk_level",
		"install_policy", "evidence_counts", "tool_id", "for_signal",
		"primary", "secondary", "skipped", "registry_version",
		"engine_version", "signals", "unknown_id_count",
		// Recommendation-ID structural literals.
		"rec", "none",
		// Version constants returned by RegistryVersion / EngineVersion.
		RegistryVersion(), EngineVersion(),
	} {
		allow[v] = true
	}
	// Decompose every allowlisted token using the same split rule the
	// scanner applies (split on '.' and '-'; keep '_' inside tokens) so
	// that compound version strings like `phase-a-2026-05-19` and
	// `v0.1-phase-a` contribute each of their sub-tokens.
	splitRe := regexp.MustCompile(`[.\-]`)
	atoms := []string{}
	for k := range allow {
		for _, piece := range splitRe.Split(k, -1) {
			if piece != "" {
				atoms = append(atoms, piece)
			}
		}
	}
	for _, a := range atoms {
		allow[a] = true
	}
	// Multi-signal joins: a recommendation_id may end in a sorted, '_'-
	// joined sequence of signal names (e.g. `context_growth_spikes_retry_loop`
	// when both signals fire on the same rule). Admit every sorted pair of
	// known signals as one allowlisted token.
	sigs := allSignals()
	sortedSigs := make([]string, len(sigs))
	for i, s := range sigs {
		sortedSigs[i] = string(s)
	}
	sort.Strings(sortedSigs)
	for i := 0; i < len(sortedSigs); i++ {
		for j := i + 1; j < len(sortedSigs); j++ {
			allow[sortedSigs[i]+"_"+sortedSigs[j]] = true
		}
	}
	return allow
}

// findNonAllowlistedSubstrings tokenises jsonBlob into runs of
// [A-Za-z0-9_] characters (treating '.' and '-' as separators) and
// returns every token that is neither allowlisted nor a bare integer.
//
// '_' is part of a token so signal/enum names like `no_usage_visibility`
// stay whole. '.' is a separator so `recommendation_id` IDs like
// `rec.usage_visibility.ccusage.<sig>` decompose into per-segment tokens.
// '-' is a separator so compound version strings like `phase-a-2026-05-19`
// decompose into per-piece tokens (all individually allowlisted via the
// atom decomposer inside recommendationAllowlist).
func findNonAllowlistedSubstrings(jsonBlob []byte) []string {
	allow := recommendationAllowlist()
	re := regexp.MustCompile(`[A-Za-z0-9_]+`)
	var leaks []string
	for _, tok := range re.FindAllString(string(jsonBlob), -1) {
		if allow[tok] {
			continue
		}
		if isIntOrDottedDigits(tok) {
			continue
		}
		leaks = append(leaks, tok)
	}
	return leaks
}

// isIntOrDottedDigits returns true if every rune in tok is a digit or
// '.'. Bare integers (counts) carry no private information.
func isIntOrDottedDigits(tok string) bool {
	if tok == "" {
		return false
	}
	for _, r := range tok {
		if r != '.' && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// T020 — TestRecommend_AcceptanceScenarios -----------------------------------

// acceptanceExpectation captures assertions for one AS-* row.
// primaryNil=true → Primary must be nil. Otherwise primary is the expected
// PrimaryToolID ("" matches the advisory branch). secondary, when set, is
// the expected Secondary.PrimaryToolID. skipped, when non-nil, is the
// expected (unordered) SkipNote set.
type acceptanceExpectation struct {
	primaryNil    bool
	primary       ToolID
	primaryReason Reason
	secondary     ToolID
	skipped       []SkipNote
}

func TestRecommend_AcceptanceScenarios(t *testing.T) {
	cases := []struct {
		name    string
		signals []Signal
		state   ToolStateMap
		want    acceptanceExpectation
	}{
		{
			name:    "AS-01 no usage visibility, no tools",
			signals: []Signal{SignalNoUsageVisibility},
			state:   buildState(),
			want: acceptanceExpectation{
				primary:       "ccusage",
				primaryReason: ReasonAbsent,
			},
		},
		{
			name:    "AS-02 ccusage active + server-quota failure evidence",
			signals: []Signal{SignalNoUsageVisibility},
			state: buildState(ToolStateEntry{
				Tool:  "ccusage",
				State: ToolStateActiveHigh,
				Sources: map[EvidenceSource]int{
					EvidenceLogActiveCommand:   3,
					EvidenceFailureOrRejection: 1,
				},
			}),
			want: acceptanceExpectation{
				primary:       "ccusage",
				primaryReason: ReasonServerQuotaCheck,
			},
		},
		{
			name:    "AS-03 shell bloat, RTK absent",
			signals: []Signal{SignalShellOutputBloat},
			state:   buildState(),
			want: acceptanceExpectation{
				primary:       "rtk",
				primaryReason: ReasonAbsent,
			},
		},
		{
			name:    "AS-04 shell bloat, RTK configured",
			signals: []Signal{SignalShellOutputBloat},
			state: buildState(ToolStateEntry{
				Tool:    "rtk",
				State:   ToolStateConfiguredMedium,
				Sources: map[EvidenceSource]int{EvidenceHookConfigured: 1},
			}),
			want: acceptanceExpectation{
				primary:       "rtk",
				primaryReason: ReasonConfiguredInactive,
			},
		},
		{
			name:    "AS-05 shell bloat, RTK active",
			signals: []Signal{SignalShellOutputBloat},
			state: buildState(ToolStateEntry{
				Tool:    "rtk",
				State:   ToolStateActiveHigh,
				Sources: map[EvidenceSource]int{EvidenceLogActiveCommand: 2},
			}),
			// Only rtk is recommend-eligible in shell_output_reducer
			// (leanctx, headroom are research_only); after skip → no primary.
			want: acceptanceExpectation{
				primaryNil: true,
				skipped: []SkipNote{
					{ToolID: "rtk", Reason: ReasonActivePersistent, ForSignal: SignalShellOutputBloat},
				},
			},
		},
		{
			name:    "AS-06 shell bloat, RTK rejected",
			signals: []Signal{SignalShellOutputBloat},
			state: buildState(ToolStateEntry{
				Tool:    "rtk",
				State:   ToolStateRejectedMedium,
				Sources: map[EvidenceSource]int{EvidenceFailureOrRejection: 1},
			}),
			want: acceptanceExpectation{
				primaryNil: true,
				skipped: []SkipNote{
					{ToolID: "rtk", Reason: ReasonRejectedAlternative, ForSignal: SignalShellOutputBloat},
				},
			},
		},
		{
			name:    "AS-07 mcp tool output bloat, context_mode absent",
			signals: []Signal{SignalMCPToolOutputBloat},
			state:   buildState(),
			want: acceptanceExpectation{
				primary:       "context_mode",
				primaryReason: ReasonAbsent,
			},
		},
		{
			name:    "AS-08 mcp tool output bloat, context_mode configured",
			signals: []Signal{SignalMCPToolOutputBloat},
			state: buildState(ToolStateEntry{
				Tool:    "context_mode",
				State:   ToolStateConfiguredMedium,
				Sources: map[EvidenceSource]int{EvidenceMCPConfigured: 1},
			}),
			want: acceptanceExpectation{
				primary:       "context_mode",
				primaryReason: ReasonConfiguredInactive,
			},
		},
		{
			name:    "AS-09 mcp tool output bloat, context_mode active",
			signals: []Signal{SignalMCPToolOutputBloat},
			state: buildState(ToolStateEntry{
				Tool:    "context_mode",
				State:   ToolStateActiveHigh,
				Sources: map[EvidenceSource]int{EvidenceMCPActive: 1},
			}),
			// context_mode is the only recommend-eligible mcp_output_reducer
			// entry (distill, token_optimizer_mcp are research_only).
			want: acceptanceExpectation{
				primaryNil: true,
				skipped: []SkipNote{
					{ToolID: "context_mode", Reason: ReasonActivePersistent, ForSignal: SignalMCPToolOutputBloat},
				},
			},
		},
		{
			name:    "AS-10 repeated reads, serena active",
			signals: []Signal{SignalRepeatedFileReads},
			state: buildState(ToolStateEntry{
				Tool:    "serena",
				State:   ToolStateActiveHigh,
				Sources: map[EvidenceSource]int{EvidenceMCPConfigured: 1},
			}),
			// serena is research_only and never enters the candidate set,
			// so the engine emits the first eligible retrieval tool
			// (claude_context, class_rank 1 after filtering).
			want: acceptanceExpectation{
				primary:       "claude_context",
				primaryReason: ReasonAbsent,
			},
		},
		{
			name:    "AS-11 mcp_skill_bloat advisory",
			signals: []Signal{SignalMCPSkillBloat},
			state:   buildState(),
			// Advisory: PrimaryToolID == "" with reason prune_first.
			want: acceptanceExpectation{
				primary:       "",
				primaryReason: ReasonPruneFirst,
			},
		},
		{
			name:    "AS-12 output verbosity, none installed",
			signals: []Signal{SignalOutputVerbosity},
			state:   buildState(),
			want: acceptanceExpectation{
				primary:       "claude_token_efficient",
				primaryReason: ReasonAbsent,
			},
		},
		{
			name:    "AS-13 shell bloat + output verbosity",
			signals: []Signal{SignalShellOutputBloat, SignalOutputVerbosity},
			state:   buildState(),
			want: acceptanceExpectation{
				primary:       "rtk",
				primaryReason: ReasonAbsent,
				secondary:     "claude_token_efficient",
			},
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			set := Recommend(tt.signals, tt.state)
			assertAcceptance(t, set, tt.want)
		})
	}
}

// assertAcceptance applies the acceptanceExpectation checks against set.
func assertAcceptance(t *testing.T, set RecommendationSet, want acceptanceExpectation) {
	t.Helper()
	if want.primaryNil {
		if set.Primary != nil {
			t.Errorf("expected Primary == nil, got %+v", set.Primary)
		}
	} else {
		if set.Primary == nil {
			t.Fatalf("expected Primary with tool=%q, got nil", want.primary)
		}
		if set.Primary.PrimaryToolID != want.primary {
			t.Errorf("Primary.PrimaryToolID = %q, want %q", set.Primary.PrimaryToolID, want.primary)
		}
		if set.Primary.Reason != want.primaryReason {
			t.Errorf("Primary.Reason = %q, want %q", set.Primary.Reason, want.primaryReason)
		}
	}
	if want.secondary != "" {
		if set.Secondary == nil {
			t.Fatalf("expected Secondary with tool=%q, got nil", want.secondary)
		}
		if set.Secondary.PrimaryToolID != want.secondary {
			t.Errorf("Secondary.PrimaryToolID = %q, want %q", set.Secondary.PrimaryToolID, want.secondary)
		}
	}
	if want.skipped != nil {
		got, exp := append([]SkipNote(nil), set.Skipped...), append([]SkipNote(nil), want.skipped...)
		sortSkipNotes(got)
		sortSkipNotes(exp)
		if len(got) != len(exp) {
			t.Errorf("Skipped len = %d, want %d (got=%+v want=%+v)", len(got), len(exp), got, exp)
			return
		}
		for i := range got {
			if got[i] != exp[i] {
				t.Errorf("Skipped[%d] = %+v, want %+v", i, got[i], exp[i])
			}
		}
	}
}

func sortSkipNotes(s []SkipNote) {
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].ToolID != s[j].ToolID {
			return string(s[i].ToolID) < string(s[j].ToolID)
		}
		return string(s[i].ForSignal) < string(s[j].ForSignal)
	})
}

// T021 — TestRecommendDeterminism --------------------------------------------

// NFR-001: Recommend's marshalled JSON must be byte-identical across runs
// for identical input. 50 deterministic input cases (single-signal and
// pair-signal variants over a fixed set of state-map combos).
func TestRecommendDeterminism(t *testing.T) {
	stateCombos := []ToolStateMap{
		buildState(),
		buildState(ToolStateEntry{Tool: "ccusage", State: ToolStateActiveHigh,
			Sources: map[EvidenceSource]int{EvidenceLogActiveCommand: 2}}),
		buildState(ToolStateEntry{Tool: "rtk", State: ToolStateConfiguredMedium,
			Sources: map[EvidenceSource]int{EvidenceHookConfigured: 1}}),
		buildState(ToolStateEntry{Tool: "context_mode", State: ToolStateRejectedMedium,
			Sources: map[EvidenceSource]int{EvidenceFailureOrRejection: 1}}),
		buildState(ToolStateEntry{Tool: "claude_token_efficient", State: ToolStateMentionedLow,
			Sources: map[EvidenceSource]int{EvidenceReportMention: 1}}),
		buildState(
			ToolStateEntry{Tool: "rtk", State: ToolStateConfiguredMedium,
				Sources: map[EvidenceSource]int{EvidenceHookConfigured: 1}},
			ToolStateEntry{Tool: "ccusage", State: ToolStateInstalledMedium,
				Sources: map[EvidenceSource]int{EvidenceCLIPresence: 1}},
		),
	}
	type row struct {
		signals []Signal
		state   ToolStateMap
	}
	var rows []row
	for _, s := range allSignals()[:5] {
		for _, st := range stateCombos {
			rows = append(rows, row{[]Signal{s}, st})
		}
	}
	pairs := [][2]Signal{
		{SignalShellOutputBloat, SignalOutputVerbosity},
		{SignalMCPToolOutputBloat, SignalRepeatedFileReads},
		{SignalNoUsageVisibility, SignalMCPSkillBloat},
		{SignalRetryLoop, SignalContextGrowthSpikes},
	}
	for _, p := range pairs {
		for _, st := range stateCombos[:5] {
			rows = append(rows, row{[]Signal{p[0], p[1]}, st})
		}
	}
	if len(rows) > 50 {
		rows = rows[:50]
	}
	for i, r := range rows {
		a := marshalSet(t, Recommend(r.signals, r.state))
		b := marshalSet(t, Recommend(r.signals, r.state))
		if !bytes.Equal(a, b) {
			t.Errorf("determinism violated for row %d (signals=%v):\nA: %s\nB: %s", i, r.signals, a, b)
			return
		}
	}
}

// T022 — TestRecommendSkipsActiveTool ----------------------------------------

// TestRecommendSkipsActiveTool: for every rule whose class has at least one
// recommend-eligible candidate, marking that candidate active_high causes
// it to (a) appear in Skipped with reason active_persistent and
// (b) NOT be emitted as Primary.PrimaryToolID.
func TestRecommendSkipsActiveTool(t *testing.T) {
	for _, rule := range rulePrecedence {
		candidates := eligibleCandidates(rule.Class)
		if len(candidates) == 0 {
			continue // advisory class — no candidate to mark active
		}
		first := candidates[0]
		sig := rule.FiringSignals[0]
		state := buildState(ToolStateEntry{
			Tool:    first.ID,
			State:   ToolStateActiveHigh,
			Sources: map[EvidenceSource]int{EvidenceLogActiveCommand: 1},
		})
		set := Recommend([]Signal{sig}, state)

		// (a) candidate must appear in Skipped with ReasonActivePersistent
		var found bool
		for _, sn := range set.Skipped {
			if sn.ToolID == first.ID && sn.Reason == ReasonActivePersistent {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rule class %q: expected Skipped to contain {%q, active_persistent}, got %+v",
				rule.Class, first.ID, set.Skipped)
		}

		// (b) candidate must NOT appear as Primary.PrimaryToolID
		if set.Primary != nil && set.Primary.PrimaryToolID == first.ID {
			t.Errorf("rule class %q: Primary.PrimaryToolID = %q must not equal active candidate %q",
				rule.Class, set.Primary.PrimaryToolID, first.ID)
		}
	}
}

// T023 — TestRecommendMCPSkillBloatNeverAddsMCP ------------------------------

// FR-013 / AS-11: when mcp_skill_bloat is among the input signals, the
// Primary MUST NOT be in the mcp_output_reducer or retrieval class —
// "do not add another MCP" by default for skill bloat. Because rule
// precedence puts mcp_skill_hygiene (rule 2) ahead of mcp_output_reducer
// (rule 3) and retrieval (rule 5), an MCP-stacking primary is structurally
// impossible while skill bloat is present.
//
// Note: usage_visibility (rule 1) outranks mcp_skill_hygiene, so when
// no_usage_visibility is also present the Primary may legitimately be
// in the usage_visibility class. The invariant is only the negative one:
// no MCP-adding primary.
func TestRecommendMCPSkillBloatNeverAddsMCP(t *testing.T) {
	for _, s := range allSignals() {
		set := Recommend([]Signal{SignalMCPSkillBloat, s}, ToolStateMap{})
		if set.Primary == nil {
			continue
		}
		class := classFromRecommendationID(set.Primary.RecommendationID)
		if class == ClassMCPOutputReducer || class == ClassRetrieval {
			t.Errorf("signal pair {mcp_skill_bloat, %s}: Primary class = %q must not be mcp_output_reducer or retrieval (id=%q)",
				s, class, set.Primary.RecommendationID)
		}
	}
	// Pure mcp_skill_bloat input always produces the advisory primary.
	set := Recommend([]Signal{SignalMCPSkillBloat}, ToolStateMap{})
	if set.Primary == nil {
		t.Fatalf("mcp_skill_bloat alone: expected advisory Primary, got nil")
	}
	if got := classFromRecommendationID(set.Primary.RecommendationID); got != ClassMCPSkillHygiene {
		t.Errorf("mcp_skill_bloat alone: Primary class = %q, want mcp_skill_hygiene", got)
	}
	if set.Primary.PrimaryToolID != "" {
		t.Errorf("mcp_skill_bloat alone: Primary.PrimaryToolID = %q, want \"\" (advisory)", set.Primary.PrimaryToolID)
	}
}

// T024 — TestRecommendOnePlusOne ---------------------------------------------

// Sweeps every (s1, s2) pair (n=121) and asserts: at most one Primary, at
// most one Secondary, distinct classes when both non-nil, and they never
// point at the same non-empty ToolID.
func TestRecommendOnePlusOne(t *testing.T) {
	signals := allSignals()
	for _, s1 := range signals {
		for _, s2 := range signals {
			set := Recommend([]Signal{s1, s2}, ToolStateMap{})
			if set.Primary == nil && set.Secondary != nil {
				t.Errorf("pair (%s,%s): Secondary without Primary (violates engine contract)", s1, s2)
				continue
			}
			if set.Primary == nil || set.Secondary == nil {
				continue
			}
			pClass := classFromRecommendationID(set.Primary.RecommendationID)
			sClass := classFromRecommendationID(set.Secondary.RecommendationID)
			if pClass == sClass {
				t.Errorf("pair (%s,%s): Primary and Secondary share class %q (ids: %q, %q)",
					s1, s2, pClass, set.Primary.RecommendationID, set.Secondary.RecommendationID)
			}
			if set.Primary.PrimaryToolID != "" &&
				set.Primary.PrimaryToolID == set.Secondary.PrimaryToolID {
				t.Errorf("pair (%s,%s): Primary and Secondary point at the same tool %q",
					s1, s2, set.Primary.PrimaryToolID)
			}
		}
	}
}

// T025 — TestRecommendPrivacyBudget ------------------------------------------

// NFR-002 / AS-14: marshalled RecommendationSet must contain no token
// outside the positive-list allowlist. The decoy state map mixes one
// valid ToolID with several unknown ToolIDs that look like leaked
// corporate identifiers, secrets, or PII. The engine MUST drop the
// unknown entries and bump UnknownIDCount; their string values must
// never appear in the marshalled output.
func TestRecommendPrivacyBudget(t *testing.T) {
	mkEntry := func(id ToolID, st ToolState, src EvidenceSource, n int) ToolStateEntry {
		return ToolStateEntry{Tool: id, State: st, Sources: map[EvidenceSource]int{src: n}}
	}
	decoyState := ToolStateMap{
		"private_company_secret_tool": mkEntry("private_company_secret_tool", ToolStateActiveHigh, EvidenceMCPActive, 1),
		"internal_corp_001":           mkEntry("internal_corp_001", ToolStateConfiguredMedium, EvidencePluginConfigured, 1),
		"sk_ant_FAKE_TOKEN_DECOY":     mkEntry("sk_ant_FAKE_TOKEN_DECOY", ToolStateMentionedLow, EvidenceReportMention, 1),
		// One valid entry so the engine still produces a real Primary.
		"rtk": mkEntry("rtk", ToolStateConfiguredMedium, EvidencePluginConfigured, 2),
	}

	var sawUnknownCount bool

	// Sweep every single-signal input.
	for _, s := range allSignals() {
		set := Recommend([]Signal{s}, decoyState)
		blob := marshalSet(t, set)
		if leaks := findNonAllowlistedSubstrings(blob); len(leaks) > 0 {
			t.Errorf("WP04 privacy probe leaked for signal %q: %v\nJSON: %s", s, leaks, blob)
		}
		if set.UnknownIDCount > 0 {
			sawUnknownCount = true
		}
	}

	// Sample of pair-signal inputs.
	pairs := [][2]Signal{
		{SignalShellOutputBloat, SignalOutputVerbosity},
		{SignalMCPSkillBloat, SignalMCPToolOutputBloat},
		{SignalNoUsageVisibility, SignalRepeatedFileReads},
		{SignalRetryLoop, SignalContextGrowthSpikes},
		{SignalUnchangedFileRereads, SignalBroadRepoExploration},
	}
	for _, p := range pairs {
		set := Recommend([]Signal{p[0], p[1]}, decoyState)
		blob := marshalSet(t, set)
		if leaks := findNonAllowlistedSubstrings(blob); len(leaks) > 0 {
			t.Errorf("WP04 privacy probe leaked for pair (%s,%s): %v\nJSON: %s",
				p[0], p[1], leaks, blob)
		}
		if set.UnknownIDCount > 0 {
			sawUnknownCount = true
		}
	}

	// At least one decoy-bearing run must have observed the unknown IDs.
	if !sawUnknownCount {
		t.Errorf("expected UnknownIDCount > 0 in at least one decoy run; engine did not observe the decoys")
	}
	// Self-check: confirm the scanner catches an injected leak (guards
	// against the scanner accidentally degenerating into a no-op).
	// SMOKE_INJECT_FOR_QA: temporarily replace the literal below with a
	// real private string to verify the scanner flags it; the test must
	// then FAIL until reverted.
	injected := []byte(`{"primary":{"primary_tool_id":"sk-ant-LEAKY","reason":"absent"}}`)
	if leaks := findNonAllowlistedSubstrings(injected); len(leaks) == 0 {
		t.Errorf("privacy scanner self-check failed: injected leak not detected in %s", injected)
	}
}
