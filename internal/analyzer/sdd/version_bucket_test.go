package sdd

import (
	"strings"
	"testing"
)

func TestNormalizeVersionBucket(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		// forbidden substrings that MUST NOT appear in the result;
		// these guard against the function ever echoing identifying
		// tokens from the raw input.
		forbidden []string
	}{
		{
			name: "simple openspec",
			in:   "openspec 1.2.3",
			want: "1.2",
		},
		{
			name:      "openspec with build path is dropped",
			in:        "openspec 1.2.3 built /private/path",
			want:      "1.2",
			forbidden: []string{"/private/path", "private", "path", "built"},
		},
		{
			name: "v-prefixed version with trailing newline",
			in:   "openspec v0.4.1\n",
			want: "0.4",
		},
		{
			name:      "user@host metadata dropped",
			in:        "openspec 1.2.3 jdoe@host",
			want:      "1.2",
			forbidden: []string{"jdoe", "host", "jdoe@host", "@"},
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "no version digits at all",
			in:   "no version here",
			want: "",
		},
		{
			name:      "ANSI color codes stripped",
			in:        "\x1b[31mopenspec 1.2.3\x1b[0m",
			want:      "1.2",
			forbidden: []string{"\x1b", "[31m", "[0m"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeVersionBucket(tc.in)
			if got != tc.want {
				t.Fatalf("NormalizeVersionBucket(%q) = %q, want %q", tc.in, got, tc.want)
			}
			for _, f := range tc.forbidden {
				if f != "" && strings.Contains(got, f) {
					t.Fatalf("result %q leaked forbidden token %q", got, f)
				}
			}
			// Defensive: the result must never exceed the documented bound.
			if len(got) > maxBucketLen {
				t.Fatalf("result length %d exceeds maxBucketLen %d", len(got), maxBucketLen)
			}
		})
	}
}

func TestNormalizeVersionBucket_PathologicalInput(t *testing.T) {
	// 10 KiB of 'x' — no digits, no version structure. The function MUST
	// return "" rather than echo any prefix of the raw input.
	raw := strings.Repeat("x", 10*1024)
	got := NormalizeVersionBucket(raw)
	if got != "" {
		t.Fatalf("pathological input produced non-empty result %q", got)
	}
	if len(got) > maxBucketLen {
		t.Fatalf("pathological result length %d exceeds maxBucketLen %d", len(got), maxBucketLen)
	}
	if strings.Contains(raw, got) && got != "" {
		t.Fatalf("result %q appears to echo raw input", got)
	}
}

func TestNormalizeVersionBucket_NeverEchoesRawInput(t *testing.T) {
	// Even when the raw input happens to be exactly "1.2.3" — which contains
	// a valid MAJOR.MINOR substring — the function must return only the
	// bucket ("1.2"), never the full raw input.
	raw := "1.2.3"
	got := NormalizeVersionBucket(raw)
	if got == raw {
		t.Fatalf("NormalizeVersionBucket returned raw input %q verbatim", raw)
	}
	if got != "1.2" {
		t.Fatalf("got %q, want %q", got, "1.2")
	}
}
