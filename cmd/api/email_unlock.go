package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	htmlstd "html"
	"log/slog"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type emailUnlockRequest struct {
	Email             string `json:"email"`
	MarketingOptIn    bool   `json:"marketing_opt_in"`
	SourceReportJobID string `json:"source_report_job_id"`
	SourceReportToken string `json:"source_report_token"`
}

type emailUnlockResponse struct {
	UnlockID string `json:"unlock_id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
}

func createEmailUnlockHandler(store app.APIStore, sender emailSender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unlockStore, ok := store.(app.EmailUnlockStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "email unlock unavailable")
			return
		}
		request, err := parseEmailUnlockRequest(r)
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusBadRequest, err.Error())
			return
		}
		normalized, err := normalizeEmail(request.Email)
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusBadRequest, err.Error())
			return
		}
		if request.SourceReportJobID != "" {
			if err := authorizeSourceReport(store, request.SourceReportJobID, request.SourceReportToken); err != nil {
				writeErrorOrHTML(w, r, http.StatusUnauthorized, err.Error())
				return
			}
		}
		confirmationToken, err := newToken()
		if err != nil {
			writeErrorOrHTML(w, r, http.StatusInternalServerError, "could not create confirmation token")
			return
		}
		now := time.Now().UTC()
		unlock := app.EmailUnlock{
			ID:                           app.NewJobID(),
			Email:                        normalized,
			EmailHash:                    app.HashEmail(normalized),
			MarketingOptIn:               request.MarketingOptIn,
			SourceReportJobID:            request.SourceReportJobID,
			ConfirmationTokenHash:        tokenHash(confirmationToken),
			Status:                       app.EmailUnlockPending,
			CreatedAt:                    now,
			UpdatedAt:                    now,
			LastTransactionalEmailSentAt: now,
		}
		if err := unlockStore.CreateEmailUnlock(unlock); err != nil {
			writeErrorOrHTML(w, r, http.StatusInternalServerError, "could not create email unlock")
			return
		}
		confirmURL := publicBaseURL(r) + "/email/confirm/" + unlock.ID + "/" + confirmationToken
		if err := sender.Send(emailMessage{
			To:      normalized,
			Subject: "Confirm your Agent Analyzer full scan",
			Body:    confirmationEmailBody(confirmURL),
		}); err != nil {
			if errors.As(err, &errEmailSuppressed{}) {
				writeErrorOrHTML(w, r, http.StatusConflict, "email address is suppressed for transactional delivery")
				return
			}
			slogEmailDeliveryFailure("confirmation", unlock.ID, err)
			writeEmailDeliveryErrorOrHTML(w, r, err)
			return
		}
		if wantsHTML(r) {
			renderSimpleHTML(w, "Check your email", fmt.Sprintf(
				"<p>We sent a confirmation link to <strong>%s</strong>.</p><p>Open your email client, look for <strong>Confirm your Agent Analyzer full scan</strong>, and click the link in that email. The browser page cannot confirm ownership by itself.</p><p>After confirmation, we will email the one-line NPX command for the full local scan and plugin generation.</p>",
				htmlstd.EscapeString(normalized),
			))
			return
		}
		writeJSON(w, http.StatusAccepted, emailUnlockResponse{
			UnlockID: unlock.ID,
			Status:   string(unlock.Status),
			Message:  "confirmation email sent",
		})
	}
}

func confirmEmailUnlockHandler(store app.APIStore, sender emailSender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unlockStore, ok := store.(app.EmailUnlockStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "email unlock unavailable")
			return
		}
		unlock, err := unlockStore.GetEmailUnlock(r.PathValue("id"))
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "email unlock not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid email unlock id")
			return
		}
		if unlock.Status == app.EmailUnlockPending {
			if !tokenMatches(unlock.ConfirmationTokenHash, r.PathValue("token")) {
				writeError(w, http.StatusUnauthorized, "invalid confirmation token")
				return
			}
			fullScanToken, err := newToken()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not create full-scan token")
				return
			}
			now := time.Now().UTC()
			unlock.Status = app.EmailUnlockConfirmed
			unlock.ConfirmedAt = now
			unlock.FullScanTokenHash = tokenHash(fullScanToken)
			unlock.FullScanTokenExpiresAt = now.Add(fullScanTokenTTL())
			unlock.LastTransactionalEmailSentAt = now
			if err := unlockStore.UpdateEmailUnlock(unlock); err != nil {
				writeError(w, http.StatusInternalServerError, "could not confirm email")
				return
			}
			command := fullScanNPXCommand(publicBaseURL(r), fullScanToken)
			if err := sender.Send(emailMessage{
				To:      unlock.Email,
				Subject: "Your Agent Analyzer full-scan command",
				Body:    fullScanCommandEmailBody(command, unlock.FullScanTokenExpiresAt),
			}); err != nil {
				if errors.As(err, &errEmailSuppressed{}) {
					writeError(w, http.StatusConflict, "email address is suppressed for transactional delivery")
					return
				}
				slogEmailDeliveryFailure("full_scan_command", unlock.ID, err)
				writeEmailDeliveryErrorOrHTML(w, r, err)
				return
			}
			renderConfirmedPage(w, command, unlock.FullScanTokenExpiresAt)
			return
		}
		if unlock.Status == app.EmailUnlockConfirmed && unlock.FullScanTokenHash != "" {
			renderConfirmedPage(w, fullScanNPXCommand(publicBaseURL(r), "<token already issued by email>"), unlock.FullScanTokenExpiresAt)
			return
		}
		renderSimpleHTML(w, "Full scan already submitted", "<p>Your full scan entitlement has already been used. Check your report page or the plugin-ready email.</p>")
	}
}

func createFullScanClientReportHandler(store app.APIStore, sender emailSender, expiresIn time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unlockStore, ok := store.(app.EmailUnlockStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "full scan unlock unavailable")
			return
		}
		reportStore, ok := store.(app.DirectReportStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "direct report upload unavailable")
			return
		}
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, "full-scan token required")
			return
		}
		unlock, err := unlockStore.GetEmailUnlockByFullScanTokenHash(tokenHash(token))
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusUnauthorized, "invalid full-scan token")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid full-scan token")
			return
		}
		if unlock.Status != app.EmailUnlockConfirmed {
			writeError(w, http.StatusConflict, "full-scan token already used")
			return
		}
		if !unlock.FullScanTokenExpiresAt.IsZero() && time.Now().UTC().After(unlock.FullScanTokenExpiresAt) {
			writeError(w, http.StatusGone, "full-scan token expired")
			return
		}
		data, err := analyzer.ReadAllLimited(r.Body, maxClientReportBytes)
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "sanitized report too large")
			return
		}
		var report analyzer.Report
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(&report); err != nil {
			writeError(w, http.StatusBadRequest, "invalid sanitized report JSON")
			return
		}
		if err := validateClientReport(report); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validateFullScanClientReport(report); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		analyzer.AttachRecommendation(&report)
		reportToken, err := newToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create report token")
			return
		}
		now := time.Now().UTC()
		jobID := app.NewJobID()
		report.JobID = jobID
		job := app.Job{
			ID:              jobID,
			Status:          app.StatusCompleted,
			ScanType:        app.ScanTypeFullScan,
			ReportTokenHash: tokenHash(reportToken),
			CreatedAt:       now,
			UpdatedAt:       now,
			CompletedAt:     now,
		}
		if err := reportStore.CreateCompletedReport(job, report); err != nil {
			writeError(w, http.StatusInternalServerError, "could not store sanitized full-scan report")
			return
		}
		unlock.Status = app.EmailUnlockUsed
		unlock.FullScanJobID = jobID
		unlock.UpdatedAt = now
		if err := unlockStore.UpdateEmailUnlock(unlock); err != nil {
			writeError(w, http.StatusInternalServerError, "could not mark full-scan token used")
			return
		}
		appendAnalyticsIfAvailable(store, analytics.FromReport(report, string(app.ScanTypeFullScan)))
		reportPath := "/r/" + jobID + "/" + reportToken
		reportURL := publicBaseURL(r) + reportPath
		artifactURL := publicBaseURL(r) + "/api/public-artifacts/" + jobID + "/" + reportToken + "/plugin.zip"
		if err := sender.Send(emailMessage{
			To:      unlock.Email,
			Subject: "Your Agent Analyzer optimization plugin is ready",
			Body:    pluginReadyEmailBody(reportURL, artifactURL),
		}); err != nil {
			slogWarnEmailSend(err)
		}
		writeJSON(w, http.StatusCreated, analysisSessionResponse{
			JobID:      jobID,
			ReportPath: reportPath,
			ReportURL:  reportURL,
			ExpiresAt:  reportExpiresAt(now, expiresIn),
			MaxBytes:   maxClientReportBytes,
		})
	}
}

func validateFullScanClientReport(report analyzer.Report) error {
	if report.AggregateEvent.ParserType != "full_scan_bundle" && report.AggregateEvent.ParserType != "paid_bundle" {
		return errors.New("full scan requires sanitized aggregate report JSON")
	}
	if report.Metrics.SessionCount <= 0 {
		return errors.New("full scan requires at least one analyzed session")
	}
	if report.SecurityReceipt.RawLogTTL != "not uploaded" {
		return errors.New("full scan report must mark raw logs as not uploaded")
	}
	return nil
}

func parseEmailUnlockRequest(r *http.Request) (emailUnlockRequest, error) {
	var request emailUnlockRequest
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return request, errors.New("invalid email unlock request")
		}
		return request, nil
	}
	if err := r.ParseForm(); err != nil {
		return request, errors.New("invalid email unlock form")
	}
	request.Email = r.Form.Get("email")
	request.MarketingOptIn = r.Form.Get("marketing_opt_in") == "1" || r.Form.Get("marketing_opt_in") == "true" || r.Form.Get("marketing_opt_in") == "on"
	request.SourceReportJobID = r.Form.Get("source_report_job_id")
	request.SourceReportToken = r.Form.Get("source_report_token")
	return request, nil
}

func normalizeEmail(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", errors.New("email is required")
	}
	address, err := mail.ParseAddress(trimmed)
	if err != nil || address.Address == "" || strings.Contains(address.Address, " ") {
		return "", errors.New("valid email is required")
	}
	return strings.ToLower(address.Address), nil
}

func authorizeSourceReport(store app.APIStore, jobID, reportToken string) error {
	job, err := store.GetJob(jobID)
	if err != nil {
		return errors.New("source report not found")
	}
	if !tokenMatches(job.ReportTokenHash, reportToken) {
		return errors.New("invalid source report token")
	}
	return nil
}

func wantsHTML(r *http.Request) bool {
	accept := strings.ToLower(r.Header.Get("Accept"))
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	return strings.Contains(accept, "text/html") || strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data")
}

func writeErrorOrHTML(w http.ResponseWriter, r *http.Request, status int, message string) {
	if wantsHTML(r) {
		renderSimpleHTMLStatus(w, status, "Request failed", "<p>"+htmlstd.EscapeString(message)+"</p>")
		return
	}
	writeError(w, status, message)
}

func writeEmailDeliveryErrorOrHTML(w http.ResponseWriter, r *http.Request, err error) {
	message := "Email delivery is temporarily unavailable. Your request was created, but we could not send the confirmation email."
	var delivery errEmailDelivery
	if errors.As(err, &delivery) && delivery.provider == "ses" && (delivery.detail == "sandbox" || delivery.detail == "message_rejected") {
		message = "Email delivery is not active yet. AWS SES is still in sandbox or has not granted production access, so confirmation mail cannot be sent to arbitrary recipient addresses."
	}
	if wantsHTML(r) {
		renderSimpleHTMLStatus(w, http.StatusServiceUnavailable, "Email delivery unavailable", "<p>"+htmlstd.EscapeString(message)+"</p><p>The local scan and report are still valid. The full-scan email flow will work once transactional email is enabled.</p>")
		return
	}
	writeError(w, http.StatusServiceUnavailable, message)
}

func slogEmailDeliveryFailure(stage, unlockID string, err error) {
	detail := emailFailureDetail(err)
	provider := "unknown"
	var delivery errEmailDelivery
	if errors.As(err, &delivery) && delivery.provider != "" {
		provider = delivery.provider
	}
	slog.Warn("transactional email send failed", "stage", stage, "unlock_id", unlockID, "provider", provider, "detail", detail)
}

func renderConfirmedPage(w http.ResponseWriter, command string, expiresAt time.Time) {
	escapedCommand := htmlstd.EscapeString(command)
	renderSimpleHTML(w, "Email confirmed", fmt.Sprintf(
		`<p>Your email is confirmed. We also emailed this command to you. Run it to analyze target-sized recent logs per supported agent source and generate your plugin:</p><div class="simple-command-copy"><pre><code>%s</code></pre><button type="button" class="copy-agents-line" data-copy="%s">Copy command</button></div><p>This full-scan token expires at %s.</p>`,
		escapedCommand,
		escapedCommand,
		htmlstd.EscapeString(expiresAt.Local().Format(time.RFC1123)),
	))
}

func renderSimpleHTML(w http.ResponseWriter, title, body string) {
	renderSimpleHTMLStatus(w, http.StatusOK, title, body)
}

func renderSimpleHTMLStatus(w http.ResponseWriter, status int, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>%s</title><link rel="stylesheet" href="/styles.css"></head><body><main class="shell"><section class="simple-page"><h1>%s</h1>%s</section></main><script src="/report-actions.js"></script></body></html>`, htmlstd.EscapeString(title), htmlstd.EscapeString(title), body)
}

