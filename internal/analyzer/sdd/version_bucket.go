package sdd

import "regexp"

// maxBucketLen defensively bounds the cardinality of the version bucket so
// a pathological input cannot widen the fingerprint surface. A MAJOR.MINOR
// pair is always shorter than 16 characters for any sane version scheme.
const maxBucketLen = 16

var (
	// ansiRE strips ANSI/CSI escape sequences (e.g. color codes) that some
	// CLIs include in their --version output when stdout is a TTY.
	ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

	// versionRE captures the first MAJOR.MINOR pair (e.g. "1.2" out of
	// "1.2.3"). Patch versions and build metadata are intentionally dropped
	// to keep the bucket low-cardinality and free of identifying tokens.
	versionRE = regexp.MustCompile(`(\d+\.\d+)`)
)

// NormalizeVersionBucket turns a CLI's raw --version output into a
// low-cardinality, privacy-preserving MAJOR.MINOR bucket per
// research.md §R-04.
//
// Invariants (asserted by version_bucket_test.go):
//
//   - The raw input is NEVER returned verbatim, even when it is itself a
//     valid version string — only the captured MAJOR.MINOR substring is.
//   - Paths, user@host, file references, ANSI escapes, and any tokens
//     outside the first MAJOR.MINOR match are dropped.
//   - The result is bounded to maxBucketLen characters; longer captures
//     collapse to "".
//   - Empty input, no-match input, and over-long input all yield "".
func NormalizeVersionBucket(raw string) string {
	if raw == "" {
		return ""
	}
	cleaned := ansiRE.ReplaceAllString(raw, "")
	m := versionRE.FindStringSubmatch(cleaned)
	if len(m) < 2 {
		return ""
	}
	bucket := m[1]
	if len(bucket) > maxBucketLen {
		return ""
	}
	return bucket
}
