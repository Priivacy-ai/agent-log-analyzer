package analyzer

// Tests for the paid aggregate merge surface introduced in WP03.
//
// Contract: kitty-specs/launch-correctness-01KRZZVK/contracts/aggregate-merge.md
// Requirements: FR-007 (fingerprint merge), FR-008 (tooling merge), NFR-005
// (100-input timing ceiling).
//
// Invariants asserted: identity, commutativity, associativity, coverage,
// bounded-cardinality. The privacy invariant is covered end-to-end by
// leak_test.go (which builds inputs from raw bytes through Analyze()).

import (
	"reflect"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// FR-007: WorkflowFingerprints merge — row-by-row rules
// -----------------------------------------------------------------------------

func TestMergeWorkflowFingerprints_GroupByID(t *testing.T) {
	a := []EcosystemFingerprint{
		{
			ID:            "spec_kitty",
			Confidence:    "medium",
			Sources:       []string{"binary_present"},
			EvidenceCount: 3,
			Active:        false,
			Installed:     true,
			VersionBucket: "v1",
		},
	}
	b := []EcosystemFingerprint{
		{
			ID:            "spec_kitty",
			Confidence:    "high",
			Sources:       []string{"transcript_command", "binary_present"},
			EvidenceCount: 5,
			Active:        true,
			Installed:     false,
			VersionBucket: "v1",
		},
	}
	got := mergeWorkflowFingerprints(a, b)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 merged fingerprint by id, got %d (%#v)", len(got), got)
	}
	fp := got[0]
	if fp.ID != "spec_kitty" {
		t.Errorf("id: got %q want spec_kitty", fp.ID)
	}
	// sources: sorted union
	wantSources := []string{"binary_present", "transcript_command"}
	if !reflect.DeepEqual(fp.Sources, wantSources) {
		t.Errorf("sources: got %v want %v", fp.Sources, wantSources)
	}
	// evidence_count: SUM (C-007)
	if fp.EvidenceCount != 8 {
		t.Errorf("evidence_count: got %d want 8 (sum per C-007)", fp.EvidenceCount)
	}
	// confidence: max-rank
	if fp.Confidence != "high" {
		t.Errorf("confidence: got %q want high", fp.Confidence)
	}
	// active / installed: OR
	if !fp.Active {
		t.Error("active: expected OR(false,true)=true")
	}
	if !fp.Installed {
		t.Error("installed: expected OR(true,false)=true")
	}
	// version_bucket: agree → retained
	if fp.VersionBucket != "v1" {
		t.Errorf("version_bucket: got %q want v1 (agreement)", fp.VersionBucket)
	}
}

func TestMergeWorkflowFingerprints_VersionBucketDisagreement(t *testing.T) {
	a := []EcosystemFingerprint{{ID: "openspec", VersionBucket: "v1", Confidence: "low", Sources: []string{"x"}}}
	b := []EcosystemFingerprint{{ID: "openspec", VersionBucket: "v2", Confidence: "low", Sources: []string{"y"}}}
	got := mergeWorkflowFingerprints(a, b)
	if len(got) != 1 || got[0].VersionBucket != "" {
		t.Errorf("disagreement should empty version_bucket (no 'mixed'): %#v", got)
	}
	// One side empty, other non-empty -> empty (the absent side disagrees).
	a = []EcosystemFingerprint{{ID: "openspec", VersionBucket: "v1", Confidence: "low", Sources: []string{"x"}}}
	b = []EcosystemFingerprint{{ID: "openspec", VersionBucket: "", Confidence: "low", Sources: []string{"y"}}}
	got = mergeWorkflowFingerprints(a, b)
	if len(got) != 1 || got[0].VersionBucket != "" {
		t.Errorf("one-side-empty should empty version_bucket: %#v", got)
	}
}

func TestMergeWorkflowFingerprints_DisjointIDs(t *testing.T) {
	a := []EcosystemFingerprint{{ID: "spec_kitty", Confidence: "high", Sources: []string{"s"}, EvidenceCount: 2}}
	b := []EcosystemFingerprint{{ID: "openspec", Confidence: "medium", Sources: []string{"t"}, EvidenceCount: 5}}
	got := mergeWorkflowFingerprints(a, b)
	if len(got) != 2 {
		t.Fatalf("expected 2 fingerprints (disjoint ids), got %d (%#v)", len(got), got)
	}
	// Sorted ascending by id
	if got[0].ID != "openspec" || got[1].ID != "spec_kitty" {
		t.Errorf("expected ascending id order, got %v", []string{got[0].ID, got[1].ID})
	}
}

// -----------------------------------------------------------------------------
// FR-008: MCPUtilization merge — row-by-row rules
// -----------------------------------------------------------------------------

func TestMergeMCPUtilization_AllFields(t *testing.T) {
	a := MCPUtilization{
		KnownServerIDs:           []string{"github", "linear"},
		UnknownServerCount:       2,
		ServerCountBucket:        "1-3",
		ExposedToolCountBucket:   "4-10",
		ContextTokenBucket:       "1k-5k",
		ExposureKnown:            true,
		InferenceSource:          "header",
		CallCount:                10,
		KnownCallCount:           7,
		UnknownCallCount:         3,
		UniqueKnownCalledIDs:     []string{"github"},
		UniqueUnknownCalledCount: 1,
		UtilizationRatioPct:      70,
		ContextEfficiencyBucket:  "moderate",
		WarningBand:              WarningBandWatch,
	}
	b := MCPUtilization{
		KnownServerIDs:           []string{"linear", "notion"},
		UnknownServerCount:       1,
		ServerCountBucket:        "4-10",
		ExposedToolCountBucket:   "11-25",
		ContextTokenBucket:       "5k-15k",
		ExposureKnown:            true,
		InferenceSource:          "calls",
		CallCount:                20,
		KnownCallCount:           5,
		UnknownCallCount:         15,
		UniqueKnownCalledIDs:     []string{"notion"},
		UniqueUnknownCalledCount: 4,
		UtilizationRatioPct:      25,
		ContextEfficiencyBucket:  "underutilized",
		WarningBand:              WarningBandHigh,
	}
	got := mergeMCPUtilization(a, b)
	if !reflect.DeepEqual(got.KnownServerIDs, []string{"github", "linear", "notion"}) {
		t.Errorf("KnownServerIDs: got %v", got.KnownServerIDs)
	}
	if got.UnknownServerCount != 3 {
		t.Errorf("UnknownServerCount: got %d want 3", got.UnknownServerCount)
	}
	if got.ServerCountBucket != "4-10" {
		t.Errorf("ServerCountBucket max-rank: got %q want 4-10", got.ServerCountBucket)
	}
	if got.ExposedToolCountBucket != "11-25" {
		t.Errorf("ExposedToolCountBucket max-rank: got %q want 11-25", got.ExposedToolCountBucket)
	}
	if got.ContextTokenBucket != "5k-15k" {
		t.Errorf("ContextTokenBucket max-rank: got %q want 5k-15k", got.ContextTokenBucket)
	}
	if got.CallCount != 30 || got.KnownCallCount != 12 || got.UnknownCallCount != 18 {
		t.Errorf("call counts: got CallCount=%d KnownCallCount=%d UnknownCallCount=%d want 30/12/18",
			got.CallCount, got.KnownCallCount, got.UnknownCallCount)
	}
	if !reflect.DeepEqual(got.UniqueKnownCalledIDs, []string{"github", "notion"}) {
		t.Errorf("UniqueKnownCalledIDs: got %v", got.UniqueKnownCalledIDs)
	}
	if got.UniqueUnknownCalledCount != 5 {
		t.Errorf("UniqueUnknownCalledCount: got %d want 5", got.UniqueUnknownCalledCount)
	}
	// UtilizationRatioPct: recomputed from summed counts: 12/30 = 40
	if got.UtilizationRatioPct != 40 {
		t.Errorf("UtilizationRatioPct: got %d want 40 (recomputed from sums)", got.UtilizationRatioPct)
	}
	if got.WarningBand != WarningBandHigh {
		t.Errorf("WarningBand max-rank: got %q want %q", got.WarningBand, WarningBandHigh)
	}
}

func TestMergeMCPUtilization_ZeroDenominator(t *testing.T) {
	got := mergeMCPUtilization(MCPUtilization{}, MCPUtilization{})
	if got.UtilizationRatioPct != 0 {
		t.Errorf("zero CallCount must produce ratio 0, got %d", got.UtilizationRatioPct)
	}
}

// -----------------------------------------------------------------------------
// FR-008: SkillUtilization merge — row-by-row rules
// -----------------------------------------------------------------------------

func TestMergeSkillUtilization_AllFields(t *testing.T) {
	a := SkillUtilization{
		KnownExposedIDs:         []string{"qa", "review"},
		UnknownExposedCount:     1,
		ExposedCountBucket:      "1-3",
		ContextTokenBucket:      "<1k",
		ExposureKnown:           true,
		InferenceSource:         "header",
		ExecutedCount:           2,
		KnownExecutedIDs:        []string{"qa"},
		UnknownExecutedCount:    0,
		UtilizationRatioPct:     100,
		ContextEfficiencyBucket: "well-utilized",
		WarningBand:             WarningBandNormal,
	}
	b := SkillUtilization{
		KnownExposedIDs:         []string{"review", "investigate"},
		UnknownExposedCount:     2,
		ExposedCountBucket:      "4-10",
		ContextTokenBucket:      "1k-5k",
		ExposureKnown:           true,
		InferenceSource:         "calls",
		ExecutedCount:           1,
		KnownExecutedIDs:        []string{"review"},
		UnknownExecutedCount:    3,
		UtilizationRatioPct:     50,
		ContextEfficiencyBucket: "underutilized",
		WarningBand:             WarningBandSevere,
	}
	got := mergeSkillUtilization(a, b)
	if !reflect.DeepEqual(got.KnownExposedIDs, []string{"investigate", "qa", "review"}) {
		t.Errorf("KnownExposedIDs: got %v", got.KnownExposedIDs)
	}
	if got.UnknownExposedCount != 3 {
		t.Errorf("UnknownExposedCount: got %d want 3", got.UnknownExposedCount)
	}
	if got.ExposedCountBucket != "4-10" {
		t.Errorf("ExposedCountBucket max-rank: got %q want 4-10", got.ExposedCountBucket)
	}
	if got.ContextTokenBucket != "1k-5k" {
		t.Errorf("ContextTokenBucket max-rank: got %q want 1k-5k", got.ContextTokenBucket)
	}
	if got.ExecutedCount != 3 {
		t.Errorf("ExecutedCount: got %d want 3", got.ExecutedCount)
	}
	if !reflect.DeepEqual(got.KnownExecutedIDs, []string{"qa", "review"}) {
		t.Errorf("KnownExecutedIDs: got %v", got.KnownExecutedIDs)
	}
	if got.UnknownExecutedCount != 3 {
		t.Errorf("UnknownExecutedCount: got %d want 3", got.UnknownExecutedCount)
	}
	// ratio recomputed from summed ExecutedCount / union(KnownExposedIDs) = 3 / 3 = 100
	if got.UtilizationRatioPct != 100 {
		t.Errorf("UtilizationRatioPct: got %d want 100 (recomputed)", got.UtilizationRatioPct)
	}
	if got.WarningBand != WarningBandSevere {
		t.Errorf("WarningBand max-rank: got %q want %q", got.WarningBand, WarningBandSevere)
	}
}

// -----------------------------------------------------------------------------
// Helper-level coverage: every warning band / confidence rank pair
// -----------------------------------------------------------------------------

func TestMaxWarningBand_AllPairs(t *testing.T) {
	// Rank: severe > high > watch > normal > unknown.
	ranks := []string{
		WarningBandUnknown,
		WarningBandNormal,
		WarningBandWatch,
		WarningBandHigh,
		WarningBandSevere,
	}
	for i, lo := range ranks {
		for j, hi := range ranks {
			expected := hi
			if i > j {
				expected = lo
			}
			if got := maxWarningBand(lo, hi); got != expected {
				t.Errorf("maxWarningBand(%q,%q) = %q want %q", lo, hi, got, expected)
			}
		}
	}
}

func TestMaxConfidence_AllPairs(t *testing.T) {
	ranks := []string{"low", "medium", "high"}
	for i, lo := range ranks {
		for j, hi := range ranks {
			expected := hi
			if i > j {
				expected = lo
			}
			if got := maxConfidence(lo, hi); got != expected {
				t.Errorf("maxConfidence(%q,%q) = %q want %q", lo, hi, got, expected)
			}
		}
	}
}

func TestUnionSorted_Determinism(t *testing.T) {
	a := []string{"c", "a", "b", "a"}
	b := []string{"d", "b", ""}
	got := unionSorted(a, b)
	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unionSorted: got %v want %v", got, want)
	}
	// Commutative.
	if !reflect.DeepEqual(unionSorted(a, b), unionSorted(b, a)) {
		t.Error("unionSorted not commutative")
	}
	// Empty inputs return nil so JSON omitempty drops the field.
	if unionSorted(nil, nil) != nil {
		t.Error("unionSorted(nil,nil) must be nil")
	}
}

