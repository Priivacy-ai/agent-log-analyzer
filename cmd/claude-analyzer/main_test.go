package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
)

// sampleJSONL is a minimal Claude Code JSONL log fixture used by the CLI
// argument-resolution tests. The content does not need to exercise the
// analyzer deeply; it only needs to parse cleanly.
const sampleJSONL = `{"type":"user","message":"hello"}
{"type":"assistant","message":"world"}
`

// writeSampleLog drops a small JSONL fixture into the given dir and returns
// the absolute path.
func writeSampleLog(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "sample.jsonl")
	if err := os.WriteFile(path, []byte(sampleJSONL), 0o600); err != nil {
		t.Fatalf("write sample log: %v", err)
	}
	return path
}

// withLatestShim replaces the package-level latestClaudeLogFn with a shim
// that returns the given path, restoring the original on test cleanup.
func withLatestShim(t *testing.T, path string) {
	t.Helper()
	original := latestClaudeLogFn
	latestClaudeLogFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { latestClaudeLogFn = original })
}

func withRecentShim(t *testing.T, paths []string) {
	t.Helper()
	original := recentClaudeLogsFn
	recentClaudeLogsFn = func(limit int) ([]string, error) {
		if limit > 0 && len(paths) > limit {
			return paths[:limit], nil
		}
		return paths, nil
	}
	t.Cleanup(func() { recentClaudeLogsFn = original })
}

func TestAnalyze_NoArgs_UsesLatest(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	outPath := filepath.Join(dir, "report.json")
	withLatestShim(t, logPath)

	err := runAnalyze([]string{"--out", outPath})
	if err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected report at %s: %v", outPath, err)
	}
}

func TestAnalyze_PositionalOnly_UsesPositional(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	outPath := filepath.Join(dir, "report.json")
	// Shim latest to a non-existent path to prove we did NOT fall through to
	// it; if the positional resolution were skipped, runAnalyze would try
	// to read the shim path and fail.
	withLatestShim(t, filepath.Join(dir, "does-not-exist.jsonl"))

	err := runAnalyze([]string{"--out", outPath, logPath})
	if err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected report at %s: %v", outPath, err)
	}
}

func TestAnalyze_PositionalBeforeOutFlag_UsesPositionalAndOut(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	outPath := filepath.Join(dir, "report.json")
	withLatestShim(t, filepath.Join(dir, "does-not-exist.jsonl"))

	err := runAnalyze([]string{logPath, "--out", outPath})
	if err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected report at %s: %v", outPath, err)
	}
}

func TestAnalyze_LogFlagOnly_UsesLogFlag(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	outPath := filepath.Join(dir, "report.json")
	withLatestShim(t, filepath.Join(dir, "does-not-exist.jsonl"))

	err := runAnalyze([]string{"--log", logPath, "--out", outPath})
	if err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected report at %s: %v", outPath, err)
	}
}

func TestAnalyze_PositionalPlusLog_Refuses(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	outPath := filepath.Join(dir, "report.json")

	err := runAnalyze([]string{"--log", logPath, "--out", outPath, logPath})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot combine positional log path with --log") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no report at %s, stat err=%v", outPath, statErr)
	}
}

func TestAnalyze_TwoPositionals_Refuses(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	secondPath := filepath.Join(dir, "second.jsonl")
	if err := os.WriteFile(secondPath, []byte(sampleJSONL), 0o600); err != nil {
		t.Fatalf("write second log: %v", err)
	}
	outPath := filepath.Join(dir, "report.json")

	err := runAnalyze([]string{"--out", outPath, logPath, secondPath})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected extra argument") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected no report at %s, stat err=%v", outPath, statErr)
	}
}

func TestAnalyze_PositionalNonExistent_Refuses(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.jsonl")
	outPath := filepath.Join(dir, "report.json")

	err := runAnalyze([]string{"--out", outPath, missing})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestAnalyzePaid_WritesSanitizedAggregate(t *testing.T) {
	dir := t.TempDir()
	first := writeSampleLog(t, dir)
	second := filepath.Join(dir, "second.jsonl")
	if err := os.WriteFile(second, []byte(sampleJSONL), 0o600); err != nil {
		t.Fatalf("write second log: %v", err)
	}
	outPath := filepath.Join(dir, "paid-report.json")
	withRecentShim(t, []string{first, second})

	err := runAnalyze([]string{"--paid", "--limit", "100", "--out", outPath})
	if err != nil {
		t.Fatalf("runAnalyze paid: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read paid report: %v", err)
	}
	var report analyzer.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("paid report is not JSON: %v", err)
	}
	if report.AggregateEvent.ParserType != "paid_bundle" {
		t.Fatalf("expected paid_bundle parser type, got %#v", report.AggregateEvent)
	}
	if report.Metrics.SessionCount != 2 {
		t.Fatalf("expected two paid sessions, got %#v", report.Metrics)
	}
	if report.SecurityReceipt.RawLogTTL != "not uploaded" || report.SecurityReceipt.RawTranscriptSentToLLM {
		t.Fatalf("expected local-only security receipt, got %#v", report.SecurityReceipt)
	}
}

func TestAnalyzePaid_RejectsUnsafeArguments(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	outPath := filepath.Join(dir, "paid-report.json")

	err := runAnalyze([]string{"--paid", "--out", outPath, logPath})
	if err == nil || !strings.Contains(err.Error(), "--paid cannot be combined") {
		t.Fatalf("expected paid positional rejection, got %v", err)
	}
	err = runAnalyze([]string{"--paid", "--limit", "101", "--out", outPath})
	if err == nil || !strings.Contains(err.Error(), "--limit cannot exceed 100") {
		t.Fatalf("expected paid limit rejection, got %v", err)
	}
}

func TestVersion_PrintsProvenance(t *testing.T) {
	var buf bytes.Buffer
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = original })

	err = run([]string{"version"})
	if err != nil {
		t.Fatalf("run version: %v", err)
	}
	if err := write.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if _, err := buf.ReadFrom(read); err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"claude-analyzer ",
		"commit:",
		"built:",
		"source: https://github.com/robertDouglass/claude-log-analyzer",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("version output missing %q:\n%s", want, output)
		}
	}
}
