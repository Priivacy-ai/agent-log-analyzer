package analyzer

import "testing"

// TestToolStateMapResolve locks the FR-018 conflict precedence directly,
// independent of whether the engine's Recommend path calls Resolve. The
// engine currently assumes callers pre-resolve evidence (see the "Known
// Phase A gaps" section of token-saving-recommendation-engine.md), so
// Resolve is a public helper for callers. This test pins its contract.
//
// Precedence (highest trust first):
//
//	rejected_medium > active_high > configured_medium >
//	installed_medium > mentioned_low > unknown
func TestToolStateMapResolve(t *testing.T) {
	var m ToolStateMap

	// Order is the precedence ladder from research.md §4.
	// Lower index = lower trust; higher index = higher trust.
	ladder := []ToolState{
		ToolStateUnknown,
		ToolStateMentionedLow,
		ToolStateInstalledMedium,
		ToolStateConfiguredMedium,
		ToolStateActiveHigh,
		ToolStateRejectedMedium,
	}

	for i, lo := range ladder {
		for j, hi := range ladder {
			got := m.Resolve(lo, hi)
			want := hi
			if i >= j {
				want = lo
			}
			if got != want {
				t.Errorf("Resolve(%q, %q) = %q; want %q (precedence: lo=%d hi=%d)",
					lo, hi, got, want, i, j)
			}
		}
	}

	// Symmetry probe: Resolve must be commutative under the precedence map.
	for _, a := range ladder {
		for _, b := range ladder {
			if got := m.Resolve(a, b); got != m.Resolve(b, a) {
				t.Errorf("Resolve not commutative: Resolve(%q,%q)=%q vs Resolve(%q,%q)=%q",
					a, b, got, b, a, m.Resolve(b, a))
			}
		}
	}

	// Spot-check the critical "rejected beats active" decision from
	// research.md §4 — the policy choice that prevents re-recommending a
	// tool the user already opted out of.
	if got := m.Resolve(ToolStateRejectedMedium, ToolStateActiveHigh); got != ToolStateRejectedMedium {
		t.Errorf("Resolve(rejected_medium, active_high) = %q; want rejected_medium (the user-opt-out signal must win)", got)
	}
}
