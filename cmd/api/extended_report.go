package main

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
	"github.com/priivacy-ai/agent-log-analyzer/internal/remediation"
)

func getExtendedReportHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		job, report, ok := loadAuthorizedReport(w, r, store)
		if !ok {
			return
		}
		packageBytes, err := renderDownloadPackage(job, report)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not render download package")
			return
		}
		filename := fmt.Sprintf("agent-analyzer-%s-download-pack.zip", job.ID)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(packageBytes)
	}
}

func renderDownloadPackage(job app.Job, report analyzer.Report) ([]byte, error) {
	guidePDF, err := renderFieldGuidePDF()
	if err != nil {
		return nil, err
	}
	reportPDF, err := renderPersonalizedReportPDF(report)
	if err != nil {
		return nil, err
	}
	voucherCode, err := randomVoucherCode()
	if err != nil {
		return nil, err
	}
	voucherPDF, err := renderSpecKittyVoucherPDF(voucherCode)
	if err != nil {
		return nil, err
	}
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	pluginPreview := renderPluginPreviewMarkdown(report)
	voucherText := renderSpecKittyVoucherText(voucherCode, job.ID)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := []struct {
		name string
		data []byte
	}{
		{name: "agent-token-saving-field-guide.pdf", data: guidePDF},
		{name: "personalized-agent-analyzer-report.pdf", data: reportPDF},
		{name: "agent-analyzer-report.json", data: append(reportJSON, '\n')},
		{name: "plugin-preview.md", data: []byte(pluginPreview)},
		{name: "partner-vouchers/spec-kitty-training-voucher.pdf", data: voucherPDF},
		{name: "partner-vouchers/spec-kitty-training-voucher.txt", data: []byte(voucherText)},
	}
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetModTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		w, err := zw.CreateHeader(header)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write(entry.data); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

func renderPluginPreviewMarkdown(report analyzer.Report) string {
	artifact := remediation.Generate(report, remediation.Options{GeneratedAt: deterministicPDFTime()})
	var b strings.Builder
	fmt.Fprintf(&b, "# Agent Analyzer Plugin Preview\n\n")
	fmt.Fprintf(&b, "The custom artifact turns this report into Claude Code plugin guidance plus harness-specific instructions for Codex, OpenCode, Cursor, Kiro, Antigravity, and Claude Desktop MCP. Reports can also originate from Claude Desktop local/session logs; Desktop remediation currently uses the MCP connector guidance. It is generated from sanitized report JSON only.\n\n")
	fmt.Fprintf(&b, "## Harness install matrix\n\n")
	for _, install := range artifact.Install.Harnesses {
		fmt.Fprintf(&b, "- **%s** (%s): %s\n", install.Harness, install.Surface, install.Install)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "## Planned customizations\n\n")
	if len(artifact.Customizations) == 0 {
		fmt.Fprintf(&b, "- No high-priority customizations detected from this report.\n")
	} else {
		for _, customization := range artifact.Customizations {
			fmt.Fprintf(&b, "- **%s**: %s\n", customization.ID, customization.Reason)
		}
	}
	fmt.Fprintf(&b, "\n## Vetted recommendations\n\n")
	if len(artifact.VettedRecommendations) == 0 {
		fmt.Fprintf(&b, "- No tool recommendations selected.\n")
	} else {
		for _, rec := range artifact.VettedRecommendations {
			fmt.Fprintf(&b, "- **%s** (%s): %s\n  - Source: %s\n", rec.ID, rec.Category, rec.Why, rec.Source)
		}
	}
	return b.String()
}

type teachingArticle struct {
	ID         string
	Title      string
	Body       string
	Source     string
	Priority   int
	FindingIDs []string
}

var teachingArticles = []teachingArticle{
	{
		ID:       "context-budget",
		Title:    "Context Is The Budget",
		Body:     "Every file read, command output, tool schema, and instruction competes for the same working context. Token savings therefore start with discipline: clear unrelated work, compact before long pivots, and keep durable project instructions short. The practical goal is not a smaller chat for its own sake; it is more useful coding work before your plan limits interrupt the session.",
		Source:   "https://code.claude.com/docs/en/best-practices",
		Priority: 100,
	},
	{
		ID:         "reread-discipline",
		Title:      "Stop Rereading Whole Files",
		Body:       "Whole-file rereads are expensive when the agent only needs a symbol, a failing function, or the changed lines. Search first, read bounded ranges second, and reserve full-file reads for short files or edits where file-wide invariants matter. A good retrieval ladder is: rg or file listing, symbol/LSP lookup, semantic search, MCP retrieval, bounded reads, then whole-file reads as the last step.",
		Source:     "https://ripgrep.dev/docs/guide/",
		Priority:   90,
		FindingIDs: []string{"repeated_file_reads"},
	},
	{
		ID:         "shell-output",
		Title:      "Treat Shell Output As Context",
		Body:       "Terminal output is not free scrollback. Coding agents often ingest command output into future context, so build and test logs can crowd out source code and instructions. Save full logs to files, then show the exit code, failing test names, the first relevant error block, and a short tail. Use jq for JSON and rg/head/tail/sed for bounded text.",
		Source:     "https://code.claude.com/docs/en/costs",
		Priority:   85,
		FindingIDs: []string{"large_tool_output"},
	},
	{
		ID:         "retry-loops",
		Title:      "Break Retry Loops Early",
		Body:       "Repeated attempts with the same failing premise are one of the fastest ways to burn context. After two similar failures, stop editing, restate the invariant, inspect the latest diff and error, and restart with a narrower scope. The generated plugin can turn recurring retry signatures into session hygiene rules.",
		Source:     "https://developers.openai.com/codex/learn/best-practices",
		Priority:   80,
		FindingIDs: []string{"retry_loop_behavior"},
	},
	{
		ID:         "context-boundaries",
		Title:      "Use Clear, Compact, And Fresh Sessions Deliberately",
		Body:       "Long sessions become less stable when debugging, architecture, and implementation all share the same context. Use fresh sessions for unrelated goals, compact when preserving a focused thread, and clear when the existing context no longer helps. Put compact-preservation rules in project instructions so important files, decisions, and test commands survive summarization.",
		Source:     "https://code.claude.com/docs/en/costs",
		Priority:   75,
		FindingIDs: []string{"context_growth_spikes", "cache_prefix_invalidation"},
	},
	{
		ID:       "instruction-hygiene",
		Title:    "Keep CLAUDE.md And AGENTS.md Lean",
		Body:     "Always-loaded instructions tax every session. Keep global rules durable and short, move task-specific workflows into skills or commands, and place directory-specific guidance near the code it governs. The point is not fewer rules; it is loading the right rules only when they can change the next action.",
		Source:   "https://developers.openai.com/codex/guides/agents-md",
		Priority: 70,
	},
	{
		ID:       "mcp-skill-surface",
		Title:    "Skills Are For Workflows; MCP Is For Live Capabilities",
		Body:     "Use skills for repeatable knowledge, checklists, scripts, and local workflows. Use MCP when the agent needs live data, authenticated actions, or an external system. Keep the default exposed tool set small, namespace tools clearly, require approval for destructive actions, and measure utilization before adding more surface area.",
		Source:   "https://developers.openai.com/api/docs/guides/function-calling",
		Priority: 65,
	},
	{
		ID:       "subagent-cost",
		Title:    "Subagents Are Useful, Not Free",
		Body:     "Subagents can keep verbose exploration out of the main thread and parallelize independent work, but each one performs its own model and tool work. Delegate bounded side tasks when they reduce parent context or unblock parallel progress. Avoid spawning agents for work that immediately blocks the next local step.",
		Source:   "https://code.claude.com/docs/en/sub-agents",
		Priority: 60,
	},
	{
		ID:       "prompt-caching",
		Title:    "Prompt Caching Helps, But Retrieval Still Matters",
		Body:     "Stable prompt prefixes can reduce cost and latency, but caching does not make noisy retrieval harmless. Keep reusable instructions stable, put variable task data last when you control the API prompt, and still avoid dumping unrelated files, logs, or tool schemas into context.",
		Source:   "https://developers.openai.com/api/docs/guides/prompt-caching",
		Priority: 50,
	},
}

func selectedTeachingArticles(report analyzer.Report, limit int) []teachingArticle {
	if limit <= 0 {
		return nil
	}
	findingIDs := map[string]bool{}
	for _, finding := range report.Findings {
		findingIDs[finding.ID] = true
	}
	var selected []teachingArticle
	for _, article := range teachingArticles {
		if len(article.FindingIDs) == 0 {
			selected = append(selected, article)
			continue
		}
		for _, id := range article.FindingIDs {
			if findingIDs[id] {
				selected = append(selected, article)
				break
			}
		}
	}
	if report.Ecosystem.ToolingUtilization.MCP.WarningBand != "" && report.Ecosystem.ToolingUtilization.MCP.WarningBand != "normal" {
		selected = appendArticleByID(selected, "mcp-skill-surface")
	}
	if report.Ecosystem.ToolingUtilization.Skill.WarningBand != "" && report.Ecosystem.ToolingUtilization.Skill.WarningBand != "normal" {
		selected = appendArticleByID(selected, "mcp-skill-surface")
	}
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].Priority == selected[j].Priority {
			return selected[i].ID < selected[j].ID
		}
		return selected[i].Priority > selected[j].Priority
	})
	selected = dedupeArticles(selected)
	if len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func appendArticleByID(articles []teachingArticle, id string) []teachingArticle {
	for _, article := range teachingArticles {
		if article.ID == id {
			return append(articles, article)
		}
	}
	return articles
}

