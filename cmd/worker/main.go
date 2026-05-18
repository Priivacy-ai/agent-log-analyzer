package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/robertDouglass/claude-log-analyzer/internal/analyzer"
	"github.com/robertDouglass/claude-log-analyzer/internal/app"
	"github.com/robertDouglass/claude-log-analyzer/internal/backend"
	"github.com/robertDouglass/claude-log-analyzer/internal/paidscan"
)

func main() {
	store, err := backend.NewWorkerStore()
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

func processOnce(store app.WorkerStore) error {
	job, ok, err := store.ClaimNextJob()
	if err != nil || !ok {
		return err
	}
	slog.Info("job claimed")
	data, err := store.ReadUpload(job.UploadPath)
	if err != nil {
		_ = store.FailJob(job, err)
		return err
	}
	var report analyzer.Report
	if job.ScanType == app.ScanTypePaidBundle {
		report, err = paidscan.AnalyzeBundle(job.ID, data, paidscan.Options{MaxFiles: paidscan.DefaultMaxFiles})
	} else {
		report, err = analyzer.Analyze(job.ID, data)
	}
	if err != nil {
		_ = store.FailJob(job, err)
		return err
	}
	if err := store.CompleteJob(job, report); err != nil {
		return err
	}
	slog.Info(
		"job completed",
		"parser_type", report.AggregateEvent.ParserType,
		"input_size_bucket", report.AggregateEvent.InputSizeBucket,
		"turn_bucket", report.AggregateEvent.TurnBucket,
		"score_bucket", report.AggregateEvent.ScoreBucket,
		"waste_bucket", report.AggregateEvent.WasteBucket,
		"findings", report.AggregateEvent.Findings,
		"redactions", report.AggregateEvent.Redactions,
		"ecosystem", report.AggregateEvent.Ecosystem,
	)
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
