package analyzer

import "testing"

// allowedBands is the closed enumeration of legal warning-band values.
// Every classifier return must be a member.
var allowedBands = map[string]bool{
	WarningBandNormal:  true,
	WarningBandWatch:   true,
	WarningBandHigh:    true,
	WarningBandSevere:  true,
	WarningBandUnknown: true,
}

func TestCountAtLeastRankHelper(t *testing.T) {
	cases := []struct {
		bucket, threshold string
		want              bool
	}{
		{"11-25", "11-25", true},
		{"26-50", "11-25", true},
		{"100+", "1-3", true},
		{"4-10", "11-25", false},
		{"none", "1-3", false},
		{"unknown", "1-3", false},
		{"unknown", "none", false},
		{"bogus", "1-3", false},
		{"11-25", "bogus", false},
	}
	for _, c := range cases {
		if got := countAtLeast(c.bucket, c.threshold); got != c.want {
			t.Errorf("countAtLeast(%q,%q)=%v want %v", c.bucket, c.threshold, got, c.want)
		}
	}
}

func TestTokenAtLeastRankHelper(t *testing.T) {
	cases := []struct {
		bucket, threshold string
		want              bool
	}{
		{"5k-15k", "5k-15k", true},
		{"50k+", "5k-15k", true},
		{"1k-5k", "5k-15k", false},
		{"<1k", "5k-15k", false},
		{"none", "<1k", false},
		{"unknown", "<1k", false},
		{"bogus", "<1k", false},
	}
	for _, c := range cases {
		if got := tokenAtLeast(c.bucket, c.threshold); got != c.want {
			t.Errorf("tokenAtLeast(%q,%q)=%v want %v", c.bucket, c.threshold, got, c.want)
		}
	}
}

// TestEfficiencyBucketClassifierInputs locks the §efficiencyBucket spec
// from data-model.md under realistic classifier inputs (T011).
func TestEfficiencyBucketClassifierInputs(t *testing.T) {
	cases := []struct {
		name        string
		ratio       int
		tokenBucket string
		known       bool
		want        string
	}{
		{"unused: low ratio + meaningful footprint", 2, "5k-15k", true, "unused"},
		{"underutilized: low ratio but tiny footprint <1k", 2, "<1k", true, "underutilized"},
		{"underutilized: low ratio but no tokens", 2, "none", true, "underutilized"},
		{"moderate mid-band", 50, "1k-5k", true, "moderate"},
		{"well-utilized high ratio", 85, "15k-50k", true, "well-utilized"},
		{"well-utilized at threshold 70", 70, "5k-15k", true, "well-utilized"},
		{"underutilized at upper edge 29", 29, "5k-15k", true, "underutilized"},
		{"moderate at lower edge 30", 30, "1k-5k", true, "moderate"},
		{"unknown when not known regardless of inputs", 50, "5k-15k", false, "unknown"},
		{"unknown when not known with zero ratio", 0, "none", false, "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := efficiencyBucket(c.ratio, c.tokenBucket, c.known); got != c.want {
				t.Errorf("efficiencyBucket(%d,%q,%v)=%q want %q", c.ratio, c.tokenBucket, c.known, got, c.want)
			}
		})
	}
}

