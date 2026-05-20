package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/signatureresearch"
)

func main() {
	configPath := flag.String("config", "docs/signature-research-sources.json", "source configuration JSON")
	outputPath := flag.String("out", ".data/signature-candidates.json", "candidate output path")
	timeout := flag.Duration("timeout", 90*time.Second, "overall crawl timeout")
	flag.Parse()

	configData, err := os.ReadFile(*configPath)
	if err != nil {
		fail("read config: %v", err)
	}
	var cfg signatureresearch.Config
	if err := json.Unmarshal(configData, &cfg); err != nil {
		fail("parse config: %v", err)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	report, err := signatureresearch.Collect(ctx, cfg, client)
	if err != nil {
		fail("collect candidates: %v", err)
	}

	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("marshal report: %v", err)
	}
	output = append(output, '\n')
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		fail("mkdir output dir: %v", err)
	}
	if err := os.WriteFile(*outputPath, output, 0o644); err != nil {
		fail("write output: %v", err)
	}
	fmt.Printf("wrote %d candidates from %d sources to %s\n", len(report.Candidates), report.SourceCount, *outputPath)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
