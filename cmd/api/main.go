package main

import (
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

	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
	"github.com/robertDouglass/claude-log-analyzer/internal/app"
	"github.com/robertDouglass/claude-log-analyzer/internal/backend"
)

const maxUploadBytes = 50 * 1024 * 1024

type analysisSessionResponse struct {
	JobID        string    `json:"job_id"`
	Token        string    `json:"token"`
	UploadPath   string    `json:"upload_path"`
	FinalizePath string    `json:"finalize_path"`
	ReportPath   string    `json:"report_path"`
	ReportURL    string    `json:"report_url"`
	ExpiresAt    time.Time `json:"expires_at"`
	MaxBytes     int64     `json:"max_bytes"`
	Command      string    `json:"command"`
	Prompt       string    `json:"prompt"`
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/analysis-sessions", createAnalysisSessionHandler(store, maxQueueDepth(), uploadTokenTTL()))
	mux.HandleFunc("PUT /api/uploads/{id}", tokenUploadHandler(store))
	mux.HandleFunc("POST /api/uploads/{id}/finalize", finalizeTokenUploadHandler(store))
	mux.HandleFunc("GET /api/public-reports/{id}/{token}", getPublicReportHandler(store))
	mux.HandleFunc("GET /r/{id}/{token}", reportPageHandler())
	mux.HandleFunc("GET /api/jobs/{id}", getJobHandler(store))
	mux.Handle("/", http.FileServer(http.Dir("web")))
	return mux
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
			ExpiresAt:    job.UploadTokenExpiresAt,
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

func reportPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/index.html")
	}
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
	if strings.HasPrefix(path, "/api/public-reports/") {
		return "/api/public-reports/:id/:token"
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

func claudePrompt(command string) string {
	return "Find my most recent Claude Code JSONL session log, show me the path and byte size, ask for my approval, then run this exact shell command to upload one log for deterministic analysis. Do not print the log contents.\n\n```sh\n" + command + "\n```"
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
