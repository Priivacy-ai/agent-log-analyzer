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
	"actionPlan":            actionPlanHTML,
	"boolText":              boolText,
	"bucketValue":           bucketValue,
	"ecosystemPanel":        ecosystemPanelHTML,
	"findingEvidence":       findingEvidence,
	"findingsBubbleChart":   findingsBubbleChartHTML,
	"formatInt":             formatInt,
	"formatTokens":          formatTokens,
	"helpTip":               helpTip,
	"join":                  joinStrings,
	"logRefs":               logRefsHTML,
	"mapLines":              mapLines,
	"recommendationLabel":   recommendationLabel,
	"recommendationName":    recommendationName,
	"recommendationSignals": recommendationSignals,
	"recommendationURL":     recommendationURL,
	"receiptPanel":          receiptPanelHTML,
	"savingsRange":          savingsRange,
	"sourceLogLabel":        sourceLogLabel,
	"timelineChart":         timelineChartHTML,
	"toolingUtilization":    toolingUtilizationHTML,
	"workflowFingerprints":  workflowFingerprintsHTML,
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
        <p class="expiry" id="report-status">{{.StatusText}} {{helpTip "Why only 15 minutes? The hosted report is short-lived because even sanitized workflow metadata can be sensitive. Raw logs are not uploaded; rerun the local command to regenerate a fresh report link."}}</p>
        {{if eq .Job.Status "completed"}}
        <div class="zero-token-banner">
          <strong>0 model tokens used to generate this report.</strong>
          <span>Agent Analyzer uses deterministic local parsing and server-side rendering, not an LLM reading your logs. {{helpTip "How can report generation cost zero model tokens? The CLI parses local logs with deterministic Go code, writes sanitized JSON, and the hosted page renders that JSON. No raw transcript is sent to a model and no model call is made to produce this report."}}</span>
        </div>
        <div class="score">
          <span id="score">{{.Report.Score}}</span>
          <small>efficiency score {{helpTip "What does this score mean? It is a heuristic score from deterministic findings: repeated reads, retry depth, context-growth events, tool-output share, and normalized signal findings. It is a profiler score, not a model-quality grade."}}</small>
        </div>
        <div>
          <h2>Estimated Waste {{helpTip "How is waste estimated? Avoidable spend is a heuristic range derived from the efficiency score and detected waste patterns. It is intended to rank severity and prompt investigation, not to reproduce provider billing."}}</h2>
          <p id="waste">{{.Report.EstimatedWaste.Low}}-{{.Report.EstimatedWaste.High}}% avoidable token spend</p>
          <p class="command-note">Analyzed token volume: {{formatTokens .Report.Metrics.EstimatedTokens}} estimated input/output tokens; {{formatTokens .Report.Metrics.ToolOutputTokens}} estimated from tool output. {{helpTip "What is counted here? Accuracy depends on the source log. When native usage fields exist, we use them. Otherwise we estimate roughly one token per four characters. Tool-output volume is derived from tool-result payload size and similar estimates. This is directional, not invoice-grade accounting."}}</p>
          <div class="report-cta-row" aria-label="Report actions">
            <a class="report-primary-cta" href="#email-unlock">Get the optimization plugin</a>
            <a class="report-secondary-cta" href="#email-unlock-section">Skip to full scan</a>
          </div>
        </div>
        <div class="problem-section">
          <h2>Top Problems {{helpTip "Why are these bubbles different sizes? Bubble size is representative impact, not a precise measurement. Tool-output and cache issues use token fields when available; rereads, retries, and context spikes use bounded count-scaled estimates so the graph shows relative severity without exposing raw content."}}</h2>
          {{findingsBubbleChart .Report}}
        </div>
        {{if not .Report.SourceReports}}
        <div>
          <h2>Session Timeline {{helpTip "What are the turns? Timeline points are sampled from parsed log rows, usually every ten rows plus the final row. Token height is estimated context/token volume at that point, not a guaranteed exact provider context window."}}</h2>
          <p id="timeline-caption" class="timeline-caption">Estimated context/token volume by parsed turn. Green overlay = estimated tokens you can save.</p>
          {{timelineChart .Report.Timeline .Report.EstimatedWaste}}
        </div>
        {{end}}
        {{if .Report.SourceReports}}
        <section class="source-reports">
          <h2>Agent Logs Analyzed {{helpTip "Which sessions were analyzed? Each section is built from logs parsed locally for that supported agent source. Full local paths and raw session IDs are intentionally not uploaded; the report shows privacy-safe local references instead."}}</h2>
          {{range .Report.SourceReports}}
          <article class="source-report">
            <header class="source-report-header">
              <div>
                <h3>{{.SourceLabel}}</h3>
                <p>{{sourceLogLabel .LogCount}} analyzed locally</p>
                {{logRefs .LogRefs}}
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
                <p class="command-note">{{formatTokens .Metrics.EstimatedTokens}} estimated input/output tokens; {{formatTokens .Metrics.ToolOutputTokens}} estimated tool-output tokens; {{formatInt .Metrics.Rereads}} rereads; {{formatInt .Metrics.FailedCommands}} retries. {{helpTip "What is counted for this agent? Token counts use native usage fields when available and fall back to rough text-size estimates. Rereads and retries are deterministic pattern detections over sanitized local parse output."}}</p>
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
            <p class="timeline-caption">Estimated context/token volume by parsed turn for this source. Green overlay = estimated tokens you can save. {{helpTip "How should I read this chart? This source timeline is based on parsed log-row order, sampled for readability. It should be used to spot growth shape and spikes, not exact provider billing."}}</p>
            {{timelineChart .Timeline .EstimatedWaste}}
            {{else}}
            <p class="timeline-caption">Per-turn timeline unavailable for this aggregated source. Totals above cover {{sourceLogLabel .LogCount}}.</p>
            {{end}}
          </article>
          {{end}}
        </section>
        {{end}}
        <section class="plugin-pitch" id="plugin-pitch">
          <div>
            <p class="eyebrow">generated remediation</p>
            <h2>Do the quick fixes now. Let the plugin enforce them next. {{helpTip "Where do these fixes come from? Fixes are generated from deterministic finding IDs and bounded evidence, not from raw prompts or an LLM reading your transcript. The full scan turns those findings into a generated plugin artifact and vetted setup instructions."}}</h2>
            <p>These are the manual moves that will reduce waste immediately. If you want this to become an operating habit instead of another checklist, run the full scan and get the generated plugin: it turns these patterns into Claude-facing instructions, context hygiene rules, retrieval guidance, and setup recommendations.</p>
            <ul class="plugin-benefits">
              <li>Session hygiene nudges before context contamination gets expensive.</li>
              <li>Retrieval and code-intelligence recommendations tuned to your actual reread patterns.</li>
              <li>CLAUDE.md trimming, hierarchy, and workflow guidance generated from the full scan.</li>
              <li>MCP, skill, and plugin bloat warnings converted into setup recommendations.</li>
            </ul>
            <a class="plugin-cta" href="#email-unlock">Generate my optimization plugin</a>
          </div>
          <div class="plugin-fixes-card">
            <h3>Do these now</h3>
            <p class="command-note">Copy the AGENTS.md line into the relevant repo. The hosted report cannot safely append to local files; the generated plugin can package these rules locally after the full scan.</p>
            {{actionPlan .Report}}
          </div>
        </section>
        {{if .Report.Recommendation}}
        <section id="recommendation-section" class="intel-section">
          <h2>Next-best recommendation {{helpTip "Why this recommendation? Ranking comes from public allowlisted tool metadata and deterministic signals such as tool-output bloat, retrieval friction, usage visibility, and MCP/skill utilization. Unknown private names are not echoed."}}</h2>
          <p class="section-note">Recommendations come from a public vetted allowlist. <a href="/allowed-tools.html">Review the allowlist and source URLs.</a></p>
          {{with .Report.Recommendation.Primary}}{{template "recommendation" .}}{{end}}
          {{with .Report.Recommendation.Secondary}}{{template "recommendation" .}}{{end}}
          {{if and (not .Report.Recommendation.Primary) (not .Report.Recommendation.Secondary)}}
          <p id="recommendation-empty">No high-priority recommendation detected from current signals.</p>
          {{end}}
        </section>
        {{end}}
        <section id="workflow-fingerprints" class="intel-section">
          <h2>Workflow Fingerprints {{helpTip "Fingerprints are bounded detections for known public tools. Evidence comes from verified signatures such as command names, config markers, MCP namespaces, package manifests, and optional CLI presence/version buckets. Private names are counted, not shown."}}</h2>
          {{workflowFingerprints .Report.Ecosystem.WorkflowFingerprints}}
        </section>
        <section id="tooling-utilization" class="intel-section">
          <h2>MCP &amp; Skill Utilization {{helpTip "Utilization compares bounded exposed MCP/skill context against observed calls/executions. The warning band is a heuristic for context bloat risk, not a claim that a specific private tool is bad."}}</h2>
          {{toolingUtilization .Report.Ecosystem.ToolingUtilization}}
        </section>
        <section class="evidence-grid">
          <div class="evidence-card ecosystem-card">
            <h2>Ecosystem {{helpTip "Ecosystem fields are bounded categories and allowlisted public names. Unknown/private MCPs, skills, plugins, and tools are counted without exposing their raw names."}}</h2>
            {{ecosystemPanel .Report}}
          </div>
          <div class="evidence-card receipt-card">
            <h2>Security Receipt {{helpTip "What happened to my raw logs? The public flow analyzes and redacts locally, then uploads sanitized report JSON. This receipt records the report's own privacy flags and local redaction counts; it is not a third-party audit."}}</h2>
            {{receiptPanel .Report}}
          </div>
        </section>
        <div class="upsell" id="email-unlock-section">
          <div class="upsell-copy">
          <p class="eyebrow">free launch unlock</p>
          <h2>Generate the optimization plugin from a full scan</h2>
          <p class="upsell-lede">Run the deeper local scan across up to 100 recent logs per supported agent source. We turn the sanitized aggregate into a generated optimization pack for session hygiene, retrieval, telemetry, and CLAUDE.md cleanup.</p>
          <ul class="upsell-proof">
            <li>Raw transcripts stay local.</li>
            <li>Email confirmation unlocks the second NPX command.</li>
            <li>The plugin and full report use the same short-lived report boundary.</li>
          </ul>
          </div>
          <div class="upsell-action">
          {{if .ArtifactURL}}
          <p>Optimization plugin artifact: <a href="{{.ArtifactURL}}">{{.ArtifactURL}}</a></p>
          {{else}}
          <p>For launch testing this is email-confirmed and free. We use your email to confirm you own the address, send the one-line full-scan NPX command, and send plugin retrieval instructions. Raw transcripts still stay on your machine; only sanitized aggregate JSON is uploaded. Product updates are sent only if you opt in below.</p>
          <form id="email-unlock" class="email-unlock-form" method="post" action="/api/email-unlocks">
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
    <script src="/report-actions.js"></script>
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

