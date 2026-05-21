package main

import (
	"errors"
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
