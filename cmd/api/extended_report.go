package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

func getExtendedReportHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		job, report, ok := loadAuthorizedReport(w, r, store)
		if !ok {
			return
		}
		filename := fmt.Sprintf("agent-analyzer-%s-extended.md", job.ID)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(renderExtendedMarkdown(report)))
	}
}

func loadAuthorizedReport(w http.ResponseWriter, r *http.Request, store app.APIStore) (app.Job, analyzer.Report, bool) {
	job, err := store.GetJob(r.PathValue("id"))
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "job not found")
		return app.Job{}, analyzer.Report{}, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return app.Job{}, analyzer.Report{}, false
	}
	if !tokenMatches(job.ReportTokenHash, r.PathValue("token")) {
		writeError(w, http.StatusUnauthorized, "invalid report token")
		return app.Job{}, analyzer.Report{}, false
	}
	report, err := store.GetReport(job.ID)
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "report not found")
		return app.Job{}, analyzer.Report{}, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid report id")
		return app.Job{}, analyzer.Report{}, false
	}
	return job, report, true
}

func renderExtendedMarkdown(report analyzer.Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Agent Analyzer Extended Report\n\n")
	fmt.Fprintf(&b, "This report was generated from sanitized local analysis JSON. Raw transcripts were not uploaded, and model tokens used to generate this report: 0.\n\n")
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "- Efficiency score: %d / 100\n", report.Score)
	fmt.Fprintf(&b, "- Estimated avoidable token spend: %d-%d%%\n", report.EstimatedWaste.Low, report.EstimatedWaste.High)
	fmt.Fprintf(&b, "- Estimated token volume: %s\n", formatTokens(report.Metrics.EstimatedTokens))
	fmt.Fprintf(&b, "- Tool-output token estimate: %s\n", formatTokens(report.Metrics.ToolOutputTokens))
	fmt.Fprintf(&b, "- Parsed turns: %s\n", formatInt(report.Metrics.Turns))
	fmt.Fprintf(&b, "- Sessions analyzed: %s\n\n", formatInt(report.Metrics.SessionCount))

	fmt.Fprintf(&b, "## Top Problems\n\n")
	if len(report.Findings) == 0 {
		fmt.Fprintf(&b, "No major deterministic problems detected.\n\n")
	} else {
		for i, finding := range report.Findings {
			fmt.Fprintf(&b, "%d. **%s** (%s / %s)\n", i+1, finding.Title, finding.Severity, finding.CostImpact)
			if evidence := findingEvidence(finding.Evidence); evidence != "" {
				fmt.Fprintf(&b, "   - Evidence: %s\n", evidence)
			}
			if finding.Recommendation != "" {
				fmt.Fprintf(&b, "   - Immediate move: %s\n", finding.Recommendation)
			}
		}
		fmt.Fprintln(&b)
	}

	if len(report.SourceReports) > 0 {
		fmt.Fprintf(&b, "## Agent Sources\n\n")
		for _, source := range report.SourceReports {
			fmt.Fprintf(&b, "### %s\n\n", source.SourceLabel)
			fmt.Fprintf(&b, "- Logs analyzed: %d\n", source.LogCount)
			fmt.Fprintf(&b, "- Efficiency score: %d / 100\n", source.Score)
			fmt.Fprintf(&b, "- Estimated waste: %d-%d%%\n", source.EstimatedWaste.Low, source.EstimatedWaste.High)
			fmt.Fprintf(&b, "- Estimated token volume: %s\n", formatTokens(source.Metrics.EstimatedTokens))
			if len(source.LogRefs) > 0 {
				fmt.Fprintf(&b, "- Local references:\n")
				for _, ref := range source.LogRefs {
					fmt.Fprintf(&b, "  - %s (%s)\n", ref.Label, ref.SizeBucket)
				}
			}
			fmt.Fprintln(&b)
		}
	}

	fmt.Fprintf(&b, "## Security Receipt\n\n")
	fmt.Fprintf(&b, "- Raw transcript sent to LLM: %s\n", boolText(report.SecurityReceipt.RawTranscriptSentToLLM))
	fmt.Fprintf(&b, "- Outbound during local analysis: %s\n", boolText(report.SecurityReceipt.OutboundDuringAnalysis))
	fmt.Fprintf(&b, "- Raw log TTL: %s\n", report.SecurityReceipt.RawLogTTL)
	fmt.Fprintf(&b, "- Secrets redacted locally before upload: %d\n", report.SecurityReceipt.SecretsRedacted)
	fmt.Fprintln(&b)

	if report.Recommendation != nil {
		fmt.Fprintf(&b, "## Recommended Tools\n\n")
		for _, rec := range []*analyzer.TokenSavingRecommendation{report.Recommendation.Primary, report.Recommendation.Secondary} {
			if rec == nil {
				continue
			}
			if rec.PrimaryToolID == "" && rec.PrimaryToolName == "" {
				continue
			}
			fmt.Fprintf(&b, "- **%s**: %s\n", recommendationName(*rec), recommendationPurpose(*rec))
			if url := recommendationURL(*rec); url != "" {
				fmt.Fprintf(&b, "  - Source: %s\n", url)
			}
		}
	}

	return b.String()
}
