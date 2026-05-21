package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
	"github.com/priivacy-ai/agent-log-analyzer/internal/backend"
	"github.com/priivacy-ai/agent-log-analyzer/internal/remediation"
)

const maxUploadBytes = 50 * 1024 * 1024
const maxPaidUploadBytes = 250 * 1024 * 1024
const maxClientReportBytes = 1024 * 1024

type analysisSessionResponse struct {
	JobID        string     `json:"job_id"`
	Token        string     `json:"token"`
	UploadPath   string     `json:"upload_path"`
	FinalizePath string     `json:"finalize_path"`
	ReportPath   string     `json:"report_path"`
	ReportURL    string     `json:"report_url"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxBytes     int64      `json:"max_bytes"`
	Command      string     `json:"command"`
	Prompt       string     `json:"prompt"`
}

type paidSessionRequest struct {
	WaiverAccepted bool   `json:"waiver_accepted"`
	Acknowledgment string `json:"acknowledgment"`
}

func main() {
	addr := getenv("CLAUDE_ANALYZER_ADDR", ":8080")
	store, err := backend.NewAPIStore()
	if err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}

	mux := buildMux(store)
	slog.Info("api listening", "addr", addr)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func buildMux(store app.APIStore) http.Handler {
	mux := http.NewServeMux()
	emailSender := guardEmailSender(configuredEmailSender(), store)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/analysis-sessions", createAnalysisSessionHandler(store, maxQueueDepth(), uploadTokenTTL()))
	mux.HandleFunc("POST /api/paid-sessions", createPaidSessionHandler(store, uploadTokenTTL()))
	mux.HandleFunc("POST /api/client-reports", createClientReportHandler(store, reportTTL()))
	mux.HandleFunc("POST /api/paid-client-reports", createPaidClientReportHandler(store, reportTTL()))
	mux.HandleFunc("POST /api/email-unlocks", createEmailUnlockHandler(store, emailSender))
	mux.HandleFunc("GET /email/confirm/{id}/{token}", confirmEmailUnlockHandler(store, emailSender))
	mux.HandleFunc("POST /api/full-scan-client-reports", createFullScanClientReportHandler(store, emailSender, reportTTL()))
	mux.HandleFunc("PUT /api/uploads/{id}", tokenUploadHandler(store))
	mux.HandleFunc("POST /api/uploads/{id}/finalize", finalizeTokenUploadHandler(store))
	mux.HandleFunc("PUT /api/paid-uploads/{id}", paidBundleUploadHandler(store))
	mux.HandleFunc("POST /api/paid-uploads/{id}/finalize", finalizeTokenUploadHandler(store))
	mux.HandleFunc("GET /api/public-reports/{id}/{token}", getPublicReportHandler(store))
	mux.HandleFunc("GET /api/public-reports/{id}/{token}/extended.md", getExtendedReportHandler(store))
	mux.HandleFunc("GET /api/public-artifacts/{id}/{token}/plugin.zip", getPublicArtifactHandler(store))
	mux.HandleFunc("GET /r/{id}/{token}", reportPageHandler(store))
	mux.HandleFunc("GET /api/jobs/{id}", getJobHandler(store))
	mux.Handle("/", http.FileServer(http.Dir("web")))
	return mux
}

func createClientReportHandler(store app.APIStore, expiresIn time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reportStore, ok := store.(app.DirectReportStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "direct report upload unavailable")
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
			ScanType:        app.ScanTypeSingle,
			ReportTokenHash: tokenHash(reportToken),
			CreatedAt:       now,
			UpdatedAt:       now,
			CompletedAt:     now,
		}
		if err := reportStore.CreateCompletedReport(job, report); err != nil {
			writeError(w, http.StatusInternalServerError, "could not store sanitized report")
			return
		}
		appendAnalyticsIfAvailable(store, analytics.FromReport(report, string(app.ScanTypeSingle)))
		reportPath := "/r/" + jobID + "/" + reportToken
		writeJSON(w, http.StatusCreated, analysisSessionResponse{
			JobID:      jobID,
			ReportPath: reportPath,
			ReportURL:  publicBaseURL(r) + reportPath,
			ExpiresAt:  reportExpiresAt(now, expiresIn),
			MaxBytes:   maxClientReportBytes,
		})
	}
}

func createPaidClientReportHandler(store app.APIStore, expiresIn time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localPaidSessionsEnabled() {
			writeError(w, http.StatusPaymentRequired, "paid checkout is not configured")
			return
		}
		reportStore, ok := store.(app.DirectReportStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "direct report upload unavailable")
			return
		}
		if !waiverAccepted(r) {
			writeError(w, http.StatusBadRequest, "waiver acknowledgment required")
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
		if err := validatePaidClientReport(report); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		reportToken, err := newToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create report token")
			return
		}
		now := time.Now().UTC()
		jobID := app.NewJobID()
		report.JobID = jobID
		job := app.Job{
			ID:               jobID,
			Status:           app.StatusCompleted,
			ScanType:         app.ScanTypePaidBundle,
			ReportTokenHash:  tokenHash(reportToken),
			WaiverAcceptedAt: now,
			CreatedAt:        now,
			UpdatedAt:        now,
			CompletedAt:      now,
		}
		if err := reportStore.CreateCompletedReport(job, report); err != nil {
			writeError(w, http.StatusInternalServerError, "could not store sanitized paid report")
			return
		}
		appendAnalyticsIfAvailable(store, analytics.FromReport(report, string(app.ScanTypePaidBundle)))
		reportPath := "/r/" + jobID + "/" + reportToken
		writeJSON(w, http.StatusCreated, analysisSessionResponse{
			JobID:      jobID,
			ReportPath: reportPath,
			ReportURL:  publicBaseURL(r) + reportPath,
			ExpiresAt:  reportExpiresAt(now, expiresIn),
			MaxBytes:   maxClientReportBytes,
		})
	}
}

func appendAnalyticsIfAvailable(store app.APIStore, event analytics.Event) {
	analyticsStore, ok := store.(app.AnalyticsStore)
	if !ok {
		return
	}
	if err := analyticsStore.AppendAnalyticsEvent(event); err != nil {
		slog.Warn("analytics event append failed", "error_category", "analytics_append")
	}
}

func validateClientReport(report analyzer.Report) error {
	if report.Version == "" {
		return errors.New("sanitized report missing analyzer version")
	}
	if report.Metrics.Turns <= 0 {
		return errors.New("sanitized report missing parsed turns")
	}
	if report.SecurityReceipt.RawTranscriptSentToLLM {
		return errors.New("sanitized report cannot claim raw transcript was sent to an LLM")
	}
	if report.SecurityReceipt.OutboundDuringAnalysis {
		return errors.New("sanitized report cannot claim outbound network during local analysis")
	}
	return nil
}

func validatePaidClientReport(report analyzer.Report) error {
	if report.AggregateEvent.ParserType != "paid_bundle" {
		return errors.New("paid scan requires sanitized aggregate report JSON")
	}
	if report.Metrics.SessionCount <= 0 {
		return errors.New("paid scan requires at least one analyzed session")
	}
	if report.SecurityReceipt.RawLogTTL != "not uploaded" {
		return errors.New("paid scan report must mark raw logs as not uploaded")
	}
	return nil
}

func createPaidSessionHandler(store app.APIStore, expiresIn time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localPaidSessionsEnabled() {
			writeError(w, http.StatusPaymentRequired, "paid checkout is not configured")
			return
		}
		sessionStore, ok := store.(app.TokenUploadStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "token upload unavailable")
			return
		}
		var request paidSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid paid session request")
			return
		}
		if !request.WaiverAccepted || !strings.Contains(strings.ToLower(request.Acknowledgment), "own risk") {
			writeError(w, http.StatusBadRequest, "waiver acknowledgment required")
			return
		}
		if r.URL.Query().Get("legacy_raw_bundle") != "1" {
			writeJSON(w, http.StatusCreated, paidLocalFirstSessionResponse(publicBaseURL(r)))
			return
		}
		uploadToken, err := newToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create upload token")
			return
		}
		reportToken, err := newToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create report token")
			return
		}
		now := time.Now().UTC()
		jobID := app.NewJobID()
		job := app.Job{
			ID:                   jobID,
			Status:               app.StatusUploading,
			ScanType:             app.ScanTypePaidBundle,
			MaxUploadBytes:       maxPaidUploadBytes,
			UploadTokenHash:      tokenHash(uploadToken),
			ReportTokenHash:      tokenHash(reportToken),
			UploadTokenExpiresAt: now.Add(expiresIn),
			WaiverAcceptedAt:     now,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := sessionStore.CreateUploadSession(job); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create paid scan session")
			return
		}
		baseURL := publicBaseURL(r)
		uploadPath := "/api/paid-uploads/" + jobID
		finalizePath := uploadPath + "/finalize"
		reportPath := "/r/" + jobID + "/" + reportToken
		response := analysisSessionResponse{
			JobID:        jobID,
			Token:        uploadToken,
			UploadPath:   uploadPath,
			FinalizePath: finalizePath,
			ReportPath:   reportPath,
			ReportURL:    baseURL + reportPath,
			ExpiresAt:    timePtr(job.UploadTokenExpiresAt),
			MaxBytes:     maxPaidUploadBytes,
		}
		response.Command = paidShellCommand(baseURL, response)
		response.Prompt = paidClaudePrompt(response.Command)
		writeJSON(w, http.StatusCreated, response)
	}
}

func waiverAccepted(r *http.Request) bool {
	accepted := strings.ToLower(r.Header.Get("X-Waiver-Accepted"))
	acknowledgment := strings.ToLower(r.Header.Get("X-Waiver-Acknowledgment"))
	return (accepted == "1" || accepted == "true" || accepted == "yes") && strings.Contains(acknowledgment, "own risk")
}

func createAnalysisSessionHandler(store app.APIStore, maxDepth int, expiresIn time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionStore, ok := store.(app.TokenUploadStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "token upload unavailable")
			return
		}
		if maxDepth > 0 {
			depth, err := store.QueueDepth()
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, "analysis queue unavailable")
				return
			}
			if depth >= maxDepth {
				w.Header().Set("Retry-After", "60")
				writeJSON(w, http.StatusServiceUnavailable, map[string]any{
					"error":       "analysis queue is busy",
					"queue_depth": depth,
					"retry_after": "60s",
				})
				return
			}
		}
		uploadToken, err := newToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create upload token")
			return
		}
		reportToken, err := newToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create report token")
			return
		}
		now := time.Now().UTC()
		jobID := app.NewJobID()
		job := app.Job{
			ID:                   jobID,
			Status:               app.StatusUploading,
			ScanType:             app.ScanTypeSingle,
			MaxUploadBytes:       maxUploadBytes,
			UploadTokenHash:      tokenHash(uploadToken),
			ReportTokenHash:      tokenHash(reportToken),
			UploadTokenExpiresAt: now.Add(expiresIn),
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := sessionStore.CreateUploadSession(job); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create analysis session")
			return
		}
		baseURL := publicBaseURL(r)
		uploadPath := "/api/uploads/" + jobID
		finalizePath := uploadPath + "/finalize"
		reportPath := "/r/" + jobID + "/" + reportToken
		response := analysisSessionResponse{
			JobID:        jobID,
			Token:        uploadToken,
			UploadPath:   uploadPath,
			FinalizePath: finalizePath,
			ReportPath:   reportPath,
			ReportURL:    baseURL + reportPath,
			ExpiresAt:    timePtr(job.UploadTokenExpiresAt),
			MaxBytes:     maxUploadBytes,
		}
		response.Command = shellCommand(baseURL, response, reportToken)
		response.Prompt = claudePrompt(response.Command)
		writeJSON(w, http.StatusCreated, response)
	}
}

func tokenUploadHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionStore, ok := store.(app.TokenUploadStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "token upload unavailable")
			return
		}
		job, ok := authorizeTokenJob(w, r, store, r.PathValue("id"), true)
		if !ok {
			return
		}
		if job.Status != app.StatusUploading {
			writeError(w, http.StatusConflict, "upload token already used")
			return
		}
		if job.ScanType != "" && job.ScanType != app.ScanTypeSingle {
			writeError(w, http.StatusBadRequest, "use paid bundle upload endpoint")
			return
		}
		if job.UploadPath != "" {
			writeError(w, http.StatusConflict, "upload already received")
			return
		}
		maxBytes := job.MaxUploadBytes
		if maxBytes <= 0 {
			maxBytes = maxUploadBytes
		}
		data, err := analyzer.ReadAllLimited(r.Body, maxBytes)
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "upload too large")
			return
		}
		job, err = sessionStore.StoreUploadSession(job, data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not store upload")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"job_id": job.ID,
			"status": string(job.Status),
			"bytes":  len(data),
		})
	}
}

func paidBundleUploadHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionStore, ok := store.(app.TokenUploadStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "token upload unavailable")
			return
		}
		job, ok := authorizeTokenJob(w, r, store, r.PathValue("id"), true)
		if !ok {
			return
		}
		if job.Status != app.StatusUploading {
			writeError(w, http.StatusConflict, "upload token already used")
			return
		}
		if job.ScanType != app.ScanTypePaidBundle {
			writeError(w, http.StatusBadRequest, "paid upload token required")
			return
		}
		if job.UploadPath != "" {
			writeError(w, http.StatusConflict, "upload already received")
			return
		}
		if r.URL.Query().Get("limit") != "100" || r.Header.Get("X-Scan-Limit") != "100" {
			writeError(w, http.StatusBadRequest, "paid scan requires limit=100 and X-Scan-Limit: 100")
			return
		}
		contentType := strings.ToLower(r.Header.Get("Content-Type"))
		if contentType != "" && !strings.Contains(contentType, "application/gzip") && !strings.Contains(contentType, "application/x-gzip") && !strings.Contains(contentType, "application/octet-stream") {
			writeError(w, http.StatusUnsupportedMediaType, "paid scan upload must be application/gzip")
			return
		}
		maxBytes := job.MaxUploadBytes
		if maxBytes <= 0 {
			maxBytes = maxPaidUploadBytes
		}
		data, err := analyzer.ReadAllLimited(r.Body, maxBytes)
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "upload too large")
			return
		}
		job, err = sessionStore.StoreUploadSession(job, data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not store upload")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"job_id": job.ID,
			"status": string(job.Status),
			"bytes":  len(data),
			"limit":  100,
		})
	}
}

func finalizeTokenUploadHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionStore, ok := store.(app.TokenUploadStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "token upload unavailable")
			return
		}
		job, ok := authorizeTokenJob(w, r, store, r.PathValue("id"), true)
		if !ok {
			return
		}
		if job.UploadPath == "" && job.Status == app.StatusUploading {
			writeError(w, http.StatusBadRequest, "upload missing")
			return
		}
		if err := sessionStore.FinalizeUploadSession(job); err != nil {
			writeError(w, http.StatusBadRequest, "could not finalize upload")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"job_id": job.ID, "status": string(app.StatusPending)})
	}
}

func getPublicReportHandler(store app.APIStore) http.HandlerFunc {
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
		report, err := store.GetReport(job.ID)
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "report not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid report id")
			return
		}
		writeJSON(w, http.StatusOK, report)
	}
}

func getPublicArtifactHandler(store app.APIStore) http.HandlerFunc {
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
			writeError(w, http.StatusConflict, "job is not completed")
			return
		}
		if !jobAllowsPluginArtifact(job) {
			writeError(w, http.StatusForbidden, "plugin artifact requires a completed report")
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
		artifactURL := publicBaseURL(r) + r.URL.Path
		artifact := remediation.Generate(report, remediation.Options{ArtifactURL: artifactURL})
		var buffer bytes.Buffer
		if err := remediation.WriteZip(&buffer, artifact); err != nil {
			writeError(w, http.StatusInternalServerError, "could not generate plugin artifact")
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="agent-analyzer-optimization.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buffer.Bytes())
	}
}

func jobAllowsPluginArtifact(job app.Job) bool {
	if job.ScanType == app.ScanTypeSingle && job.Status == app.StatusCompleted {
		return true
	}
	if job.ScanType == app.ScanTypeFullScan {
		return true
	}
	return job.ScanType == app.ScanTypePaidBundle && !job.WaiverAcceptedAt.IsZero()
}

func getJobHandler(store app.APIStore) http.HandlerFunc {
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
		job.UploadPath = ""
		job.ReportPath = ""
		job.UploadTokenHash = ""
		job.ReportTokenHash = ""
		writeJSON(w, http.StatusOK, job)
	}
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "path", sanitizePath(r.URL.Path))
		next.ServeHTTP(w, r)
	})
}

func sanitizePath(path string) string {
	if strings.HasPrefix(path, "/api/uploads/") {
		return "/api/uploads/:id"
	}
	if strings.HasPrefix(path, "/api/paid-uploads/") {
		return "/api/paid-uploads/:id"
	}
	if strings.HasPrefix(path, "/api/public-reports/") {
		return "/api/public-reports/:id/:token"
	}
	if strings.HasPrefix(path, "/api/public-artifacts/") {
		return "/api/public-artifacts/:id/:token/plugin.zip"
	}
	if strings.HasPrefix(path, "/api/email-unlocks") {
		return "/api/email-unlocks"
	}
	if strings.HasPrefix(path, "/api/full-scan-client-reports") {
		return "/api/full-scan-client-reports"
	}
	if strings.HasPrefix(path, "/email/confirm/") {
		return "/email/confirm/:id/:token"
	}
	if strings.HasPrefix(path, "/api/jobs/") {
		return "/api/jobs/:id"
	}
	if strings.HasPrefix(path, "/r/") {
		return "/r/:id/:token"
	}
	return path
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func authorizeTokenJob(w http.ResponseWriter, r *http.Request, store app.APIStore, jobID string, enforceExpiry bool) (app.Job, bool) {
	job, err := store.GetJob(jobID)
	if errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "job not found")
		return app.Job{}, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return app.Job{}, false
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" || !tokenMatches(job.UploadTokenHash, token) {
		writeError(w, http.StatusUnauthorized, "invalid upload token")
		return app.Job{}, false
	}
	if enforceExpiry && !job.UploadTokenExpiresAt.IsZero() && time.Now().UTC().After(job.UploadTokenExpiresAt) {
		writeError(w, http.StatusGone, "upload token expired")
		return app.Job{}, false
	}
	return job, true
}

func bearerToken(header string) string {
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func newToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func tokenMatches(hash, token string) bool {
	if hash == "" || token == "" {
		return false
	}
	got := tokenHash(token)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(got)) == 1
}

func publicBaseURL(r *http.Request) string {
	if configured := os.Getenv("CLAUDE_ANALYZER_PUBLIC_BASE_URL"); configured != "" {
		return strings.TrimRight(configured, "/")
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}
	host := r.Host
	return proto + "://" + host
}

func shellCommand(baseURL string, response analysisSessionResponse, reportToken string) string {
	uploadURL := baseURL + response.UploadPath
	finalizeURL := baseURL + response.FinalizePath
	return strings.Join([]string{
		`LOG="$(python3 - <<'PY'`,
		`from pathlib import Path`,
		`logs = list(Path.home().glob(".claude/projects/**/*.jsonl"))`,
		`if not logs:`,
		`    raise SystemExit("No Claude Code JSONL logs found under ~/.claude/projects")`,
		`latest = max(logs, key=lambda p: p.stat().st_mtime)`,
		`print(latest)`,
		`PY`,
		`)"`,
		`SIZE="$(wc -c < "$LOG" | tr -d ' ')"`,
		`echo "Claude log: $LOG"`,
		`echo "Bytes: $SIZE"`,
		`printf 'Upload this one log for deterministic analysis? [y/N] '`,
		`read -r OK`,
		`case "$OK" in y|Y|yes|YES) ;; *) echo "Upload cancelled"; exit 1 ;; esac`,
		`curl -fsS -X PUT ` + shellQuote(uploadURL) + ` \`,
		`  -H ` + shellQuote("Authorization: Bearer "+response.Token) + ` \`,
		`  -H 'Content-Type: application/x-ndjson' \`,
		`  --data-binary "@$LOG"`,
		`curl -fsS -X POST ` + shellQuote(finalizeURL) + ` \`,
		`  -H ` + shellQuote("Authorization: Bearer "+response.Token),
		`echo`,
		`echo "Report: ` + response.ReportURL + `"`,
		`(open ` + shellQuote(response.ReportURL) + ` 2>/dev/null || xdg-open ` + shellQuote(response.ReportURL) + ` 2>/dev/null || true)`,
	}, "\n")
}

func paidShellCommand(baseURL string, response analysisSessionResponse) string {
	uploadURL := baseURL + response.UploadPath + "?limit=100"
	finalizeURL := baseURL + response.FinalizePath
	return strings.Join([]string{
		`BUNDLE="$(mktemp -t agent-analyzer-paid.XXXXXX.tar.gz)"`,
		`LIST="$(mktemp -t agent-analyzer-paid.XXXXXX.txt)"`,
		`python3 - <<'PY' > "$LIST"`,
		`from pathlib import Path`,
		`home = Path.home()`,
		`logs = sorted(home.glob(".claude/projects/**/*.jsonl"), key=lambda p: p.stat().st_mtime, reverse=True)[:100]`,
		`if not logs:`,
		`    raise SystemExit("No Claude Code JSONL logs found under ~/.claude/projects")`,
		`for path in logs:`,
		`    print(path.relative_to(home))`,
		`PY`,
		`COUNT="$(wc -l < "$LIST" | tr -d ' ')"`,
		`tar -C "$HOME" -czf "$BUNDLE" -T "$LIST"`,
		`BYTES="$(wc -c < "$BUNDLE" | tr -d ' ')"`,
		`echo "Claude logs selected: $COUNT most recent JSONL files"`,
		`echo "Bundle bytes: $BYTES"`,
		`printf 'Upload these logs for paid deterministic analysis? [y/N] '`,
		`read -r OK`,
		`case "$OK" in y|Y|yes|YES) ;; *) echo "Upload cancelled"; exit 1 ;; esac`,
		`curl -fsS -X PUT ` + shellQuote(uploadURL) + ` \`,
		`  -H ` + shellQuote("Authorization: Bearer "+response.Token) + ` \`,
		`  -H 'Content-Type: application/gzip' \`,
		`  -H 'X-Scan-Limit: 100' \`,
		`  --data-binary "@$BUNDLE"`,
		`curl -fsS -X POST ` + shellQuote(finalizeURL) + ` \`,
		`  -H ` + shellQuote("Authorization: Bearer "+response.Token),
		`echo`,
		`echo "Paid report: ` + response.ReportURL + `"`,
		`(open ` + shellQuote(response.ReportURL) + ` 2>/dev/null || xdg-open ` + shellQuote(response.ReportURL) + ` 2>/dev/null || true)`,
	}, "\n")
}

func paidLocalFirstSessionResponse(baseURL string) analysisSessionResponse {
	command := paidLocalFirstShellCommand(baseURL)
	return analysisSessionResponse{
		UploadPath: "/api/paid-client-reports",
		MaxBytes:   maxClientReportBytes,
		Command:    command,
		Prompt:     paidLocalFirstClaudePrompt(command),
	}
}

func paidLocalFirstShellCommand(baseURL string) string {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/paid-client-reports"
	return strings.Join([]string{
		`REPORT="${REPORT:-agent-analyzer-paid-aggregate.json}"`,
		`npx --yes agent-analyzer@latest analyze --paid --limit 10 --out "$REPORT"`,
		`echo "Review the sanitized aggregate before upload:"`,
		`jq . "$REPORT" >/dev/null && jq '{version,score,metrics,findings,aggregate_event,security_receipt}' "$REPORT"`,
		`printf 'Upload only this sanitized aggregate report? [y/N] '`,
		`read -r OK`,
		`case "$OK" in y|Y|yes|YES) ;; *) echo "Upload cancelled"; exit 1 ;; esac`,
		`RESPONSE="$(curl -fsS -X POST ` + shellQuote(endpoint) + ` \`,
		`  -H 'Content-Type: application/json' \`,
		`  -H 'X-Waiver-Accepted: true' \`,
		`  -H 'X-Waiver-Acknowledgment: I accept at my own risk' \`,
		`  --data-binary "@$REPORT")"`,
		`echo "$RESPONSE"`,
		`REPORT_URL="$(printf '%s' "$RESPONSE" | jq -r .report_url)"`,
		`python3 -m webbrowser "$REPORT_URL" >/dev/null 2>&1 || true`,
	}, "\n")
}

