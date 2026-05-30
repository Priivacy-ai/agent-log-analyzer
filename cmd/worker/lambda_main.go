//go:build lambda

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analytics"
	"github.com/priivacy-ai/agent-log-analyzer/internal/analyzer"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
	"github.com/priivacy-ai/agent-log-analyzer/internal/backend"
	"github.com/priivacy-ai/agent-log-analyzer/internal/paidscan"
)

func main() {
	store, err := backend.NewWorkerStore()
	if err != nil {
		slog.Error("store init failed", "error", err)
		os.Exit(1)
	}
	lambda.Start(func(ctx context.Context, event events.SQSEvent) error {
		return handleSQSEvent(ctx, store, event)
	})
}

func handleSQSEvent(_ context.Context, store app.WorkerStore, event events.SQSEvent) error {
	var batchErr error
	for _, record := range event.Records {
		if err := processRecord(store, record.Body); err != nil {
			slog.Error("worker record failed", "error_category", "process_record")
			batchErr = err
		}
	}
	return batchErr
}

func processRecord(store app.WorkerStore, body string) error {
	var payload map[string]string
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return err
	}
	jobID := payload["job_id"]
	if jobID == "" {
		return errors.New("SQS message missing job_id")
	}
	job, err := store.GetJob(jobID)
	if err != nil {
		return err
	}
	if job.Status == app.StatusCompleted {
		return nil
	}
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
	if analyticsStore, ok := store.(app.AnalyticsStore); ok {
		if err := analyticsStore.AppendAnalyticsEvent(analytics.FromReport(report, string(job.ScanType))); err != nil {
			slog.Warn("analytics event append failed", "error_category", "analytics_append")
		}
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
