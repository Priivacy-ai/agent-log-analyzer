package main

import (
	"bytes"
	"errors"
	"fmt"
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
		if job.ScanType == app.ScanTypePaidBundle && !job.WaiverAcceptedAt.IsZero() {
			artifactURL = publicBaseURL(r) + "/api/public-artifacts/" + job.ID + "/" + r.PathValue("token") + "/plugin.zip"
		}
		renderReportHTML(w, reportPageData{
			Report:      report,
			Job:         job,
			ArtifactURL: artifactURL,
			StatusText:  "This report is visible for 15 minutes.",
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
	"join":                  joinStrings,
	"mapLines":              mapLines,
	"recommendationLabel":   recommendationLabel,
	"recommendationName":    recommendationName,
	"recommendationSignals": recommendationSignals,
	"recommendationURL":     recommendationURL,
	"savingsRange":          savingsRange,
}).Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Agent Analyzer Report</title>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml" />
    <link rel="stylesheet" href="/styles.css" />
  </head>
  <body>
    <main class="shell">
      <section class="report" id="report">
        <p class="expiry" id="report-status">{{.StatusText}}</p>
        {{if eq .Job.Status "completed"}}
        <div class="score">
          <span id="score">{{.Report.Score}}</span>
          <small>efficiency score</small>
        </div>
        <div>
          <h2>Estimated Waste</h2>
          <p id="waste">{{.Report.EstimatedWaste.Low}}-{{.Report.EstimatedWaste.High}}% avoidable token spend</p>
          <p class="command-note">Observed tokens: {{.Report.Metrics.EstimatedTokens}} total, {{.Report.Metrics.ToolOutputTokens}} from tool output.</p>
        </div>
        <div>
          <h2>Top Problems</h2>
          <ol id="findings">
            {{range .Report.Findings}}
            <li>
              <strong>{{.Title}}</strong>
              <p>{{.Severity}} - {{.CostImpact}}</p>
              <p>{{findingEvidence .Evidence}}</p>
              <p>{{.Recommendation}}</p>
            </li>
            {{else}}
            <li>No major deterministic problems detected.</li>
            {{end}}
          </ol>
        </div>
        <div>
          <h2>Session Timeline</h2>
          <p id="timeline-caption" class="timeline-caption">Estimated context size by turn. Highlighted area shows avoidable spend.</p>
          <div class="timeline-frame ssr-timeline">
            <table>
              <thead>
                <tr>
                  <th>Turn</th>
                  <th>Estimated tokens</th>
                  <th>Potentially avoidable tokens</th>
                  <th>Tool-output tokens</th>
                  <th>Rereads</th>
                  <th>Retries</th>
                </tr>
              </thead>
              <tbody>
                {{range .Report.Timeline}}
                <tr>
                  <td>{{.Turn}}</td>
                  <td>{{.EstimatedTokens}}</td>
                  <td>{{savingsRange .EstimatedTokens $.Report.EstimatedWaste}}</td>
                  <td>{{.ToolTokens}}</td>
                  <td>{{.Rereads}}</td>
                  <td>{{.Retries}}</td>
                </tr>
                {{else}}
                <tr><td colspan="6">No timeline points were available.</td></tr>
                {{end}}
              </tbody>
            </table>
          </div>
        </div>
        <div>
          <h2>Suggested Immediate Fixes</h2>
          <ul id="fixes">
            {{range .Report.ImmediateFixes}}<li>{{.}}</li>{{else}}<li>No immediate fixes were generated.</li>{{end}}
          </ul>
        </div>
        {{if .Report.Recommendation}}
        <section id="recommendation-section" class="intel-section">
          <h2>Next-best recommendation</h2>
          {{with .Report.Recommendation.Primary}}{{template "recommendation" .}}{{end}}
          {{with .Report.Recommendation.Secondary}}{{template "recommendation" .}}{{end}}
          {{if and (not .Report.Recommendation.Primary) (not .Report.Recommendation.Secondary)}}
          <p id="recommendation-empty">No action needed — your tooling is already in shape.</p>
          {{end}}
        </section>
        {{end}}
        <section id="workflow-fingerprints" class="intel-section">
          <h2>Workflow Fingerprints</h2>
          <ol id="workflow-fingerprints-list">
            {{range .Report.Ecosystem.WorkflowFingerprints}}
            <li>{{.ID}} — {{.Confidence}} confidence; sources: {{join .Sources}}; evidence: {{.EvidenceCount}}{{if .Active}}; active{{end}}{{if .Installed}}; installed{{end}}{{if .VersionBucket}}; version: {{.VersionBucket}}{{end}}</li>
            {{else}}
            <li>No known workflow fingerprints detected.</li>
            {{end}}
          </ol>
        </section>
        <section id="tooling-utilization" class="intel-section">
          <h2>MCP &amp; Skill Utilization</h2>
          <div id="tooling-utilization-rows">
            <p><strong>MCP:</strong> {{.Report.Ecosystem.ToolingUtilization.MCP.WarningBand}} band; {{.Report.Ecosystem.ToolingUtilization.MCP.UtilizationRatioPct}}% utilization; exposed servers {{.Report.Ecosystem.ToolingUtilization.MCP.ServerCountBucket}}; context {{.Report.Ecosystem.ToolingUtilization.MCP.ContextTokenBucket}}; calls {{.Report.Ecosystem.ToolingUtilization.MCP.CallCount}}; known called: {{join .Report.Ecosystem.ToolingUtilization.MCP.UniqueKnownCalledIDs}}; unknown called count: {{.Report.Ecosystem.ToolingUtilization.MCP.UniqueUnknownCalledCount}}</p>
            <p><strong>Skills:</strong> {{.Report.Ecosystem.ToolingUtilization.Skill.WarningBand}} band; {{.Report.Ecosystem.ToolingUtilization.Skill.UtilizationRatioPct}}% utilization; exposed {{.Report.Ecosystem.ToolingUtilization.Skill.ExposedCountBucket}}; context {{.Report.Ecosystem.ToolingUtilization.Skill.ContextTokenBucket}}; executions {{.Report.Ecosystem.ToolingUtilization.Skill.ExecutedCount}}; known executed: {{join .Report.Ecosystem.ToolingUtilization.Skill.KnownExecutedIDs}}; unknown executed count: {{.Report.Ecosystem.ToolingUtilization.Skill.UnknownExecutedCount}}</p>
          </div>
        </section>
        <div>
          <h2>Ecosystem</h2>
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
          <h2>Security Receipt</h2>
          <pre id="receipt">Raw transcript sent to LLM: {{boolText .Report.SecurityReceipt.RawTranscriptSentToLLM}}
Outbound during analysis: {{boolText .Report.SecurityReceipt.OutboundDuringAnalysis}}
Secrets redacted: {{.Report.SecurityReceipt.SecretsRedacted}}
Raw log TTL: {{.Report.SecurityReceipt.RawLogTTL}}
Redactions:
{{mapLines .Report.Redactions}}</pre>
        </div>
        <div class="upsell">
          <h2>Install the optimization pack generated from this analysis</h2>
          <p>Unlock a waiver-gated paid scan across up to 100 recent logs per supported agent source, then install a generated optimization pack with vetted context, retrieval, telemetry, and CLAUDE.md recommendations.</p>
          {{if .ArtifactURL}}
          <p>Optimization plugin artifact: <a href="{{.ArtifactURL}}">{{.ArtifactURL}}</a></p>
          {{else}}
          <p>The paid scan will use the same local-first model: analyze up to 100 recent Claude Code, Codex, and OpenCode sessions locally, review the sanitized aggregate, then upload only the generated report JSON.</p>
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
    <script src="/app.js"></script>
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
