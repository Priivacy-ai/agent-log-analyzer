//go:build lambda

package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
	"github.com/priivacy-ai/agent-log-analyzer/internal/backend"
	"github.com/priivacy-ai/agent-log-analyzer/internal/sesmonitor"
)

func main() {
	store, err := backend.NewEmailOperationsStore()
	if err != nil {
		slog.Error("store init failed", "error_category", "email_event_store")
		os.Exit(1)
	}
	lambda.Start(func(ctx context.Context, event events.SQSEvent) error {
		return handleEmailSQSEvent(ctx, store, event)
	})
}

func handleEmailSQSEvent(_ context.Context, store app.EmailOperationsStore, event events.SQSEvent) error {
	var batchErr error
	for _, record := range event.Records {
		parsed, err := sesmonitor.ParseSNSMessage([]byte(record.Body))
		if err != nil {
			slog.Warn("email event parse failed", "error_category", "email_event_parse")
			continue
		}
		for _, emailEvent := range parsed {
			if emailEvent.ID == "" {
				emailEvent.ID = app.NewJobID()
			}
			if err := store.RecordEmailEvent(emailEvent); err != nil {
				batchErr = err
				continue
			}
			slog.Info(
				"email event recorded",
				"type", emailEvent.Type,
				"source", emailEvent.Source,
				"to_hash", emailEvent.EmailHash,
				"message_id", emailEvent.MessageID,
			)
		}
	}
	return batchErr
}
