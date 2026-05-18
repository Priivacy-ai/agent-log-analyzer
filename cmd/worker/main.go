package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
	"github.com/robertDouglass/claude-log-analyzer/internal/localstore"
)

func main() {
	store, err := localstore.New(getenv("CLAUDE_ANALYZER_DATA_DIR", "/tmp/claude-log-analyzer"))
	if err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}
	interval, err := time.ParseDuration(getenv("CLAUDE_ANALYZER_WORKER_INTERVAL", "2s"))
	if err != nil {
		interval = 2 * time.Second
	}
	slog.Info("worker started", "interval", interval.String())
	for {
		if err := processOnce(store); err != nil {
			slog.Error("worker process failed", "error_category", "process_once")
		}
		time.Sleep(interval)
	}
}

func processOnce(store *localstore.Store) error {
	job, ok, err := store.ClaimNextJob()
	if err != nil || !ok {
		return err
	}
	slog.Info("job claimed", "job_id", job.ID)
	data, err := store.ReadUpload(job.UploadPath)
	if err != nil {
		_ = store.FailJob(job, err)
		return err
	}
	report, err := analyzer.Analyze(job.ID, data)
	if err != nil {
		_ = store.FailJob(job, err)
		return err
	}
	if err := store.CompleteJob(job, report); err != nil {
		return err
	}
	slog.Info("job completed", "job_id", job.ID, "score_bucket", report.AggregateEvent.ScoreBucket)
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