func TestClassifyMCPBand(t *testing.T) {
	cases := []struct {
		name string
		in   mcpBandInput
		want string
	}{
		// --- exposure unknown ---
		{
			name: "exposure unknown → unknown (small count)",
			in:   mcpBandInput{ServerCountBucket: "1-3", ContextTokenBucket: "<1k", UtilizationRatioPct: 50, ExposureKnown: false},
			want: WarningBandUnknown,
		},
		{
			name: "exposure unknown → unknown (huge count, high util)",
			in:   mcpBandInput{ServerCountBucket: "100+", ContextTokenBucket: "50k+", UtilizationRatioPct: 90, ExposureKnown: false, Rereads: 10, RetryDepthMax: 10, ContextGrowthEvents: 10},
			want: WarningBandUnknown,
		},
		{
			name: "exposure unknown with unknown bucket → unknown",
			in:   mcpBandInput{ServerCountBucket: "unknown", ContextTokenBucket: "unknown", ExposureKnown: false},
			want: WarningBandUnknown,
		},

		// --- small count → normal ---
		{
			name: "small count 1-3 low util → normal",
			in:   mcpBandInput{ServerCountBucket: "1-3", ExposedToolCountBucket: "1-3", ContextTokenBucket: "<1k", UtilizationRatioPct: 0, ExposureKnown: true},
			want: WarningBandNormal,
		},
		{
			name: "small count 4-10 low util → normal",
			in:   mcpBandInput{ServerCountBucket: "4-10", ExposedToolCountBucket: "4-10", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 0, ExposureKnown: true},
			want: WarningBandNormal,
		},
		{
			name: "none servers → normal",
			in:   mcpBandInput{ServerCountBucket: "none", ContextTokenBucket: "none", UtilizationRatioPct: 0, ExposureKnown: true},
			want: WarningBandNormal,
		},

		// --- SC-5: large count overridden by high utilization ---
		{
			name: "SC-5: 100+ servers but 40% util → normal",
			in:   mcpBandInput{ServerCountBucket: "100+", ExposedToolCountBucket: "100+", ContextTokenBucket: "50k+", UtilizationRatioPct: 40, ExposureKnown: true, Rereads: 10, RetryDepthMax: 10, ContextGrowthEvents: 10},
			want: WarningBandNormal,
		},
		{
			name: "SC-5: 51-100 servers at 60% util → normal",
			in:   mcpBandInput{ServerCountBucket: "51-100", ExposedToolCountBucket: "51-100", ContextTokenBucket: "15k-50k", UtilizationRatioPct: 60, ExposureKnown: true},
			want: WarningBandNormal,
		},

		// --- watch ---
		{
			name: "watch: 11-25 servers, low util, no degradation",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "11-25", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 25, ExposureKnown: true},
			want: WarningBandWatch,
		},
		{
			name: "watch: 26-50 servers low util no degradation",
			in:   mcpBandInput{ServerCountBucket: "26-50", ExposedToolCountBucket: "11-25", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 25, ExposureKnown: true},
			want: WarningBandWatch,
		},
		{
			name: "watch boundary: util=39 (just below normal cutoff) and moderate count",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "4-10", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 39, ExposureKnown: true},
			want: WarningBandWatch,
		},

		// --- high (low util, big footprint, no degradation) ---
		{
			name: "high: 11-25 servers, util<20, token≥5k-15k, no degradation",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 10, ExposureKnown: true},
			want: WarningBandHigh,
		},
		{
			name: "high: 11-25 servers, util<20, exposed-tools≥26-50, low tokens",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "26-50", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 5, ExposureKnown: true},
			want: WarningBandHigh,
		},
		{
			name: "high: 100+ servers (huge), util<20, 50k+ tokens",
			in:   mcpBandInput{ServerCountBucket: "100+", ExposedToolCountBucket: "100+", ContextTokenBucket: "50k+", UtilizationRatioPct: 10, ExposureKnown: true},
			want: WarningBandHigh,
		},

		// --- severe (high + degradation) ---
		{
			name: "severe: same as high with rereads≥3",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 10, ExposureKnown: true, Rereads: 3},
			want: WarningBandSevere,
		},
		{
			name: "severe: same as high with retry-depth≥3",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "26-50", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 5, ExposureKnown: true, RetryDepthMax: 4},
			want: WarningBandSevere,
		},
		{
			name: "severe: same as high with context-growth≥2",
			in:   mcpBandInput{ServerCountBucket: "26-50", ExposedToolCountBucket: "26-50", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 10, ExposureKnown: true, ContextGrowthEvents: 2},
			want: WarningBandSevere,
		},

		// --- boundary cases ---
		{
			name: "boundary: ServerCountBucket=4-10 stays normal even with degradation+low util",
			in:   mcpBandInput{ServerCountBucket: "4-10", ExposedToolCountBucket: "26-50", ContextTokenBucket: "50k+", UtilizationRatioPct: 0, ExposureKnown: true, Rereads: 10},
			want: WarningBandNormal,
		},
		{
			name: "boundary: ServerCountBucket=11-25 with util=20 → not severe gate (util≥20 fails)",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 20, ExposureKnown: true},
			want: WarningBandWatch,
		},
		{
			name: "boundary: util=40 exactly → normal",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "26-50", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 40, ExposureKnown: true, Rereads: 10},
			want: WarningBandNormal,
		},
		{
			name: "boundary: util=19 with high footprint → high",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 19, ExposureKnown: true},
			want: WarningBandHigh,
		},
		{
			name: "low footprint, moderate count, low util → watch (not high)",
			in:   mcpBandInput{ServerCountBucket: "11-25", ExposedToolCountBucket: "4-10", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 10, ExposureKnown: true},
			want: WarningBandWatch,
		},
		{
			name: "huge count >26-50 with low util no degradation → not watch (capped)",
			in:   mcpBandInput{ServerCountBucket: "51-100", ExposedToolCountBucket: "11-25", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 25, ExposureKnown: true},
			want: WarningBandNormal,
		},
		{
			name: "unknown server bucket but ExposureKnown=true defaults to normal",
			in:   mcpBandInput{ServerCountBucket: "unknown", ExposedToolCountBucket: "unknown", ContextTokenBucket: "unknown", UtilizationRatioPct: 0, ExposureKnown: true},
			want: WarningBandNormal,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyMCPBand(c.in)
			if !allowedBands[got] {
				t.Fatalf("classifyMCPBand returned %q not in allowed enum", got)
			}
			if got != c.want {
				t.Errorf("classifyMCPBand=%q want %q", got, c.want)
			}
		})
	}
}

