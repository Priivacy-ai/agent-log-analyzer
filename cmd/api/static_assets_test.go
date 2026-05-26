package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStaticAssetPathFromRootUsesHashedSharedCSS(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "styles-abc123.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := staticAssetPathFromRoot(root, "styles.css"); got != "/assets/styles-abc123.css" {
		t.Fatalf("staticAssetPathFromRoot() = %q, want hashed CSS path", got)
	}
}

func TestStaticAssetPathFromRootUsesHashedVendorCSS(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "tippy-def456.css"), []byte(".tippy-box{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := staticAssetPathFromRoot(root, "vendor/tippy/tippy.css"); got != "/assets/tippy-def456.css" {
		t.Fatalf("staticAssetPathFromRoot() = %q, want hashed vendor CSS path", got)
	}
}

func TestStaticAssetPathFromRootUsesHashedPassThroughJS(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app-abc123.js", "report-actions-def456.js", "popper.min-fed321.js"} {
		if err := os.WriteFile(filepath.Join(root, "assets", name), []byte("console.log('ok')"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tests := map[string]string{
		"app.js":                     "/assets/app-abc123.js",
		"report-actions.js":          "/assets/report-actions-def456.js",
		"vendor/tippy/popper.min.js": "/assets/popper.min-fed321.js",
	}
	for asset, want := range tests {
		if got := staticAssetPathFromRoot(root, asset); got != want {
			t.Fatalf("staticAssetPathFromRoot(%q) = %q, want %q", asset, got, want)
		}
	}
}

func TestStaticAssetPathFromRootFallsBackToOriginalPath(t *testing.T) {
	root := t.TempDir()
	if got := staticAssetPathFromRoot(root, "report-actions.js"); got != "/report-actions.js" {
		t.Fatalf("staticAssetPathFromRoot() = %q, want raw asset path", got)
	}
	if got := staticAssetPathFromRoot(root, "styles.css"); got != "/styles.css" {
		t.Fatalf("staticAssetPathFromRoot() = %q, want raw CSS fallback", got)
	}
	if got := staticAssetPathFromRoot(root, "vendor/tippy/tippy.css"); got != "/vendor/tippy/tippy.css" {
		t.Fatalf("staticAssetPathFromRoot() = %q, want raw vendor CSS fallback", got)
	}
}
