package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type recordingSender struct {
	sent int
	err  error
}

func (sender *recordingSender) Send(emailMessage) error {
	sender.sent++
	return sender.err
}

func TestSuppressionGuardBlocksSuppressedRecipient(t *testing.T) {
	store := newMemoryEmailOps()
	emailHash := app.HashEmail("blocked@example.com")
	store.suppressions[emailHash] = app.EmailSuppression{
		EmailHash: emailHash,
		Reason:    app.EmailEventComplaint,
	}
	next := &recordingSender{}
	err := suppressionGuardedEmailSender{next: next, store: store}.Send(emailMessage{To: "blocked@example.com"})
	if !errors.As(err, &errEmailSuppressed{}) {
		t.Fatalf("expected errEmailSuppressed, got %v", err)
	}
	if next.sent != 0 {
		t.Fatal("suppressed recipient should not be sent to provider")
	}
}

func TestSuppressionGuardRecordsSendFailureWithoutSuppressing(t *testing.T) {
	store := newMemoryEmailOps()
	next := &recordingSender{err: errors.New("provider down")}
	err := suppressionGuardedEmailSender{next: next, store: store}.Send(emailMessage{To: "fail@example.com"})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if len(store.events) != 1 {
		t.Fatalf("expected one event, got %d", len(store.events))
	}
	if store.events[0].Type != app.EmailEventSendFailure {
		t.Fatalf("expected send_failure, got %q", store.events[0].Type)
	}
	if _, err := store.GetEmailSuppression(app.HashEmail("fail@example.com")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("send failure should not suppress recipient, got %v", err)
	}
}

func TestPostmarkEmailSenderSendsTransactionalMessage(t *testing.T) {
	var gotToken string
	var got postmarkEmailRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Postmark-Server-Token")
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("invalid postmark request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ErrorCode":0,"Message":"OK","MessageID":"msg-123"}`))
	}))
	defer server.Close()

	sender := postmarkEmailSender{
		client:        server.Client(),
		apiURL:        server.URL,
		serverToken:   "server-token",
		from:          "robert@spec-kitty.ai",
		messageStream: "outbound",
	}
	err := sender.Send(emailMessage{To: "dev@example.com", Subject: "Confirm", Body: "body"})

	if err != nil {
		t.Fatalf("expected postmark send success, got %v", err)
	}
	if gotToken != "server-token" {
		t.Fatalf("expected server token header, got %q", gotToken)
	}
	if got.From != "robert@spec-kitty.ai" || got.To != "dev@example.com" || got.Subject != "Confirm" || got.TextBody != "body" {
		t.Fatalf("unexpected request body: %#v", got)
	}
	if got.MessageStream != "outbound" {
		t.Fatalf("expected outbound message stream, got %q", got.MessageStream)
	}
	if got.TrackOpens || got.TrackLinks != "None" {
		t.Fatalf("expected tracking disabled, got opens=%v links=%q", got.TrackOpens, got.TrackLinks)
	}
}

func TestPostmarkEmailSenderClassifiesProviderErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"ErrorCode":300,"Message":"Sender signature not confirmed"}`))
	}))
	defer server.Close()

	sender := postmarkEmailSender{
		client:      server.Client(),
		apiURL:      server.URL,
		serverToken: "server-token",
		from:        "robert@spec-kitty.ai",
	}
	err := sender.Send(emailMessage{To: "dev@example.com", Subject: "Confirm", Body: "body"})

	var delivery errEmailDelivery
	if !errors.As(err, &delivery) {
		t.Fatalf("expected errEmailDelivery, got %v", err)
	}
	if delivery.provider != "postmark" || delivery.detail != "sender_unverified" {
		t.Fatalf("unexpected delivery classification: %#v", delivery)
	}
	if detail := emailFailureDetail(err); detail != "postmark_sender_unverified" {
		t.Fatalf("unexpected failure detail %q", detail)
	}
}

func TestConfiguredEmailSenderSelectsPostmark(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_EMAIL_PROVIDER", "postmark")
	t.Setenv("CLAUDE_ANALYZER_EMAIL_FROM", "robert@spec-kitty.ai")
	t.Setenv("POSTMARK_SERVER_TOKEN", "server-token")
	t.Setenv("CLAUDE_ANALYZER_POSTMARK_MESSAGE_STREAM", "outbound")

	sender, ok := configuredEmailSender().(postmarkEmailSender)
	if !ok {
		t.Fatalf("expected postmarkEmailSender, got %T", configuredEmailSender())
	}
	if sender.from != "robert@spec-kitty.ai" || sender.serverToken != "server-token" || sender.messageStream != "outbound" {
		t.Fatalf("unexpected postmark config: %#v", sender)
	}
}

func TestConfiguredEmailSenderKeepsSES(t *testing.T) {
	t.Setenv("CLAUDE_ANALYZER_EMAIL_PROVIDER", "ses")
	t.Setenv("CLAUDE_ANALYZER_EMAIL_FROM", "reports@spec-kitty.ai")

	if _, ok := configuredEmailSender().(sesEmailSender); !ok {
		t.Fatalf("expected sesEmailSender for SES provider, got %T", configuredEmailSender())
	}
}

type memoryEmailOps struct {
	suppressions map[string]app.EmailSuppression
	events       []app.EmailDeliveryEvent
}

func newMemoryEmailOps() *memoryEmailOps {
	return &memoryEmailOps{suppressions: map[string]app.EmailSuppression{}}
}

func (store *memoryEmailOps) GetEmailSuppression(emailHash string) (app.EmailSuppression, error) {
	suppression, ok := store.suppressions[emailHash]
	if !ok {
		return app.EmailSuppression{}, os.ErrNotExist
	}
	return suppression, nil
}

func (store *memoryEmailOps) RecordEmailEvent(event app.EmailDeliveryEvent) error {
	store.events = append(store.events, event)
	if event.IsSuppressing() {
		store.suppressions[event.EmailHash] = app.EmailSuppression{
			EmailHash: event.EmailHash,
			Reason:    event.Type,
		}
	}
	return nil
}
