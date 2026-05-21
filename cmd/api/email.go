package main

import (
	"context"
	"errors"
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
	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type emailMessage struct {
	To      string
	Subject string
	Body    string
}

type emailSender interface {
	Send(message emailMessage) error
}

type errEmailDelivery struct {
	provider string
	detail   string
	err      error
}

func (err errEmailDelivery) Error() string {
	if err.detail == "" {
		return err.provider + " email delivery failed"
	}
	return err.provider + " email delivery failed: " + err.detail
}

func (err errEmailDelivery) Unwrap() error {
	return err.err
}

type loggingEmailSender struct{}

func (loggingEmailSender) Send(message emailMessage) error {
	slog.Info("email queued", "to_hash", app.HashEmail(message.To), "subject", message.Subject)
	return nil
}

type fileEmailSender struct {
	dir string
}

type sesEmailSender struct {
	client           *sesv2.Client
	from             string
	configurationSet string
}

func (sender sesEmailSender) Send(message emailMessage) error {
	input := &sesv2.SendEmailInput{
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
	}
	if sender.configurationSet != "" {
		input.ConfigurationSetName = aws.String(sender.configurationSet)
	}
	_, err := sender.client.SendEmail(context.Background(), input)
	if err != nil {
		return errEmailDelivery{provider: "ses", detail: classifySESError(err), err: err}
	}
	return nil
}

type errEmailSuppressed struct {
	emailHash string
	reason    app.EmailEventType
}

func (err errEmailSuppressed) Error() string {
	return "recipient suppressed"
}

type suppressionGuardedEmailSender struct {
	next  emailSender
	store app.EmailOperationsStore
}

func (sender suppressionGuardedEmailSender) Send(message emailMessage) error {
	emailHash := app.HashEmail(message.To)
	suppression, err := sender.store.GetEmailSuppression(emailHash)
	if err == nil {
		return errEmailSuppressed{emailHash: emailHash, reason: suppression.Reason}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	err = sender.next.Send(message)
	eventType := app.EmailEventSend
	detail := ""
	if err != nil {
		eventType = app.EmailEventSendFailure
		detail = emailFailureDetail(err)
	}
	recordErr := sender.store.RecordEmailEvent(app.EmailDeliveryEvent{
		EmailHash: emailHash,
		Type:      eventType,
		Source:    "app_send",
		Detail:    detail,
	})
	if err != nil {
		return err
	}
	return recordErr
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
		return sesEmailSender{
			client:           sesv2.NewFromConfig(cfg),
			from:             from,
			configurationSet: strings.TrimSpace(os.Getenv("CLAUDE_ANALYZER_SES_CONFIGURATION_SET")),
		}
	}
	return loggingEmailSender{}
}

func guardEmailSender(sender emailSender, store app.APIStore) emailSender {
	emailOps, ok := store.(app.EmailOperationsStore)
	if !ok {
		return sender
	}
	return suppressionGuardedEmailSender{next: sender, store: emailOps}
}

func emailFailureDetail(err error) string {
	var delivery errEmailDelivery
	if errors.As(err, &delivery) && delivery.detail != "" {
		return delivery.provider + "_" + delivery.detail
	}
	return "provider_error"
}

func classifySESError(err error) string {
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "sandbox"):
		return "sandbox"
	case strings.Contains(message, "not verified") || strings.Contains(message, "verified"):
		return "identity_unverified"
	case strings.Contains(message, "configuration set"):
		return "configuration_set"
	case strings.Contains(message, "message rejected"):
		return "message_rejected"
	case strings.Contains(message, "accessdenied") || strings.Contains(message, "access denied"):
		return "access_denied"
	default:
		return "provider_error"
	}
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