func dedupeArticles(articles []teachingArticle) []teachingArticle {
	seen := map[string]bool{}
	out := make([]teachingArticle, 0, len(articles))
	for _, article := range articles {
		if seen[article.ID] {
			continue
		}
		seen[article.ID] = true
		out = append(out, article)
	}
	return out
}

func renderFieldGuidePDF() ([]byte, error) {
	pdf := newBrandedPDF("Agent Token Saving Field Guide")
	addCover(pdf, "Agent Token Saving Field Guide", "Deterministic operating habits for Claude Code, Codex, OpenCode, MCPs, skills, retrieval, and shell output.")
	addSection(pdf, "The Core Rule")
	addParagraph(pdf, "Context is the budget. The more irrelevant context an agent carries, the fewer useful turns you get before degradation, compaction, or plan limits interrupt real software work. Token saving is therefore not penny-pinching; it is a way to write more software with the same agentic coding plan.")
	addSection(pdf, "Field Guide")
	for _, article := range teachingArticles {
		addArticle(pdf, article)
	}
	addSection(pdf, "Deterministic Defaults")
	for _, bullet := range []string{
		"Search before reading whole files.",
		"Cap shell output and save full logs to files.",
		"Keep always-loaded project instructions short.",
		"Expose the smallest MCP/tool surface that can complete the task.",
		"Compact or clear after task pivots.",
		"Use subagents only when parallelism or context isolation pays for itself.",
	} {
		addBullet(pdf, bullet)
	}
	return outputPDF(pdf)
}

