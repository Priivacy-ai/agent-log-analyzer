package main

import (
	"bytes"
	"errors"
	"fmt"
	htmlstd "html"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type reportPageData struct {
	Report      analyzer.Report
	Job         app.Job
	ArtifactURL string
	StatusText  string
	ReportToken string
}

func reportPageHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		job, err := store.GetJob(r.PathValue("id"))
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job id")
			return
		}
		if !tokenMatches(job.ReportTokenHash, r.PathValue("token")) {
			writeError(w, http.StatusUnauthorized, "invalid report token")
			return
		}
		if job.Status != app.StatusCompleted {
			renderReportStatusPage(w, job)
			return
		}
		report, err := store.GetReport(job.ID)
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "report not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid report id")
			return
		}
		artifactURL := ""
		if jobAllowsPluginArtifact(job) {
			artifactURL = publicBaseURL(r) + "/api/public-artifacts/" + job.ID + "/" + r.PathValue("token") + "/plugin.zip"
		}
		renderReportHTML(w, reportPageData{
			Report:      report,
			Job:         job,
			ArtifactURL: artifactURL,
			StatusText:  "This report is visible for 15 minutes.",
			ReportToken: r.PathValue("token"),
		})
	}
}

func renderReportStatusPage(w http.ResponseWriter, job app.Job) {
	renderReportHTML(w, reportPageData{
		Job:        job,
		StatusText: fmt.Sprintf("This report is visible for 15 minutes after completion. Current status: %s.", job.Status),
	})
}

func renderReportHTML(w http.ResponseWriter, data reportPageData) {
	var body bytes.Buffer
	if err := reportHTMLTemplate.Execute(&body, data); err != nil {
		writeError(w, http.StatusInternalServerError, "could not render report")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body.Bytes())
}

