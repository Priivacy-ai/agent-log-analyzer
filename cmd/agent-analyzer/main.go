package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
)

const defaultBaseURL = "https://analyzer.spec-kitty.ai"
const freeAutoMaxLogBytes = 2 * 1024 * 1024
const freeAutoMinLogBytes = 4 * 1024
const freeAutoCandidatePool = 25

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("missing command")
	}
	switch args[0] {
	case "run":
		return runOneShot(args[1:])
	case "analyze":
		return runAnalyze(args[1:])
	case "full-scan":
		return runFullScan(args[1:])
	case "upload":
		return runUpload(args[1:])
	case "version", "--version", "-v":
		printVersion()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// latestSupportedLogsFn and recentSupportedLogsFn are package-level
// indirections so tests can shim source discovery without touching the user's
// real home directory or installed agent CLIs.
var latestSupportedLogsFn = latestSupportedLogs
var recentSupportedLogsFn = recentSupportedLogs

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	out := fs.String("out", "agent-analyzer-report.json", "path to write sanitized report JSON")
	logPath := fs.String("log", "", "explicit supported JSON/JSONL log path")
	paid := fs.Bool("paid", false, "analyze recent supported agent logs locally and write a sanitized paid aggregate report")
	limit := fs.Int("limit", 100, "maximum recent logs per supported source to analyze with --paid")
	orderedArgs := reorderAnalyzeArgs(args)
	if err := fs.Parse(orderedArgs); err != nil {
		return err
	}

	positional := fs.Args()
	if *paid {
		if *logPath != "" || len(positional) > 0 {
			return errors.New("agent-analyzer analyze: --paid cannot be combined with --log or positional log paths")
		}
		return runAnalyzePaid(*out, *limit)
	}
	// FR-002 takes precedence over FR-003 when both a positional and --log
	// are supplied alongside extra positional arguments.
	if len(positional) >= 1 && *logPath != "" {
		return errors.New("agent-analyzer analyze: cannot combine positional log path with --log flag")
	}
	if len(positional) >= 2 {
		return fmt.Errorf("agent-analyzer analyze: unexpected extra argument %q", positional[1])
	}

	path := *logPath
	if path == "" && len(positional) == 1 {
		path = positional[0]
	}
	if path == "" {
		candidates, err := latestSupportedLogsFn()
		if err != nil {
			return err
		}
		return analyzeDiscovered(candidates, *out, "", true)
	}
	return analyzeSingle(path, *out, true)
}

func analyzeSingle(path, out string, printNextSteps bool) error {
	progress := newProgressBar(3)
	progress.Update(0, "reading "+shortDisplay(path))
	data, err := os.ReadFile(path)
	if err != nil {
		progress.Fail()
		return err
	}
	progress.Update(1, "analyzing "+shortDisplay(path))
	report, err := analyzeBytesForSource("local", "unknown", data)
	if err != nil {
		progress.Fail()
		return err
	}
	progress.Update(2, "writing sanitized report")
	if err := writeReport(out, report); err != nil {
		progress.Fail()
		return err
	}
	progress.Finish("complete")

	fmt.Printf("Analyzed locally: %s\n", path)
	fmt.Printf("Raw bytes read locally: %d\n", len(data))
	fmt.Printf("Secrets redacted before report write: %d\n", report.SecurityReceipt.SecretsRedacted)
	printReportWrite(out, report)
	if printNextSteps {
		printNextStepsFor(out)
	}
	return nil
}

func analyzeBytesForSource(jobID string, sourceID string, data []byte) (analyzer.Report, error) {
	report, err := analyzer.AnalyzeForSource(jobID, sourceID, data)
	if err != nil {
		return analyzer.Report{}, err
	}
	report.SecurityReceipt.RawLogTTL = "not uploaded"
	return report, nil
}

func writeReport(out string, report analyzer.Report) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, append(encoded, '\n'), 0o600); err != nil {
		return err
	}
	return nil
}

func reportFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func printReportWrite(out string, report analyzer.Report) {
	label := "Sanitized report"
	if report.AggregateEvent.ParserType == "paid_bundle" {
		label = "Sanitized paid aggregate report"
	} else if report.AggregateEvent.ParserType == "full_scan_bundle" {
		label = "Sanitized full-scan aggregate report"
	}
	fmt.Printf("%s: %s (%d bytes)\n", label, out, reportFileSize(out))
}

