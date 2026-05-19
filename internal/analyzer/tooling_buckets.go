package analyzer

// Closed-enumeration bucketing helpers for tooling utilization metrics.
//
// NOTE: The existing bucket() helper in analyzer.go uses a different label
// format (e.g. "0_1024", "1024_plus") and is preserved for backward
// compatibility with aggregate-event tests. Do NOT call it from this file —
// the helpers below produce the spec-mandated labels for the
// ToolingUtilization contract (see
// kitty-specs/.../contracts/tooling-utilization.json).

// countBucket maps a non-negative count to one of:
// "none", "1-3", "4-10", "11-25", "26-50", "51-100", "100+", "unknown".
// When known is false, returns "unknown" regardless of n.
func countBucket(n int, known bool) string {
	if !known {
		return "unknown"
	}
	switch {
	case n <= 0:
		return "none"
	case n <= 3:
		return "1-3"
	case n <= 10:
		return "4-10"
	case n <= 25:
		return "11-25"
	case n <= 50:
		return "26-50"
	case n <= 100:
		return "51-100"
	default:
		return "100+"
	}
}

// tokenBucket maps a non-negative token estimate to one of:
// "none", "<1k", "1k-5k", "5k-15k", "15k-50k", "50k+", "unknown".
// When known is false, returns "unknown" regardless of tokens.
func tokenBucket(tokens int, known bool) string {
	if !known {
		return "unknown"
	}
	switch {
	case tokens <= 0:
		return "none"
	case tokens < 1000:
		return "<1k"
	case tokens < 5000:
		return "1k-5k"
	case tokens < 15000:
		return "5k-15k"
	case tokens < 50000:
		return "15k-50k"
	default:
		return "50k+"
	}
}

// efficiencyBucket combines a utilization ratio (0..100) and a context-token
// bucket label into one of:
// "unused", "underutilized", "moderate", "well-utilized", "unknown".
// When known is false, returns "unknown" regardless of inputs.
//
// Rule:
//   - ratioPct < 5 AND tokenBucketLabel not in {"none","<1k"} -> "unused"
//   - ratioPct < 30 -> "underutilized"
//   - ratioPct < 70 -> "moderate"
//   - otherwise     -> "well-utilized"
func efficiencyBucket(ratioPct int, tokenBucketLabel string, known bool) string {
	if !known {
		return "unknown"
	}
	if ratioPct < 5 && tokenBucketLabel != "none" && tokenBucketLabel != "<1k" {
		return "unused"
	}
	if ratioPct < 30 {
		return "underutilized"
	}
	if ratioPct < 70 {
		return "moderate"
	}
	return "well-utilized"
}

// Closed-enumeration string constants referenced by WP03 and WP04.
// Values exactly match the enumerations in
// contracts/tooling-utilization.json.
const (
	WarningBandNormal  = "normal"
	WarningBandWatch   = "watch"
	WarningBandHigh    = "high"
	WarningBandSevere  = "severe"
	WarningBandUnknown = "unknown"

	InferenceSourceHeader = "header"
	InferenceSourceCalls  = "calls"
	InferenceSourceNone   = "none"
)
