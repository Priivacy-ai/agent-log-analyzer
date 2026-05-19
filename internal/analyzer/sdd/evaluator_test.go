package sdd

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// mustBuildDetectors decodes a JSON array of SDDDetector records through
// parseDetectors so that regex compilation and field validation run exactly
// as they do in production. Tests use synthetic IDs/markers — the real
// embedded registry is intentionally not exercised here.
func mustBuildDetectors(t *testing.T, jsonBody string) []SDDDetector {
	t.Helper()
	got, err := parseDetectors([]byte(jsonBody))
	if err != nil {
		t.Fatalf("parseDetectors failed: %v\nbody:\n%s", err, jsonBody)
	}
	return got
}

func TestEvaluateHighConfidenceWithCLIBinary(t *testing.T) {
	registry := mustBuildDetectors(t, `[{
        "id": "tool_a",
        "display_name": "Tool A",
        "category": "spec_driven_workflow",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [
            {"source_class":"config_dir","pattern":"\\.tool-a/"},
            {"source_class":"cli_binary","binary":"tool-a"}
        ],
        "confidence_rules": [
            {"confidence":"high","requires_distinct_classes":2}
        ]
    }]`)

	probe := FakeProbe{Installed: map[string]bool{"tool-a": true}}
	got := Evaluate(context.Background(), "this transcript mentions .tool-a/ heavily", nil, probe, registry)
	if len(got) != 1 {
		t.Fatalf("expected 1 fingerprint, got %d: %+v", len(got), got)
	}
	fp := got[0]
	if fp.ID != "tool_a" {
		t.Errorf("ID: want tool_a, got %q", fp.ID)
	}
	if fp.Confidence != "high" {
		t.Errorf("Confidence: want high, got %q", fp.Confidence)
	}
	if !fp.Installed {
		t.Errorf("Installed: want true, got false")
	}
	if !fp.Active {
		t.Errorf("Active: want true (high + cli_binary runtime-touch), got false")
	}
	wantSources := []string{"cli_binary", "config_dir"}
	if !reflect.DeepEqual(fp.Sources, wantSources) {
		t.Errorf("Sources: want %v, got %v", wantSources, fp.Sources)
	}
	if fp.EvidenceCount != 2 {
		t.Errorf("EvidenceCount: want 2, got %d", fp.EvidenceCount)
	}
}

func TestEvaluateLowConfidenceTextualMention(t *testing.T) {
	registry := mustBuildDetectors(t, `[{
        "id": "tool_b",
        "display_name": "Tool B",
        "category": "spec_driven_workflow",
        "competitor_priority": 5,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [
            {"source_class":"command_name","pattern":"\\btoolb\\b"}
        ],
        "confidence_rules": [
            {"confidence":"low","requires_any_of":["command_name"]}
        ]
    }]`)

	probe := FakeProbe{}
	got := Evaluate(context.Background(), "I just ran toolb yesterday", nil, probe, registry)
	if len(got) != 1 {
		t.Fatalf("expected 1 fingerprint, got %d", len(got))
	}
	fp := got[0]
	if fp.Confidence != "low" {
		t.Errorf("Confidence: want low, got %q", fp.Confidence)
	}
	if fp.Installed {
		t.Errorf("Installed: want false, got true")
	}
	if fp.Active {
		t.Errorf("Active: want false for low-confidence, got true")
	}
}

func TestEvaluateNegativeMarkerVeto(t *testing.T) {
	registry := mustBuildDetectors(t, `[{
        "id": "tool_c",
        "display_name": "Tool C",
        "category": "spec_driven_workflow",
        "competitor_priority": 3,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [
            {"source_class":"config_dir","pattern":"\\.tool-c/"},
            {"source_class":"config_file","pattern":"not-tool-c","negative":true}
        ],
        "confidence_rules": [
            {"confidence":"high","requires_any_of":["config_dir"]}
        ]
    }]`)

	probe := FakeProbe{}
	text := "I see .tool-c/ and also not-tool-c in here"
	got := Evaluate(context.Background(), text, nil, probe, registry)
	if len(got) != 0 {
		t.Fatalf("expected 0 fingerprints (negative veto), got %d: %+v", len(got), got)
	}
}

func TestEvaluateOrderingByCompetitorPriority(t *testing.T) {
	registry := mustBuildDetectors(t, `[
        {
            "id": "lower_priority_tool",
            "display_name": "Lower",
            "category": "x",
            "competitor_priority": 2,
            "status": "verified",
            "source_references": [{"kind":"docs","url":"https://x"}],
            "markers": [{"source_class":"command_name","pattern":"lower"}],
            "confidence_rules": [{"confidence":"low","requires_any_of":["command_name"]}]
        },
        {
            "id": "higher_priority_tool",
            "display_name": "Higher",
            "category": "x",
            "competitor_priority": 1,
            "status": "verified",
            "source_references": [{"kind":"docs","url":"https://x"}],
            "markers": [{"source_class":"command_name","pattern":"higher"}],
            "confidence_rules": [{"confidence":"low","requires_any_of":["command_name"]}]
        }
    ]`)

	got := Evaluate(context.Background(), "lower and higher both appear", nil, FakeProbe{}, registry)
	if len(got) != 2 {
		t.Fatalf("expected 2 fingerprints, got %d", len(got))
	}
	if got[0].ID != "higher_priority_tool" {
		t.Errorf("priority=1 should sort first, got order: %s, %s", got[0].ID, got[1].ID)
	}
	if got[1].ID != "lower_priority_tool" {
		t.Errorf("priority=2 should sort second, got order: %s, %s", got[0].ID, got[1].ID)
	}
}

