package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
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
	writeLogContent(t, path, sampleJSONL)
	return path
}

func writeMeaningfulLog(t *testing.T, path string) {
	t.Helper()
	var builder strings.Builder
	for builder.Len() < freeAutoMinLogBytes+1024 {
		builder.WriteString(sampleJSONL)
	}
	writeLogContent(t, path, builder.String())
}

func writeLogContent(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write sample log: %v", err)
	}
}

func withLatestShim(t *testing.T, path string) {
	t.Helper()
	original := latestSupportedLogsFn
	latestSupportedLogsFn = func() ([]logCandidate, error) {
		return []logCandidate{fileCandidate("claude_code", "Claude Code", path)}, nil
	}
	t.Cleanup(func() { latestSupportedLogsFn = original })
}

func withRecentShim(t *testing.T, candidates []logCandidate) {
	t.Helper()
	original := recentSupportedLogsFn
	recentSupportedLogsFn = func(limit int) ([]logCandidate, error) {
		if limit > 0 && len(candidates) > limit {
			return candidates[:limit], nil
		}
		return candidates, nil
	}
	t.Cleanup(func() { recentSupportedLogsFn = original })
}

func fileCandidate(sourceID, sourceLabel, path string) logCandidate {
	return logCandidate{
		SourceID:    sourceID,
		SourceLabel: sourceLabel,
		Display:     path,
	}
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

func TestAnalyze_NoArgs_UsesOnePerSupportedSource(t *testing.T) {
	dir := t.TempDir()
	claude := writeSampleLog(t, dir)
	codex := filepath.Join(dir, "codex.jsonl")
	opencode := filepath.Join(dir, "opencode.json")
	for _, path := range []string{codex, opencode} {
		if err := os.WriteFile(path, []byte(sampleJSONL), 0o600); err != nil {
			t.Fatalf("write source log: %v", err)
		}
	}
	outPath := filepath.Join(dir, "report.json")
	original := latestSupportedLogsFn
	latestSupportedLogsFn = func() ([]logCandidate, error) {
		return []logCandidate{
			fileCandidate("claude_code", "Claude Code", claude),
			fileCandidate("codex", "Codex", codex),
			fileCandidate("opencode", "OpenCode", opencode),
		}, nil
	}
	t.Cleanup(func() { latestSupportedLogsFn = original })

	err := runAnalyze([]string{"--out", outPath})
	if err != nil {
		t.Fatalf("runAnalyze: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report analyzer.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("report is not JSON: %v", err)
	}
	if report.AggregateEvent.ParserType != "multi_source" {
		t.Fatalf("expected multi_source parser type, got %#v", report.AggregateEvent)
	}
	if report.Metrics.SessionCount != 3 {
		t.Fatalf("expected one session per source, got %#v", report.Metrics)
	}
	if len(report.SourceReports) != 3 {
		t.Fatalf("expected source reports for three sources, got %#v", report.SourceReports)
	}
	if report.SourceReports[0].SourceID != "claude_code" || report.SourceReports[1].SourceID != "codex" || report.SourceReports[2].SourceID != "opencode" {
		t.Fatalf("expected source reports to preserve discovery order, got %#v", report.SourceReports)
	}
}

func TestRecentSupportedLogs_LimitsPerSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	claudeRoot := filepath.Join(home, ".claude", "projects", "repo")
	codexRoot := filepath.Join(home, ".codex", "sessions", "2026")
	if err := os.MkdirAll(claudeRoot, 0o700); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}
	if err := os.MkdirAll(codexRoot, 0o700); err != nil {
		t.Fatalf("mkdir codex: %v", err)
	}
	paths := []string{
		filepath.Join(claudeRoot, "old.jsonl"),
		filepath.Join(claudeRoot, "new.jsonl"),
		filepath.Join(codexRoot, "old.jsonl"),
		filepath.Join(codexRoot, "new.jsonl"),
	}
	for index, path := range paths {
		writeMeaningfulLog(t, path)
		mtime := time.Unix(int64(100+index), 0)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}

	candidates, err := recentSupportedLogs(1)
	if err != nil {
		t.Fatalf("recentSupportedLogs: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected one candidate per file-backed source, got %#v", candidates)
	}
	if candidates[0].SourceID != "claude_code" || filepath.Base(candidates[0].Display) != "new.jsonl" {
		t.Fatalf("expected newest Claude log first, got %#v", candidates[0])
	}
	if candidates[1].SourceID != "codex" || filepath.Base(candidates[1].Display) != "new.jsonl" {
		t.Fatalf("expected newest Codex log second, got %#v", candidates[1])
	}
}