func renderPersonalizedReportPDF(report analyzer.Report) ([]byte, error) {
	pdf := newBrandedPDF("Personalized Agent Analyzer Report")
	addCover(pdf, "Personalized Agent Analyzer Report", "Generated from sanitized local analysis JSON. Raw transcripts were not uploaded. Model tokens used to generate this PDF: 0.")
	addSection(pdf, "Summary")
	addMetricRow(pdf, "Efficiency score", fmt.Sprintf("%d / 100", report.Score))
	addMetricRow(pdf, "Estimated avoidable token spend", fmt.Sprintf("%d-%d%%", report.EstimatedWaste.Low, report.EstimatedWaste.High))
	addMetricRow(pdf, "Estimated token volume", formatTokens(report.Metrics.EstimatedTokens))
	addMetricRow(pdf, "Tool-output token estimate", formatTokens(report.Metrics.ToolOutputTokens))
	addMetricRow(pdf, "Sessions analyzed", formatInt(report.Metrics.SessionCount))
	addParagraph(pdf, "The strongest savings opportunity is to reduce rereads, retries, large tool outputs, and stale context. That means more useful coding turns before plan limits become the bottleneck.")

	addSection(pdf, "Top Problems")
	if len(report.Findings) == 0 {
		addParagraph(pdf, "No major deterministic problems were detected.")
	} else {
		for i, finding := range report.Findings {
			if i >= 6 {
				break
			}
			addFinding(pdf, i+1, finding)
		}
	}

	addSection(pdf, "Selected Coaching")
	for _, article := range selectedTeachingArticles(report, 5) {
		addArticle(pdf, article)
	}

	addSection(pdf, "Security Receipt")
	addMetricRow(pdf, "Raw transcript sent to LLM", boolText(report.SecurityReceipt.RawTranscriptSentToLLM))
	addMetricRow(pdf, "Outbound during local analysis", boolText(report.SecurityReceipt.OutboundDuringAnalysis))
	addMetricRow(pdf, "Raw log TTL", report.SecurityReceipt.RawLogTTL)
	addMetricRow(pdf, "Secrets redacted locally before upload", formatInt(report.SecurityReceipt.SecretsRedacted))
	return outputPDF(pdf)
}