type actionCopy struct {
	Title      string
	Now        string
	Why        string
	AgentsLine string
	Plugin     string
}

func actionPlanHTML(report analyzer.Report) template.HTML {
	var b strings.Builder
	b.WriteString(`<ul id="fixes" class="action-list">`)
	if len(report.Findings) > 0 {
		limit := len(report.Findings)
		if limit > 4 {
			limit = 4
		}
		for _, finding := range report.Findings[:limit] {
			actionItemHTML(&b, actionForFinding(finding))
		}
		b.WriteString(`</ul>`)
		return template.HTML(b.String())
	}
	if len(report.ImmediateFixes) > 0 {
		limit := len(report.ImmediateFixes)
		if limit > 4 {
			limit = 4
		}
		for _, fix := range report.ImmediateFixes[:limit] {
			actionItemHTML(&b, actionCopy{
				Title:  "Apply the detected fix",
				Now:    fix,
				Plugin: "The full scan turns recurring fixes into a generated plugin instead of a one-off note.",
			})
		}
		b.WriteString(`</ul>`)
		return template.HTML(b.String())
	}
	actionItemHTML(&b, actionCopy{
		Title:  "No urgent manual fix detected",
		Now:    "Keep sessions scoped and avoid pasting unnecessary tool output.",
		Plugin: "Still run the full scan if you want plugin guidance across more sessions, tools, and projects.",
	})
	b.WriteString(`</ul>`)
	return template.HTML(b.String())
}