var reportHTMLTemplate = template.Must(template.New("report").Funcs(template.FuncMap{
	"add":                   func(a, b int) int { return a + b },
	"boolText":              boolText,
	"bucketValue":           bucketValue,
	"findingEvidence":       findingEvidence,
	"findingsBubbleChart":   findingsBubbleChartHTML,
	"helpTip":               helpTip,
	"join":                  joinStrings,
	"mapLines":              mapLines,
	"recommendationLabel":   recommendationLabel,
	"recommendationName":    recommendationName,
	"recommendationSignals": recommendationSignals,
	"recommendationURL":     recommendationURL,
	"savingsRange":          savingsRange,
	"sourceLogLabel":        sourceLogLabel,
	"timelineChart":         timelineChartHTML,
}).Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    {{if ne .Job.Status "completed"}}<meta http-equiv="refresh" content="2" />{{end}}
    <title>Agent Analyzer Report</title>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml" />
    <link rel="stylesheet" href="/vendor/tippy/tippy.css" />
    <link rel="stylesheet" href="/styles.css" />
  </head>
  <body>
    <main class="shell">
      <section class="report" id="report">
        <p class="expiry" id="report-status">{{.StatusText}}</p>
        {{if eq .Job.Status "completed"}}
        <div class="score">
          <span id="score">{{.Report.Score}}</span>
          <small>efficiency score {{helpTip "Heuristic score from deterministic findings: repeated reads, retry depth, context-growth events, tool-output share, and normalized signal findings. It is a profiler score, not a model-quality grade."}}</small>
        </div>
        <div>
          <h2>Estimated Waste {{helpTip "Avoidable spend is a heuristic range derived from the efficiency score and detected waste patterns. It is intended to rank severity and prompt investigation, not to reproduce provider billing."}}</h2>
          <p id="waste">{{.Report.EstimatedWaste.Low}}-{{.Report.EstimatedWaste.High}}% avoidable token spend</p>
          <p class="command-note">Analyzed token volume: {{.Report.Metrics.EstimatedTokens}} estimated input/output tokens; {{.Report.Metrics.ToolOutputTokens}} estimated from tool output. {{helpTip "Accuracy depends on the source log. When native usage fields exist, we use them. Otherwise we estimate roughly one token per four characters. Tool-output volume is derived from tool-result payload size and similar estimates. This is directional, not invoice-grade accounting."}}</p>
        </div>
        <div class="problem-section">
          <h2>Top Problems {{helpTip "Bubble size is representative impact, not a precise measurement. Tool-output and cache issues use token fields when available; rereads, retries, and context spikes use bounded count-scaled estimates so the graph shows relative severity without exposing raw content."}}</h2>
          {{findingsBubbleChart .Report}}
        </div>
        {{if not .Report.SourceReports}}
        <div>
          <h2>Session Timeline {{helpTip "Timeline points are sampled from parsed log rows, usually every ten rows plus the final row. Token height is estimated context/token volume at that point, not a guaranteed exact provider context window."}}</h2>
          <p id="timeline-caption" class="timeline-caption">Estimated context/token volume by parsed turn. Highlighted area shows potential savings.</p>
          {{timelineChart .Report.Timeline .Report.EstimatedWaste}}
        </div>
        {{end}}
        {{if .Report.SourceReports}}
        <section class="source-reports">
          <h2>Agent Logs Analyzed {{helpTip "Each section is built from logs parsed locally for that supported agent source. The top-level report is an aggregate; source timelines are the only charts that represent parsed turns."}}</h2>
          {{range .Report.SourceReports}}
          <article class="source-report">
            <header class="source-report-header">
              <div>
                <h3>{{.SourceLabel}}</h3>
                <p>{{sourceLogLabel .LogCount}} analyzed locally</p>
              </div>
              <div class="source-score">
                <strong>{{.Score}}</strong>
                <span>efficiency</span>
              </div>
            </header>
            <div class="source-report-grid">
              <div>
                <h4>Waste {{helpTip "Source-level waste is computed with the same deterministic heuristic as the full report, using only this source's parsed logs."}}</h4>
                <p>{{.EstimatedWaste.Low}}-{{.EstimatedWaste.High}}% avoidable token spend</p>
                <p class="command-note">{{.Metrics.EstimatedTokens}} estimated input/output tokens; {{.Metrics.ToolOutputTokens}} estimated tool-output tokens; {{.Metrics.Rereads}} rereads; {{.Metrics.FailedCommands}} retries. {{helpTip "Token counts use native usage fields when available and fall back to rough text-size estimates. Rereads and retries are deterministic pattern detections over sanitized local parse output."}}</p>
              </div>
              <div>
                <h4>Top Problems</h4>
                <ol class="source-findings">
                  {{range .Findings}}
                  <li><strong>{{.Title}}</strong><span>{{.Severity}} - {{.CostImpact}}</span></li>
                  {{else}}
                  <li>No major deterministic problems detected.</li>
                  {{end}}
                </ol>
              </div>
            </div>
            {{if .Timeline}}
            <p class="timeline-caption">Estimated context/token volume by parsed turn for this source. Highlighted area shows potential savings. {{helpTip "This source timeline is based on parsed log-row order, sampled for readability. It should be used to spot growth shape and spikes, not exact provider billing."}}</p>
            {{timelineChart .Timeline .EstimatedWaste}}
            {{else}}
            <p class="timeline-caption">Per-turn timeline unavailable for this aggregated source. Totals above cover {{sourceLogLabel .LogCount}}.</p>
            {{end}}
          </article>
          {{end}}
        </section>
        {{end}}
        <div>
          <h2>Suggested Immediate Fixes {{helpTip "Fixes are generated from deterministic finding IDs and bounded evidence, not from raw prompts or an LLM reading your transcript."}}</h2>
          <ul id="fixes">
            {{range .Report.ImmediateFixes}}<li>{{.}}</li>{{else}}<li>No immediate fixes were generated.</li>{{end}}
          </ul>
        </div>
        {{if .Report.Recommendation}}
        <section id="recommendation-section" class="intel-section">
          <h2>Next-best recommendation {{helpTip "Recommendation ranking comes from allowlisted tool metadata and deterministic signals such as tool-output bloat, retrieval friction, usage visibility, and MCP/skill utilization. Unknown private names are not echoed."}}</h2>
          {{with .Report.Recommendation.Primary}}{{template "recommendation" .}}{{end}}
          {{with .Report.Recommendation.Secondary}}{{template "recommendation" .}}{{end}}
          {{if and (not .Report.Recommendation.Primary) (not .Report.Recommendation.Secondary)}}
          <p id="recommendation-empty">No high-priority recommendation detected from current signals.</p>
          {{end}}
        </section>
        {{end}}
        <section id="workflow-fingerprints" class="intel-section">
          <h2>Workflow Fingerprints {{helpTip "Fingerprints are bounded detections for known public tools. Evidence comes from verified signatures such as command names, config markers, MCP namespaces, package manifests, and optional CLI presence/version buckets. Private names are counted, not shown."}}</h2>
          <ol id="workflow-fingerprints-list">
            {{range .Report.Ecosystem.WorkflowFingerprints}}
            <li>{{.ID}} — {{.Confidence}} confidence; sources: {{join .Sources}}; evidence: {{.EvidenceCount}}{{if .Active}}; active{{end}}{{if .Installed}}; installed{{end}}{{if .VersionBucket}}; version: {{.VersionBucket}}{{end}}</li>
            {{else}}
            <li>No known workflow fingerprints detected.</li>
            {{end}}
          </ol>
        </section>
        <section id="tooling-utilization" class="intel-section">
          <h2>MCP &amp; Skill Utilization {{helpTip "Utilization compares bounded exposed MCP/skill context against observed calls/executions. The warning band is a heuristic for context bloat risk, not a claim that a specific private tool is bad."}}</h2>
          <div id="tooling-utilization-rows">
            <p><strong>MCP:</strong> {{.Report.Ecosystem.ToolingUtilization.MCP.WarningBand}} band; {{.Report.Ecosystem.ToolingUtilization.MCP.UtilizationRatioPct}}% utilization; exposed servers {{.Report.Ecosystem.ToolingUtilization.MCP.ServerCountBucket}}; context {{.Report.Ecosystem.ToolingUtilization.MCP.ContextTokenBucket}}; calls {{.Report.Ecosystem.ToolingUtilization.MCP.CallCount}}; known called: {{join .Report.Ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs}}; unknown called count: {{.Report.Ecosystem.ToolingUtilization.MCP.UniqueUnknownCalledCount}}</p>
            <p><strong>Skills:</strong> {{.Report.Ecosystem.ToolingUtilization.Skill.WarningBand}} band; {{.Report.Ecosystem.ToolingUtilization.Skill.UtilizationRatioPct}}% utilization; exposed {{.Report.Ecosystem.ToolingUtilization.Skill.ExposedCountBucket}}; context {{.Report.Ecosystem.ToolingUtilization.Skill.ContextTokenBucket}}; executions {{.Report.Ecosystem.ToolingUtilization.Skill.ExecutedCount}}; known executed: {{join .Report.Ecosystem.ToolingUtilization.Skill.KnownExecutedIDs}}; unknown executed count: {{.Report.Ecosystem.ToolingUtilization.Skill.UnknownExecutedCount}}</p>
          </div>
        </section>
        <div>
          <h2>Ecosystem {{helpTip "Ecosystem fields are bounded categories and allowlisted public names. Unknown/private MCPs, skills, plugins, and tools are counted without exposing their raw names."}}</h2>
          <pre id="ecosystem">Client: {{.Report.Ecosystem.Client}}
Coding agents: {{join .Report.Ecosystem.CodingAgents}}
OS: {{.Report.Ecosystem.OperatingSystem}}
Shell: {{.Report.Ecosystem.Shell}}
Workflow frameworks: {{join .Report.Ecosystem.WorkflowFrameworks}}
MCPs: {{join .Report.Ecosystem.MCPServersKnown}}
Unknown MCP count: {{.Report.Ecosystem.UnknownMCPServerCount}}
Skills: {{join .Report.Ecosystem.KnownSkills}}
Unknown skill count: {{.Report.Ecosystem.UnknownSkillCount}}
Plugins: {{join .Report.Ecosystem.KnownPlugins}}
Unknown plugin count: {{.Report.Ecosystem.UnknownPluginCount}}
Package managers: {{join .Report.Ecosystem.PackageManagers}}
Version control: {{.Report.Ecosystem.VersionControl}}</pre>
        </div>
        <div>
          <h2>Security Receipt {{helpTip "The public flow analyzes locally and uploads sanitized report JSON. This receipt records the report's own privacy flags and redaction counts; it is not a third-party audit."}}</h2>
          <pre id="receipt">Raw transcript sent to LLM: {{boolText .Report.SecurityReceipt.RawTranscriptSentToLLM}}
Outbound during analysis: {{boolText .Report.SecurityReceipt.OutboundDuringAnalysis}}
Secrets redacted: {{.Report.SecurityReceipt.SecretsRedacted}}
Raw log TTL: {{.Report.SecurityReceipt.RawLogTTL}}
Redactions:
{{mapLines .Report.Redactions}}</pre>
        </div>
        <div class="upsell">
          <h2>Generate the optimization plugin from a full scan</h2>
          <p>Run the deeper local scan across up to 100 recent logs per supported agent source, then install a generated optimization pack with vetted context, retrieval, telemetry, and CLAUDE.md recommendations.</p>
          {{if .ArtifactURL}}
          <p>Optimization plugin artifact: <a href="{{.ArtifactURL}}">{{.ArtifactURL}}</a></p>
          {{else}}
          <p>For launch testing this is email-confirmed and free. Confirm your email and we will send a one-line NPX command that authorizes the full local scan. Raw transcripts still stay on your machine; only sanitized aggregate JSON is uploaded.</p>
          <form class="email-unlock-form" method="post" action="/api/email-unlocks">
            <input type="hidden" name="source_report_job_id" value="{{.Job.ID}}">
            <input type="hidden" name="source_report_token" value="{{.ReportToken}}">
            <label>Email for full-scan command
              <input type="email" name="email" autocomplete="email" required placeholder="you@example.com">
            </label>
            <label class="checkbox-row">
              <input type="checkbox" name="marketing_opt_in" value="1">
              <span>Also send me occasional updates about agentic coding products.</span>
            </label>
            <button type="submit">Email me the full-scan command</button>
          </form>
          {{end}}
        </div>
        {{else}}
        <div class="score">
          <span id="score">--</span>
          <small>efficiency score</small>
        </div>
        <p>This page will show the deterministic report after analysis completes. Browser clients can poll for completion; non-JS clients should retry this URL.</p>
        {{end}}
      </section>
    </main>
    <script src="/vendor/tippy/popper.min.js"></script>
    <script src="/vendor/tippy/tippy-bundle.umd.min.js"></script>
    <script src="/tooltips.js"></script>
  </body>
</html>
{{define "recommendation"}}
<div class="recommendation-card">
  <div class="recommendation-tool">{{recommendationName .}}</div>
  {{with recommendationURL .}}<a class="recommendation-source" href="{{.}}" rel="noopener noreferrer">{{.}}</a>{{end}}
  <div class="recommendation-meta">
    <span class="recommendation-reason">{{.Reason}}</span>
    <span class="recommendation-confidence">{{.Confidence}}</span>
    <span class="recommendation-risk">{{.RiskLevel}} risk</span>
    <span class="recommendation-policy">{{.InstallPolicy}}</span>
  </div>
  <p>{{recommendationLabel .}}</p>
  <ul class="recommendation-signals">
    {{range .SignalIDs}}<li class="recommendation-signal">{{.}}</li>{{end}}
  </ul>
</div>
{{end}}`))

func recommendationName(rec analyzer.TokenSavingRecommendation) string {
	if rec.PrimaryToolName != "" {
		return rec.PrimaryToolName
	}
	if rec.PrimaryToolID != "" {
		return string(rec.PrimaryToolID)
	}
	return recommendationLabel(rec)
}

func recommendationURL(rec analyzer.TokenSavingRecommendation) string {
	if strings.HasPrefix(rec.PrimaryToolURL, "https://") {
		return rec.PrimaryToolURL
	}
	return ""
}

func recommendationLabel(rec analyzer.TokenSavingRecommendation) string {
	for _, signal := range rec.SignalIDs {
		switch signal {
		case analyzer.SignalMCPSkillBloat:
			return "Prune / lazy-load MCPs and skills"
		case analyzer.SignalRetryLoop, analyzer.SignalContextGrowthSpikes:
			return "Session hygiene audit"
		}
	}
	if rec.PrimaryToolID == "" {
		return "Tooling recommendation"
	}
	return "Install or configure this tool only after reviewing the source URL."
}

func recommendationSignals(rec analyzer.TokenSavingRecommendation) string {
	signals := make([]string, 0, len(rec.SignalIDs))
	for _, signal := range rec.SignalIDs {
		signals = append(signals, string(signal))
	}
	return joinStrings(signals)
}

func findingEvidence(e analyzer.FindingEvidence) string {
	var parts []string
	if e.Count > 0 {
		parts = append(parts, fmt.Sprintf("count: %d", e.Count))
	}
	if e.TokenShare > 0 {
		parts = append(parts, fmt.Sprintf("token share: %d%%", e.TokenShare))
	}
	if len(e.TopFiles) > 0 {
		parts = append(parts, "top files: "+joinStrings(e.TopFiles))
	}
	if e.Description != "" {
		parts = append(parts, e.Description)
	}
	if len(parts) == 0 {
		return "deterministic evidence recorded"
	}
	return strings.Join(parts, " | ")
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return "none detected"
	}
	return strings.Join(values, ", ")
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func bucketValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func savingsRange(tokens int, waste analyzer.WasteRange) string {
	low := tokens * waste.Low / 100
	high := tokens * waste.High / 100
	return fmt.Sprintf("%d-%d", low, high)
}

func sourceLogLabel(count int) string {
	if count == 1 {
		return "1 log"
	}
	return fmt.Sprintf("%d logs", count)
}

func helpTip(text string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<button type="button" class="help-tip" data-tippy-content="%s" aria-label="More information">?</button>`,
		htmlstd.EscapeString(text),
	))
}