func TestClassifySkillBand(t *testing.T) {
	cases := []struct {
		name string
		in   skillBandInput
		want string
	}{
		// --- exposure unknown ---
		{
			name: "exposure unknown → unknown",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 0, ExposureKnown: false},
			want: WarningBandUnknown,
		},
		{
			name: "exposure unknown + max degradation → unknown",
			in:   skillBandInput{ExposedCountBucket: "100+", ContextTokenBucket: "50k+", UtilizationRatioPct: 0, ExposureKnown: false, Rereads: 10, RetryDepthMax: 10, ContextGrowthEvents: 10},
			want: WarningBandUnknown,
		},

		// --- normal ---
		{
			name: "small exposed count 1-3 low util → normal",
			in:   skillBandInput{ExposedCountBucket: "1-3", ContextTokenBucket: "<1k", UtilizationRatioPct: 0, ExposureKnown: true},
			want: WarningBandNormal,
		},
		{
			name: "small exposed count 4-10 low util → normal",
			in:   skillBandInput{ExposedCountBucket: "4-10", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 0, ExposureKnown: true},
			want: WarningBandNormal,
		},
		{
			name: "SC-5: huge exposed count at 30% util → normal",
			in:   skillBandInput{ExposedCountBucket: "100+", ContextTokenBucket: "50k+", UtilizationRatioPct: 30, ExposureKnown: true, Rereads: 10, RetryDepthMax: 10, ContextGrowthEvents: 10},
			want: WarningBandNormal,
		},
		{
			name: "SC-5: 11-25 count at 50% util → normal",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "15k-50k", UtilizationRatioPct: 50, ExposureKnown: true},
			want: WarningBandNormal,
		},

		// --- watch ---
		{
			name: "watch: 11-25 count, util<30, no degradation, mid token",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 20, ExposureKnown: true},
			want: WarningBandWatch,
		},
		{
			name: "watch: 26-50 count, util just under 30, no degradation",
			in:   skillBandInput{ExposedCountBucket: "26-50", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 29, ExposureKnown: true},
			want: WarningBandWatch,
		},
		{
			name: "watch boundary: util=14 but low footprint→watch (not high)",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 14, ExposureKnown: true},
			want: WarningBandWatch,
		},

		// --- high ---
		{
			name: "high: 11-25 count, util<15, token≥5k-15k, no degradation",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 10, ExposureKnown: true},
			want: WarningBandHigh,
		},
		{
			name: "high: 51-100 count, util<15, 50k+ tokens, no degradation",
			in:   skillBandInput{ExposedCountBucket: "51-100", ContextTokenBucket: "50k+", UtilizationRatioPct: 5, ExposureKnown: true},
			want: WarningBandHigh,
		},

		// --- severe ---
		{
			name: "severe: high gate + rereads≥3",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 10, ExposureKnown: true, Rereads: 3},
			want: WarningBandSevere,
		},
		{
			name: "severe: high gate + retry-depth≥3",
			in:   skillBandInput{ExposedCountBucket: "26-50", ContextTokenBucket: "15k-50k", UtilizationRatioPct: 5, ExposureKnown: true, RetryDepthMax: 5},
			want: WarningBandSevere,
		},
		{
			name: "severe: high gate + context-growth≥2",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "50k+", UtilizationRatioPct: 0, ExposureKnown: true, ContextGrowthEvents: 2},
			want: WarningBandSevere,
		},

		// --- boundaries ---
		{
			name: "boundary: ExposedCountBucket=4-10 stays normal regardless",
			in:   skillBandInput{ExposedCountBucket: "4-10", ContextTokenBucket: "50k+", UtilizationRatioPct: 0, ExposureKnown: true, Rereads: 10},
			want: WarningBandNormal,
		},
		{
			name: "boundary: util=30 exactly → normal",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 30, ExposureKnown: true, Rereads: 10},
			want: WarningBandNormal,
		},
		{
			name: "boundary: util=15 with high footprint → watch (not high)",
			in:   skillBandInput{ExposedCountBucket: "11-25", ContextTokenBucket: "5k-15k", UtilizationRatioPct: 15, ExposureKnown: true},
			want: WarningBandWatch,
		},
		{
			name: "huge exposed count > 26-50 with no degradation, low util → not watch (capped)",
			in:   skillBandInput{ExposedCountBucket: "51-100", ContextTokenBucket: "1k-5k", UtilizationRatioPct: 20, ExposureKnown: true},
			want: WarningBandNormal,
		},
		{
			name: "unknown exposed bucket but ExposureKnown=true → normal",
			in:   skillBandInput{ExposedCountBucket: "unknown", ContextTokenBucket: "unknown", UtilizationRatioPct: 0, ExposureKnown: true},
			want: WarningBandNormal,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifySkillBand(c.in)
			if !allowedBands[got] {
				t.Fatalf("classifySkillBand returned %q not in allowed enum", got)
			}
			if got != c.want {
				t.Errorf("classifySkillBand=%q want %q", got, c.want)
			}
		})
	}
}