func printNextStepsFor(out string) {
	fmt.Println()
	fmt.Printf("Review before upload: jq . %s\n", shellQuote(out))
	fmt.Printf("Upload sanitized report: agent-analyzer upload %s\n", shellQuote(out))
}

type progressBar struct {
	total       int
	width       int
	lastLen     int
	lastDone    int
	lastMessage string
	mode        progressMode
}

type progressMode string

const (
	progressModeBar   progressMode = "bar"
	progressModeLines progressMode = "lines"
	progressModeNone  progressMode = "none"
)

func newProgressBar(total int) *progressBar {
	if total < 1 {
		total = 1
	}
	return &progressBar{total: total, width: 24, lastDone: -1, mode: detectProgressMode()}
}

func detectProgressMode() progressMode {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AGENT_ANALYZER_PROGRESS"))) {
	case "bar":
		return progressModeBar
	case "line", "lines":
		return progressModeLines
	case "none", "off", "false", "0":
		return progressModeNone
	}

	// Claude Code, Codex, CI logs, and dumb terminals often render carriage
	// return updates as stacked lines. Prefer boring milestone output there.
	if os.Getenv("CODEX_SHELL") != "" ||
		os.Getenv("CODEX_CI") != "" ||
		os.Getenv("CLAUDE_CODE_OAUTH_TOKEN") != "" ||
		os.Getenv("CI") != "" ||
		os.Getenv("TERM") == "dumb" {
		return progressModeLines
	}
	if info, err := os.Stdout.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return progressModeBar
	}
	return progressModeLines
}

func (bar *progressBar) Update(done int, message string) {
	if bar.mode == progressModeNone {
		return
	}
	if done < 0 {
		done = 0
	}
	if done > bar.total {
		done = bar.total
	}
	if bar.mode == progressModeLines {
		if done == bar.lastDone && message == bar.lastMessage {
			return
		}
		fmt.Printf("[%d/%d] %s\n", done, bar.total, message)
		bar.lastDone = done
		bar.lastMessage = message
		return
	}
	filled := done * bar.width / bar.total
	empty := bar.width - filled
	head := ""
	if done < bar.total && empty > 0 {
		head = ">"
		empty--
	}
	line := fmt.Sprintf("\r[%s%s%s] %d/%d %s",
		strings.Repeat("=", filled),
		head,
		strings.Repeat(" ", empty),
		done,
		bar.total,
		message,
	)
	if bar.lastLen > len(line) {
		line += strings.Repeat(" ", bar.lastLen-len(line))
	}
	fmt.Print(line)
	bar.lastLen = len(line)
}

func (bar *progressBar) Finish(message string) {
	bar.Update(bar.total, message)
	if bar.mode == progressModeBar {
		fmt.Println()
	}
}

func (bar *progressBar) Fail() {
	if bar.mode == progressModeBar && bar.lastLen > 0 {
		fmt.Println()
	}
}

type sourceAnalysisResult struct {
	Candidate logCandidate
	Report    analyzer.Report
	Bytes     int
}

func reportsFromResults(results []sourceAnalysisResult) []analyzer.Report {
	reports := make([]analyzer.Report, 0, len(results))
	for _, result := range results {
		reports = append(reports, result.Report)
	}
	return reports
}

func buildSourceReports(results []sourceAnalysisResult) []analyzer.SourceReport {
	if len(results) == 0 {
		return nil
	}
	type group struct {
		sourceID    string
		sourceLabel string
		reports     []analyzer.Report
		bytes       int
	}
	order := []string{}
	groups := map[string]*group{}
	for _, result := range results {
		key := result.Candidate.SourceID
		if _, ok := groups[key]; !ok {
			order = append(order, key)
			groups[key] = &group{
				sourceID:    result.Candidate.SourceID,
				sourceLabel: result.Candidate.SourceLabel,
			}
		}
		groups[key].reports = append(groups[key].reports, result.Report)
		groups[key].bytes += result.Bytes
	}
	sourceReports := make([]analyzer.SourceReport, 0, len(order))
	for _, key := range order {
		group := groups[key]
		report := group.reports[0]
		if len(group.reports) > 1 {
			merged, err := analyzer.AggregateReportsWithParserType("local-"+group.sourceID, group.reports, group.bytes, "multi_source")
			if err == nil {
				report = merged
			}
		}
		sourceReports = append(sourceReports, analyzer.SourceReport{
			SourceID:        group.sourceID,
			SourceLabel:     group.sourceLabel,
			LogCount:        len(group.reports),
			Score:           report.Score,
			EstimatedWaste:  report.EstimatedWaste,
			Metrics:         report.Metrics,
			Findings:        report.Findings,
			Timeline:        report.Timeline,
			AnalysisSignals: report.AnalysisSignals,
			ImmediateFixes:  report.ImmediateFixes,
		})
	}
	return sourceReports
}