func findingsBubbleChartHTML(report analyzer.Report) template.HTML {
	if len(report.Findings) == 0 {
		return template.HTML(`<div id="findings" class="problem-bubbles problem-bubbles-empty"><p>No major deterministic problems detected.</p></div>`)
	}
	estimates := make([]int, len(report.Findings))
	maxEstimate := 1
	for index, finding := range report.Findings {
		estimate := representativeProblemTokens(finding, report)
		if estimate > maxEstimate {
			maxEstimate = estimate
		}
		estimates[index] = estimate
	}
	var b strings.Builder
	b.WriteString(`<div id="findings" class="problem-bubbles" role="list" aria-label="Top problems sized by representative token impact">`)
	for index, finding := range report.Findings {
		diameter := bubbleDiameter(estimates[index], maxEstimate)
		tone := bubbleTone(finding, index)
		detail := fmt.Sprintf("%s. %s", findingEvidence(finding.Evidence), finding.Recommendation)
		fmt.Fprintf(
			&b,
			`<article class="problem-bubble problem-bubble-%s" role="listitem" style="--bubble-size:%dpx; --bubble-offset:%dpx" aria-label="%s">`,
			htmlstd.EscapeString(tone),
			diameter,
			bubbleOffset(index),
			htmlstd.EscapeString(fmt.Sprintf("%s. %s. %s representative tokens. %s", finding.Title, finding.Severity, compactNumber(estimates[index]), detail)),
		)
		fmt.Fprintf(&b, `<span class="problem-rank">%d</span>`, index+1)
		fmt.Fprintf(&b, `<strong>%s</strong>`, htmlstd.EscapeString(finding.Title))
		fmt.Fprintf(&b, `<span class="problem-meta">%s - %s</span>`, htmlstd.EscapeString(finding.Severity), htmlstd.EscapeString(finding.CostImpact))
		fmt.Fprintf(&b, `<span class="problem-impact">%s representative tokens</span>`, htmlstd.EscapeString(compactNumber(estimates[index])))
		fmt.Fprintf(&b, `<p>%s</p>`, htmlstd.EscapeString(findingEvidence(finding.Evidence)))
		fmt.Fprintf(&b, `<p>%s</p>`, htmlstd.EscapeString(finding.Recommendation))
		b.WriteString(`</article>`)
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

func representativeProblemTokens(finding analyzer.Finding, report analyzer.Report) int {
	total := report.Metrics.EstimatedTokens
	if total <= 0 {
		total = report.AnalysisSignals.InputTokens + report.AnalysisSignals.OutputTokens
	}
	if total <= 0 {
		total = 1000
	}
	if finding.Evidence.TokenShare > 0 {
		return clampRepresentativeTokens(total*finding.Evidence.TokenShare/100, total)
	}
	switch finding.ID {
	case "tool_output_bloat":
		if report.Metrics.ToolOutputTokens > 0 {
			return clampRepresentativeTokens(report.Metrics.ToolOutputTokens, total)
		}
	case "cache_invalidation_spike":
		if report.AnalysisSignals.CacheCreationTokens > 0 {
			return clampRepresentativeTokens(report.AnalysisSignals.CacheCreationTokens, total)
		}
	case "args_hashed_retry_loop":
		return percentageTokens(total, finding.Evidence.Count, 5, 34)
	case "retry_loop":
		count := finding.Evidence.Count
		if count == 0 {
			count = report.Metrics.RetryDepthMax
		}
		return percentageTokens(total, count, 5, 32)
	case "repeated_file_reads":
		count := finding.Evidence.Count
		if count == 0 {
			count = report.Metrics.Rereads
		}
		return percentageTokens(total, count, 3, 38)
	case "context_growth_spikes":
		count := finding.Evidence.Count
		if count == 0 {
			count = report.Metrics.ContextGrowthEvents
		}
		return percentageTokens(total, count, 4, 42)
	}
	wasteLow, wasteHigh := normalizedWaste(report.EstimatedWaste)
	wasteMid := (wasteLow + wasteHigh) / 2
	if wasteMid <= 0 {
		wasteMid = 12
	}
	return clampRepresentativeTokens(total*wasteMid/100, total)
}

func percentageTokens(total, count, perCountPct, maxPct int) int {
	if count <= 0 {
		count = 1
	}
	pct := count * perCountPct
	if pct > maxPct {
		pct = maxPct
	}
	if pct < 4 {
		pct = 4
	}
	return clampRepresentativeTokens(total*pct/100, total)
}

func clampRepresentativeTokens(tokens, total int) int {
	if tokens < 1 {
		return 1
	}
	limit := total
	if limit < 1 {
		limit = tokens
	}
	if tokens > limit {
		return limit
	}
	return tokens
}

func bubbleDiameter(tokens, maxTokens int) int {
	if maxTokens <= 0 {
		maxTokens = 1
	}
	ratio := float64(tokens) / float64(maxTokens)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return 170 + int(ratio*98)
}

func bubbleTone(finding analyzer.Finding, index int) string {
	switch finding.ID {
	case "tool_output_bloat", "cache_invalidation_spike":
		return "orange"
	case "repeated_file_reads", "context_growth_spikes":
		return "blue"
	case "retry_loop", "args_hashed_retry_loop":
		return "green"
	default:
		tones := []string{"orange", "blue", "green"}
		return tones[index%len(tones)]
	}
}

func bubbleOffset(index int) int {
	offsets := []int{0, 28, -8, 18, -18, 10}
	return offsets[index%len(offsets)]
}

func timelineChartHTML(points []analyzer.TimelinePoint, waste analyzer.WasteRange) template.HTML {
	if len(points) == 0 {
		return template.HTML(`<div class="timeline-empty">No timeline points were available.</div>`)
	}
	visible := points
	if len(visible) > 60 {
		visible = visible[len(visible)-60:]
	}
	maxTokens := 1
	for _, point := range visible {
		if point.EstimatedTokens > maxTokens {
			maxTokens = point.EstimatedTokens
		}
	}
	wasteLow, wasteHigh := normalizedWaste(waste)
	savingsPct := (wasteLow + wasteHigh) / 2
	if savingsPct > 95 {
		savingsPct = 95
	}
	firstTurn := visible[0].Turn
	lastTurn := visible[len(visible)-1].Turn
	var b strings.Builder
	fmt.Fprintf(&b, `<div class="timeline-legend" aria-hidden="true"><span class="timeline-legend-item"><span class="timeline-legend-swatch timeline-legend-observed"></span>estimated volume</span><span class="timeline-legend-item"><span class="timeline-legend-swatch timeline-legend-savings"></span>%d-%d%% potential savings</span></div>`, wasteLow, wasteHigh)
	fmt.Fprintf(&b, `<div class="timeline-frame"><div class="timeline-y-axis" aria-hidden="true"><span>%s tokens</span><span>0</span></div>`, compactNumber(maxTokens))
	fmt.Fprintf(&b, `<div class="timeline" role="img" aria-label="%s">`, htmlstd.EscapeString(fmt.Sprintf("Session timeline from turn %d to turn %d; maximum %d estimated token volume; potential savings range %d-%d percent.", firstTurn, lastTurn, maxTokens, wasteLow, wasteHigh)))
	for _, point := range visible {
		height := point.EstimatedTokens * 100 / maxTokens
		if height < 4 {
			height = 4
		}
		savedLow := point.EstimatedTokens * wasteLow / 100
		savedHigh := point.EstimatedTokens * wasteHigh / 100
		tooltip := fmt.Sprintf("turn %d | %s estimated token volume | %s-%s estimated potential savings | %s estimated tool-output tokens | %s rereads | %s retries",
			point.Turn,
			groupNumber(point.EstimatedTokens),
			groupNumber(savedLow),
			groupNumber(savedHigh),
			groupNumber(point.ToolTokens),
			groupNumber(point.Rereads),
			groupNumber(point.Retries),
		)
		escapedTooltip := htmlstd.EscapeString(tooltip)
		fmt.Fprintf(&b, `<span class="timeline-bar" style="height:%d%%" data-tooltip="%s" tabindex="0" role="img" aria-label="%s">`, height, escapedTooltip, escapedTooltip)
		if savingsPct > 0 {
			fmt.Fprintf(&b, `<span class="timeline-savings" style="height:%d%%" aria-hidden="true"></span>`, savingsPct)
		}
		b.WriteString(`</span>`)
	}
	b.WriteString(`</div></div>`)
	b.WriteString(timelineAxisHTML(visible))
	return template.HTML(b.String())
}

func normalizedWaste(waste analyzer.WasteRange) (int, int) {
	low := clampPercent(waste.Low)
	high := clampPercent(waste.High)
	if low > high {
		low, high = high, low
	}
	return low, high
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func timelineAxisHTML(points []analyzer.TimelinePoint) string {
	if len(points) == 0 {
		return ""
	}
	type tick struct {
		class string
		label string
	}
	candidates := []tick{
		{class: "first", label: fmt.Sprintf("turn %d", points[0].Turn)},
		{class: "middle", label: fmt.Sprintf("turn %d", points[(len(points)-1)/2].Turn)},
		{class: "last", label: fmt.Sprintf("turn %d", points[len(points)-1].Turn)},
	}
	seen := map[string]bool{}
	var b strings.Builder
	b.WriteString(`<div class="timeline-x-axis" aria-hidden="true">`)
	for _, candidate := range candidates {
		if seen[candidate.label] {
			continue
		}
		seen[candidate.label] = true
		fmt.Fprintf(&b, `<span class="timeline-tick timeline-tick-%s">%s</span>`, candidate.class, htmlstd.EscapeString(candidate.label))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func compactNumber(value int) string {
	if value >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(value)/1000000)
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fK", float64(value)/1000)
	}
	return fmt.Sprintf("%d", value)
}

func groupNumber(value int) string {
	text := fmt.Sprintf("%d", value)
	if len(text) <= 3 {
		return text
	}
	var b strings.Builder
	prefix := len(text) % 3
	if prefix == 0 {
		prefix = 3
	}
	b.WriteString(text[:prefix])
	for i := prefix; i < len(text); i += 3 {
		b.WriteByte(',')
		b.WriteString(text[i : i+3])
	}
	return b.String()
}

func mapLines(values map[string]int) string {
	if len(values) == 0 {
		return "none\n"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "- %s: %d\n", key, values[key])
	}
	return b.String()
}