func renderSpecKittyVoucherPDF(code string) ([]byte, error) {
	pdf := newBrandedPDF("Spec Kitty Training Voucher")
	addSpecKittyVoucherCover(pdf)
	addSection(pdf, "Team Training Credit")
	addParagraph(pdf, "Use this voucher when booking Spec Kitty training for your team. Training helps teams turn agentic coding from ad hoc prompting into a repeatable specification, implementation, review, and merge workflow.")
	addParagraph(pdf, "Book or request details at "+specKittyTrainingURL+". Agent Analyzer records the voucher code only; redemption is handled through Spec Kitty.")
	pdf.Ln(6)
	pdf.SetFillColor(8, 24, 32)
	pdf.SetDrawColor(126, 231, 135)
	pdf.SetTextColor(126, 231, 135)
	pdf.SetFont("Courier", "B", 36)
	pdf.CellFormat(0, 26, code, "1", 1, "C", true, 0, "")
	pdf.Ln(4)
	pdf.SetTextColor(152, 162, 176)
	pdf.SetFont("Helvetica", "", 11)
	pdf.MultiCell(0, 6, "Discount: 20% off Spec Kitty training. Code format: six alphanumeric characters.", "", "C", false)
	pdf.Ln(8)
	pdf.SetFillColor(126, 231, 135)
	pdf.SetTextColor(13, 19, 27)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(0, 12, "Open spec-kitty.ai/training", "", 1, "C", true, 0, specKittyTrainingURL)
	pdf.Ln(4)
	pdf.SetTextColor(55, 169, 243)
	pdf.SetFont("Courier", "", 10)
	pdf.CellFormat(0, 7, specKittyTrainingURL, "", 1, "C", false, 0, specKittyTrainingURL)
	return outputPDF(pdf)
}

const specKittyTrainingURL = "https://spec-kitty.ai/training"

func renderSpecKittyVoucherText(code, jobID string) string {
	return fmt.Sprintf(`Spec Kitty training voucher
============================

Agent Analyzer partner credit for Spec Kitty team training.

Code: %s
Discount: 20%% off Spec Kitty training
Training page: %s

Bring your Agent Analyzer report to the training session. It gives the instructor a concrete starting point for your team's agentic coding workflow: session hygiene, specs, work packages, reviews, and merge discipline.

Generated by Agent Analyzer for report %s.
`, code, specKittyTrainingURL, jobID)
}

func addSpecKittyVoucherCover(pdf *fpdf.Fpdf) {
	pdf.AddPage()
	pdf.SetFillColor(8, 17, 26)
	pdf.Rect(0, 0, 210, 297, "F")
	pdf.SetFillColor(126, 231, 135)
	pdf.Rect(0, 0, 210, 14, "F")
	pdf.SetTextColor(13, 19, 27)
	pdf.SetFont("Courier", "B", 12)
	pdf.SetXY(18, 4)
	pdf.CellFormat(0, 7, "SPEC KITTY x AGENT ANALYZER", "", 1, "L", false, 0, "")
	pdf.SetY(44)
	pdf.SetTextColor(126, 231, 135)
	pdf.SetFont("Courier", "B", 15)
	pdf.CellFormat(0, 8, "SPEC KITTY TRAINING", "", 1, "L", false, 0, specKittyTrainingURL)
	pdf.Ln(8)
	pdf.SetTextColor(232, 238, 246)
	pdf.SetFont("Helvetica", "B", 34)
	pdf.MultiCell(0, 13, "20% off team training", "", "L", false)
	pdf.Ln(6)
	pdf.SetTextColor(152, 162, 176)
	pdf.SetFont("Helvetica", "", 14)
	pdf.MultiCell(0, 7, "Learn the Spec Kitty workflow with your own backlog: discovery, specs, work packages, implementation lanes, review, and merge.", "", "L", false)
	pdf.Ln(14)
	pdf.SetTextColor(255, 170, 92)
	pdf.SetFont("Courier", "B", 12)
	pdf.CellFormat(0, 8, specKittyTrainingURL, "", 1, "L", false, 0, specKittyTrainingURL)
}

