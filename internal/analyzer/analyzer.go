package analyzer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

const Version = "0.1.0"

var fileReadRE = regexp.MustCompile(`(?i)\b(?:cat|sed|nl|bat|less|head|tail)\s+(?:-[^\s]+\s+)*([A-Za-z0-9_./@~+-]+\.[A-Za-z0-9_+-]+)`)

type parsedLine struct {
	Text      string
	IsTool    bool
	IsError   bool
	Command   string
	ToolName  string
	TurnIndex int
}

func Analyze(jobID string, input []byte) (Report, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return Report{}, errors.New("empty upload")
	}

	scrubbed, redactions := Scrub(input)
	lines, parserType := parseLines(scrubbed)
	if len(lines) == 0 {
		return Report{}, errors.New("no parseable content")
	}

	metrics, timeline := computeMetrics(lines)
	ecosystem := DetectEcosystem(scrubbed, lines)
	findings := buildFindings(metrics, lines)
	score := score(metrics, findings)
	waste := wasteRange(score, metrics)

	report := Report{
		JobID:          jobID,
		Version:        Version,
		Score:          score,
		EstimatedWaste: waste,
		Metrics:        metrics,
		Findings:       findings,
		Ecosystem:      ecosystem,
		Redactions:     redactions,
		SecurityReceipt: SecurityReceipt{
			RawTranscriptSentToLLM: false,
			OutboundDuringAnalysis: false,
			SecretsRedacted:        sumRedactions(redactions),
			RawLogTTL:              "15m production target; local Docker data is developer-controlled",
		},
		Timeline:       timeline,
		ImmediateFixes: immediateFixes(findings),
	}
	report.AggregateEvent = aggregateEvent(report, parserType, len(input))
	return report, nil
}

func parseLines(input []byte) ([]parsedLine, string) {
	scanner := bufio.NewScanner(bytes.NewReader(input))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	var lines []parsedLine
	parserType := "text"
	turn := 0
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		line := parsedLine{Text: raw}
		var obj map[string]any
		if json.Unmarshal([]byte(raw), &obj) == nil {
			parserType = "jsonl"
			line.Text = flattenJSON(obj)
			if hasJSONTypeContaining(obj, "tool") {
				line.IsTool = true
			}
			if name := firstJSONStringByKey(obj, "name"); name != "" {
				line.ToolName = name
			}
			if tool := firstJSONStringByKey(obj, "tool"); tool != "" {
				line.ToolName = tool
				line.IsTool = true
			}
			if cmd := firstJSONStringByKey(obj, "command"); cmd != "" {
				line.Command = cmd
				line.IsTool = true
			}
			if errText := firstJSONStringByKey(obj, "error"); errText != "" {
				line.IsError = true
			}
			if firstJSONBoolByKey(obj, "is_error") {
				line.IsError = true
			}
		}
		lower := strings.ToLower(line.Text)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "traceback") {
			line.IsError = true
		}
		if line.Command == "" {
			line.Command = extractCommand(line.Text)
		}
		if line.IsTool || line.Command != "" {
			line.IsTool = true
		}
		turn++
		line.TurnIndex = turn
		lines = append(lines, line)
	}
	return lines, parserType
}

func flattenJSON(obj map[string]any) string {
	var parts []string
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case string:
			parts = append(parts, typed)
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(obj)
	return strings.Join(parts, " ")
}

