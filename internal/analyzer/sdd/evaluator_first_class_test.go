package sdd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// firstClassRegistryPath is the on-disk location of the WP05 detector tier
// file relative to this test file. We load it directly (rather than via
// LoadRegistry) so the global sync.Once memoization owned by other tests
// in this package — notably TestLoadRegistryEmptyBase, which intentionally
// exercises the nil-ChunksProvider path — is left untouched.
const firstClassRegistryPath = "../signatures/sdd_detectors_first_class.json"

// readFixture loads a fixture file from internal/analyzer/sdd/testdata/fixtures
// and returns its contents as a string. Fixtures are hand-curated, sanitized
// claude-code-style transcripts that exercise each first-class SDD detector.
func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "fixtures", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

// loadFirstClassRegistry parses the WP05 JSON tier file via the production
// parseDetectors path (so regex compilation and field validation run exactly
// as they do at startup) and returns the verified detector slice.
func loadFirstClassRegistry(t *testing.T) []SDDDetector {
	t.Helper()
	data, err := os.ReadFile(firstClassRegistryPath)
	if err != nil {
		t.Fatalf("read first-class registry: %v", err)
	}
	detectors, err := parseDetectors(data)
	if err != nil {
		t.Fatalf("parse first-class registry: %v", err)
	}
	// Mirror LoadRegistry's verified-only filter so tests reflect production
	// evaluation semantics.
	verified := make([]SDDDetector, 0, len(detectors))
	for _, d := range detectors {
		if d.Status == StatusVerified {
			verified = append(verified, d)
		}
	}
	if len(verified) == 0 {
		t.Fatalf("first-class registry has zero verified detectors")
	}
	return verified
}

// evaluateWithRealRegistry runs Evaluate against the WP05 detector tier with
// a zero-installed FakeProbe so detection is driven purely by text markers
// (matching what the cross-negative matrix in NFR-004 is designed to verify).
func evaluateWithRealRegistry(t *testing.T, text string) []Fingerprint {
	t.Helper()
	return Evaluate(context.Background(), text, nil, FakeProbe{}, loadFirstClassRegistry(t))
}

// assertHasID fails the test if no fingerprint with the given id is present
// in fps.
func assertHasID(t *testing.T, fps []Fingerprint, id string) {
	t.Helper()
	for _, fp := range fps {
		if fp.ID == id {
			return
		}
	}
	t.Fatalf("expected fingerprint with id=%q in %+v", id, fps)
}

// assertNotHasID fails the test if any fingerprint with the given id is
// present in fps. This is the cross-negative assertion: a fixture for one
// tool must not produce a fingerprint for a sibling tool.
func assertNotHasID(t *testing.T, fps []Fingerprint, id string) {
	t.Helper()
	for _, fp := range fps {
		if fp.ID == id {
			t.Fatalf("did not expect fingerprint with id=%q; got %+v", id, fps)
		}
	}
}

// TestCrossNegativeFirstClass enforces NFR-004: each first-class SDD fixture
// triggers exactly its own detector and none of the other two. The 3×3
// matrix produces 3 positive assertions (assertHasID) and 6 cross-negative
// assertions (assertNotHasID) for 9 total checks across the matrix.
func TestCrossNegativeFirstClass(t *testing.T) {
	cases := []struct {
		fixture string
		want    string
	}{
		{"spec_kitty.txt", "spec_kitty"},
		{"github_spec_kit.txt", "github_spec_kit"},
		{"openspec.txt", "openspec"},
	}
	allFirstClass := []string{"spec_kitty", "github_spec_kit", "openspec"}
	for _, c := range cases {
		c := c
		t.Run(c.fixture, func(t *testing.T) {
			text := readFixture(t, c.fixture)
			fps := evaluateWithRealRegistry(t, text)
			// Assertion 1: the expected detector fires.
			assertHasID(t, fps, c.want)
			// Assertions 2 & 3: the other two first-class detectors do NOT fire.
			for _, other := range allFirstClass {
				if other == c.want {
					continue
				}
				assertNotHasID(t, fps, other)
			}
		})
	}
}

// TestGenericOnlyTriggersNothing enforces FR-012: a transcript containing
// only generic SDD-adjacent terminology (specs/, tasks.md, design.md, hooks,
// STATE.md, requirements.md) must trigger zero first-class fingerprints.
func TestGenericOnlyTriggersNothing(t *testing.T) {
	text := readFixture(t, "generic_only.txt")
	fps := evaluateWithRealRegistry(t, text)
	if len(fps) != 0 {
		t.Fatalf("expected zero fingerprints from generic-only fixture; got %+v", fps)
	}
}
