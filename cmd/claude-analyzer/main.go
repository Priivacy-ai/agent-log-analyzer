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
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
)

const defaultBaseURL = "https://claude-code.spec-kitty.ai"

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
	case "analyze":
		return runAnalyze(args[1:])
	case "upload":
		return runUpload(args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// latestClaudeLogFn is a package-level indirection so tests can shim the
// "find the latest log under ~/.claude/projects" behavior without touching
// the user's real home directory.
var latestClaudeLogFn = latestClaudeLog

func runAnalyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ContinueOnError)
	out := fs.String("out", "claude-analyzer-report.json", "path to write sanitized report JSON")
	logPath := fs.String("log", "", "explicit Claude Code JSONL log path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	positional := fs.Args()
	// FR-002 takes precedence over FR-003 when both a positional and --log
	// are supplied alongside extra positional arguments.
	if len(positional) >= 1 && *logPath != "" {
		return errors.New("claude-analyzer analyze: cannot combine positional log path with --log flag")
	}
	if len(positional) >= 2 {
		return fmt.Errorf("claude-analyzer analyze: unexpected extra argument %q", positional[1])
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
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o600); err != nil {
		return err
	}

	fmt.Printf("Analyzed locally: %s\n", path)
	fmt.Printf("Raw bytes read locally: %d\n", len(data))
	fmt.Printf("Secrets redacted before report write: %d\n", report.SecurityReceipt.SecretsRedacted)
	fmt.Printf("Sanitized report: %s (%d bytes)\n", *out, len(encoded)+1)
	fmt.Println()
	fmt.Printf("Review before upload: jq . %s\n", shellQuote(*out))
	fmt.Printf("Upload sanitized report: claude-analyzer upload %s\n", shellQuote(*out))
	return nil
}

func runUpload(args []string) error {
	fs := flag.NewFlagSet("upload", flag.ContinueOnError)
	baseURL := fs.String("base-url", defaultBaseURL, "Claude Analyzer base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: claude-analyzer upload <sanitized-report.json>")
	}
	reportPath := fs.Arg(0)
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}
	var report analyzer.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("report is not valid analyzer JSON: %w", err)
	}
	if report.SecurityReceipt.RawTranscriptSentToLLM {
		return errors.New("refusing to upload report that claims raw transcript was sent to an LLM")
	}

	endpoint := strings.TrimRight(*baseURL, "/") + "/api/client-reports"
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("upload failed: %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var result struct {
		ReportURL string    `json:"report_url"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	fmt.Printf("Uploaded sanitized report only: %s\n", reportPath)
	fmt.Printf("Report: %s\n", result.ReportURL)
	if !result.ExpiresAt.IsZero() {
		fmt.Printf("Expires: %s\n", result.ExpiresAt.Local().Format(time.RFC1123))
	}
	return nil
}

func latestClaudeLog() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, ".claude", "projects")
	var matches []string
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		matches = append(matches, path)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if len(matches) == 0 {
		return "", errors.New("no Claude Code JSONL logs found under ~/.claude/projects")
	}
	sort.Slice(matches, func(i, j int) bool {
		left, leftErr := os.Stat(matches[i])
		right, rightErr := os.Stat(matches[j])
		if leftErr != nil || rightErr != nil {
			return matches[i] > matches[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	return matches[0], nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: claude-analyzer analyze [<log-path>] [--log <path>] [--out <path>] ...")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  <log-path>     path to a Claude Code JSONL log; mutually exclusive with --log.")
	fmt.Fprintln(os.Stderr, "                 if neither is supplied, the latest log in ~/.claude/projects/")
	fmt.Fprintln(os.Stderr, "                 is used.")
	fmt.Fprintln(os.Stderr, "  --log <path>   explicit log path; mutually exclusive with a positional <log-path>.")
	fmt.Fprintln(os.Stderr, "  --out <path>   output path for the sanitized report JSON (default: ./claude-analyzer-report.json).")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  claude-analyzer upload <sanitized-report.json> [--base-url https://claude-code.spec-kitty.ai]")
}