func analyzeDiscovered(candidates []logCandidate, out string, mode string, printNextSteps bool) error {
	if len(candidates) == 0 {
		return errors.New("no supported agent logs found; checked Claude Code, Codex, and OpenCode")
	}
	results := make([]sourceAnalysisResult, 0, len(candidates))
	totalBytes := 0
	totalRedacted := 0
	progress := newProgressBar(len(candidates)*2 + 1)
	step := 0
	for index, candidate := range candidates {
		progress.Update(step, fmt.Sprintf("reading %s %s", candidate.SourceLabel, candidate.shortDisplay()))
		data, err := candidate.readBytes()
		if err != nil {
			progress.Fail()
			return fmt.Errorf("read %s log %q: %w", candidate.SourceLabel, candidate.Display, err)
		}
		step++
		progress.Update(step, fmt.Sprintf("analyzing %s %s", candidate.SourceLabel, candidate.shortDisplay()))
		report, err := analyzeBytesForSource(fmt.Sprintf("local-%s-%03d", candidate.SourceID, index+1), candidate.SourceID, data)
		if err != nil {
			progress.Fail()
			return fmt.Errorf("analyze %s log %d: %w", candidate.SourceLabel, index+1, err)
		}
		results = append(results, sourceAnalysisResult{
			Candidate: candidate,
			Report:    report,
			Bytes:     len(data),
		})
		totalBytes += len(data)
		totalRedacted += report.SecurityReceipt.SecretsRedacted
		step++
		progress.Update(step, fmt.Sprintf("complete %s", candidate.SourceLabel))
	}

	var report analyzer.Report
	var err error
	reports := reportsFromResults(results)
	if mode == "" && len(reports) == 1 {
		report = reports[0]
		report.SecurityReceipt.RawLogTTL = "not uploaded"
	} else {
		parserType := "multi_source"
		jobID := "local-multi"
		if mode == "paid" {
			parserType = "paid_bundle"
			jobID = "local-paid"
		} else if mode == "full_scan" {
			parserType = "full_scan_bundle"
			jobID = "local-full-scan"
		}
		report, err = analyzer.AggregateReportsWithParserType(jobID, reports, totalBytes, parserType)
		if err != nil {
			progress.Fail()
			return err
		}
		report.SecurityReceipt.RawLogTTL = "not uploaded"
	}
	report.SourceReports = buildSourceReports(results)
	progress.Update(step, "writing sanitized report")
	if err := writeReport(out, report); err != nil {
		progress.Fail()
		return err
	}
	progress.Finish("complete")

	if mode == "paid" || mode == "full_scan" {
		fmt.Printf("Analyzed locally: %d supported agent logs across %d sources (%s)\n", len(candidates), sourceCount(candidates), sourceSummary(candidates))
	} else {
		fmt.Printf("Analyzed locally: %d representative recent supported agent log(s) across %d sources (%s)\n", len(candidates), sourceCount(candidates), sourceSummary(candidates))
	}
	fmt.Printf("Raw bytes read locally: %d\n", totalBytes)
	fmt.Printf("Secrets redacted before report write: %d\n", totalRedacted)
	printReportWrite(out, report)
	if printNextSteps {
		if mode == "paid" {
			fmt.Println()
			fmt.Printf("Review before upload: jq . %s\n", shellQuote(out))
			fmt.Printf("Upload sanitized paid aggregate with the command from your paid unlock page.\n")
		} else if mode == "full_scan" {
			fmt.Println()
			fmt.Printf("Review before upload: jq . %s\n", shellQuote(out))
			fmt.Printf("Upload sanitized full-scan aggregate with the email-confirmed full-scan command.\n")
		} else {
			printNextStepsFor(out)
		}
	}
	return nil
}

