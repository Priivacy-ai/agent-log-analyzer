package sdd

import (
	"strings"
	"testing"
)

func TestParseDetectorsValidMinimal(t *testing.T) {
	data := []byte(`[
        {
            "id": "spec_kitty",
            "display_name": "Spec Kitty",
            "category": "spec_driven_workflow",
            "competitor_priority": 1,
            "status": "verified",
            "source_references": [
                {"kind": "official_repo", "url": "https://example.com"}
            ],
            "markers": [
                {"source_class": "config_dir", "pattern": "\\.kittify/"}
            ],
            "confidence_rules": [
                {"confidence": "high", "requires_any_of": ["config_dir"]}
            ]
        }
    ]`)

	got, err := parseDetectors(data)
	if err != nil {
		t.Fatalf("expected valid parse, got error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 detector, got %d", len(got))
	}
	d := got[0]
	if d.ID != "spec_kitty" {
		t.Errorf("ID: want spec_kitty, got %q", d.ID)
	}
	if d.Status != StatusVerified {
		t.Errorf("Status: want verified, got %q", d.Status)
	}
	if got[0].Markers[0].compiled == nil {
		t.Errorf("expected compiled regex on marker, got nil")
	}
}

func TestParseDetectorsInvalidID(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{"uppercase", "SpecKitty"},
		{"dashes", "spec-kitty"},
		{"too_short", "sk"},
		{"leading_digit", "1spec"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte(`[{
                "id": "` + tc.id + `",
                "display_name": "X",
                "category": "c",
                "competitor_priority": 1,
                "status": "verified",
                "source_references": [{"kind":"docs","url":"https://x"}],
                "markers": [{"source_class":"config_dir","pattern":"x"}],
                "confidence_rules": []
            }]`)
			if _, err := parseDetectors(data); err == nil {
				t.Fatalf("expected error for id=%q, got nil", tc.id)
			} else if !strings.Contains(err.Error(), "invalid id") {
				t.Fatalf("expected invalid-id error, got: %v", err)
			}
		})
	}
}

func TestParseDetectorsInvalidStatus(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "draft",
        "source_references": [],
        "markers": [{"source_class":"config_dir","pattern":"x"}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("expected invalid-status error, got: %v", err)
	}
}

func TestParseDetectorsUnknownSourceClass(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [{"source_class":"telepathy","pattern":"x"}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "invalid source_class") {
		t.Fatalf("expected invalid-source_class error, got: %v", err)
	}
}

func TestParseDetectorsCLIBinaryRequiresBinary(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [{"source_class":"cli_binary"}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "cli_binary requires binary") {
		t.Fatalf("expected cli_binary-requires-binary error, got: %v", err)
	}
}

func TestParseDetectorsNonCLIRequiresPattern(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [{"source_class":"config_dir"}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "requires pattern") {
		t.Fatalf("expected requires-pattern error, got: %v", err)
	}
}

func TestParseDetectorsBadRegex(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [{"source_class":"config_dir","pattern":"["}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "compile pattern") {
		t.Fatalf("expected compile-pattern error, got: %v", err)
	}
}

func TestParseDetectorsVerifiedRequiresSourceRef(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [],
        "markers": [{"source_class":"config_dir","pattern":"x"}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "source_reference") {
		t.Fatalf("expected source_reference error, got: %v", err)
	}
}

func TestParseDetectorsResearchNeededNoSourceRefOK(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "research_needed",
        "source_references": [],
        "markers": [{"source_class":"config_dir","pattern":"x"}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err != nil {
		t.Fatalf("research_needed without source_references should be ok, got: %v", err)
	}
}

func TestParseDetectorsVersionArgsDenied(t *testing.T) {
	for _, flag := range []string{"--config", "--registry", "--token", "--server", "--login"} {
		t.Run(flag, func(t *testing.T) {
			data := []byte(`[{
                "id": "ok_tool",
                "display_name": "X",
                "category": "c",
                "competitor_priority": 1,
                "status": "verified",
                "source_references": [{"kind":"docs","url":"https://x"}],
                "markers": [{"source_class":"cli_version_probe","binary":"foo","version_args":["` + flag + `"]}],
                "confidence_rules": []
            }]`)
			if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "denied flag") {
				t.Fatalf("expected denied-flag error for %q, got: %v", flag, err)
			}
		})
	}
}

func TestParseDetectorsVersionArgsSlashRejected(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [{"source_class":"cli_version_probe","binary":"foo","version_args":["bin/version"]}],
        "confidence_rules": []
    }]`)
	if _, err := parseDetectors(data); err == nil || !strings.Contains(err.Error(), "must not contain '/'") {
		t.Fatalf("expected slash-rejected error, got: %v", err)
	}
}

func TestParseDetectorsVersionArgsDefaults(t *testing.T) {
	data := []byte(`[{
        "id": "ok_tool",
        "display_name": "X",
        "category": "c",
        "competitor_priority": 1,
        "status": "verified",
        "source_references": [{"kind":"docs","url":"https://x"}],
        "markers": [{"source_class":"cli_version_probe","binary":"foo"}],
        "confidence_rules": []
    }]`)
	got, err := parseDetectors(data)
	if err != nil {
		t.Fatalf("expected valid parse, got: %v", err)
	}
	args := got[0].Markers[0].VersionArgs
	if len(args) != 1 || args[0] != "--version" {
		t.Fatalf("expected default version_args=[--version], got %#v", args)
	}
}

func TestLoadRegistryEmptyBase(t *testing.T) {
	// The shipped base file is an empty array; LoadRegistry must succeed
	// and return an empty (non-nil) slice.
	//
	// WP10 note: WP08 introduced internal/analyzer/sdd/structural_test.go
	// in package sdd_test, which imports the parent analyzer package. That
	// import triggers analyzer.init(), which assigns a non-nil
	// ChunksProvider in this package — defeating this test's premise that
	// an unwired registry yields an empty slice. Save and restore the
	// global around the test body so the empty-base contract still holds
	// regardless of whether sibling test compilation units have wired it.
	saved := ChunksProvider
	ChunksProvider = nil
	defer func() { ChunksProvider = saved }()

	got := LoadRegistry()
	if got == nil {
		t.Fatalf("LoadRegistry returned nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty registry, got %d entries", len(got))
	}
	bins := LookupBinaries()
	if len(bins) != 0 {
		t.Fatalf("expected no binaries from empty registry, got %v", bins)
	}
}
