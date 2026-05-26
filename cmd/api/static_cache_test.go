package main

import "testing"

func TestStaticCacheControl(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/assets/styles-abc123.css", "public, max-age=31536000, immutable"},
		{"/assets/report-preview-waste-abc123.webp", "public, max-age=31536000, immutable"},
		{"/app.js", "public, max-age=86400, stale-while-revalidate=604800"},
		{"/vendor/tippy/popper.min.js", "public, max-age=86400, stale-while-revalidate=604800"},
		{"/images/report-preview-waste.png", "public, max-age=86400, stale-while-revalidate=604800"},
		{"/", "no-cache"},
		{"/proof/", "no-cache"},
		{"/index.html", "no-cache"},
		{"/docs/benchmarks/repeated-benchmark-suite.md", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := staticCacheControl(tt.path); got != tt.want {
				t.Fatalf("staticCacheControl(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