func hasJSONTypeContaining(value any, needle string) bool {
	needle = strings.ToLower(needle)
	var found bool
	var walk func(any)
	walk = func(v any) {
		if found {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				if key == "type" {
					if text, ok := item.(string); ok && strings.Contains(strings.ToLower(text), needle) {
						found = true
						return
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func firstJSONStringByKey(value any, target string) string {
	var found string
	var walk func(any)
	walk = func(v any) {
		if found != "" {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				if key == target {
					if text, ok := item.(string); ok {
						found = text
						return
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func firstJSONBoolByKey(value any, target string) bool {
	var found bool
	var walk func(any)
	walk = func(v any) {
		if found {
			return
		}
		switch typed := v.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				if key == target {
					if boolean, ok := item.(bool); ok && boolean {
						found = true
						return
					}
				}
				walk(item)
			}
		}
	}
	walk(value)
	return found
}

func extractCommand(text string) string {
	for _, marker := range []string{"command:", "$ ", "bash -lc"} {
		if idx := strings.Index(strings.ToLower(text), marker); idx >= 0 {
			return strings.TrimSpace(text[idx+len(marker):])
		}
	}
	return ""
}

func computeMetrics(lines []parsedLine) (Metrics, []TimelinePoint) {
	var m Metrics
	m.Turns = len(lines)
	fileReads := map[string]int{}
	var timeline []TimelinePoint
	currentTokens := 0
	currentToolTokens := 0
	currentRereads := 0
	currentRetries := 0
	consecutiveErrors := 0

	for _, line := range lines {
		tokens := estimateTokens(line.Text)
		m.EstimatedTokens += tokens
		currentTokens += tokens
		if line.IsTool {
			m.ToolOutputTokens += tokens
			currentToolTokens += tokens
		}
		if line.IsError {
			m.FailedCommands++
			consecutiveErrors++
			if consecutiveErrors > m.RetryDepthMax {
				m.RetryDepthMax = consecutiveErrors
			}
			currentRetries++
		} else if !line.IsTool {
			consecutiveErrors = 0
		}
		for _, match := range fileReadRE.FindAllStringSubmatch(line.Text, -1) {
			if len(match) > 1 {
				key := normalizeEvidencePath(match[1])
				fileReads[key]++
				if fileReads[key] > 1 {
					m.Rereads++
					currentRereads++
				}
			}
		}
		if line.TurnIndex%10 == 0 {
			timeline = append(timeline, TimelinePoint{
				Turn:            line.TurnIndex,
				EstimatedTokens: currentTokens,
				ToolTokens:      currentToolTokens,
				Rereads:         currentRereads,
				Retries:         currentRetries,
			})
		}
	}
	if len(lines)%10 != 0 {
		timeline = append(timeline, TimelinePoint{
			Turn:            len(lines),
			EstimatedTokens: currentTokens,
			ToolTokens:      currentToolTokens,
			Rereads:         currentRereads,
			Retries:         currentRetries,
		})
	}
	for i := 1; i < len(timeline); i++ {
		if timeline[i].EstimatedTokens-timeline[i-1].EstimatedTokens > 6000 {
			m.ContextGrowthEvents++
		}
	}
	return m, timeline
}

func buildFindings(m Metrics, lines []parsedLine) []Finding {
	var findings []Finding
	if m.Rereads >= 3 {
		findings = append(findings, Finding{
			ID:         "repeated_file_reads",
			Title:      "Excessive repeated file reads",
			Severity:   severity(m.Rereads, 3, 10),
			CostImpact: "medium-high",
			Evidence: FindingEvidence{
				Count:    m.Rereads,
				TopFiles: topRereadFiles(lines),
			},
			Recommendation: "Prefer targeted searches and summarize file state before rereading the same files.",
			Deterministic:  true,
		})
	}
	if m.ToolOutputTokens > 0 && m.EstimatedTokens > 0 {
		share := int(float64(m.ToolOutputTokens) / float64(m.EstimatedTokens) * 100)
		if share >= 35 {
			findings = append(findings, Finding{
				ID:         "tool_output_bloat",
				Title:      "Large shell/tool output overhead",
				Severity:   severity(share, 35, 55),
				CostImpact: "high",
				Evidence: FindingEvidence{
					TokenShare: share,
				},
				Recommendation: "Cap command output and use narrower queries before pasting long terminal output into context.",
				Deterministic:  true,
			})
		}
	}
	if m.RetryDepthMax >= 3 {
		findings = append(findings, Finding{
			ID:         "retry_loop",
			Title:      "Retry-loop behavior",
			Severity:   severity(m.RetryDepthMax, 3, 6),
			CostImpact: "medium",
			Evidence: FindingEvidence{
				Count: m.RetryDepthMax,
			},
			Recommendation: "Stop after repeated failures, inspect the invariant, and restart with a smaller debugging scope.",
			Deterministic:  true,
		})
	}
	if m.ContextGrowthEvents >= 2 {
		findings = append(findings, Finding{
			ID:         "context_growth_spikes",
			Title:      "Context growth spikes",
			Severity:   severity(m.ContextGrowthEvents, 2, 5),
			CostImpact: "medium",
			Evidence: FindingEvidence{
				Count:       m.ContextGrowthEvents,
				Description: fmt.Sprintf("%d timeline windows exceeded the growth threshold", m.ContextGrowthEvents),
			},
			Recommendation: "Compact after task pivots and avoid combining architecture, debugging, and implementation in one long session.",
			Deterministic:  true,
		})
	}
	return findings
}

func score(m Metrics, findings []Finding) int {
	score := 100
	score -= min(m.Rereads*2, 25)
	score -= min(m.RetryDepthMax*4, 20)
	if m.EstimatedTokens > 0 {
		share := int(float64(m.ToolOutputTokens) / float64(m.EstimatedTokens) * 100)
		score -= min(max(share-25, 0), 25)
	}
	score -= min(m.ContextGrowthEvents*5, 20)
	score -= len(findings) * 3
	if score < 0 {
		return 0
	}
	return score
}

func wasteRange(score int, m Metrics) WasteRange {
	low := max(0, (100-score)/2)
	high := min(65, low+max(8, m.Rereads/2+m.RetryDepthMax))
	return WasteRange{Low: low, High: high}
}

func immediateFixes(findings []Finding) []string {
	if len(findings) == 0 {
		return []string{"Keep sessions scoped and compact before major task pivots."}
	}
	fixes := make([]string, 0, len(findings))
	for _, finding := range findings {
		fixes = append(fixes, finding.Recommendation)
	}
	return fixes
}

func topRereadFiles(lines []parsedLine) []string {
	counts := map[string]int{}
	for _, line := range lines {
		for _, match := range fileReadRE.FindAllStringSubmatch(line.Text, -1) {
			if len(match) > 1 {
				counts[normalizeEvidencePath(match[1])]++
			}
		}
	}
	type pair struct {
		file  string
		count int
	}
	var pairs []pair
	for file, count := range counts {
		if count > 1 {
			pairs = append(pairs, pair{file: file, count: count})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	var out []string
	for i, p := range pairs {
		if i >= 5 {
			break
		}
		out = append(out, p.file)
	}
	return out
}

func normalizeEvidencePath(path string) string {
	path = strings.Trim(path, `"'`)
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func aggregateEvent(report Report, parserType string, inputSize int) AggregateSafeEvent {
	findings := map[string]string{}
	for _, finding := range report.Findings {
		findings[finding.ID] = finding.Severity
	}
	return AggregateSafeEvent{
		Event:           "analysis_completed",
		ParserType:      parserType,
		InputSizeBucket: bucket(inputSize, []int{1024, 1024 * 1024, 10 * 1024 * 1024, 50 * 1024 * 1024}),
		TurnBucket:      bucket(report.Metrics.Turns, []int{10, 50, 100, 200}),
		ScoreBucket:     bucket(report.Score, []int{20, 40, 60, 80}),
		WasteBucket:     bucket(report.EstimatedWaste.High, []int{10, 20, 40, 60}),
		Findings:        findings,
		Redactions:      report.Redactions,
		Ecosystem:       report.Ecosystem,
	}
}

func estimateTokens(text string) int {
	return max(1, len(text)/4)
}

func severity(value, medium, high int) string {
	if value >= high {
		return "high"
	}
	if value >= medium {
		return "medium"
	}
	return "low"
}

func bucket(value int, thresholds []int) string {
	prev := 0
	for _, threshold := range thresholds {
		if value < threshold {
			return fmt.Sprintf("%d_%d", prev, threshold)
		}
		prev = threshold
	}
	return fmt.Sprintf("%d_plus", prev)
}

func sumRedactions(redactions map[string]int) int {
	total := 0
	for _, count := range redactions {
		total += count
	}
	return total
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ReadAllLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("upload exceeds %d bytes", maxBytes)
	}
	return data, nil
}
