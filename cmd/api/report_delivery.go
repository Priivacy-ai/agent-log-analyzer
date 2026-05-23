package main

import (
	"encoding/json"
	"errors"
	"fmt"
	htmlstd "html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type reportDeliveryRequest struct {
	Email             string `json:"email"`
	MarketingOptIn    bool   `json:"marketing_opt_in"`
	SourceReportJobID string `json:"source_report_job_id"`
	SourceReportToken string `json:"source_report_token"`
}

type reportDeliveryResponse struct {
	DeliveryID string `json:"delivery_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

func createReportDeliveryHandler(store app.APIStore, sender emailSender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unlockStore, ok := store.(app.EmailUnlockStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "report delivery unavailable")
			return
		}
		request, err := parseReportDeliveryRequest(r)
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusBadRequest, err.Error())
			return
		}
		normalized, err := normalizeEmail(request.Email)
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusBadRequest, err.Error())
			return
		}
		if request.SourceReportJobID == "" || request.SourceReportToken == "" {
			writeErrorOrHTML(w, r, http.StatusBadRequest, "source report is required")
			return
		}
		job, report, err := authorizedReport(store, request.SourceReportJobID, request.SourceReportToken)
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusUnauthorized, err.Error())
			return
		}
		reportPack, err := renderDownloadPackage(job, report)
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusInternalServerError, "could not generate report pack")
			return
		}
		artifactURL := publicBaseURL(r) + "/api/public-artifacts/" + job.ID + "/" + request.SourceReportToken + "/plugin.zip"
		pluginZip, err := renderPluginArtifactZip(report, artifactURL)
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusInternalServerError, "could not generate plugin")
			return
		}
		now := time.Now().UTC()
		delivery := app.EmailUnlock{
			ID:                           app.NewJobID(),
			Email:                        normalized,
			EmailHash:                    app.HashEmail(normalized),
			MarketingOptIn:               request.MarketingOptIn,
			SourceReportJobID:            job.ID,
			Status:                       app.EmailUnlockUsed,
			CreatedAt:                    now,
			UpdatedAt:                    now,
			ConfirmedAt:                  now,
			LastTransactionalEmailSentAt: now,
		}
		if err := unlockStore.CreateEmailUnlock(delivery); err != nil {
			writeErrorOrHTML(w, r, http.StatusInternalServerError, "could not record email request")
			return
		}
		message := emailMessage{
			To:      normalized,
			Subject: "Your Agent Analyzer report pack and plugin",
			Body:    reportDeliveryEmailBody(),
			Attachments: []emailAttachment{
				{
					Filename:    "agent-analyzer-report-pack.zip",
					ContentType: "application/zip",
					Data:        reportPack,
				},
				{
					Filename:    "agent-analyzer-optimization-plugin.zip",
					ContentType: "application/zip",
					Data:        pluginZip,
				},
			},
		}
		if err := sender.Send(message); err != nil {
			if errors.As(err, &errEmailSuppressed{}) {
				writeErrorOrHTML(w, r, http.StatusConflict, "email address is suppressed for transactional delivery")
				return
			}
			slogEmailDeliveryFailure("report_delivery", delivery.ID, err)
			writeEmailDeliveryErrorOrHTML(w, r, err)
			return
		}
		slog.Info("report delivery sent", "delivery_id", delivery.ID, "email_hash", delivery.EmailHash, "marketing_opt_in", delivery.MarketingOptIn)
		if wantsHTML(r) {
			renderReportDeliverySentPage(w, normalized)
			return
		}
		writeJSON(w, http.StatusAccepted, reportDeliveryResponse{
			DeliveryID: delivery.ID,
			Status:     string(delivery.Status),
			Message:    "report pack and plugin sent",
		})
	}
}

func parseReportDeliveryRequest(r *http.Request) (reportDeliveryRequest, error) {
	var request reportDeliveryRequest
	if isJSONRequest(r) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return request, errors.New("invalid report delivery request")
		}
		return request, nil
	}
	if err := r.ParseForm(); err != nil {
		return request, errors.New("invalid report delivery form")
	}
	request.Email = r.Form.Get("email")
	request.MarketingOptIn = r.Form.Get("marketing_opt_in") == "1" || r.Form.Get("marketing_opt_in") == "true" || r.Form.Get("marketing_opt_in") == "on"
	request.SourceReportJobID = r.Form.Get("source_report_job_id")
	request.SourceReportToken = r.Form.Get("source_report_token")
	return request, nil
}

func authorizedReport(store app.APIStore, jobID, reportToken string) (app.Job, analyzer.Report, error) {
	job, err := store.GetJob(jobID)
	if err != nil {
		return app.Job{}, analyzer.Report{}, errors.New("source report not found")
	}
	if !tokenMatches(job.ReportTokenHash, reportToken) {
		return app.Job{}, analyzer.Report{}, errors.New("invalid source report token")
	}
	report, err := store.GetReport(job.ID)
	if err != nil {
		return app.Job{}, analyzer.Report{}, errors.New("source report not found")
	}
	return job, report, nil
}

func isJSONRequest(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}

func renderReportDeliverySentPage(w http.ResponseWriter, email string) {
	command := `PLUGIN_ZIP="/path/to/agent-analyzer-optimization-plugin.zip"
claude --plugin-dir "$PLUGIN_ZIP"`
	escapedCommand := htmlstd.EscapeString(command)
	body := fmt.Sprintf(
		`<p>We sent the report pack and generated plugin to <strong>%s</strong>.</p><p>Open the email and save the attachment named <strong>agent-analyzer-optimization-plugin.zip</strong>. Then point Claude Code at that zip:</p><div class="simple-command-copy"><pre><code>%s</code></pre><button type="button" class="copy-agents-line" data-copy="%s">Copy command</button></div><p>The plugin was generated from sanitized report JSON only. Raw transcripts were not attached or uploaded.</p>`,
		htmlstd.EscapeString(email),
		escapedCommand,
		escapedCommand,
	)
	renderSimpleHTML(w, "Report pack sent", body)
}

func reportDeliveryEmailBody() string {
	return `Your Agent Analyzer report pack and optimization plugin are attached.

Attachments:
- agent-analyzer-report-pack.zip: branded PDF guide, personalized PDF report, sanitized report JSON, plugin preview, and partner voucher.
- agent-analyzer-optimization-plugin.zip: generated Claude Code optimization plugin for this report.

Install the plugin:

1. Save agent-analyzer-optimization-plugin.zip somewhere local.
2. Run:

   PLUGIN_ZIP="/path/to/agent-analyzer-optimization-plugin.zip"
   claude --plugin-dir "$PLUGIN_ZIP"

3. Ask Claude Code to explain what the plugin installs before approving any recommended tool setup.

Privacy boundary:
- Raw transcripts were not attached.
- Raw transcripts were not uploaded to Agent Analyzer.
- These attachments were generated from the sanitized report JSON for your private report link.
`
}