func confirmationEmailBody(confirmURL string) string {
	return "Confirm your email to unlock the full Agent Analyzer scan.\n\n" +
		"Raw logs still stay on your machine. The next command analyzes target-sized recent logs per supported source locally and uploads only sanitized aggregate JSON.\n\n" +
		"Confirm here:\n" + confirmURL + "\n"
}

func fullScanCommandEmailBody(command string, expiresAt time.Time) string {
	return "Your full-scan command is ready.\n\nRun this one-liner:\n\n" + command + "\n\n" +
		"It analyzes supported agent logs locally, then uploads only the sanitized aggregate report JSON. Token expires: " + expiresAt.Local().Format(time.RFC1123) + "\n"
}

func pluginReadyEmailBody(reportURL, artifactURL string) string {
	return "Your full Agent Analyzer report and generated optimization plugin are ready.\n\nReport:\n" + reportURL + "\n\nPlugin zip:\n" + artifactURL + "\n\nInstall notes:\n- Claude Code: download the zip and run `claude --plugin-dir /path/to/agent-analyzer-optimization-plugin.zip`.\n- Codex: use harnesses/codex/.\n- OpenCode: use harnesses/opencode/.\n- Cursor: use harnesses/cursor/.\n- Kiro: use harnesses/kiro/.\n- Antigravity: use harnesses/antigravity/.\n- Claude Desktop local/session logs: analyzed automatically; no plugin install surface.\n- Claude Desktop MCP: use harnesses/claude-desktop-mcp/ for connector or .mcpb guidance.\n\nBoth links use the private report token from this run.\n"
}

func fullScanNPXCommand(baseURL, token string) string {
	command := "npx --yes agent-analyzer@latest full-scan --token " + shellQuote(token)
	if strings.TrimRight(baseURL, "/") != defaultPublicBaseURL() {
		command += " --base-url " + shellQuote(strings.TrimRight(baseURL, "/"))
	}
	return command
}

func defaultPublicBaseURL() string {
	return "https://analyzer.spec-kitty.ai"
}

func fullScanTokenTTL() time.Duration {
	raw := getenv("CLAUDE_ANALYZER_FULL_SCAN_TOKEN_TTL", "24h")
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 || duration > 7*24*time.Hour {
		return 24 * time.Hour
	}
	return duration
}

func slogWarnEmailSend(err error) {
	// Keep report creation successful if the follow-up email fails; the API
	// response still contains the report URL and artifact is generated on demand.
	if err != nil {
		fmt.Fprintf(os.Stderr, "email send failed: %v\n", err)
	}
}
