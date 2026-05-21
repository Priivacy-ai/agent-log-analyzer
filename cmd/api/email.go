package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type emailMessage struct {
	To      string
	Subject string
	Body    string
}

type emailSender interface {
	Send(message emailMessage) error
}

type loggingEmailSender struct{}

func (loggingEmailSender) Send(message emailMessage) error {
	slog.Info("email queued", "to_hash", tokenHash(strings.ToLower(strings.TrimSpace(message.To))), "subject", message.Subject)
	return nil
}

type fileEmailSender struct {
	dir string
}

type sesEmailSender struct {
	client *sesv2.Client
	from   string
}

func (sender sesEmailSender) Send(message emailMessage) error {
	_, err := sender.client.SendEmail(context.Background(), &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(sender.from),
		Destination: &sestypes.Destination{
			ToAddresses: []string{message.To},
		},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{Data: aws.String(message.Subject)},
				Body: &sestypes.Body{
					Text: &sestypes.Content{Data: aws.String(message.Body)},
				},
			},
		},
	})
	return err
}

func (sender fileEmailSender) Send(message emailMessage) error {
	if err := os.MkdirAll(sender.dir, 0o700); err != nil {
		return err
	}
	name := fmt.Sprintf("%d-%s.eml", time.Now().UTC().UnixNano(), safeEmailFilename(message.To))
	path := filepath.Join(sender.dir, name)
	body := strings.Join([]string{
		"To: " + message.To,
		"Subject: " + message.Subject,
		"",
		message.Body,
		"",
	}, "\n")
	return os.WriteFile(path, []byte(body), 0o600)
}

func configuredEmailSender() emailSender {
	if dir := os.Getenv("CLAUDE_ANALYZER_EMAIL_SINK_DIR"); dir != "" {
		return fileEmailSender{dir: dir}
	}
	if strings.EqualFold(os.Getenv("CLAUDE_ANALYZER_EMAIL_PROVIDER"), "ses") {
		from := strings.TrimSpace(os.Getenv("CLAUDE_ANALYZER_EMAIL_FROM"))
		if from == "" {
			slog.Error("SES email provider configured without CLAUDE_ANALYZER_EMAIL_FROM")
			return loggingEmailSender{}
		}
		cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(getenv("AWS_REGION", "us-east-1")))
		if err != nil {
			slog.Error("SES email provider configuration failed", "error_category", "email_provider")
			return loggingEmailSender{}
		}
		return sesEmailSender{client: sesv2.NewFromConfig(cfg), from: from}
	}
	return loggingEmailSender{}
}

func safeEmailFilename(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	var b strings.Builder
	for _, r := range email {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	if b.Len() == 0 {
		return "recipient"
	}
	return b.String()
}
