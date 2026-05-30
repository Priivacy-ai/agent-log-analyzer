//go:build !lambda

package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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
	queueURL := os.Getenv("CLAUDE_ANALYZER_EMAIL_EVENTS_QUEUE_URL")
	if queueURL == "" {
		slog.Error("email event worker missing CLAUDE_ANALYZER_EMAIL_EVENTS_QUEUE_URL")
		os.Exit(1)
	}
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(getenv("AWS_REGION", "us-east-1")))
	if err != nil {
		slog.Error("aws config failed", "error_category", "email_event_config")
		os.Exit(1)
	}
	client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	slog.Info("email event worker started")
	for {
		if err := processOnce(context.Background(), client, queueURL, store); err != nil {
			slog.Error("email event process failed", "error_category", "email_event_process")
		}
	}
}

func processOnce(ctx context.Context, client *sqs.Client, queueURL string, store app.EmailOperationsStore) error {
	output, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queueURL),
		MaxNumberOfMessages: 5,
		WaitTimeSeconds:     10,
		VisibilityTimeout:   60,
	})
	if err != nil {
		return err
	}
	for _, message := range output.Messages {
		body := []byte(aws.ToString(message.Body))
		events, err := sesmonitor.ParseSNSMessage(body)
		if err != nil {
			slog.Warn("email event parse failed", "error_category", "email_event_parse")
		} else {
			for _, event := range events {
				if event.ID == "" {
					event.ID = app.NewJobID()
				}
				if err := store.RecordEmailEvent(event); err != nil {
					return err
				}
				slog.Info(
					"email event recorded",
					"type", event.Type,
					"source", event.Source,
					"to_hash", event.EmailHash,
					"message_id", event.MessageID,
				)
			}
		}
		if message.ReceiptHandle != nil {
			if _, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl:      aws.String(queueURL),
				ReceiptHandle: message.ReceiptHandle,
			}); err != nil {
				return err
			}
		}
	}
	if len(output.Messages) == 0 {
		time.Sleep(250 * time.Millisecond)
	}
	return nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