func runOneShot(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	out := fs.String("out", "agent-analyzer-report.json", "path to write sanitized report JSON")
	logPath := fs.String("log", "", "explicit supported JSON/JSONL log path")
	baseURL := fs.String("base-url", defaultBaseURL, "Agent Analyzer base URL")
	yes := fs.Bool("yes", false, "upload the sanitized report without an interactive confirmation")
	noOpen := fs.Bool("no-open", false, "do not open the generated report URL in a browser")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("agent-analyzer run: unexpected extra argument %q", fs.Arg(0))
	}
	path := *logPath
	if path == "" {
		candidates, err := latestSupportedLogsFn()
		if err != nil {
			return err
		}
		if err := analyzeDiscovered(candidates, *out, "", false); err != nil {
			return err
		}
	} else if err := analyzeSingle(path, *out, false); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Are you ready to get your report?")
	fmt.Println("- raw agent logs stayed on this machine")
	fmt.Println("- only the sanitized report JSON will be uploaded")
	fmt.Printf("- report file: %s\n", *out)
	if !*yes && !confirmUpload(os.Stdin, os.Stdout) {
		fmt.Println("Upload cancelled.")
		return nil
	}
	result, err := uploadReport(*out, *baseURL)
	if err != nil {
		return err
	}
	fmt.Printf("Uploaded sanitized report only: %s\n", *out)
	fmt.Printf("Report: %s\n", result.ReportURL)
	if !result.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", result.ExpiresAt.Local().Format(time.RFC1123))
	}
	if !*noOpen {
		_ = openBrowser(result.ReportURL)
	}
	return nil
}

func reorderAnalyzeArgs(args []string) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--out" || arg == "-out" || arg == "--log" || arg == "-log" || arg == "--limit" || arg == "-limit":
			flags = append(flags, arg)
			if i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		case strings.HasPrefix(arg, "--out=") || strings.HasPrefix(arg, "-out=") ||
			strings.HasPrefix(arg, "--log=") || strings.HasPrefix(arg, "-log=") ||
			strings.HasPrefix(arg, "--limit=") || strings.HasPrefix(arg, "-limit="):
			flags = append(flags, arg)
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	return append(flags, positionals...)
}

func runAnalyzePaid(out string, limit int) error {
	if limit <= 0 {
		return errors.New("agent-analyzer analyze: --limit must be greater than zero")
	}
	if limit > 100 {
		return errors.New("agent-analyzer analyze: --limit cannot exceed 100")
	}
	candidates, err := recentSupportedLogsFn(limit)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return errors.New("no supported agent logs found; checked Claude Code, Codex, and OpenCode")
	}
	return analyzeDiscovered(candidates, out, "paid", true)
}

func runAnalyzeFullScan(out string, limit int) error {
	if limit <= 0 {
		return errors.New("agent-analyzer full-scan: --limit must be greater than zero")
	}
	if limit > 100 {
		return errors.New("agent-analyzer full-scan: --limit cannot exceed 100")
	}
	candidates, err := recentSupportedLogsFn(limit)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return errors.New("no supported agent logs found; checked Claude Code, Codex, and OpenCode")
	}
	return analyzeDiscovered(candidates, out, "full_scan", true)
}

func runFullScan(args []string) error {
	fs := flag.NewFlagSet("full-scan", flag.ContinueOnError)
	out := fs.String("out", "agent-analyzer-full-scan-report.json", "path to write sanitized full-scan report JSON")
	baseURL := fs.String("base-url", defaultBaseURL, "Agent Analyzer base URL")
	token := fs.String("token", "", "email-confirmed full-scan entitlement token")
	limit := fs.Int("limit", 100, "maximum recent logs per supported source to analyze")
	noOpen := fs.Bool("no-open", false, "do not open the generated report URL in a browser")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("agent-analyzer full-scan: unexpected extra argument %q", fs.Arg(0))
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("agent-analyzer full-scan: --token is required")
	}
	if err := runAnalyzeFullScan(*out, *limit); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Uploading sanitized full-scan aggregate.")
	fmt.Println("- raw agent logs stayed on this machine")
	fmt.Println("- only the sanitized aggregate JSON will be uploaded")
	fmt.Printf("- report file: %s\n", *out)
	result, err := uploadFullScanReport(*out, *baseURL, *token)
	if err != nil {
		return err
	}
	fmt.Printf("Uploaded sanitized full-scan report only: %s\n", *out)
	fmt.Printf("Report: %s\n", result.ReportURL)
	if !result.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", result.ExpiresAt.Local().Format(time.RFC1123))
	}
	if !*noOpen {
		_ = openBrowser(result.ReportURL)
	}
	return nil
}

