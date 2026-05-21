package localstore

import (
	"errors"
	"os"
	"testing"

	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

func TestRecordEmailEventCreatesSuppression(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	emailHash := app.HashEmail("bounce@example.com")
	if _, err := store.GetEmailSuppression(emailHash); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no suppression, got %v", err)
	}
	if err := store.RecordEmailEvent(app.EmailDeliveryEvent{
		EmailHash: emailHash,
		Type:      app.EmailEventBounce,
		Source:    "ses_event",
		MessageID: "message-1",
	}); err != nil {
		t.Fatal(err)
	}
	suppression, err := store.GetEmailSuppression(emailHash)
	if err != nil {
		t.Fatal(err)
	}
	if suppression.Reason != app.EmailEventBounce {
		t.Fatalf("unexpected reason %q", suppression.Reason)
	}
	if suppression.BounceCount != 1 {
		t.Fatalf("unexpected bounce count %d", suppression.BounceCount)
	}
}

func TestRecordEmailEventDeliveryDoesNotSuppress(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	emailHash := app.HashEmail("ok@example.com")
	if err := store.RecordEmailEvent(app.EmailDeliveryEvent{
		EmailHash: emailHash,
		Type:      app.EmailEventDelivery,
		Source:    "ses_event",
		MessageID: "message-2",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetEmailSuppression(emailHash); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no suppression, got %v", err)
	}
}
