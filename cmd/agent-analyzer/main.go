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

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
)

const defaultBaseURL = "https://analyzer.spec-kitty.ai"

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

// latestClaudeLogFn is a package-level indirection so tests can shim the
// "find the latest log under ~/.claude/projects" behavior without touching
// the user's real home directory.
var latestClaudeLogFn = latestClaudeLog
var recentClaudeLogsFn = recentClaudeLogs

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	out := fs.String("out", "agent-analyzer-report.json", "path to write sanitized report JSON")
	logPath := fs.String("log", "", "explicit Claude Code JSONL log path")
	paid := fs.Bool("paid", false, "analyze recent Claude Code logs locally and write a sanitized paid aggregate report")
	limit := fs.Int("limit", 100, "maximum recent Claude Code JSONL logs to analyze with --paid")
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
		latest, err := latestClaudeLogFn()
		if err != nil {
			return err
		}
		path = latest
	}
	return analyzeSingle(path, *out, true)
}

func analyzeSingle(path, out string, printNextSteps bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	report, err := analyzer.Analyze("local", data)
	if err != nil {
		return err
	}
	report.SecurityReceipt.RawLogTTL = "not uploaded"
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, append(encoded, '\n'), 0o600); err != nil {
		return err
	}

	fmt.Printf("Analyzed locally: %s\n", path)
	fmt.Printf("Raw bytes read locally: %d\n", len(data))
	fmt.Printf("Secrets redacted before report write: %d\n", report.SecurityReceipt.SecretsRedacted)
	fmt.Printf("Sanitized report: %s (%d bytes)\n", out, len(encoded)+1)
	if printNextSteps {
		fmt.Println()
		fmt.Printf("Review before upload: jq . %s\n", shellQuote(out))
		fmt.Printf("Upload sanitized report: agent-analyzer upload %s\n", shellQuote(out))
	}
	return nil
}

func runOneShot(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	out := fs.String("out", "agent-analyzer-report.json", "path to write sanitized report JSON")
	logPath := fs.String("log", "", "explicit Claude Code JSONL log path")
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
		latest, err := latestClaudeLogFn()
		if err != nil {
			return err
		}
		path = latest
	}
	if err := analyzeSingle(path, *out, false); err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("Upload boundary:")
	fmt.Println("- raw Claude Code logs stayed on this machine")
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
	paths, err := recentClaudeLogsFn(limit)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("no Claude Code JSONL logs found under ~/.claude/projects")
	}
	reports := make([]analyzer.Report, 0, len(paths))
	totalBytes := 0
	totalRedacted := 0
	for index, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		report, err := analyzer.Analyze(fmt.Sprintf("local-paid-%03d", index+1), data)
		if err != nil {
			return fmt.Errorf("analyze paid log %d: %w", index+1, err)
		}
		reports = append(reports, report)
		totalBytes += len(data)
		totalRedacted += report.SecurityReceipt.SecretsRedacted
	}
	report, err := analyzer.AggregateReports("local-paid", reports, totalBytes)
	if err != nil {
		return err
	}
	report.SecurityReceipt.RawLogTTL = "not uploaded"
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, append(encoded, '\n'), 0o600); err != nil {
		return err
	}

	fmt.Printf("Analyzed locally: %d recent Claude Code logs\n", len(paths))
	fmt.Printf("Raw bytes read locally: %d\n", totalBytes)
	fmt.Printf("Secrets redacted before report write: %d\n", totalRedacted)
	fmt.Printf("Sanitized paid aggregate report: %s (%d bytes)\n", out, len(encoded)+1)
	fmt.Println()
	fmt.Printf("Review before upload: jq . %s\n", shellQuote(out))
	fmt.Printf("Upload sanitized paid aggregate with the command from your paid unlock page.\n")
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

	endpoint := strings.TrimRight(baseURL, "/") + "/api/client-reports"
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return uploadResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
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

func latestClaudeLog() (string, error) {
	matches, err := recentClaudeLogs(1)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", errors.New("no Claude Code JSONL logs found under ~/.claude/projects")
	}
	return matches[0], nil
}

func recentClaudeLogs(limit int) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(home, ".claude", "projects")
	var matches []logMatch
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		matches = append(matches, logMatch{path: path, modTime: info.ModTime()})
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, errors.New("no Claude Code JSONL logs found under ~/.claude/projects")
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
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		paths = append(paths, match.path)
	}
	return paths, nil
}

type logMatch struct {
	path    string
	modTime time.Time
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
	fmt.Fprintln(os.Stderr, "  <log-path>     path to a Claude Code JSONL log; mutually exclusive with --log.")
	fmt.Fprintln(os.Stderr, "                 if neither is supplied, the latest log in ~/.claude/projects/")
	fmt.Fprintln(os.Stderr, "                 is used.")
	fmt.Fprintln(os.Stderr, "  --log <path>   explicit log path; mutually exclusive with a positional <log-path>.")
	fmt.Fprintln(os.Stderr, "  --out <path>   output path for the sanitized report JSON (default: ./agent-analyzer-report.json).")
	fmt.Fprintln(os.Stderr, "  --paid         analyze recent logs locally and write a sanitized aggregate report.")
	fmt.Fprintln(os.Stderr, "  --limit <n>    maximum recent logs for --paid, capped at 100 (default: 100).")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  agent-analyzer upload <sanitized-report.json> [--base-url https://analyzer.spec-kitty.ai]")
	fmt.Fprintln(os.Stderr, "  agent-analyzer version")
}