// -----------------------------------------------------------------------------
// Invariants on mergeEcosystems (identity / commutativity / associativity /
// coverage / bounded-cardinality)
// -----------------------------------------------------------------------------

func ecosystemA() Ecosystem {
	return Ecosystem{
		Client:                "claude_code",
		CodingAgents:          []string{"claude_code"},
		OperatingSystem:       "macos",
		Shell:                 "zsh",
		WorkflowFrameworks:    []string{"spec_kitty"},
		MCPServersKnown:       []string{"github"},
		UnknownMCPServerCount: 1,
		KnownSkills:           []string{"qa"},
		UnknownSkillCount:     0,
		KnownPlugins:          []string{"notion"},
		UnknownPluginCount:    0,
		PackageManagers:       []string{"npm"},
		VersionControl:        "git",
		ToolingUtilization: ToolingUtilization{
			MCP: MCPUtilization{
				KnownServerIDs:       []string{"github"},
				UnknownServerCount:   1,
				CallCount:            10,
				KnownCallCount:       7,
				UniqueKnownCalledIDs: []string{"github"},
				WarningBand:          WarningBandNormal,
				ExposureKnown:        true,
				InferenceSource:      "header",
				ServerCountBucket:    "1-3",
				UtilizationRatioPct:  70, // matches 7/10*100 — must match the merge formula for identity
			},
			Skill: SkillUtilization{
				KnownExposedIDs:     []string{"qa"},
				ExecutedCount:       1,
				KnownExecutedIDs:    []string{"qa"},
				WarningBand:         WarningBandNormal,
				ExposedCountBucket:  "1-3",
				UtilizationRatioPct: 100, // matches 1/1*100
			},
		},
		WorkflowFingerprints: []EcosystemFingerprint{
			{ID: "spec_kitty", Confidence: "medium", Sources: []string{"binary_present"}, EvidenceCount: 2, Installed: true, VersionBucket: "v1"},
		},
	}
}

