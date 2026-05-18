package analyzer

import (
	"errors"
	"sort"
)

func AggregateReports(jobID string, reports []Report, inputSize int) (Report, error) {
	if len(reports) == 0 {
		return Report{}, errors.New("no reports to aggregate")
	}
	var metrics Metrics
	metrics.SessionCount = len(reports)
	redactions := map[string]int{}
	ecosystem := Ecosystem{}
	timeline := make([]TimelinePoint, 0, len(reports))
	for index, report := range reports {
		metrics.Turns += report.Metrics.Turns
		metrics.EstimatedTokens += report.Metrics.EstimatedTokens
		metrics.ToolOutputTokens += report.Metrics.ToolOutputTokens
		metrics.Rereads += report.Metrics.Rereads
		metrics.FailedCommands += report.Metrics.FailedCommands
		metrics.ContextGrowthEvents += report.Metrics.ContextGrowthEvents
		if report.Metrics.RetryDepthMax > metrics.RetryDepthMax {
			metrics.RetryDepthMax = report.Metrics.RetryDepthMax
		}
		for family, count := range report.Redactions {
			redactions[family] += count
		}
		ecosystem = mergeEcosystems(ecosystem, report.Ecosystem)
		timeline = append(timeline, TimelinePoint{
			Turn:            index + 1,
			EstimatedTokens: metrics.EstimatedTokens,
			ToolTokens:      metrics.ToolOutputTokens,
			Rereads:         metrics.Rereads,
			Retries:         metrics.FailedCommands,
		})
	}
	findings := aggregateFindings(metrics)
	score := score(metrics, findings)
	report := Report{
		JobID:          jobID,
		Version:        Version,
		Score:          score,
		EstimatedWaste: wasteRange(score, metrics),
		Metrics:        metrics,
		Findings:       findings,
		Ecosystem:      ecosystem,
		Redactions:     redactions,
		SecurityReceipt: SecurityReceipt{
			RawTranscriptSentToLLM: false,
			OutboundDuringAnalysis: false,
			SecretsRedacted:        sumRedactions(redactions),
			RawLogTTL:              "15m",
		},
		Timeline:       timeline,
		ImmediateFixes: immediateFixes(findings),
	}
	normalizeReportCollections(&report)
	report.AggregateEvent = aggregateEvent(report, "paid_bundle", inputSize)
	return report, nil
}

func aggregateFindings(metrics Metrics) []Finding {
	var findings []Finding
	if metrics.Rereads >= 3 {
		findings = append(findings, Finding{
			ID:         "repeated_file_reads",
			Title:      "Excessive repeated file reads",
			Severity:   severity(metrics.Rereads, 3, 25),
			CostImpact: "medium-high",
			Evidence: FindingEvidence{
				Count:       metrics.Rereads,
				Description: "Repeated reads across paid scan sessions",
			},
			Recommendation: "Prefer targeted searches and summarize file state before rereading the same files.",
			Deterministic:  true,
		})
	}
	if metrics.ToolOutputTokens > 0 && metrics.EstimatedTokens > 0 {
		share := int(float64(metrics.ToolOutputTokens) / float64(metrics.EstimatedTokens) * 100)
		if share >= 35 {
			findings = append(findings, Finding{
				ID:         "tool_output_bloat",
				Title:      "Large shell/tool output overhead",
				Severity:   severity(share, 35, 55),
				CostImpact: "high",
				Evidence: FindingEvidence{
					TokenShare:  share,
					Description: "Tool output share across paid scan sessions",
				},
				Recommendation: "Cap command output and use narrower queries before pasting long terminal output into context.",
				Deterministic:  true,
			})
		}
	}
	if metrics.RetryDepthMax >= 3 {
		findings = append(findings, Finding{
			ID:         "retry_loop",
			Title:      "Retry-loop behavior",
			Severity:   severity(metrics.RetryDepthMax, 3, 6),
			CostImpact: "medium",
			Evidence: FindingEvidence{
				Count:       metrics.RetryDepthMax,
				Description: "Maximum retry depth across paid scan sessions",
			},
			Recommendation: "Stop after repeated failures, inspect the invariant, and restart with a smaller debugging scope.",
			Deterministic:  true,
		})
	}
	if metrics.ContextGrowthEvents >= 2 {
		findings = append(findings, Finding{
			ID:         "context_growth_spikes",
			Title:      "Context growth spikes",
			Severity:   severity(metrics.ContextGrowthEvents, 2, 5),
			CostImpact: "medium",
			Evidence: FindingEvidence{
				Count:       metrics.ContextGrowthEvents,
				Description: "Timeline windows exceeded growth threshold across paid scan sessions",
			},
			Recommendation: "Compact after task pivots and avoid combining architecture, debugging, and implementation in one long session.",
			Deterministic:  true,
		})
	}
	return findings
}

func mergeEcosystems(left, right Ecosystem) Ecosystem {
	left.Client = firstNonEmpty(left.Client, right.Client)
	left.OperatingSystem = firstNonEmpty(left.OperatingSystem, right.OperatingSystem)
	left.Shell = firstNonEmpty(left.Shell, right.Shell)
	left.VersionControl = firstNonEmpty(left.VersionControl, right.VersionControl)
	left.CodingAgents = mergeStrings(left.CodingAgents, right.CodingAgents)
	left.WorkflowFrameworks = mergeStrings(left.WorkflowFrameworks, right.WorkflowFrameworks)
	left.MCPServersKnown = mergeStrings(left.MCPServersKnown, right.MCPServersKnown)
	left.KnownSkills = mergeStrings(left.KnownSkills, right.KnownSkills)
	left.KnownPlugins = mergeStrings(left.KnownPlugins, right.KnownPlugins)
	left.PackageManagers = mergeStrings(left.PackageManagers, right.PackageManagers)
	left.UnknownMCPServerCount += right.UnknownMCPServerCount
	left.UnknownSkillCount += right.UnknownSkillCount
	left.UnknownPluginCount += right.UnknownPluginCount
	return left
}

func mergeStrings(left, right []string) []string {
	seen := map[string]bool{}
	for _, value := range left {
		if value != "" {
			seen[value] = true
		}
	}
	for _, value := range right {
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(left, right string) string {
	if left != "" {
		return left
	}
	return right
}