func newBrandedPDF(title string) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetHeaderFunc(func() {
		pdf.SetFillColor(13, 19, 27)
		pdf.Rect(0, 0, 210, 297, "F")
	})
	pdf.SetFooterFunc(func() {
		pdf.SetY(-14)
		pdf.SetTextColor(74, 88, 107)
		pdf.SetFont("Courier", "", 8)
		pdf.CellFormat(0, 6, fmt.Sprintf("Agent Analyzer // page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	pdf.SetTitle(title, false)
	pdf.SetAuthor("Agent Analyzer", false)
	pdf.SetCreator("Agent Analyzer deterministic PDF generator", false)
	pdf.SetSubject("Agentic coding token-saving report", false)
	pdf.SetCreationDate(deterministicPDFTime())
	pdf.SetCompression(false)
	return pdf
}

func deterministicPDFTime() time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
}

func addCover(pdf *fpdf.Fpdf, title, subtitle string) {
	pdf.AddPage()
	pdf.SetFillColor(13, 19, 27)
	pdf.Rect(0, 0, 210, 297, "F")
	pdf.SetTextColor(126, 231, 135)
	pdf.SetFont("Courier", "B", 12)
	pdf.CellFormat(0, 8, "AGENT ANALYZER", "", 1, "L", false, 0, "")
	pdf.Ln(24)
	pdf.SetTextColor(232, 238, 246)
	pdf.SetFont("Helvetica", "B", 32)
	pdf.MultiCell(0, 12, title, "", "L", false)
	pdf.Ln(6)
	pdf.SetTextColor(152, 162, 176)
	pdf.SetFont("Helvetica", "", 14)
	pdf.MultiCell(0, 7, subtitle, "", "L", false)
	pdf.Ln(16)
	pdf.SetTextColor(255, 170, 92)
	pdf.SetFont("Courier", "B", 13)
	pdf.CellFormat(0, 8, "Save tokens. Write more software.", "", 1, "L", false, 0, "")
}

func addSection(pdf *fpdf.Fpdf, title string) {
	if pdf.GetY() > 250 {
		pdf.AddPage()
	}
	pdf.Ln(6)
	pdf.SetTextColor(126, 231, 135)
	pdf.SetFont("Helvetica", "B", 17)
	pdf.CellFormat(0, 9, title, "", 1, "L", false, 0, "")
	pdf.SetDrawColor(48, 64, 82)
	pdf.Line(18, pdf.GetY(), 192, pdf.GetY())
	pdf.Ln(3)
}

func addArticle(pdf *fpdf.Fpdf, article teachingArticle) {
	if pdf.GetY() > 238 {
		pdf.AddPage()
	}
	pdf.SetTextColor(232, 238, 246)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.MultiCell(0, 6, article.Title, "", "L", false)
	pdf.SetTextColor(55, 169, 243)
	pdf.SetFont("Courier", "", 8)
	pdf.MultiCell(0, 4.5, article.Source, "", "L", false)
	addParagraph(pdf, article.Body)
	pdf.Ln(2)
}

func addParagraph(pdf *fpdf.Fpdf, text string) {
	pdf.SetTextColor(202, 211, 222)
	pdf.SetFont("Helvetica", "", 10.5)
	pdf.MultiCell(0, 5.4, asciiClean(text), "", "L", false)
	pdf.Ln(1.5)
}

func addBullet(pdf *fpdf.Fpdf, text string) {
	pdf.SetTextColor(202, 211, 222)
	pdf.SetFont("Helvetica", "", 10.5)
	pdf.MultiCell(0, 5.4, "- "+asciiClean(text), "", "L", false)
}

func addMetricRow(pdf *fpdf.Fpdf, label, value string) {
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(152, 162, 176)
	pdf.CellFormat(70, 7, asciiClean(label), "", 0, "L", false, 0, "")
	pdf.SetFont("Courier", "B", 11)
	pdf.SetTextColor(232, 238, 246)
	pdf.CellFormat(0, 7, asciiClean(value), "", 1, "L", false, 0, "")
}

func addFinding(pdf *fpdf.Fpdf, index int, finding analyzer.Finding) {
	if pdf.GetY() > 244 {
		pdf.AddPage()
	}
	pdf.SetTextColor(232, 238, 246)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.MultiCell(0, 6, fmt.Sprintf("%d. %s", index, asciiClean(finding.Title)), "", "L", false)
	pdf.SetTextColor(255, 170, 92)
	pdf.SetFont("Courier", "B", 9)
	pdf.CellFormat(0, 5, asciiClean(finding.Severity+" / "+finding.CostImpact), "", 1, "L", false, 0, "")
	if evidence := findingEvidence(finding.Evidence); evidence != "" {
		addParagraph(pdf, "Evidence: "+evidence)
	}
	if finding.Recommendation != "" {
		addParagraph(pdf, "Immediate move: "+finding.Recommendation)
	}
	pdf.Ln(1)
}

func outputPDF(pdf *fpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func asciiClean(value string) string {
	replacements := strings.NewReplacer(
		"—", "-",
		"–", "-",
		"“", `"`,
		"”", `"`,
		"‘", "'",
		"’", "'",
		"→", "->",
		"≤", "<=",
		"≥", ">=",
	)
	return replacements.Replace(value)
}

const voucherAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func randomVoucherCode() (string, error) {
	var b strings.Builder
	b.Grow(6)
	max := big.NewInt(int64(len(voucherAlphabet)))
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(voucherAlphabet[n.Int64()])
	}
	return b.String(), nil
}

func readZipEntry(data []byte, name string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, os.ErrNotExist
}