func ecosystemB() Ecosystem {
	return Ecosystem{
		Client:                "claude_code",
		CodingAgents:          []string{"claude_code"},
		OperatingSystem:       "macos",
		Shell:                 "zsh",
		WorkflowFrameworks:    []string{"openspec"},
		MCPServersKnown:       []string{"linear", "notion"},
		UnknownMCPServerCount: 2,
		KnownSkills:           []string{"review"},
		KnownPlugins:          []string{"linear"},
		PackageManagers:       []string{"pnpm"},
		VersionControl:        "git",
		ToolingUtilization: ToolingUtilization{
			MCP: MCPUtilization{
				KnownServerIDs:       []string{"linear", "notion"},
				UnknownServerCount:   2,
				CallCount:            20,
				KnownCallCount:       8,
				UniqueKnownCalledIDs: []string{"linear"},
				WarningBand:          WarningBandHigh,
				ExposureKnown:        true,
				InferenceSource:      "calls",
				ServerCountBucket:    "4-10",
			},
			Skill: SkillUtilization{
				KnownExposedIDs:    []string{"review", "investigate"},
				ExecutedCount:      2,
				KnownExecutedIDs:   []string{"review"},
				WarningBand:        WarningBandWatch,
				ExposedCountBucket: "4-10",
			},
		},
		WorkflowFingerprints: []EcosystemFingerprint{
			{ID: "spec_kitty", Confidence: "high", Sources: []string{"transcript_command"}, EvidenceCount: 4, Active: true, VersionBucket: "v1"},
			{ID: "openspec", Confidence: "low", Sources: []string{"binary_present"}, EvidenceCount: 1, Installed: true, VersionBucket: "v2"},
		},
	}
}

