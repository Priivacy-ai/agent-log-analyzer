package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/robertdouglass/claude-log-analyzer/internal/analytics"
)

func main() {
	inputPath := flag.String("input", "", "analytics JSONL input path")
	outputPath := flag.String("out", "", "summary JSON output path; stdout when empty")
	minCohort := flag.Int("min-cohort", 10, "minimum cohort count before emitting a row")
	flag.Parse()

	if *inputPath == "" {
		fmt.Fprintln(os.Stderr, "--input is required")
		os.Exit(2)
	}
	input, err := os.Open(*inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open input: %v\n", err)
		os.Exit(1)
	}
	defer input.Close()

	summary, err := analytics.SummarizeJSONL(input, *minCohort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "summarize: %v\n", err)
		os.Exit(1)
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal summary: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')
	if *outputPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(*outputPath, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		os.Exit(1)
	}
}