func actionItemHTML(b *strings.Builder, action actionCopy) {
	fmt.Fprintf(
		b,
		`<li class="action-item"><strong>%s</strong><span><b>Do now:</b> %s</span><span><b>Why:</b> %s</span><code>%s</code><button type="button" class="copy-agents-line" data-copy="%s">Copy AGENTS.md line</button><em>%s</em></li>`,
		htmlstd.EscapeString(action.Title),
		htmlstd.EscapeString(action.Now),
		htmlstd.EscapeString(defaultString(action.Why, "deterministic evidence recorded")),
		htmlstd.EscapeString(defaultString(action.AgentsLine, "Keep agent sessions scoped and avoid unnecessary context.")),
		htmlstd.EscapeString(defaultString(action.AgentsLine, "Keep agent sessions scoped and avoid unnecessary context.")),
		htmlstd.EscapeString(action.Plugin),
	)
}

func actionForFinding(finding analyzer.Finding) actionCopy {
	switch finding.ID {
	case "repeated_file_reads":
		return actionCopy{
			Title:      "Stop rereading files blindly",
			Now:        "Before another broad read, name the exact file or symbol and ask the agent to summarize only what changed since the last read.",
			Why:        findingEvidence(finding.Evidence),
			AgentsLine: "Before rereading files, summarize known state and prefer targeted symbol searches or narrow line ranges over whole-file reads.",
			Plugin:     "The full report finds repeated paths across up to 100 logs; the plugin adds retrieval hygiene prompts.",
		}
	case "tool_output_bloat":
		return actionCopy{
			Title:      "Cap noisy command output",
			Now:        "Use rg filters, head/tail, --json summaries, or redirect logs to a file. Paste only the failing excerpt back into context.",
			Why:        findingEvidence(finding.Evidence),
			AgentsLine: "Cap shell command output by default; use focused filters and paste only the relevant failing excerpt back into context.",
			Plugin:     "The plugin can recommend shell-output reducers and context-safe command habits for your setup.",
		}
	case "retry_loop", "args_hashed_retry_loop":
		return actionCopy{
			Title:      "Break retry loops after two misses",
			Now:        "After two similar failures, stop editing. Restate the invariant, inspect the diff/test output, then restart with a smaller scope.",
			Why:        findingEvidence(finding.Evidence),
			AgentsLine: "After two failed attempts on the same issue, stop, inspect the invariant and latest error, then restart with a smaller scope.",
			Plugin:     "The full scan surfaces recurring retry signatures and turns them into session hygiene rules.",
		}
	case "context_growth_spikes", "cache_invalidation_spike":
		return actionCopy{
			Title:      "Treat context spikes as boundaries",
			Now:        "Use /compact or start a fresh session after large tool output, model/config changes, or a pivot from debugging to architecture.",
			Why:        findingEvidence(finding.Evidence),
			AgentsLine: "Treat major tool-output, model/config changes, and task pivots as context boundaries; compact or split the session before continuing.",
			Plugin:     "The plugin adds compact/split/restart nudges at the points your history shows degradation.",
		}
	case "mcp_bloat_high", "mcp_bloat_severe":
		return actionCopy{
			Title:      "Disable unused MCPs by default",
			Now:        "Move project-specific MCPs out of global config and lazy-load heavy servers only when the task needs them.",
			Why:        findingEvidence(finding.Evidence),
			AgentsLine: "Keep only frequently used MCP servers enabled by default; lazy-load project-specific or heavy MCPs when the task requires them.",
			Plugin:     "The full report converts MCP bloat into a concrete setup checklist.",
		}
	case "skill_bloat_high", "skill_bloat_severe":
		return actionCopy{
			Title:      "Trim always-on skills",
			Now:        "Keep only high-use skills active by default. Move rarely used skills behind explicit invocation.",
			Why:        findingEvidence(finding.Evidence),
			AgentsLine: "Keep only high-signal skills in default context; invoke rare or project-specific skills explicitly when needed.",
			Plugin:     "The plugin can recommend a smaller skill surface from observed usage ratios.",
		}
	default:
		title := finding.Title
		if title == "" {
			title = "Apply the detected fix"
		}
		now := finding.Recommendation
		if now == "" {
			now = "Use a narrower workflow before continuing."
		}
		return actionCopy{
			Title:      title,
			Now:        now,
			Why:        findingEvidence(finding.Evidence),
			AgentsLine: "Keep agent sessions scoped, evidence-backed, and explicit about when context should be compacted or split.",
			Plugin:     "The full scan turns this from one-session advice into a generated remediation pack.",
		}
	}
}