func ecosystemC() Ecosystem {
	return Ecosystem{
		CodingAgents:    []string{"codex"},
		MCPServersKnown: []string{"github", "supabase"},
		KnownSkills:     []string{"qa", "ship"},
		PackageManagers: []string{"go"},
		ToolingUtilization: ToolingUtilization{
			MCP: MCPUtilization{
				KnownServerIDs:       []string{"github", "supabase"},
				CallCount:            5,
				KnownCallCount:       5,
				UniqueKnownCalledIDs: []string{"supabase"},
				WarningBand:          WarningBandSevere,
				ServerCountBucket:    "1-3",
			},
			Skill: SkillUtilization{
				KnownExposedIDs:  []string{"qa", "ship"},
				ExecutedCount:    3,
				KnownExecutedIDs: []string{"qa"},
				WarningBand:      WarningBandHigh,
			},
		},
		WorkflowFingerprints: []EcosystemFingerprint{
			{ID: "bmad", Confidence: "medium", Sources: []string{"transcript_command"}, EvidenceCount: 2},
		},
	}
}

func TestMergeEcosystems_Identity(t *testing.T) {
	a := ecosystemA()
	empty := Ecosystem{}
	leftMerge := mergeEcosystems(deepCopyEcosystem(a), deepCopyEcosystem(empty))
	rightMerge := mergeEcosystems(deepCopyEcosystem(empty), deepCopyEcosystem(a))
	if !reflect.DeepEqual(leftMerge, rightMerge) {
		t.Fatalf("identity violated: m(A,empty) != m(empty,A)\nleft=%#v\nright=%#v", leftMerge, rightMerge)
	}
	if !reflect.DeepEqual(leftMerge, a) {
		// Note: KnownServerIDs etc. should round-trip; UtilizationRatioPct
		// is recomputed from the same CallCount/KnownCallCount so it must
		// match the per-report value when one side is empty.
		t.Fatalf("identity violated: m(A,empty) != A\nleft=%#v\nA=%#v", leftMerge, a)
	}
}

