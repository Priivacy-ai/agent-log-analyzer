package analyzer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenSampleReport(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "sample-claude.jsonl")
	golden := filepath.Join("..", "..", "testdata", "golden", "sample-report.json")
	input, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	report, err := Analyze("job-golden-sample", input)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Validate the bounded shape of any emitted fingerprints (NFR-003)
	// before normalization — if any entry is malformed, the test should
	// fail loudly rather than silently passing after the slice is wiped.
	// The seven-field cap on EcosystemFingerprint is enforced structurally
	// by internal/analyzer/sdd/structural_test.go.
	for i, fp := range report.Ecosystem.WorkflowFingerprints {
		if fp.ID == "" {
			t.Errorf("fingerprint %d: empty ID", i)
		}
		if fp.Confidence == "" {
			t.Errorf("fingerprint %d (%q): empty Confidence", i, fp.ID)
		}
		if len(fp.Sources) == 0 {
			t.Errorf("fingerprint %d (%q): empty Sources", i, fp.ID)
		}
	}

	// Normalize away the entire WorkflowFingerprints slice before golden
	// comparison. The SDD evaluator's emission is gated on which CLI
	// binaries the host happens to have installed (sdd.NewRealProbe walks
	// $PATH for spec-kitty / openspec / etc.), so neither the slice
	// contents nor its presence is reproducible across environments —
	// the same fixture yields zero fingerprints inside a clean container
	// and two fingerprints on a developer laptop with spec-kitty installed.
	// Per-field scrubbing isn't enough: Sources and EvidenceCount also
	// depend on which markers fired, which is environment-dependent here.
	// Setting the slice to nil makes the json `omitempty` tag drop the
	// field entirely, yielding a deterministic golden artifact. The SDD
	// evaluator's behavior is covered exhaustively by unit tests in
	// internal/analyzer/sdd/.
	//
	// Both Ecosystem.WorkflowFingerprints (the report-level copy) and
	// AggregateEvent.Ecosystem.WorkflowFingerprints (the aggregate copy)
	// must be cleared; the aggregator deep-copies from the same source.
	report.Ecosystem.WorkflowFingerprints = nil
	report.AggregateEvent.Ecosystem.WorkflowFingerprints = nil

	actual, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	actual = append(actual, '\n')

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(golden, actual, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}

	expected, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(expected, actual) {
		t.Fatalf("golden report mismatch; run UPDATE_GOLDEN=1 go test ./internal/analyzer -run TestGoldenSampleReport")
	}
}

func TestAnalyzeRejectsEmptyUpload(t *testing.T) {
	if _, err := Analyze("job-empty", []byte(" \n\t ")); err == nil {
		t.Fatal("expected empty upload error")
	}
}

func TestAnalyzeHandlesMalformedJSONLAsText(t *testing.T) {
	report, err := Analyze("job-malformed", []byte("{not-json\ncommand: cat src/auth.ts\ncommand: cat src/auth.ts"))
	if err != nil {
		t.Fatalf("Analyze should fall back to text parsing: %v", err)
	}
	if report.Metrics.Turns != 3 {
		t.Fatalf("unexpected turn count: %#v", report.Metrics)
	}
	if report.Metrics.Rereads == 0 {
		t.Fatalf("expected repeated file read detection: %#v", report.Metrics)
	}
}
