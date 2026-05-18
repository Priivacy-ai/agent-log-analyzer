package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/robertdouglass/claude-log-analyzer/internal/backend"
)

func main() {
	store, err := backend.NewSweeperStore()
	if err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}
	uploadTTL := mustDuration(getenv("CLAUDE_ANALYZER_UPLOAD_TTL", "15m"))
	reportTTL := mustDuration(getenv("CLAUDE_ANALYZER_REPORT_TTL", "15m"))
	result, err := store.SweepExpired(time.Now().UTC(), uploadTTL, reportTTL)
	if err != nil {
		slog.Error("sweep failed", "error_category", "retention_sweep")
		os.Exit(1)
	}
	slog.Info("sweep completed", "uploads_deleted", result.UploadsDeleted, "reports_deleted", result.ReportsDeleted)
}

func mustDuration(value string) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		slog.Error("invalid duration", "error_category", "configuration")
		os.Exit(1)
	}
	return duration
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