func TestMergeEcosystems_Commutativity(t *testing.T) {
	a := ecosystemA()
	b := ecosystemB()
	ab := mergeEcosystems(deepCopyEcosystem(a), deepCopyEcosystem(b))
	ba := mergeEcosystems(deepCopyEcosystem(b), deepCopyEcosystem(a))
	if !reflect.DeepEqual(ab, ba) {
		t.Fatalf("commutativity violated: m(A,B) != m(B,A)\nab=%#v\nba=%#v", ab, ba)
	}
}

func TestMergeEcosystems_Associativity(t *testing.T) {
	a := ecosystemA()
	b := ecosystemB()
	c := ecosystemC()
	left := mergeEcosystems(mergeEcosystems(deepCopyEcosystem(a), deepCopyEcosystem(b)), deepCopyEcosystem(c))
	right := mergeEcosystems(deepCopyEcosystem(a), mergeEcosystems(deepCopyEcosystem(b), deepCopyEcosystem(c)))
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("associativity violated: m(m(A,B),C) != m(A,m(B,C))\nleft=%#v\nright=%#v", left, right)
	}
}

func TestMergeEcosystems_Coverage(t *testing.T) {
	a := ecosystemA()
	b := ecosystemB()
	merged := mergeEcosystems(deepCopyEcosystem(a), deepCopyEcosystem(b))
	// Every fingerprint id from either input must appear in the merged output.
	wantIDs := map[string]bool{}
	for _, fp := range a.WorkflowFingerprints {
		wantIDs[fp.ID] = true
	}
	for _, fp := range b.WorkflowFingerprints {
		wantIDs[fp.ID] = true
	}
	for id := range wantIDs {
		found := false
		for _, fp := range merged.WorkflowFingerprints {
			if fp.ID == id {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("coverage violated: fingerprint id %q dropped by merge", id)
		}
	}
}

func TestMergeEcosystems_BoundedCardinality(t *testing.T) {
	merged := mergeEcosystems(ecosystemA(), ecosystemB())
	// WarningBand must be one of the closed enum values.
	closedWarningBands := map[string]bool{
		WarningBandUnknown: true, WarningBandNormal: true, WarningBandWatch: true,
		WarningBandHigh: true, WarningBandSevere: true,
	}
	if !closedWarningBands[merged.ToolingUtilization.MCP.WarningBand] {
		t.Errorf("MCP WarningBand %q outside closed enum", merged.ToolingUtilization.MCP.WarningBand)
	}
	if !closedWarningBands[merged.ToolingUtilization.Skill.WarningBand] {
		t.Errorf("Skill WarningBand %q outside closed enum", merged.ToolingUtilization.Skill.WarningBand)
	}
	// Confidence values on merged fingerprints must be one of the closed enum.
	closedConfidence := map[string]bool{"low": true, "medium": true, "high": true}
	for _, fp := range merged.WorkflowFingerprints {
		if !closedConfidence[fp.Confidence] {
			t.Errorf("fingerprint %q confidence %q outside closed enum", fp.ID, fp.Confidence)
		}
	}
	// UtilizationRatioPct in [0,100].
	if r := merged.ToolingUtilization.MCP.UtilizationRatioPct; r < 0 || r > 100 {
		t.Errorf("MCP ratio %d outside [0,100]", r)
	}
	if r := merged.ToolingUtilization.Skill.UtilizationRatioPct; r < 0 || r > 100 {
		t.Errorf("Skill ratio %d outside [0,100]", r)
	}
}

// deepCopyEcosystem creates a defensive copy of an Ecosystem so commutativity/
// associativity tests aren't sensitive to in-place mutation by the helper.
// (mergeEcosystems takes `left` by value but unionSorted etc. only return
// fresh slices, so this is belt-and-braces.)
func deepCopyEcosystem(e Ecosystem) Ecosystem {
	out := e
	out.CodingAgents = append([]string(nil), e.CodingAgents...)
	out.WorkflowFrameworks = append([]string(nil), e.WorkflowFrameworks...)
	out.MCPServersKnown = append([]string(nil), e.MCPServersKnown...)
	out.KnownSkills = append([]string(nil), e.KnownSkills...)
	out.KnownPlugins = append([]string(nil), e.KnownPlugins...)
	out.PackageManagers = append([]string(nil), e.PackageManagers...)
	out.ToolingUtilization.MCP.KnownServerIDs = append([]string(nil), e.ToolingUtilization.MCP.KnownServerIDs...)
	out.ToolingUtilization.MCP.UniqueKnownCalledIDs = append([]string(nil), e.ToolingUtilization.MCP.UniqueKnownCalledIDs...)
	out.ToolingUtilization.Skill.KnownExposedIDs = append([]string(nil), e.ToolingUtilization.Skill.KnownExposedIDs...)
	out.ToolingUtilization.Skill.KnownExecutedIDs = append([]string(nil), e.ToolingUtilization.Skill.KnownExecutedIDs...)
	out.WorkflowFingerprints = append([]EcosystemFingerprint(nil), e.WorkflowFingerprints...)
	for i, fp := range out.WorkflowFingerprints {
		out.WorkflowFingerprints[i].Sources = append([]string(nil), fp.Sources...)
	}
	return out
}

// -----------------------------------------------------------------------------
// NFR-005: 100-input merge under 5s
// -----------------------------------------------------------------------------

func TestAggregateReports100_PerfCeiling(t *testing.T) {
	// Build 100 reports with realistic-sized Ecosystem payloads. We construct
	// them directly (not through Analyze()) so the timing isolates aggregate
	// merge cost rather than parsing throughput.
	reports := make([]Report, 0, 100)
	for i := 0; i < 100; i++ {
		reports = append(reports, Report{
			JobID:     "perf-input",
			Version:   Version,
			Metrics:   Metrics{Turns: 5, EstimatedTokens: 1000, ToolOutputTokens: 200},
			Ecosystem: ecosystemA(),
		})
	}
	start := time.Now()
	_, err := AggregateReports("perf-100", reports, len(reports))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("AggregateReports failed: %v", err)
	}
	// NFR-005 ceiling: < 5s on a developer laptop. Failure message names
	// the actual elapsed time so flakes are diagnosable.
	if elapsed >= 5*time.Second {
		t.Fatalf("NFR-005 violated: 100-input AggregateReports took %v (ceiling 5s)", elapsed)
	}
	t.Logf("100-input AggregateReports elapsed: %v", elapsed)
}
