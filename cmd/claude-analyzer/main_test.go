package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
