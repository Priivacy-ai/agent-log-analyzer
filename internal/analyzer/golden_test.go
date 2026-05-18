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
