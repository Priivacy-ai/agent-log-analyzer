package sdd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// longTailRegistryPaths lists every on-disk detector tier file we evaluate
// for the long-tail matrix. We load first-class + second-ring + long-tail
// tiers because FR-014 requires that a long-tail fixture must NOT trigger any
// of the first-class or second-ring detectors; that cross-negative check
// needs those detectors to actually be present in the evaluator's registry
// slice.
//
// Loading directly (not via LoadRegistry) keeps the sync.Once-memoized global
// registry untouched, just as evaluator_first_class_test.go and
// evaluator_second_ring_test.go do.
//
// NOTE: post-mission re-research promoted 5 additional long-tail tools to
// verified (sdd_pilot, spec2ship, paul, fspec, tessl). The long-tail tier
// now ships 9 verified detectors. The 5 remaining tools from the original
// list (spec_driven_develop, whenwords, intent, agentic_code, codespeak)
// were removed entirely — no public-source anchor was found for them.
var longTailRegistryPaths = []string{
	"../signatures/sdd_detectors_first_class.json",
	"../signatures/sdd_detectors_second_ring.json",
	"../signatures/sdd_detectors_long_tail.json",
}

// readFixtureLT loads a fixture file from internal/analyzer/sdd/testdata/fixtures
// and returns its contents as a string. LT-suffixed to avoid colliding with
// readFixture / readFixtureSR in sibling test files (Go package-level scope).
func readFixtureLT(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// loadLongTailRegistry parses every detector tier file (first-class +
// second-ring + long-tail) and returns the verified subset, mirroring
// LoadRegistry's verified-only filter. This lets the long-tail test matrix
// verify both positive detection and cross-negative against the 5 prior
// detectors (spec_kitty, github_spec_kit, openspec, kiro, bmad).
func loadLongTailRegistry(t *testing.T) []SDDDetector {
	t.Helper()
	all := make([]SDDDetector, 0)
	for _, p := range longTailRegistryPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read registry %s: %v", p, err)
		}
		parsed, err := parseDetectors(data)
		if err != nil {
			t.Fatalf("parse registry %s: %v", p, err)
		}
		all = append(all, parsed...)
	}
	verified := make([]SDDDetector, 0, len(all))
	for _, d := range all {
		if d.Status == StatusVerified {
			verified = append(verified, d)
		}
	}
	// 3 first-class + 3 second-ring + 9 long-tail = 15 verified detectors.
	if len(verified) != 15 {
		t.Fatalf("expected exactly 15 verified detectors (3 first-class + 3 second-ring + 9 long-tail); got %d", len(verified))
	}
	return verified
}

// evaluateWithLongTailRegistry runs Evaluate against the combined registry
// (first-class + second-ring + long-tail) with a zero-installed FakeProbe so
// detection is driven purely by text markers.
func evaluateWithLongTailRegistry(t *testing.T, text string) []Fingerprint {
	t.Helper()
	return Evaluate(context.Background(), text, nil, FakeProbe{}, loadLongTailRegistry(t))
}

// assertHasIDLT fails the test if no fingerprint with the given id is present
// in fps. LT-suffixed to avoid colliding with assertHasID / assertHasIDSR in
// sibling test files.
func assertHasIDLT(t *testing.T, fps []Fingerprint, id string) {
	t.Helper()
	for _, fp := range fps {
		if fp.ID == id {
			return
		}
	}
	t.Fatalf("expected fingerprint with id=%q in %+v", id, fps)
}

// assertNotHasIDLT fails the test if any fingerprint with the given id is
// present in fps. LT-suffixed for the same reason as assertHasIDLT.
func assertNotHasIDLT(t *testing.T, fps []Fingerprint, id string) {
	t.Helper()
	for _, fp := range fps {
		if fp.ID == id {
			t.Fatalf("did not expect fingerprint with id=%q; got %+v", id, fps)
		}
	}
}

// TestLongTailPositive verifies that each verified long-tail fixture triggers
// its own detector. This is the positive arm of FR-014 for the long-tail
// tier (post-re-research: 9 verified long-tail detectors).
func TestLongTailPositive(t *testing.T) {
	cases := []struct {
		fixture string
		want    string
	}{
		{"spec_workflow_mcp.txt", "spec_workflow_mcp"},
		{"chatdev.txt", "chatdev"},
		{"cognition_devin.txt", "cognition_devin"},
		{"microsoft_agent_framework.txt", "microsoft_agent_framework"},
		{"sdd_pilot.txt", "sdd_pilot"},
		{"spec2ship.txt", "spec2ship"},
		{"paul.txt", "paul"},
		{"fspec.txt", "fspec"},
		{"tessl.txt", "tessl"},
	}
	longTail := []string{
		"spec_workflow_mcp", "chatdev", "cognition_devin", "microsoft_agent_framework",
		"sdd_pilot", "spec2ship", "paul", "fspec", "tessl",
	}
	for _, c := range cases {
		c := c
		t.Run(c.fixture, func(t *testing.T) {
			text := readFixtureLT(t, c.fixture)
			fps := evaluateWithLongTailRegistry(t, text)
			// Positive: target detector fires.
			assertHasIDLT(t, fps, c.want)
			// Cross-negative within long-tail: other long-tail detectors
			// do not fire on this fixture.
			for _, lt := range longTail {
				if lt == c.want {
					continue
				}
				assertNotHasIDLT(t, fps, lt)
			}
		})
	}
}

// TestLongTailCrossNegative enforces FR-014 for the long-tail tier against
// the 6 prior detectors: for each long-tail fixture, none of spec_kitty,
// github_spec_kit, openspec, kiro, bmad, gsd may fire.
//
// Note: the 5 removed tools (spec_driven_develop, whenwords, intent,
// agentic_code, codespeak) are absent from the registry entirely and
// therefore have no IDs to assert against.
func TestLongTailCrossNegative(t *testing.T) {
	fixtures := []string{
		"spec_workflow_mcp.txt",
		"chatdev.txt",
		"cognition_devin.txt",
		"microsoft_agent_framework.txt",
		"sdd_pilot.txt",
		"spec2ship.txt",
		"paul.txt",
		"fspec.txt",
		"tessl.txt",
	}
	priorIDs := []string{"spec_kitty", "github_spec_kit", "openspec", "kiro", "bmad", "gsd"}
	for _, f := range fixtures {
		f := f
		t.Run(f, func(t *testing.T) {
			text := readFixtureLT(t, f)
			fps := evaluateWithLongTailRegistry(t, text)
			for _, id := range priorIDs {
				assertNotHasIDLT(t, fps, id)
			}
		})
	}
}

// TestLongTailGenericOnlyTriggersNothing re-runs the FR-012 regression for
// the full 9-detector registry: a transcript containing only generic
// SDD-adjacent terminology must trigger zero fingerprints across the
// combined first-class + second-ring + long-tail registry.
func TestLongTailGenericOnlyTriggersNothing(t *testing.T) {
	text := readFixtureLT(t, "generic_only.txt")
	fps := evaluateWithLongTailRegistry(t, text)
	if len(fps) != 0 {
		t.Fatalf("expected zero fingerprints from generic-only fixture against combined 9-detector registry; got %+v", fps)
	}
}
