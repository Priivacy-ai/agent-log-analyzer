package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/robertdouglass/claude-log-analyzer/internal/analyzer"
)

type candidate struct {
	path string
	size int64
}

type result struct {
	Index          int                 `json:"index"`
	SizeBucket     string              `json:"size_bucket"`
	Score          int                 `json:"score"`
	Turns          int                 `json:"turns"`
	EstimatedWaste analyzer.WasteRange `json:"estimated_waste_pct"`
	Findings       map[string]string   `json:"findings"`
	Redactions     int                 `json:"redactions"`
	Ecosystem      analyzer.Ecosystem  `json:"ecosystem"`
}

type summary struct {
	Root              string         `json:"root"`
	Discovered        int            `json:"discovered"`
	Analyzed          int            `json:"analyzed"`
	SkippedTooLarge   int            `json:"skipped_too_large"`
	Failed            int            `json:"failed"`
	ScoreMin          int            `json:"score_min"`
	ScoreMax          int            `json:"score_max"`
	ScoreAverage      int            `json:"score_average"`
	FindingCounts     map[string]int `json:"finding_counts"`
	EcosystemCounts   map[string]int `json:"ecosystem_counts"`
	RedactionFamilies map[string]int `json:"redaction_families"`
	Results           []result       `json:"results"`
}

func main() {
	defaultRoot := filepath.Join(os.Getenv("HOME"), ".claude", "projects")
	root := flag.String("root", defaultRoot, "Claude projects/log root")
	limit := flag.Int("limit", 20, "maximum logs to analyze")
	maxBytes := flag.Int64("max-bytes", 50*1024*1024, "maximum bytes per log")
	flag.Parse()

	candidates, err := discover(*root)
	if err != nil {
		fail("discover logs: %v", err)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].size > candidates[j].size
	})

	report := summary{
		Root:              *root,
		Discovered:        len(candidates),
		ScoreMin:          100,
		FindingCounts:     map[string]int{},
		EcosystemCounts:   map[string]int{},
		RedactionFamilies: map[string]int{},
	}
	scoreTotal := 0
	for _, candidate := range candidates {
		if report.Analyzed >= *limit {
			break
		}
		if candidate.size > *maxBytes {
			report.SkippedTooLarge++
			continue
		}
		data, err := os.ReadFile(candidate.path)
		if err != nil {
			report.Failed++
			continue
		}
		analysis, err := analyzer.Analyze(fmt.Sprintf("local-%d", report.Analyzed+1), data)
		if err != nil {
			report.Failed++
			continue
		}
		report.Analyzed++
		scoreTotal += analysis.Score
		if analysis.Score < report.ScoreMin {
			report.ScoreMin = analysis.Score
		}
		if analysis.Score > report.ScoreMax {
			report.ScoreMax = analysis.Score
		}
		for _, finding := range analysis.Findings {
			report.FindingCounts[finding.ID]++
		}
		countEcosystem(report.EcosystemCounts, analysis.Ecosystem)
		for family, count := range analysis.Redactions {
			report.RedactionFamilies[family] += count
		}
		report.Results = append(report.Results, result{
			Index:          report.Analyzed,
			SizeBucket:     analysis.AggregateEvent.InputSizeBucket,
			Score:          analysis.Score,
			Turns:          analysis.Metrics.Turns,
			EstimatedWaste: analysis.EstimatedWaste,
			Findings:       analysis.AggregateEvent.Findings,
			Redactions:     sum(analysis.Redactions),
			Ecosystem:      analysis.Ecosystem,
		})
	}
	if report.Analyzed == 0 {
		report.ScoreMin = 0
	} else {
		report.ScoreAverage = scoreTotal / report.Analyzed
	}

	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail("marshal summary: %v", err)
	}
	fmt.Println(string(output))
}

func discover(root string) ([]candidate, error) {
	var out []candidate
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		out = append(out, candidate{path: path, size: info.Size()})
		return nil
	})
	return out, err
}

func countEcosystem(counts map[string]int, eco analyzer.Ecosystem) {
	for _, value := range eco.CodingAgents {
		counts["coding_agent:"+value]++
	}
	for _, value := range eco.WorkflowFrameworks {
		counts["framework:"+value]++
	}
	for _, value := range eco.MCPServersKnown {
		counts["mcp:"+value]++
	}
	for _, value := range eco.KnownPlugins {
		counts["plugin:"+value]++
	}
	for _, value := range eco.KnownSkills {
		counts["skill:"+value]++
	}
	for _, value := range eco.PackageManagers {
		counts["package_manager:"+value]++
	}
	if eco.OperatingSystem != "" {
		counts["os:"+eco.OperatingSystem]++
	}
	if eco.Shell != "" {
		counts["shell:"+eco.Shell]++
	}
	if eco.VersionControl != "" {
		counts["vcs:"+eco.VersionControl]++
	}
}

func sum(values map[string]int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