func TestLatestSupportedLogs_SkipsOversizedFreeAutoLogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	codexRoot := filepath.Join(home, ".codex", "sessions", "2026")
	if err := os.MkdirAll(codexRoot, 0o700); err != nil {
		t.Fatalf("mkdir codex: %v", err)
	}
	small := filepath.Join(codexRoot, "small.jsonl")
	huge := filepath.Join(codexRoot, "huge.jsonl")
	writeMeaningfulLog(t, small)
	writeMeaningfulLog(t, huge)
	if err := os.Truncate(huge, freeAutoMaxLogBytes+1); err != nil {
		t.Fatalf("truncate huge log: %v", err)
	}
	oldTime := time.Unix(100, 0)
	newTime := time.Unix(200, 0)
	if err := os.Chtimes(small, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes small: %v", err)
	}
	if err := os.Chtimes(huge, newTime, newTime); err != nil {
		t.Fatalf("chtimes huge: %v", err)
	}

	candidates, err := latestSupportedLogs()
	if err != nil {
		t.Fatalf("latestSupportedLogs: %v", err)
	}
	if len(candidates) != 1 || filepath.Base(candidates[0].Display) != "small.jsonl" {
		t.Fatalf("expected oversized newest log to be skipped for free auto scan, got %#v", candidates)
	}
}

func TestLatestSupportedLogs_PrefersLargestRecentMeaningfulLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")
	root := filepath.Join(home, ".claude", "projects", "repo")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}
	tinyNewest := filepath.Join(root, "tiny-newest.jsonl")
	smaller := filepath.Join(root, "smaller.jsonl")
	larger := filepath.Join(root, "larger.jsonl")
	writeLogContent(t, tinyNewest, "{}\n")
	writeMeaningfulLog(t, smaller)
	writeMeaningfulLog(t, larger)
	if err := os.Truncate(smaller, freeAutoMinLogBytes+512); err != nil {
		t.Fatalf("truncate smaller: %v", err)
	}
	if err := os.Truncate(larger, freeAutoMinLogBytes+4096); err != nil {
		t.Fatalf("truncate larger: %v", err)
	}
	for path, modTime := range map[string]time.Time{
		smaller:    time.Unix(100, 0),
		larger:     time.Unix(200, 0),
		tinyNewest: time.Unix(300, 0),
	} {
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", path, err)
		}
	}

	candidates, err := latestSupportedLogs()
	if err != nil {
		t.Fatalf("latestSupportedLogs: %v", err)
	}
	if len(candidates) != 1 || filepath.Base(candidates[0].Display) != "larger.jsonl" {
		t.Fatalf("expected largest meaningful recent Claude log, got %#v", candidates)
	}
}

func TestRecentOpenCodeSessions_ReadsMessageDirectoriesAndSkipsTinySessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, ".local", "share", "opencode", "storage", "message")
	partRoot := filepath.Join(home, ".local", "share", "opencode", "storage", "part")
	tiny := filepath.Join(root, "ses_tiny")
	big := filepath.Join(root, "ses_big")
	for _, dir := range []string{tiny, big} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir opencode session: %v", err)
		}
	}
	writeLogContent(t, filepath.Join(tiny, "msg_1.json"), "{}")
	writeLogContent(t, filepath.Join(big, "msg_big.json"), `{"id":"msg_big","sessionID":"ses_big","role":"assistant","text":"`+strings.Repeat("x", freeAutoMinLogBytes+1024)+`"}`)
	if err := os.MkdirAll(filepath.Join(partRoot, "msg_big"), 0o700); err != nil {
		t.Fatalf("mkdir opencode parts: %v", err)
	}
	writeLogContent(t, filepath.Join(partRoot, "msg_big", "part_1.json"), `{"id":"part_1","messageID":"msg_big","type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"npm test"},"output":"ok"}}`)

	candidates, err := recentOpenCodeSessions(10, 0, freeAutoMinLogBytes)
	if err != nil {
		t.Fatalf("recentOpenCodeSessions: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Display != "opencode session ses_big" {
		t.Fatalf("expected only meaningful OpenCode message session, got %#v", candidates)
	}
	data, err := candidates[0].readBytes()
	if err != nil {
		t.Fatalf("read opencode session: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") || !strings.Contains(string(data), `"id":"msg_big"`) {
		t.Fatalf("expected JSONL message output, got %q", string(data[:min(len(data), 80)]))
	}
	if !strings.Contains(string(data), `"type":"tool"`) || !strings.Contains(string(data), `"tool":"bash"`) {
		t.Fatalf("expected OpenCode part JSONL to be joined, got %q", string(data[:min(len(data), 200)]))
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
	withRecentShim(t, []logCandidate{
		fileCandidate("claude_code", "Claude Code", first),
		fileCandidate("codex", "Codex", second),
	})

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
	if len(report.SourceReports) != 2 {
		t.Fatalf("expected per-source paid report sections, got %#v", report.SourceReports)
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

func TestRunOneShot_AnalyzesAndUploadsSanitizedReport(t *testing.T) {
	dir := t.TempDir()
	logPath := writeSampleLog(t, dir)
	outPath := filepath.Join(dir, "agent-analyzer-report.json")
	var received analyzer.Report
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/client-reports" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode uploaded report: %v", err)
		}
		if received.SecurityReceipt.RawLogTTL != "not uploaded" || received.SecurityReceipt.RawTranscriptSentToLLM {
			t.Fatalf("uploaded report violated local-first receipt: %#v", received.SecurityReceipt)
		}
		_ = json.NewEncoder(w).Encode(uploadResult{
			ReportURL: serverURL(r) + "/r/job-token/report-token",
			ExpiresAt: time.Now().Add(15 * time.Minute),
		})
	}))
	defer server.Close()

	err := runOneShot([]string{
		"--log", logPath,
		"--out", outPath,
		"--base-url", server.URL,
		"--yes",
		"--no-open",
	})
	if err != nil {
		t.Fatalf("runOneShot: %v", err)
	}
	if received.Version == "" {
		t.Fatalf("expected uploaded report, got %#v", received)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected local report file at %s: %v", outPath, err)
	}
}

