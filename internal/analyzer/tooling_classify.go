package analyzer

// Deterministic warning-band classifier for MCP and skill tooling
// utilization. Implements plan.md §D-4 verbatim. Pure functions: no I/O,
// no globals, no time, no randomness. Inputs are the closed-enum bucket
// labels produced by tooling_buckets.go plus the integer utilization
// ratio (0..100) and the three degradation signals from Metrics.
//
// Invariants enforced (see data-model.md §Invariants and tests in
// tooling_classify_test.go):
//   I-1 ExposureKnown=false → "unknown" regardless of any other input.
//   I-2 UtilizationRatioPct >= normal-cutoff → "normal" regardless of
//       count or degradation (count alone never warns; SC-5).
//   I-3 Returned band is always one of {normal, watch, high, severe,
//       unknown}.
//
// Plan §D-4 — MCP table:
//   exposure_known=false                                             → unknown
//   server_count_bucket ≤ 4-10  OR  utilization_ratio_pct ≥ 40       → normal
//   server_count_bucket ≥ 11-25 AND utilization_ratio_pct < 20
//       AND (context_token_bucket ≥ 5k-15k
//            OR exposed_tool_count_bucket ≥ 26-50)
//       AND degradation                                              → severe
//   same as above, without degradation                               → high
//   server_count_bucket ≥ 11-25 AND utilization_ratio_pct < 40
//       AND NOT degradation
//       AND server_count_bucket ≤ 26-50                              → watch
//   default                                                          → normal
//
// Plan §D-4 — Skill table:
//   exposure_known=false                                             → unknown
//   exposed_count_bucket ≤ 4-10  OR  utilization_ratio_pct ≥ 30      → normal
//   exposed_count_bucket ≥ 11-25 AND utilization_ratio_pct < 15
//       AND context_token_bucket ≥ 5k-15k
//       AND degradation                                              → severe
//   same as above, without degradation                               → high
//   exposed_count_bucket ≥ 11-25 AND utilization_ratio_pct < 30
//       AND NOT degradation
//       AND exposed_count_bucket ≤ 26-50                             → watch
//   default                                                          → normal
//
// degradation := Rereads ≥ 3 OR RetryDepthMax ≥ 3 OR ContextGrowthEvents ≥ 2.

// mcpBandInput is the closed input shape for classifyMCPBand. All
// bucket fields use the labels emitted by countBucket/tokenBucket in
// tooling_buckets.go (including "unknown" for indeterminate buckets).
type mcpBandInput struct {
	ServerCountBucket      string
	ExposedToolCountBucket string
	ContextTokenBucket     string
	UtilizationRatioPct    int
	ExposureKnown          bool
	Rereads                int
	RetryDepthMax          int
	ContextGrowthEvents    int
}

// skillBandInput is the closed input shape for classifySkillBand.
type skillBandInput struct {
	ExposedCountBucket  string
	ContextTokenBucket  string
	UtilizationRatioPct int
	ExposureKnown       bool
	Rereads             int
	RetryDepthMax       int
	ContextGrowthEvents int
}

// countBucketRanks maps each count-bucket label to its ordinal rank.
// "unknown" intentionally has no entry — *AtLeast helpers return false
// for unknown, treating it as "below all thresholds".
var countBucketRanks = map[string]int{
	"none":   0,
	"1-3":    1,
	"4-10":   2,
	"11-25":  3,
	"26-50":  4,
	"51-100": 5,
	"100+":   6,
}

// tokenBucketRanks maps each token-bucket label to its ordinal rank.
var tokenBucketRanks = map[string]int{
	"none":    0,
	"<1k":     1,
	"1k-5k":   2,
	"5k-15k":  3,
	"15k-50k": 4,
	"50k+":    5,
}

// countAtLeast reports whether bucket is at or above threshold in the
// count-bucket ordering. Returns false for "unknown" or any unrecognized
// label. Returns false if threshold itself is unknown (defensive).
func countAtLeast(bucket, threshold string) bool {
	if bucket == "unknown" {
		return false
	}
	br, ok := countBucketRanks[bucket]
	if !ok {
		return false
	}
	tr, ok := countBucketRanks[threshold]
	if !ok {
		return false
	}
	return br >= tr
}