func TestEvaluateNoMatches(t *testing.T) {
	registry := mustBuildDetectors(t, `[{
        "id": "absent_tool",
        "display_name": "Absent",
        "category": "x",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [{"source_class":"command_name","pattern":"never-appears-here"}],
        "confidence_rules": [{"confidence":"low","requires_any_of":["command_name"]}]
    }]`)

	got := Evaluate(context.Background(), "totally unrelated transcript content", nil, FakeProbe{}, registry)
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %d entries", len(got))
	}
}

func TestEvaluateCLIVersionProbe(t *testing.T) {
	registry := mustBuildDetectors(t, `[{
        "id": "foo_tool",
        "display_name": "Foo",
        "category": "x",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [
            {"source_class":"cli_binary","binary":"foo"},
            {"source_class":"cli_version_probe","binary":"foo","version_args":["--version"]}
        ],
        "confidence_rules": [
            {"confidence":"high","requires_distinct_classes":2}
        ]
    }]`)

	probe := FakeProbe{
		Installed: map[string]bool{"foo": true},
		Versions:  map[string]string{"foo": "foo 1.2.3"},
	}
	got := Evaluate(context.Background(), "", nil, probe, registry)
	if len(got) != 1 {
		t.Fatalf("expected 1 fingerprint, got %d", len(got))
	}
	fp := got[0]
	if !fp.Installed {
		t.Errorf("Installed: want true, got false")
	}
	if fp.VersionBucket != "1.2" {
		t.Errorf("VersionBucket: want 1.2, got %q", fp.VersionBucket)
	}
	srcStr := strings.Join(fp.Sources, ",")
	if !strings.Contains(srcStr, "cli_binary") || !strings.Contains(srcStr, "cli_version_probe") {
		t.Errorf("Sources should include both cli_binary and cli_version_probe, got %v", fp.Sources)
	}
	if fp.Confidence != "high" {
		t.Errorf("Confidence: want high, got %q", fp.Confidence)
	}
}

func TestEvaluateActiveRequiresRuntimeTouch(t *testing.T) {
	// config_dir + package_manifest both match → 2 distinct classes → high
	// confidence — but neither is a runtime-touch source, so Active must be
	// false per data-model.md.
	registry := mustBuildDetectors(t, `[{
        "id": "passive_tool",
        "display_name": "Passive",
        "category": "x",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [
            {"source_class":"config_dir","pattern":"\\.passive/"},
            {"source_class":"package_manifest","pattern":"passive-package"}
        ],
        "confidence_rules": [
            {"confidence":"high","requires_distinct_classes":2}
        ]
    }]`)

	text := "I see .passive/ and passive-package referenced"
	got := Evaluate(context.Background(), text, nil, FakeProbe{}, registry)
	if len(got) != 1 {
		t.Fatalf("expected 1 fingerprint, got %d", len(got))
	}
	fp := got[0]
	if fp.Confidence != "high" {
		t.Errorf("Confidence: want high, got %q", fp.Confidence)
	}
	if fp.Active {
		t.Errorf("Active: want false (no runtime-touch source), got true")
	}
}

// TestEvaluateSlashHitsFallback exercises the slashHits fallback path:
// a slash_command marker that does NOT appear in the raw text but DOES
// appear in the caller-extracted slashHits slice must still fire. This
// is the RISK-3 regression — earlier versions of Evaluate discarded
// slashHits entirely, so the marker silently never matched.
func TestEvaluateSlashHitsFallback(t *testing.T) {
	registry := mustBuildDetectors(t, `[{
        "id": "tool_slash",
        "display_name": "Tool Slash",
        "category": "spec_driven_workflow",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [
            {"source_class":"slash_command","pattern":"(?i)/tool-slash\\.do"}
        ],
        "confidence_rules": [
            {"confidence":"medium","requires_any_of":["slash_command"]}
        ]
    }]`)

	// Raw transcript text does NOT contain the slash invocation. A
	// normalizing parser stripped it out and surfaced the token via
	// slashHits instead.
	const text = "unrelated transcript prose with no slash invocation"
	slashHits := []string{"/tool-slash.do"}

	got := Evaluate(context.Background(), text, slashHits, FakeProbe{}, registry)
	if len(got) != 1 {
		t.Fatalf("expected 1 fingerprint via slashHits fallback, got %d: %+v", len(got), got)
	}
	fp := got[0]
	if fp.ID != "tool_slash" {
		t.Errorf("ID: want tool_slash, got %q", fp.ID)
	}
	if fp.Confidence != "medium" {
		t.Errorf("Confidence: want medium, got %q", fp.Confidence)
	}

	// Sanity check: without slashHits, the same input must NOT fire.
	got = Evaluate(context.Background(), text, nil, FakeProbe{}, registry)
	if len(got) != 0 {
		t.Fatalf("expected 0 fingerprints when slashHits is nil, got %d: %+v", len(got), got)
	}
}
