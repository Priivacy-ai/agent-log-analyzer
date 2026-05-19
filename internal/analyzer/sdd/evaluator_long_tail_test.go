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
// NOTE: WP07 ships only 4 verified long-tail detectors (spec_workflow_mcp,
// chatdev, cognition_devin, microsoft_agent_framework). The 10 other
// long-tail tools enumerated in WP04 (sdd_pilot, spec_driven_develop,
// spec2ship, paul, fspec, whenwords, intent, tessl, agentic_code, codespeak)
// remain research_needed and intentionally do NOT ship production detectors
// or fixtures per FR-013. C-001 scope decision is awaiting mission-review
// per assumption A-04.
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
	// 3 first-class + 2 second-ring + 4 long-tail = 9 verified detectors.
	if len(verified) != 9 {
		t.Fatalf("expected exactly 9 verified detectors (3 first-class + 2 second-ring + 4 long-tail); got %d", len(verified))
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
// tier (WP07 ships 4 verified detectors).
func TestLongTailPositive(t *testing.T) {
	cases := []struct {
		fixture string
		want    string
	}{
		{"spec_workflow_mcp.txt", "spec_workflow_mcp"},
		{"chatdev.txt", "chatdev"},
		{"cognition_devin.txt", "cognition_devin"},
		{"microsoft_agent_framework.txt", "microsoft_agent_framework"},
	}
	longTail := []string{"spec_workflow_mcp", "chatdev", "cognition_devin", "microsoft_agent_framework"}
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
// the 5 prior detectors: for each of the 4 long-tail fixtures, none of
// spec_kitty, github_spec_kit, openspec, kiro, bmad may fire. The full
// matrix produces 4 fixtures × 5 cross-negative targets = 20 explicit
// cross-negative assertions.
//
// Note: gsd is intentionally absent — it does not ship in WP06 (research_needed
// per WP04 research and deferred per C-001 / A-04). Likewise, the 10
// research_needed long-tail tools (sdd_pilot, spec_driven_develop, spec2ship,
// paul, fspec, whenwords, intent, tessl, agentic_code, codespeak) are absent
// from the registry per FR-013 and therefore have no IDs to assert against.
func TestLongTailCrossNegative(t *testing.T) {
	fixtures := []string{
		"spec_workflow_mcp.txt",
		"chatdev.txt",
		"cognition_devin.txt",
		"microsoft_agent_framework.txt",
	}
	priorIDs := []string{"spec_kitty", "github_spec_kit", "openspec", "kiro", "bmad"}
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