// tokenAtLeast reports whether bucket is at or above threshold in the
// token-bucket ordering. Returns false for "unknown" or any unrecognized
// label.
func tokenAtLeast(bucket, threshold string) bool {
	if bucket == "unknown" {
		return false
	}
	br, ok := tokenBucketRanks[bucket]
	if !ok {
		return false
	}
	tr, ok := tokenBucketRanks[threshold]
	if !ok {
		return false
	}
	return br >= tr
}

// countRankLE reports whether bucket's rank is less than or equal to
// threshold's rank. Unknown buckets return false (they are not "low").
func countRankLE(bucket, threshold string) bool {
	if bucket == "unknown" {
		return false
	}
	br, ok := countBucketRanks[bucket]
	if !ok {
		return false
	}
	tr, ok := countBucketRanks[threshold]
	if !ok {
		return false
	}
	return br <= tr
}

// classifyMCPBand returns the warning band for an MCP utilization input
// per plan §D-4. The returned value is always one of WarningBand*.
func classifyMCPBand(in mcpBandInput) string {
	// I-1: exposure unknown dominates everything.
	if !in.ExposureKnown {
		return WarningBandUnknown
	}

	degradation := in.Rereads >= 3 || in.RetryDepthMax >= 3 || in.ContextGrowthEvents >= 2

	// Normal precondition (I-2 / SC-5): low server count OR meaningful
	// utilization wins over every other signal.
	if countRankLE(in.ServerCountBucket, "4-10") || in.UtilizationRatioPct >= 40 {
		return WarningBandNormal
	}

	// Common gate for severe/high: large enough exposure AND very low
	// utilization AND high footprint (either tokens or exposed-tool count).
	severeGate := countAtLeast(in.ServerCountBucket, "11-25") &&
		in.UtilizationRatioPct < 20 &&
		(tokenAtLeast(in.ContextTokenBucket, "5k-15k") ||
			countAtLeast(in.ExposedToolCountBucket, "26-50"))

	if severeGate && degradation {
		return WarningBandSevere
	}
	if severeGate {
		return WarningBandHigh
	}

	// Watch: moderate-not-huge exposure with low utilization and no
	// degradation. Capped at 26-50 to keep "huge" from sliding into watch.
	if countAtLeast(in.ServerCountBucket, "11-25") &&
		in.UtilizationRatioPct < 40 &&
		!degradation &&
		countRankLE(in.ServerCountBucket, "26-50") {
		return WarningBandWatch
	}

	return WarningBandNormal
}

// classifySkillBand returns the warning band for a skill utilization
// input per plan §D-4. The returned value is always one of WarningBand*.
func classifySkillBand(in skillBandInput) string {
	// I-1.
	if !in.ExposureKnown {
		return WarningBandUnknown
	}

	degradation := in.Rereads >= 3 || in.RetryDepthMax >= 3 || in.ContextGrowthEvents >= 2

	// Normal precondition (I-2 / SC-5): skill cutoff is 30%.
	if countRankLE(in.ExposedCountBucket, "4-10") || in.UtilizationRatioPct >= 30 {
		return WarningBandNormal
	}

	// Common gate for severe/high (skill): only the token bucket counts
	// as a footprint signal — there is no separate "exposed-tool" axis.
	severeGate := countAtLeast(in.ExposedCountBucket, "11-25") &&
		in.UtilizationRatioPct < 15 &&
		tokenAtLeast(in.ContextTokenBucket, "5k-15k")

	if severeGate && degradation {
		return WarningBandSevere
	}
	if severeGate {
		return WarningBandHigh
	}

	if countAtLeast(in.ExposedCountBucket, "11-25") &&
		in.UtilizationRatioPct < 30 &&
		!degradation &&
		countRankLE(in.ExposedCountBucket, "26-50") {
		return WarningBandWatch
	}

	return WarningBandNormal
}