func workflowFingerprintsHTML(fingerprints []analyzer.EcosystemFingerprint) template.HTML {
	var b strings.Builder
	b.WriteString(`<ol id="workflow-fingerprints-list">`)
	if len(fingerprints) == 0 {
		b.WriteString(`<li class="fingerprint-card"><p class="empty-evidence">No known workflow fingerprints detected.</p></li>`)
		b.WriteString(`</ol>`)
		return template.HTML(b.String())
	}
	for _, fp := range fingerprints {
		b.WriteString(`<li class="fingerprint-card">`)
		b.WriteString(`<div class="fingerprint-header">`)
		fmt.Fprintf(&b, `<strong class="fingerprint-id">%s</strong>`, htmlstd.EscapeString(fp.ID))
		confidence := defaultString(fp.Confidence, "unknown")
		fmt.Fprintf(&b, `<span class="fingerprint-confidence status-chip confidence-%s">%s confidence</span>`, htmlstd.EscapeString(confidence), htmlstd.EscapeString(confidence))
		if fp.Active {
			statusChipHTML(&b, "active", "good")
		}
		if fp.Installed {
			statusChipHTML(&b, "installed", "good")
		}
		if fp.VersionBucket != "" {
			statusChipHTML(&b, "version "+fp.VersionBucket, "")
		}
		b.WriteString(`</div>`)
		b.WriteString(`<div class="fingerprint-body">`)
		factTileHTML(&b, "Evidence", fmt.Sprintf("%d", fp.EvidenceCount))
		sourceGroupsHTML(&b, fp.Sources)
		b.WriteString(`</div></li>`)
	}
	b.WriteString(`</ol>`)
	return template.HTML(b.String())
}

