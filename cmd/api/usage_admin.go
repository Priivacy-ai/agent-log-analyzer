package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

const (
	defaultUsageStatsWindow = 7 * 24 * time.Hour
	maxUsageStatsWindow     = 90 * 24 * time.Hour
	defaultUsageStatsLimit  = 20000
	maxUsageStatsLimit      = 100000
)

func usageStatsHandler(store app.APIStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminTokenConfigured() {
			writeError(w, http.StatusServiceUnavailable, "usage stats auth is not configured")
			return
		}
		if !adminTokenMatches(bearerToken(r.Header.Get("Authorization"))) {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}
		usageStore, ok := store.(app.UsageStore)
		if !ok {
			writeError(w, http.StatusNotImplemented, "usage stats unavailable")
			return
		}
		since, until, err := usageStatsWindow(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		limit := usageStatsLimit(r)
		events, err := usageStore.ReadUsageEvents(since, limit)
		if err != nil {
			slog.Warn("usage stats read failed", "error_category", "usage_stats_read")
			writeError(w, http.StatusInternalServerError, "could not read usage stats")
			return
		}
		stats := analytics.SummarizeUsageEvents(events, since, until, limit > 0 && len(events) >= limit)
		writeJSON(w, http.StatusOK, stats)
	}
}

func logRequests(next http.Handler, store app.APIStore) http.Handler {
	usageStore, _ := store.(app.UsageStore)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		path := sanitizePath(r.URL.Path)
		slog.Info("request",
			"method", r.Method,
			"path", path,
			"status", recorder.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		if usageStore == nil || path == "/healthz" {
			return
		}
		event := analytics.NewUsageEvent(start)
		event.Method = r.Method
		event.Path = path
		event.Status = recorder.status
		event.DurationMS = time.Since(start).Milliseconds()
		event.RequestBytes = r.ContentLength
		event.ResponseBytes = recorder.bytes
		event.AuthSurface = authSurface(path)
		event.Authenticated = requestAuthenticated(event.AuthSurface, recorder.status)
		event.ClientHash = clientHash(r)
		event.UserAgent = userAgentFamily(r.UserAgent())
		if err := usageStore.AppendUsageEvent(event); err != nil {
			slog.Warn("usage event append failed", "error_category", "usage_append")
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
	wrote  bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(data []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(w.status)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	return n, err
}

func adminTokenConfigured() bool {
	return os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN_SHA256") != "" || os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN") != ""
}

func adminTokenMatches(token string) bool {
	if token == "" {
		return false
	}
	if hash := os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN_SHA256"); hash != "" {
		got := tokenHash(token)
		return subtle.ConstantTimeCompare([]byte(hash), []byte(got)) == 1
	}
	expected := os.Getenv("CLAUDE_ANALYZER_ADMIN_TOKEN")
	return expected != "" && subtle.ConstantTimeCompare([]byte(expected), []byte(token)) == 1
}

func usageStatsWindow(r *http.Request) (time.Time, time.Time, error) {
	until := time.Now().UTC()
	window := defaultUsageStatsWindow
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		duration, err := time.ParseDuration(raw)
		if err != nil || duration <= 0 || duration > maxUsageStatsWindow {
			return time.Time{}, time.Time{}, httpError("since must be a duration between 1ns and 90d")
		}
		window = duration
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		days, err := strconv.Atoi(raw)
		if err != nil || days <= 0 || days > 90 {
			return time.Time{}, time.Time{}, httpError("days must be between 1 and 90")
		}
		window = time.Duration(days) * 24 * time.Hour
	}
	return until.Add(-window), until, nil
}

type httpError string

func (e httpError) Error() string { return string(e) }

func usageStatsLimit(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return defaultUsageStatsLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return defaultUsageStatsLimit
	}
	if limit > maxUsageStatsLimit {
		return maxUsageStatsLimit
	}
	return limit
}

func authSurface(path string) string {
	switch {
	case path == "/api/admin/usage-stats":
		return "admin_token"
	case strings.HasPrefix(path, "/api/uploads/"), strings.HasPrefix(path, "/api/paid-uploads/"):
		return "upload_token"
	case strings.HasPrefix(path, "/api/public-reports/"), strings.HasPrefix(path, "/api/public-artifacts/"), strings.HasPrefix(path, "/r/"):
		return "report_token"
	case strings.HasPrefix(path, "/email/confirm/"):
		return "email_confirmation_token"
	default:
		return "none"
	}
}

func requestAuthenticated(surface string, status int) bool {
	if surface == "none" {
		return false
	}
	return status < http.StatusBadRequest
}

func clientHash(r *http.Request) string {
	salt := os.Getenv("CLAUDE_ANALYZER_USAGE_HASH_SALT")
	if salt == "" {
		return ""
	}
	client := r.Header.Get("X-Forwarded-For")
	if client != "" {
		client = strings.TrimSpace(strings.Split(client, ",")[0])
	} else {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			client = host
		} else {
			client = r.RemoteAddr
		}
	}
	if client == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(salt + "\x00" + client))
	return hex.EncodeToString(sum[:])
}

func userAgentFamily(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case ua == "":
		return "unknown"
	case strings.Contains(ua, "curl/"):
		return "curl"
	case strings.Contains(ua, "npm/"), strings.Contains(ua, "node"):
		return "node"
	case strings.Contains(ua, "python-requests"):
		return "python-requests"
	case strings.Contains(ua, "go-http-client"):
		return "go-http-client"
	case strings.Contains(ua, "mozilla/"):
		return "browser"
	default:
		return "other"
	}
}
