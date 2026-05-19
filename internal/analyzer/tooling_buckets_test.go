package analyzer

import "testing"

func TestCountBucket(t *testing.T) {
	allowed := map[string]bool{
		"none": true, "1-3": true, "4-10": true, "11-25": true,
		"26-50": true, "51-100": true, "100+": true, "unknown": true,
	}
	cases := []struct {
		n     int
		known bool
		want  string
	}{
		{0, true, "none"},
		{1, true, "1-3"},
		{3, true, "1-3"},
		{4, true, "4-10"},
		{10, true, "4-10"},
		{11, true, "11-25"},
		{25, true, "11-25"},
		{26, true, "26-50"},
		{50, true, "26-50"},
		{51, true, "51-100"},
		{100, true, "51-100"},
		{101, true, "100+"},
		{10000, true, "100+"},
		{0, false, "unknown"},
		{500, false, "unknown"},
	}
	for _, tc := range cases {
		got := countBucket(tc.n, tc.known)
		if got != tc.want {
			t.Errorf("countBucket(%d, %v) = %q, want %q", tc.n, tc.known, got, tc.want)
		}
		if !allowed[got] {
			t.Errorf("countBucket(%d, %v) returned non-enum value %q", tc.n, tc.known, got)
		}
	}
}

func TestTokenBucket(t *testing.T) {
	allowed := map[string]bool{
		"none": true, "<1k": true, "1k-5k": true, "5k-15k": true,
		"15k-50k": true, "50k+": true, "unknown": true,
	}
	cases := []struct {
		tokens int
		known  bool
		want   string
	}{
		{0, true, "none"},
		{1, true, "<1k"},
		{999, true, "<1k"},
		{1000, true, "1k-5k"},
		{4999, true, "1k-5k"},
		{5000, true, "5k-15k"},
		{14999, true, "5k-15k"},
		{15000, true, "15k-50k"},
		{49999, true, "15k-50k"},
		{50000, true, "50k+"},
		{100000, true, "50k+"},
		{0, false, "unknown"},
		{12345, false, "unknown"},
	}
	for _, tc := range cases {
		got := tokenBucket(tc.tokens, tc.known)
		if got != tc.want {
			t.Errorf("tokenBucket(%d, %v) = %q, want %q", tc.tokens, tc.known, got, tc.want)
		}
		if !allowed[got] {
			t.Errorf("tokenBucket(%d, %v) returned non-enum value %q", tc.tokens, tc.known, got)
		}
	}
}

func TestEfficiencyBucket(t *testing.T) {
	allowed := map[string]bool{
		"unused": true, "underutilized": true, "moderate": true,
		"well-utilized": true, "unknown": true,
	}
	// Cross-product of ratios x token-bucket labels with known=true.
	// Rule recap:
	//   known=false                                       -> "unknown"
	//   ratio < 5 AND tokenBucket not in {"none","<1k"}   -> "unused"
	//   ratio < 30                                        -> "underutilized"
	//   ratio < 70                                        -> "moderate"
	//   otherwise                                         -> "well-utilized"
	cases := []struct {
		ratio  int
		tokBkt string
		known  bool
		want   string
	}{
		// ratio 0 (below 5)
		{0, "none", true, "underutilized"},
		{0, "<1k", true, "underutilized"},
		{0, "1k-5k", true, "unused"},
		{0, "50k+", true, "unused"},
		// ratio 4 (still below 5)
		{4, "none", true, "underutilized"},
		{4, "<1k", true, "underutilized"},
		{4, "1k-5k", true, "unused"},
		{4, "50k+", true, "unused"},
		// ratio 5 (boundary into underutilized)
		{5, "none", true, "underutilized"},
		{5, "<1k", true, "underutilized"},
		{5, "1k-5k", true, "underutilized"},
		{5, "50k+", true, "underutilized"},
		// ratio 29 (last underutilized)
		{29, "none", true, "underutilized"},
		{29, "1k-5k", true, "underutilized"},
		{29, "50k+", true, "underutilized"},
		// ratio 30 (boundary into moderate)
		{30, "none", true, "moderate"},
		{30, "1k-5k", true, "moderate"},
		{30, "50k+", true, "moderate"},
		// ratio 69 (last moderate)
		{69, "none", true, "moderate"},
		{69, "50k+", true, "moderate"},
		// ratio 70 (boundary into well-utilized)
		{70, "none", true, "well-utilized"},
		{70, "1k-5k", true, "well-utilized"},
		{70, "50k+", true, "well-utilized"},
		// ratio 100
		{100, "none", true, "well-utilized"},
		{100, "50k+", true, "well-utilized"},
		// known=false cases — must return "unknown" regardless of inputs
		{0, "none", false, "unknown"},
		{50, "1k-5k", false, "unknown"},
		{100, "50k+", false, "unknown"},
	}
	for _, tc := range cases {
		got := efficiencyBucket(tc.ratio, tc.tokBkt, tc.known)
		if got != tc.want {
			t.Errorf("efficiencyBucket(%d, %q, %v) = %q, want %q",
				tc.ratio, tc.tokBkt, tc.known, got, tc.want)
		}
		if !allowed[got] {
			t.Errorf("efficiencyBucket(%d, %q, %v) returned non-enum value %q",
				tc.ratio, tc.tokBkt, tc.known, got)
		}
	}
}
