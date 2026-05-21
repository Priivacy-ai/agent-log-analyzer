package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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

type postmarkEmailSender struct {
	client        *http.Client
	apiURL        string
	serverToken   string
	from          string
	messageStream string
}

type postmarkEmailRequest struct {
	From          string `json:"From"`
	To            string `json:"To"`
	Subject       string `json:"Subject"`
	TextBody      string `json:"TextBody"`
	MessageStream string `json:"MessageStream,omitempty"`
	TrackOpens    bool   `json:"TrackOpens"`
	TrackLinks    string `json:"TrackLinks"`
}

type postmarkEmailResponse struct {
	ErrorCode int    `json:"ErrorCode"`
	Message   string `json:"Message"`
	MessageID string `json:"MessageID"`
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

func (sender postmarkEmailSender) Send(message emailMessage) error {
	requestBody := postmarkEmailRequest{
		From:          sender.from,
		To:            message.To,
		Subject:       message.Subject,
		TextBody:      message.Body,
		MessageStream: sender.messageStream,
		TrackOpens:    false,
		TrackLinks:    "None",
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, sender.apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Postmark-Server-Token", sender.serverToken)
	resp, err := sender.client.Do(req)
	if err != nil {
		return errEmailDelivery{provider: "postmark", detail: "network", err: err}
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if readErr != nil {
		return readErr
	}
	var postmarkResp postmarkEmailResponse
	if len(responseBody) > 0 {
		_ = json.Unmarshal(responseBody, &postmarkResp)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || postmarkResp.ErrorCode != 0 {
		return errEmailDelivery{
			provider: "postmark",
			detail:   classifyPostmarkError(resp.StatusCode, postmarkResp),
			err:      fmt.Errorf("postmark status %d error code %d", resp.StatusCode, postmarkResp.ErrorCode),
		}
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
	if strings.EqualFold(os.Getenv("CLAUDE_ANALYZER_EMAIL_PROVIDER"), "postmark") {
		from := strings.TrimSpace(os.Getenv("CLAUDE_ANALYZER_EMAIL_FROM"))
		token := strings.TrimSpace(os.Getenv("POSTMARK_SERVER_TOKEN"))
		if from == "" || token == "" {
			slog.Error("Postmark email provider configured without required sender or token", "error_category", "email_provider")
			return loggingEmailSender{}
		}
		return postmarkEmailSender{
			client:        &http.Client{Timeout: 10 * time.Second},
			apiURL:        getenv("CLAUDE_ANALYZER_POSTMARK_API_URL", "https://api.postmarkapp.com/email"),
			serverToken:   token,
			from:          from,
			messageStream: strings.TrimSpace(getenv("CLAUDE_ANALYZER_POSTMARK_MESSAGE_STREAM", "outbound")),
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

func classifyPostmarkError(statusCode int, response postmarkEmailResponse) string {
	message := strings.ToLower(response.Message)
	switch {
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return "access_denied"
	case strings.Contains(message, "inactive recipient") || strings.Contains(message, "suppression"):
		return "suppressed_recipient"
	case strings.Contains(message, "sender signature") || strings.Contains(message, "from"):
		return "sender_unverified"
	case strings.Contains(message, "recipient") || strings.Contains(message, "to"):
		return "recipient_rejected"
	case statusCode == http.StatusTooManyRequests:
		return "rate_limited"
	case statusCode >= 500:
		return "provider_unavailable"
	default:
		return "provider_error"
	}
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