func toolingUtilizationHTML(util analyzer.ToolingUtilization) template.HTML {
	var b strings.Builder
	b.WriteString(`<div id="tooling-utilization-rows">`)
	mcpUtilizationHTML(&b, util.MCP)
	skillUtilizationHTML(&b, util.Skill)
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

func mcpUtilizationHTML(b *strings.Builder, mcp analyzer.MCPUtilization) {
	b.WriteString(`<div class="utilization-card" aria-label="MCP: utilization summary">`)
	utilizationHeaderHTML(b, "MCP", normalizeBandString(mcp.WarningBand), utilizationRatioText(mcp.ExposureKnown, mcp.UtilizationRatioPct, mcp.InferenceSource))
	b.WriteString(`<div class="utilization-body">`)
	utilizationGroupHTML(b, "Exposure", [][2]string{
		{"Servers", bucketLabel(mcp.ServerCountBucket)},
		{"Tools", bucketLabel(mcp.ExposedToolCountBucket)},
		{"Context", bucketLabel(mcp.ContextTokenBucket)},
		{"Efficiency", bucketLabel(mcp.ContextEfficiencyBucket)},
	})
	utilizationGroupHTML(b, "Usage", [][2]string{
		{"Calls", fmt.Sprintf("%d", mcp.CallCount)},
		{"Known", fmt.Sprintf("%d", mcp.KnownCallCount)},
		{"Unknown", fmt.Sprintf("%d", mcp.UnknownCallCount)},
	})
	chipPanelHTML(b, "Known called", mcp.UniqueKnownCalledIDs, unknownCountLabelWithSuffix(mcp.UniqueUnknownCalledCount, "unknown called"))
	chipPanelHTML(b, "Inventory", mcp.KnownServerIDs, unknownCountLabelWithSuffix(mcp.UnknownServerCount, "unknown servers"))
	b.WriteString(`</div></div>`)
}

func skillUtilizationHTML(b *strings.Builder, skill analyzer.SkillUtilization) {
	b.WriteString(`<div class="utilization-card" aria-label="Skills: utilization summary">`)
	utilizationHeaderHTML(b, "Skills", normalizeBandString(skill.WarningBand), utilizationRatioText(skill.ExposureKnown, skill.UtilizationRatioPct, skill.InferenceSource))
	b.WriteString(`<div class="utilization-body">`)
	utilizationGroupHTML(b, "Exposure", [][2]string{
		{"Skills", bucketLabel(skill.ExposedCountBucket)},
		{"Context", bucketLabel(skill.ContextTokenBucket)},
		{"Efficiency", bucketLabel(skill.ContextEfficiencyBucket)},
	})
	utilizationGroupHTML(b, "Usage", [][2]string{
		{"Executions", fmt.Sprintf("%d", skill.ExecutedCount)},
		{"Known", fmt.Sprintf("%d", len(skill.KnownExecutedIDs))},
		{"Unknown", fmt.Sprintf("%d", skill.UnknownExecutedCount)},
	})
	chipPanelHTML(b, "Known executed", skill.KnownExecutedIDs, unknownCountLabelWithSuffix(skill.UnknownExecutedCount, "unknown executed"))
	chipPanelHTML(b, "Known exposed", skill.KnownExposedIDs, unknownCountLabelWithSuffix(skill.UnknownExposedCount, "unknown exposed"))
	b.WriteString(`</div></div>`)
}

func utilizationHeaderHTML(b *strings.Builder, label, band, ratio string) {
	b.WriteString(`<header class="utilization-header">`)
	fmt.Fprintf(b, `<strong>%s</strong>`, htmlstd.EscapeString(label))
	b.WriteString(`<div class="utilization-header-meta">`)
	fmt.Fprintf(b, `<span class="band-chip band-%s">%s band</span>`, htmlstd.EscapeString(band), htmlstd.EscapeString(band))
	fmt.Fprintf(b, `<span class="utilization-ratio">%s</span>`, htmlstd.EscapeString(ratio))
	b.WriteString(`</div></header>`)
}

func utilizationGroupHTML(b *strings.Builder, label string, entries [][2]string) {
	fmt.Fprintf(b, `<section class="utilization-group"><h3>%s</h3><div class="fact-grid">`, htmlstd.EscapeString(label))
	for _, entry := range entries {
		factTileHTML(b, entry[0], entry[1])
	}
	b.WriteString(`</div></section>`)
}

func factTileHTML(b *strings.Builder, label, value string) {
	fmt.Fprintf(
		b,
		`<span class="fact-tile"><small>%s</small><strong>%s</strong></span>`,
		htmlstd.EscapeString(label),
		htmlstd.EscapeString(defaultString(value, "unknown")),
	)
}

func chipPanelHTML(b *strings.Builder, label string, values []string, extra string) {
	fmt.Fprintf(b, `<section class="mini-chip-group"><h3>%s</h3><div class="chip-list">`, htmlstd.EscapeString(label))
	if len(values) == 0 {
		chipHTML(b, "none detected", "muted")
	} else {
		for _, value := range values {
			if value != "" {
				chipHTML(b, value, "")
			}
		}
	}
	if extra != "" && !strings.HasPrefix(extra, "0 ") {
		chipHTML(b, extra, "unknown")
	}
	b.WriteString(`</div></section>`)
}

func sourceGroupsHTML(b *strings.Builder, sources []string) {
	b.WriteString(`<div class="fingerprint-source-groups">`)
	grouped := map[string][]string{
		"CLI":           {},
		"Config":        {},
		"Agent surface": {},
		"Other":         {},
	}
	for _, source := range sources {
		grouped[sourceCategory(source)] = append(grouped[sourceCategory(source)], sourceLabel(source))
	}
	order := []string{"CLI", "Config", "Agent surface", "Other"}
	wrote := false
	for _, group := range order {
		values := grouped[group]
		if len(values) == 0 {
			continue
		}
		wrote = true
		chipPanelHTML(b, group, values, "")
	}
	if !wrote {
		chipHTML(b, "no public source markers", "muted")
	}
	b.WriteString(`</div>`)
}

func statusChipHTML(b *strings.Builder, text, tone string) {
	className := "status-chip"
	if tone != "" {
		className += " status-chip-" + tone
	}
	fmt.Fprintf(b, `<span class="%s">%s</span>`, htmlstd.EscapeString(className), htmlstd.EscapeString(text))
}

func normalizeBandString(value string) string {
	switch value {
	case "severe", "high", "watch", "normal":
		return value
	default:
		return "unknown"
	}
}

func utilizationRatioText(exposureKnown bool, ratio int, source string) string {
	if exposureKnown {
		return fmt.Sprintf("%d%% utilization", ratio)
	}
	return "inferred from " + defaultString(source, "unknown exposure")
}

func bucketLabel(value string) string {
	return defaultString(value, "unknown")
}

func unknownCountLabelWithSuffix(count int, suffix string) string {
	return fmt.Sprintf("%d %s", count, suffix)
}

func sourceCategory(source string) string {
	if strings.HasPrefix(source, "cli_") || source == "command_name" {
		return "CLI"
	}
	if strings.HasPrefix(source, "config_") || source == "package_manifest" || source == "hook_config" {
		return "Config"
	}
	if source == "skill_name" || source == "slash_command" || source == "mcp_namespace" {
		return "Agent surface"
	}
	return "Other"
}

func sourceLabel(source string) string {
	labels := map[string]string{
		"cli_binary":        "binary present",
		"cli_version_probe": "version probe",
		"command_name":      "command name",
		"config_dir":        "config directory",
		"config_file":       "config file",
		"package_manifest":  "package manifest",
		"skill_name":        "skill name",
		"slash_command":     "slash command",
		"mcp_namespace":     "MCP namespace",
		"hook_config":       "hook config",
	}
	if label, ok := labels[source]; ok {
		return label
	}
	return strings.ReplaceAll(defaultString(source, "unknown"), "_", " ")
}

func ecosystemPanelHTML(report analyzer.Report) template.HTML {
	e := report.Ecosystem
	var b strings.Builder
	b.WriteString(`<div id="ecosystem" class="ecosystem-panel">`)
	b.WriteString(`<div class="ecosystem-summary">`)
	metricPillHTML(&b, "Client", defaultString(e.Client, "unknown"))
	metricPillHTML(&b, "OS", defaultString(e.OperatingSystem, "unknown"))
	metricPillHTML(&b, "Shell", defaultString(e.Shell, "unknown"))
	metricPillHTML(&b, "Version control", defaultString(e.VersionControl, "unknown"))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="evidence-groups">`)
	chipGroupHTML(&b, "Coding agents", e.CodingAgents, "")
	chipGroupHTML(&b, "Workflow frameworks", e.WorkflowFrameworks, "")
	chipGroupHTML(&b, "MCPs", e.MCPServersKnown, unknownCountLabel(e.UnknownMCPServerCount))
	chipGroupHTML(&b, "Skills", e.KnownSkills, unknownCountLabel(e.UnknownSkillCount))
	chipGroupHTML(&b, "Plugins", e.KnownPlugins, unknownCountLabel(e.UnknownPluginCount))
	chipGroupHTML(&b, "Package managers", e.PackageManagers, "")
	b.WriteString(`</div></div>`)
	return template.HTML(b.String())
}

func receiptPanelHTML(report analyzer.Report) template.HTML {
	receipt := report.SecurityReceipt
	var b strings.Builder
	b.WriteString(`<div id="receipt" class="receipt-panel">`)
	b.WriteString(`<p class="receipt-boundary"><strong>Local redaction boundary:</strong> secrets are removed on your computer before upload. The hosted service receives sanitized report JSON with category counts and placeholders, not the original redacted values.</p>`)
	b.WriteString(`<div class="receipt-status-grid">`)
	receiptTileHTML(&b, "Model tokens for report", "0", "good")
	receiptTileHTML(&b, "Raw transcript to LLM", boolText(receipt.RawTranscriptSentToLLM), receiptTone(!receipt.RawTranscriptSentToLLM))
	receiptTileHTML(&b, "Outbound during analysis", boolText(receipt.OutboundDuringAnalysis), receiptTone(!receipt.OutboundDuringAnalysis))
	rawTTL := defaultString(receipt.RawLogTTL, "unknown")
	ttlTone := "warn"
	if rawTTL == "not uploaded" {
		ttlTone = "good"
	}
	receiptTileHTML(&b, "Raw log TTL", rawTTL, ttlTone)
	redactionTone := "good"
	if receipt.SecretsRedacted > 0 {
		redactionTone = "warn"
	}
	receiptTileHTML(&b, "Secrets redacted locally", fmt.Sprintf("%d", receipt.SecretsRedacted), redactionTone)
	b.WriteString(`</div>`)
	redactionGroupHTML(&b, report.Redactions)
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

func metricPillHTML(b *strings.Builder, label, value string) {
	fmt.Fprintf(
		b,
		`<span class="metric-pill"><small>%s</small><strong>%s</strong></span>`,
		htmlstd.EscapeString(label),
		htmlstd.EscapeString(value),
	)
}

func chipGroupHTML(b *strings.Builder, label string, values []string, extra string) {
	fmt.Fprintf(b, `<section class="chip-group"><h3>%s</h3><div class="chip-list">`, htmlstd.EscapeString(label))
	if len(values) == 0 {
		chipHTML(b, "none detected", "muted")
	} else {
		for _, value := range values {
			if value == "" {
				continue
			}
			chipHTML(b, value, "")
		}
	}
	if extra != "" && !strings.HasPrefix(extra, "0 ") {
		chipHTML(b, extra, "unknown")
	}
	b.WriteString(`</div></section>`)
}

func chipHTML(b *strings.Builder, text, tone string) {
	className := "info-chip"
	if tone != "" {
		className += " info-chip-" + tone
	}
	fmt.Fprintf(b, `<span class="%s">%s</span>`, htmlstd.EscapeString(className), htmlstd.EscapeString(text))
}

func receiptTileHTML(b *strings.Builder, label, value, tone string) {
	fmt.Fprintf(
		b,
		`<div class="receipt-tile receipt-tile-%s" aria-label="%s: %s"><small>%s</small><strong>%s</strong></div>`,
		htmlstd.EscapeString(tone),
		htmlstd.EscapeString(label),
		htmlstd.EscapeString(value),
		htmlstd.EscapeString(label),
		htmlstd.EscapeString(value),
	)
}

func redactionGroupHTML(b *strings.Builder, redactions map[string]int) {
	b.WriteString(`<section class="chip-group redaction-group"><h3>Local redaction categories</h3><p>Only these category counts are included in the uploaded report.</p><div class="chip-list">`)
	keys := make([]string, 0, len(redactions))
	for key, value := range redactions {
		if value > 0 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		chipHTML(b, "none detected", "muted")
	} else {
		for _, key := range keys {
			chipHTML(b, fmt.Sprintf("%s: %d", key, redactions[key]), "unknown")
		}
	}
	b.WriteString(`</div></section>`)
}

func receiptTone(ok bool) string {
	if ok {
		return "good"
	}
	return "bad"
}

func unknownCountLabel(count int) string {
	return fmt.Sprintf("%d unknown", count)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
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
	return fmt.Sprintf("%s-%s", formatTokens(low), formatTokens(high))
}

func sourceLogLabel(count int) string {
	if count == 1 {
		return "1 log"
	}
	return fmt.Sprintf("%d logs", count)
}

func logRefsHTML(refs []analyzer.AnalyzedLogRef) template.HTML {
	if len(refs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="log-ref-list" aria-label="Privacy-safe local log references">`)
	for _, ref := range refs {
		label := defaultString(ref.Label, "analyzed log")
		localRef := defaultString(ref.LocalRef, "local-ref")
		sizeBucket := defaultString(ref.SizeBucket, "size unknown")
		fmt.Fprintf(
			&b,
			`<span><strong>%s</strong><code>%s</code><small>%s</small></span>`,
			htmlstd.EscapeString(label),
			htmlstd.EscapeString(localRef),
			htmlstd.EscapeString(sizeBucket),
		)
	}
	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

func formatInt(n int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return sign + s
	}
	var out []byte
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	out = append(out, s[:first]...)
	for i := first; i < len(s); i += 3 {
		out = append(out, ',')
		out = append(out, s[i:i+3]...)
	}
	return sign + string(out)
}

