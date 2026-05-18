package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
	"github.com/robertDouglass/claude-log-analyzer/internal/app"
	"github.com/robertDouglass/claude-log-analyzer/internal/localstore"
)

const maxUploadBytes = 50 * 1024 * 1024

func main() {
	addr := getenv("CLAUDE_ANALYZER_ADDR", ":8080")
	store, err := localstore.New(getenv("CLAUDE_ANALYZER_DATA_DIR", "/tmp/claude-log-analyzer"))
	if err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/jobs", createJobHandler(store))
	mux.HandleFunc("GET /api/jobs/{id}", getJobHandler(store))
	mux.HandleFunc("GET /api/reports/{id}", getReportHandler(store))
	mux.Handle("/", http.FileServer(http.Dir("web")))

	slog.Info("api listening", "addr", addr)
	if err := http.ListenAndServe(addr, logRequests(mux)); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func createJobHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			writeError(w, http.StatusBadRequest, "invalid multipart upload")
			return
		}
		file, _, err := r.FormFile("log")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing log file")
			return
		}
		defer file.Close()
		data, err := analyzer.ReadAllLimited(file, maxUploadBytes)
		if err != nil {
			writeError(w, http.StatusRequestEntityTooLarge, "upload too large")
			return
		}
		id := localstore.NewJobID()
		uploadPath, err := store.SaveUpload(id, data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not store upload")
			return
		}
		job := app.Job{ID: id, UploadPath: uploadPath}
		if err := store.CreateJob(job); err != nil {
			writeError(w, http.StatusInternalServerError, "could not create job")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{"job_id": id, "status": string(app.StatusPending)})
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
		writeJSON(w, http.StatusOK, job)
	}
}

func getReportHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := store.GetReport(r.PathValue("id"))
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

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "path", sanitizePath(r.URL.Path))
		next.ServeHTTP(w, r)
	})
}

func sanitizePath(path string) string {
	if strings.HasPrefix(path, "/api/jobs/") {
		return "/api/jobs/:id"
	}
	if strings.HasPrefix(path, "/api/reports/") {
		return "/api/reports/:id"
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

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