func TestRunFullScan_AnalyzesPaidAggregateAndUploadsWithEntitlementToken(t *testing.T) {
	dir := t.TempDir()
	first := writeSampleLog(t, dir)
	second := filepath.Join(dir, "second.jsonl")
	if err := os.WriteFile(second, []byte(sampleJSONL), 0o600); err != nil {
		t.Fatalf("write second log: %v", err)
	}
	withRecentShim(t, []logCandidate{
		fileCandidate("claude_code", "Claude Code", first),
		fileCandidate("codex", "Codex", second),
	})
	outPath := filepath.Join(dir, "full-scan.json")
	var authHeader string
	var received analyzer.Report
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/full-scan-client-reports" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		authHeader = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode uploaded report: %v", err)
		}
		_ = json.NewEncoder(w).Encode(uploadResult{
			ReportURL: serverURL(r) + "/r/job-token/report-token",
			ExpiresAt: time.Now().Add(15 * time.Minute),
		})
	}))
	defer server.Close()

	err := runFullScan([]string{
		"--token", "email-token",
		"--out", outPath,
		"--base-url", server.URL,
		"--no-open",
	})
	if err != nil {
		t.Fatalf("runFullScan: %v", err)
	}
	if authHeader != "Bearer email-token" {
		t.Fatalf("expected bearer entitlement token, got %q", authHeader)
	}
	if received.AggregateEvent.ParserType != "full_scan_bundle" || received.SecurityReceipt.RawLogTTL != "not uploaded" {
		t.Fatalf("expected sanitized full-scan aggregate upload, got %#v", received)
	}
}

func serverURL(r *http.Request) string {
	return "http://" + r.Host
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
		"agent-analyzer ",
		"commit:",
		"built:",
		"source: https://github.com/Priivacy-ai/agent-log-analyzer",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("version output missing %q:\n%s", want, output)
		}
	}
}

func TestProgressLinesAvoidsCarriageReturnRepaints(t *testing.T) {
	t.Setenv("AGENT_ANALYZER_PROGRESS", "lines")
	output := captureStdout(t, func() {
		progress := newProgressBar(3)
		progress.Update(0, "reading")
		progress.Update(1, "analyzing")
		progress.Update(2, "writing")
		progress.Finish("complete")
	})

	if strings.Contains(output, "\r") {
		t.Fatalf("line progress should not repaint with carriage returns: %q", output)
	}
	for _, want := range []string{"[0/3] reading", "[1/3] analyzing", "[2/3] writing", "[3/3] complete"} {
		if !strings.Contains(output, want) {
			t.Fatalf("line progress missing %q:\n%s", want, output)
		}
	}
}

func TestProgressBarOverrideUsesSingleLineRepaints(t *testing.T) {
	t.Setenv("AGENT_ANALYZER_PROGRESS", "bar")
	output := captureStdout(t, func() {
		progress := newProgressBar(2)
		progress.Update(0, "reading")
		progress.Finish("complete")
	})

	if !strings.Contains(output, "\r") {
		t.Fatalf("bar override should repaint with carriage returns: %q", output)
	}
	if strings.Count(output, "\n") != 1 {
		t.Fatalf("bar progress should only end with one newline: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	original := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = write
	defer func() { os.Stdout = original }()

	fn()

	if err := write.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if _, err := buf.ReadFrom(read); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