func formatTokens(n int) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	switch {
	case n >= 1_000_000:
		return sign + trimOneDecimal(float64(n)/1_000_000) + "M"
	case n >= 1_000:
		return sign + trimOneDecimal(float64(n)/1_000) + "k"
	default:
		return sign + formatInt(n)
	}
}

func trimOneDecimal(value float64) string {
	text := fmt.Sprintf("%.1f", value)
	return strings.TrimSuffix(text, ".0")
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
			`<article class="problem-bubble problem-bubble-%s" role="listitem" style="--bubble-size:%dpx; --bubble-offset:%dpx; --problem-title-size:%.1fpx; --problem-detail-size:%.1fpx" aria-label="%s">`,
			htmlstd.EscapeString(tone),
			diameter,
			bubbleOffset(index),
			bubbleLabelFontSize(finding.Title, diameter, 21, 10.5),
			bubbleLabelFontSize(finding.Severity+" - "+finding.CostImpact+" "+compactNumber(estimates[index])+" representative tokens", diameter, 12, 9.5),
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

func bubbleLabelFontSize(text string, diameter int, maxPx, minPx float64) float64 {
	chars := len([]rune(text))
	if chars < 1 {
		chars = 1
	}
	available := float64(diameter) * 0.72
	if available < 90 {
		available = 90
	}
	estimated := available / (float64(chars) * 0.56)
	if estimated < minPx {
		return minPx
	}
	if estimated > maxPx {
		return maxPx
	}
	return estimated
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
	fmt.Fprintf(&b, `<div class="timeline-legend" aria-hidden="true"><span class="timeline-legend-item"><span class="timeline-legend-swatch timeline-legend-observed"></span>estimated volume consumed</span><span class="timeline-legend-item"><span class="timeline-legend-swatch timeline-legend-savings"></span>green overlay = %d-%d%% you may save</span></div>`, wasteLow, wasteHigh)
	fmt.Fprintf(&b, `<div class="timeline-frame"><div class="timeline-y-axis" aria-hidden="true"><span>%s tokens</span><span>0</span></div>`, compactNumber(maxTokens))
	fmt.Fprintf(&b, `<div class="timeline" role="img" aria-label="%s"><div class="timeline-savings-callout" aria-hidden="true"><span>This is what you can save</span></div>`, htmlstd.EscapeString(fmt.Sprintf("Session timeline from turn %d to turn %d; maximum %d estimated token volume; potential savings range %d-%d percent.", firstTurn, lastTurn, maxTokens, wasteLow, wasteHigh)))
	for _, point := range visible {
		height := point.EstimatedTokens * 100 / maxTokens
		if height < 4 {
			height = 4
		}
		savedLow := point.EstimatedTokens * wasteLow / 100
		savedHigh := point.EstimatedTokens * wasteHigh / 100
		tooltip := fmt.Sprintf("turn %d | %s estimated token volume | %s-%s estimated potential savings | %s estimated tool-output tokens | %s rereads | %s retries",
			point.Turn,
			formatTokens(point.EstimatedTokens),
			formatTokens(savedLow),
			formatTokens(savedHigh),
			formatTokens(point.ToolTokens),
			formatInt(point.Rereads),
			formatInt(point.Retries),
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
	return formatTokens(value)
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