// TestClassifyInvariantCountAloneNeverWarns proves SC-5 (I-2): when
// utilization is at or above the normal cutoff, no combination of
// count, token bucket, or degradation can push the band above normal.
func TestClassifyInvariantCountAloneNeverWarns(t *testing.T) {
	counts := []string{"none", "1-3", "4-10", "11-25", "26-50", "51-100", "100+"}
	tokens := []string{"none", "<1k", "1k-5k", "5k-15k", "15k-50k", "50k+"}

	t.Run("MCP at util=40 (cutoff) with max degradation", func(t *testing.T) {
		for _, c := range counts {
			for _, tk := range tokens {
				got := classifyMCPBand(mcpBandInput{
					ServerCountBucket:      c,
					ExposedToolCountBucket: c,
					ContextTokenBucket:     tk,
					UtilizationRatioPct:    40,
					ExposureKnown:          true,
					Rereads:                10,
					RetryDepthMax:          10,
					ContextGrowthEvents:    10,
				})
				if got != WarningBandNormal {
					t.Errorf("MCP util=40 count=%s token=%s: got %q want normal", c, tk, got)
				}
			}
		}
	})

	t.Run("MCP at util=50 with max degradation", func(t *testing.T) {
		for _, c := range counts {
			for _, tk := range tokens {
				got := classifyMCPBand(mcpBandInput{
					ServerCountBucket:      c,
					ExposedToolCountBucket: c,
					ContextTokenBucket:     tk,
					UtilizationRatioPct:    50,
					ExposureKnown:          true,
					Rereads:                10,
					RetryDepthMax:          10,
					ContextGrowthEvents:    10,
				})
				if got != WarningBandNormal {
					t.Errorf("MCP util=50 count=%s token=%s: got %q want normal", c, tk, got)
				}
			}
		}
	})

	t.Run("Skill at util=30 (cutoff) with max degradation", func(t *testing.T) {
		for _, c := range counts {
			for _, tk := range tokens {
				got := classifySkillBand(skillBandInput{
					ExposedCountBucket:  c,
					ContextTokenBucket:  tk,
					UtilizationRatioPct: 30,
					ExposureKnown:       true,
					Rereads:             10,
					RetryDepthMax:       10,
					ContextGrowthEvents: 10,
				})
				if got != WarningBandNormal {
					t.Errorf("Skill util=30 count=%s token=%s: got %q want normal", c, tk, got)
				}
			}
		}
	})

	t.Run("Skill at util=50 with max degradation", func(t *testing.T) {
		for _, c := range counts {
			for _, tk := range tokens {
				got := classifySkillBand(skillBandInput{
					ExposedCountBucket:  c,
					ContextTokenBucket:  tk,
					UtilizationRatioPct: 50,
					ExposureKnown:       true,
					Rereads:             10,
					RetryDepthMax:       10,
					ContextGrowthEvents: 10,
				})
				if got != WarningBandNormal {
					t.Errorf("Skill util=50 count=%s token=%s: got %q want normal", c, tk, got)
				}
			}
		}
	})
}

