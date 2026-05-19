package sdd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// secondRingRegistryPaths lists every on-disk detector tier file we evaluate
// for the second-ring matrix. We load both first-class and second-ring tiers
// because FR-014 requires that a second-ring fixture must NOT trigger any of
// the first-class detectors; that cross-negative check needs the first-class
// detectors to actually be present in the evaluator's registry slice.
//
// Loading directly (not via LoadRegistry) keeps the sync.Once-memoized global
// registry untouched, just as evaluator_first_class_test.go does.
//
// NOTE: post-mission re-research promoted GSD to verified. The second-ring
// tier now ships kiro + bmad + gsd. The 10 long-tail tools that previously sat
// in research_needed were re-researched in the same pass: 5 were verified and
// promoted to production detectors (sdd_pilot, spec2ship, paul, fspec, tessl);
// 5 had no public-source anchor and were removed entirely (spec_driven_develop,
// whenwords, intent, agentic_code, codespeak).
var secondRingRegistryPaths = []string{
	"../signatures/sdd_detectors_first_class.json",
	"../signatures/sdd_detectors_second_ring.json",
}

// readFixtureSR loads a fixture file from internal/analyzer/sdd/testdata/fixtures
// and returns its contents as a string. SR-suffixed to avoid colliding with
// readFixture in evaluator_first_class_test.go (Go package-level scope).
func readFixtureSR(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// loadSecondRingRegistry parses every detector tier file (first-class +
// second-ring) and returns the verified subset, mirroring LoadRegistry's
// verified-only filter. This lets the second-ring test matrix verify both
// positive detection and cross-negative against the first-class trio.
func loadSecondRingRegistry(t *testing.T) []SDDDetector {
	t.Helper()
	all := make([]SDDDetector, 0)
	for _, p := range secondRingRegistryPaths {
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
	if len(verified) < 6 {
		t.Fatalf("expected at least 6 verified detectors (3 first-class + 3 second-ring); got %d", len(verified))
	}
	return verified
}

// evaluateWithSecondRingRegistry runs Evaluate against the combined registry
// (first-class + second-ring) with a zero-installed FakeProbe so detection
// is driven purely by text markers.
func evaluateWithSecondRingRegistry(t *testing.T, text string) []Fingerprint {
	t.Helper()
	return Evaluate(context.Background(), text, nil, FakeProbe{}, loadSecondRingRegistry(t))
}

// assertHasIDSR fails the test if no fingerprint with the given id is present
// in fps. SR-suffixed to avoid colliding with assertHasID in
// evaluator_first_class_test.go.
func assertHasIDSR(t *testing.T, fps []Fingerprint, id string) {
	t.Helper()
	for _, fp := range fps {
		if fp.ID == id {
			return
		}
	}
	t.Fatalf("expected fingerprint with id=%q in %+v", id, fps)
}

// assertNotHasIDSR fails the test if any fingerprint with the given id is
// present in fps. SR-suffixed for the same reason as assertHasIDSR.
func assertNotHasIDSR(t *testing.T, fps []Fingerprint, id string) {
	t.Helper()
	for _, fp := range fps {
		if fp.ID == id {
			t.Fatalf("did not expect fingerprint with id=%q; got %+v", id, fps)
		}
	}
}

// TestSecondRingPositive verifies that each second-ring fixture triggers its
// own detector and triggers NONE of the first-class detectors (spec_kitty,
// github_spec_kit, openspec). This is the FR-014 cross-negative assertion for
// the second ring.
//
// Per WP06 scope plus the post-mission re-research follow-up: second-ring now
// covers kiro, bmad, and gsd.
func TestSecondRingPositive(t *testing.T) {
	cases := []struct {
		fixture string
		want    string
	}{
		{"kiro.txt", "kiro"},
		{"bmad.txt", "bmad"},
		{"gsd.txt", "gsd"},
	}
	firstClass := []string{"spec_kitty", "github_spec_kit", "openspec"}
	secondRing := []string{"kiro", "bmad", "gsd"}
	for _, c := range cases {
		c := c
		t.Run(c.fixture, func(t *testing.T) {
			text := readFixtureSR(t, c.fixture)
			fps := evaluateWithSecondRingRegistry(t, text)
			// Positive: target detector fires.
			assertHasIDSR(t, fps, c.want)
			// Cross-negative: none of the first-class trio fires.
			for _, fc := range firstClass {
				assertNotHasIDSR(t, fps, fc)
			}
			// Cross-negative within the second ring: other second-ring
			// detectors do not fire.
			for _, sr := range secondRing {
				if sr == c.want {
					continue
				}
				assertNotHasIDSR(t, fps, sr)
			}
		})
	}
}

// TestSecondRingGenericOnlyTriggersNothing re-runs the FR-012 regression for
// the combined first-class + second-ring registry: a transcript containing
// only generic SDD-adjacent terminology must trigger zero fingerprints across
// the full registry.
func TestSecondRingGenericOnlyTriggersNothing(t *testing.T) {
	text := readFixtureSR(t, "generic_only.txt")
	fps := evaluateWithSecondRingRegistry(t, text)
	if len(fps) != 0 {
		t.Fatalf("expected zero fingerprints from generic-only fixture against combined registry; got %+v", fps)
	}
}