func claudePrompt(command string) string {
	return "Review this shell command for me, but do not run it. It finds my most recent Claude Code JSONL session log, asks for approval, uploads that raw log to my own analyzer endpoint, finalizes the analysis, and opens the report. Explain the data-exposure risk and tell me to run it myself only if I trust that endpoint.\n\n```sh\n" + command + "\n```"
}

func paidClaudePrompt(command string) string {
	return "Review this shell command for me, but do not run it. It bundles my 100 most recent Claude Code JSONL logs, asks for approval, uploads the raw bundle to my own analyzer endpoint, finalizes the analysis, and opens the report. Explain the data-exposure risk and tell me to run it myself only if I trust that endpoint.\n\n```sh\n" + command + "\n```"
}

func paidLocalFirstClaudePrompt(command string) string {
	return "Review this shell command for me, but do not run it. It should analyze up to 10 largest-recent logs per supported local agent source, currently Claude Code, Codex, and OpenCode, write a sanitized aggregate report I can inspect, and upload only that sanitized JSON report for plugin generation. Confirm that it does not upload raw logs or tar bundles.\n\n```sh\n" + command + "\n```"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func uploadTokenTTL() time.Duration {
	raw := getenv("CLAUDE_ANALYZER_UPLOAD_TOKEN_TTL", "15m")
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 || duration > 15*time.Minute {
		slog.Warn("invalid upload token ttl", "error_category", "configuration")
		return 15 * time.Minute
	}
	return duration
}

func reportTTL() time.Duration {
	raw := getenv("CLAUDE_ANALYZER_REPORT_TTL", "0")
	duration, err := time.ParseDuration(raw)
	if err != nil {
		slog.Warn("invalid report ttl", "error_category", "configuration")
		return 0
	}
	return duration
}

func reportExpiresAt(now time.Time, ttl time.Duration) *time.Time {
	if ttl <= 0 {
		return nil
	}
	return timePtr(now.Add(ttl))
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func maxQueueDepth() int {
	raw := os.Getenv("CLAUDE_ANALYZER_MAX_QUEUE_DEPTH")
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		slog.Warn("invalid max queue depth", "error_category", "configuration")
		return 0
	}
	return value
}

func localPaidSessionsEnabled() bool {
	value := strings.ToLower(os.Getenv("CLAUDE_ANALYZER_ENABLE_LOCAL_PAID_SESSIONS"))
	return value == "1" || value == "true" || value == "yes"
}