// TestClassifyInvariantExposureUnknownAlwaysUnknown proves I-1: when
// exposure is unknown, no input can produce anything other than the
// unknown band.
func TestClassifyInvariantExposureUnknownAlwaysUnknown(t *testing.T) {
	ratios := []int{0, 14, 15, 19, 20, 29, 30, 39, 40, 50, 100}
	counts := []string{"unknown", "none", "1-3", "4-10", "11-25", "26-50", "51-100", "100+"}
	tokens := []string{"unknown", "none", "<1k", "1k-5k", "5k-15k", "15k-50k", "50k+"}

	t.Run("MCP", func(t *testing.T) {
		for _, r := range ratios {
			for _, c := range counts {
				for _, tk := range tokens {
					got := classifyMCPBand(mcpBandInput{
						ServerCountBucket:      c,
						ExposedToolCountBucket: c,
						ContextTokenBucket:     tk,
						UtilizationRatioPct:    r,
						ExposureKnown:          false,
						Rereads:                5,
						RetryDepthMax:          5,
						ContextGrowthEvents:    5,
					})
					if got != WarningBandUnknown {
						t.Errorf("MCP exposure_known=false count=%s token=%s util=%d: got %q want unknown", c, tk, r, got)
					}
				}
			}
		}
	})

	t.Run("Skill", func(t *testing.T) {
		for _, r := range ratios {
			for _, c := range counts {
				for _, tk := range tokens {
					got := classifySkillBand(skillBandInput{
						ExposedCountBucket:  c,
						ContextTokenBucket:  tk,
						UtilizationRatioPct: r,
						ExposureKnown:       false,
						Rereads:             5,
						RetryDepthMax:       5,
						ContextGrowthEvents: 5,
					})
					if got != WarningBandUnknown {
						t.Errorf("Skill exposure_known=false count=%s token=%s util=%d: got %q want unknown", c, tk, r, got)
					}
				}
			}
		}
	})
}