func runUpload(args []string) error {
	fs := flag.NewFlagSet("upload", flag.ContinueOnError)
	baseURL := fs.String("base-url", defaultBaseURL, "Agent Analyzer base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: agent-analyzer upload <sanitized-report.json>")
	}
	reportPath := fs.Arg(0)
	result, err := uploadReport(reportPath, *baseURL)
	if err != nil {
		return err
	}
	fmt.Printf("Uploaded sanitized report only: %s\n", reportPath)
	fmt.Printf("Report: %s\n", result.ReportURL)
	if !result.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", result.ExpiresAt.Local().Format(time.RFC1123))
	}
	return nil
}

type uploadResult struct {
	ReportURL string    `json:"report_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func uploadReport(reportPath, baseURL string) (uploadResult, error) {
	return uploadReportToEndpoint(reportPath, strings.TrimRight(baseURL, "/")+"/api/client-reports", "")
}

func uploadFullScanReport(reportPath, baseURL, token string) (uploadResult, error) {
	return uploadReportToEndpoint(reportPath, strings.TrimRight(baseURL, "/")+"/api/full-scan-client-reports", token)
}

func uploadReportToEndpoint(reportPath, endpoint, bearer string) (uploadResult, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return uploadResult{}, err
	}
	var report analyzer.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return uploadResult{}, fmt.Errorf("report is not valid analyzer JSON: %w", err)
	}
	if report.SecurityReceipt.RawTranscriptSentToLLM {
		return uploadResult{}, errors.New("refusing to upload report that claims raw transcript was sent to an LLM")
	}

	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return uploadResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return uploadResult{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return uploadResult{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return uploadResult{}, fmt.Errorf("upload failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var result uploadResult
	if err := json.Unmarshal(body, &result); err != nil {
		return uploadResult{}, err
	}
	return result, nil
}

type logCandidate struct {
	SourceID    string
	SourceLabel string
	Display     string
	ModTime     time.Time
	Size        int64
	Read        func() ([]byte, error)
}

func (candidate logCandidate) readBytes() ([]byte, error) {
	if candidate.Read != nil {
		return candidate.Read()
	}
	return os.ReadFile(candidate.Display)
}

func (candidate logCandidate) shortDisplay() string {
	return shortDisplay(candidate.Display)
}

func shortDisplay(value string) string {
	if value == "" {
		return "log"
	}
	if strings.Contains(value, string(os.PathSeparator)) {
		if base := filepath.Base(value); base != "." && base != string(os.PathSeparator) {
			return base
		}
	}
	if len(value) > 48 {
		return value[:45] + "..."
	}
	return value
}

func latestSupportedLogs() ([]logCandidate, error) {
	return representativeSupportedLogs()
}

func recentSupportedLogs(limit int) ([]logCandidate, error) {
	return recentSupportedLogsWithBounds(limit, 0, freeAutoMinLogBytes)
}

func representativeSupportedLogs() ([]logCandidate, error) {
	candidates, err := recentSupportedLogsWithBounds(freeAutoCandidatePool, freeAutoMaxLogBytes, freeAutoMinLogBytes)
	if err != nil {
		return nil, err
	}
	selected := make([]logCandidate, 0, 3)
	for _, sourceID := range []string{"claude_code", "codex", "opencode"} {
		bestIndex := -1
		for index := range candidates {
			if candidates[index].SourceID != sourceID {
				continue
			}
			if bestIndex == -1 ||
				candidates[index].Size > candidates[bestIndex].Size ||
				candidates[index].Size == candidates[bestIndex].Size && candidates[index].ModTime.After(candidates[bestIndex].ModTime) {
				bestIndex = index
			}
		}
		if bestIndex >= 0 {
			selected = append(selected, candidates[bestIndex])
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("no supported agent logs found; checked Claude Code, Codex, and OpenCode")
	}
	return selected, nil
}

func recentSupportedLogsWithBounds(limit int, maxBytes int64, minBytes int64) ([]logCandidate, error) {
	if limit <= 0 {
		return nil, errors.New("log discovery limit must be greater than zero")
	}
	var candidates []logCandidate
	fileSources := []struct {
		id    string
		label string
		root  []string
		exts  map[string]bool
	}{
		{
			id:    "claude_code",
			label: "Claude Code",
			root:  []string{".claude", "projects"},
			exts:  map[string]bool{".jsonl": true},
		},
		{
			id:    "codex",
			label: "Codex",
			root:  []string{".codex", "sessions"},
			exts:  map[string]bool{".jsonl": true},
		},
	}
	for _, source := range fileSources {
		found, err := recentFileLogs(source.id, source.label, source.root, source.exts, limit, maxBytes, minBytes)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, found...)
	}
	openCode, err := recentOpenCodeSessions(limit, maxBytes, minBytes)
	if err != nil {
		return nil, err
	}
	candidates = append(candidates, openCode...)
	if len(candidates) == 0 {
		return nil, errors.New("no supported agent logs found; checked Claude Code, Codex, and OpenCode")
	}
	return candidates, nil
}

func recentFileLogs(sourceID, sourceLabel string, rootParts []string, extensions map[string]bool, limit int, maxBytes int64, minBytes int64) ([]logCandidate, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	parts := append([]string{home}, rootParts...)
	root := filepath.Join(parts...)
	var matches []logMatch
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || !extensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if maxBytes > 0 && info.Size() > maxBytes {
			return nil
		}
		if minBytes > 0 && info.Size() < minBytes {
			return nil
		}
		matches = append(matches, logMatch{path: path, modTime: info.ModTime(), size: info.Size()})
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].modTime.Equal(matches[j].modTime) {
			return matches[i].path > matches[j].path
		}
		return matches[i].modTime.After(matches[j].modTime)
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	candidates := make([]logCandidate, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, logCandidate{
			SourceID:    sourceID,
			SourceLabel: sourceLabel,
			Display:     match.path,
			ModTime:     match.modTime,
			Size:        match.size,
		})
	}
	return candidates, nil
}

type logMatch struct {
	path    string
	modTime time.Time
	size    int64
}

func recentOpenCodeSessions(limit int, maxBytes int64, minBytes int64) ([]logCandidate, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, ".local", "share", "opencode", "storage", "message")
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	matches := make([]logMatch, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "ses_") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		size, modTime, err := openCodeSessionStats(path)
		if err != nil || size == 0 {
			continue
		}
		if maxBytes > 0 && size > maxBytes {
			continue
		}
		if minBytes > 0 && size < minBytes {
			continue
		}
		matches = append(matches, logMatch{path: path, modTime: modTime, size: size})
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].modTime.Equal(matches[j].modTime) {
			return matches[i].path > matches[j].path
		}
		return matches[i].modTime.After(matches[j].modTime)
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	candidates := make([]logCandidate, 0, len(matches))
	for _, match := range matches {
		sessionPath := match.path
		sessionID := filepath.Base(sessionPath)
		candidates = append(candidates, logCandidate{
			SourceID:    "opencode",
			SourceLabel: "OpenCode",
			Display:     "opencode session " + sessionID,
			ModTime:     match.modTime,
			Size:        match.size,
			Read: func() ([]byte, error) {
				return readOpenCodeSessionMessages(sessionPath)
			},
		})
	}
	return candidates, nil
}

func openCodeSessionStats(path string) (int64, time.Time, error) {
	var total int64
	var latest time.Time
	var messageIDs []string
	err := filepath.WalkDir(path, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(filePath)) != ".json" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		data, err := os.ReadFile(filePath)
		if err == nil {
			if messageID := openCodeMessageID(bytes.TrimSpace(data)); messageID != "" {
				messageIDs = append(messageIDs, messageID)
			}
		}
		return nil
	})
	if err != nil {
		return total, latest, err
	}
	partRoot := openCodePartRootForMessageSession(path)
	for _, messageID := range messageIDs {
		partFiles, err := openCodePartFiles(partRoot, messageID)
		if err != nil {
			return total, latest, err
		}
		for _, part := range partFiles {
			total += part.size
			if part.modTime.After(latest) {
				latest = part.modTime
			}
		}
	}
	return total, latest, err
}

func readOpenCodeSessionMessages(path string) ([]byte, error) {
	var files []logMatch
	err := filepath.WalkDir(path, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(filePath)) != ".json" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		files = append(files, logMatch{path: filePath, modTime: info.ModTime(), size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].path < files[j].path
		}
		return files[i].modTime.Before(files[j].modTime)
	})
	var output bytes.Buffer
	for _, file := range files {
		data, err := os.ReadFile(file.path)
		if err != nil {
			return nil, err
		}
		trimmed := bytes.TrimSpace(data)
		if len(trimmed) == 0 {
			continue
		}
		output.Write(trimmed)
		output.WriteByte('\n')
		messageID := openCodeMessageID(trimmed)
		if messageID == "" {
			continue
		}
		parts, err := readOpenCodeMessageParts(openCodePartRootForMessageSession(path), messageID)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			output.Write(part)
			output.WriteByte('\n')
		}
	}
	return output.Bytes(), nil
}

func openCodePartRootForMessageSession(messageSessionPath string) string {
	storageRoot := filepath.Dir(filepath.Dir(messageSessionPath))
	partRoot := filepath.Join(storageRoot, "part")
	if _, err := os.Stat(partRoot); err != nil {
		return ""
	}
	return partRoot
}

func openCodeMessageID(data []byte) string {
	var decoded map[string]any
	if json.Unmarshal(data, &decoded) != nil {
		return ""
	}
	id, _ := decoded["id"].(string)
	return strings.TrimSpace(id)
}

func readOpenCodeMessageParts(partRoot string, messageID string) ([][]byte, error) {
	files, err := openCodePartFiles(partRoot, messageID)
	if err != nil {
		return nil, err
	}
	parts := make([][]byte, 0, len(files))
	for _, file := range files {
		data, err := os.ReadFile(file.path)
		if err != nil {
			return nil, err
		}
		trimmed := bytes.TrimSpace(data)
		if len(trimmed) > 0 {
			parts = append(parts, trimmed)
		}
	}
	return parts, nil
}

func openCodePartFiles(partRoot string, messageID string) ([]logMatch, error) {
	if partRoot == "" || messageID == "" {
		return nil, nil
	}
	root := filepath.Join(partRoot, messageID)
	var files []logMatch
	err := filepath.WalkDir(root, func(filePath string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(filePath)) != ".json" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		files = append(files, logMatch{path: filePath, modTime: info.ModTime(), size: info.Size()})
		return nil
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].modTime.Equal(files[j].modTime) {
			return files[i].path < files[j].path
		}
		return files[i].modTime.Before(files[j].modTime)
	})
	return files, nil
}

func sourceCount(candidates []logCandidate) int {
	seen := map[string]bool{}
	for _, candidate := range candidates {
		seen[candidate.SourceID] = true
	}
	return len(seen)
}

func sourceSummary(candidates []logCandidate) string {
	counts := map[string]int{}
	labels := map[string]string{}
	for _, candidate := range candidates {
		counts[candidate.SourceID]++
		labels[candidate.SourceID] = candidate.SourceLabel
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", labels[key], counts[key]))
	}
	return strings.Join(parts, ", ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func confirmUpload(input io.Reader, output io.Writer) bool {
	fmt.Fprint(output, "Upload only this sanitized report? [y/N] ")
	var answer string
	_, _ = fmt.Fscanln(input, &answer)
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func openBrowser(url string) error {
	if url == "" {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: agent-analyzer run [--out <path>] [--base-url <url>] [--yes] [--no-open]")
	fmt.Fprintln(os.Stderr, "       agent-analyzer analyze [<log-path>] [--log <path>] [--out <path>] ...")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  run            analyze locally, ask for upload confirmation, upload sanitized JSON, and open the report.")
	fmt.Fprintln(os.Stderr, "  <log-path>     path to a supported JSON/JSONL log; mutually exclusive with --log.")
	fmt.Fprintln(os.Stderr, "                 if neither is supplied, one latest bounded-size log per supported source is used.")
	fmt.Fprintln(os.Stderr, "                 currently auto-discovers Claude Code, Codex, and OpenCode.")
	fmt.Fprintln(os.Stderr, "  --log <path>   explicit log path; mutually exclusive with a positional <log-path>.")
	fmt.Fprintln(os.Stderr, "  --out <path>   output path for the sanitized report JSON (default: ./agent-analyzer-report.json).")
	fmt.Fprintln(os.Stderr, "  --paid         analyze recent supported logs locally and write a sanitized aggregate report.")
	fmt.Fprintln(os.Stderr, "  --limit <n>    maximum recent logs per source for --paid, capped at 100 (default: 100).")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  agent-analyzer full-scan --token <token> [--base-url https://analyzer.spec-kitty.ai] [--no-open]")
	fmt.Fprintln(os.Stderr, "  agent-analyzer upload <sanitized-report.json> [--base-url https://analyzer.spec-kitty.ai]")
	fmt.Fprintln(os.Stderr, "  agent-analyzer version")
}
