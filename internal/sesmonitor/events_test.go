package sesmonitor

import (
	"encoding/json"
	"testing"

	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

func TestParseSNSMessageBounceSuppressing(t *testing.T) {
	body := snsBody(t, map[string]any{
		"eventType": "Bounce",
		"mail": map[string]any{
			"timestamp":   "2026-05-21T10:00:00Z",
			"messageId":   "message-1",
			"destination": []string{"fallback@example.com"},
		},
		"bounce": map[string]any{
			"bounceType": "Permanent",
			"bouncedRecipients": []map[string]string{
				{"emailAddress": "USER@example.com"},
			},
		},
	})
	events, err := ParseSNSMessage(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected one event, got %d", len(events))
	}
	event := events[0]
	if event.Type != app.EmailEventBounce {
		t.Fatalf("expected bounce, got %q", event.Type)
	}
	if !event.IsSuppressing() {
		t.Fatal("bounce should suppress future sends")
	}
	if event.EmailHash != app.HashEmail("user@example.com") {
		t.Fatal("email hash mismatch")
	}
	if event.Detail != "permanent" {
		t.Fatalf("unexpected detail %q", event.Detail)
	}
}

func TestParseSNSMessageComplaintSuppressing(t *testing.T) {
	body := snsBody(t, map[string]any{
		"eventType": "Complaint",
		"mail": map[string]any{
			"timestamp": "2026-05-21T10:00:00Z",
			"messageId": "message-2",
		},
		"complaint": map[string]any{
			"complaintFeedbackType": "abuse",
			"complainedRecipients": []map[string]string{
				{"emailAddress": "complaint@example.com"},
			},
		},
	})
	events, err := ParseSNSMessage(body)
	if err != nil {
		t.Fatal(err)
	}
	if events[0].Type != app.EmailEventComplaint || !events[0].IsSuppressing() {
		t.Fatalf("unexpected event %#v", events[0])
	}
}

func TestParseSNSMessageDeliveryDoesNotSuppress(t *testing.T) {
	body := snsBody(t, map[string]any{
		"eventType": "Delivery",
		"mail": map[string]any{
			"timestamp":   "2026-05-21T10:00:00Z",
			"messageId":   "message-3",
			"destination": []string{"ok@example.com"},
		},
	})
	events, err := ParseSNSMessage(body)
	if err != nil {
		t.Fatal(err)
	}
	if events[0].Type != app.EmailEventDelivery {
		t.Fatalf("expected delivery, got %q", events[0].Type)
	}
	if events[0].IsSuppressing() {
		t.Fatal("delivery should not suppress future sends")
	}
}

func snsBody(t *testing.T, message map[string]any) []byte {
	t.Helper()
	messageBytes, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	envelopeBytes, err := json.Marshal(map[string]string{
		"Type":    "Notification",
		"Message": string(messageBytes),
	})
	if err != nil {
		t.Fatal(err)
	}
	return envelopeBytes
}
