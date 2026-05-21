package app

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type EmailEventType string

const (
	EmailEventSend             EmailEventType = "send"
	EmailEventSendFailure      EmailEventType = "send_failure"
	EmailEventDelivery         EmailEventType = "delivery"
	EmailEventBounce           EmailEventType = "bounce"
	EmailEventComplaint        EmailEventType = "complaint"
	EmailEventReject           EmailEventType = "reject"
	EmailEventRenderingFailure EmailEventType = "rendering_failure"
	EmailEventDeliveryDelay    EmailEventType = "delivery_delay"
)

type EmailDeliveryEvent struct {
	ID        string         `json:"id"`
	EmailHash string         `json:"email_hash"`
	Type      EmailEventType `json:"type"`
	Source    string         `json:"source"`
	MessageID string         `json:"message_id,omitempty"`
	Detail    string         `json:"detail,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type EmailSuppression struct {
	EmailHash      string         `json:"email_hash"`
	Reason         EmailEventType `json:"reason"`
	BounceCount    int            `json:"bounce_count,omitempty"`
	ComplaintCount int            `json:"complaint_count,omitempty"`
	RejectCount    int            `json:"reject_count,omitempty"`
	LastMessageID  string         `json:"last_message_id,omitempty"`
	SuppressedAt   time.Time      `json:"suppressed_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func HashEmail(email string) string {
	sum := sha256.Sum256([]byte(NormalizeEmail(email)))
	return hex.EncodeToString(sum[:])
}

func (event EmailDeliveryEvent) IsSuppressing() bool {
	switch event.Type {
	case EmailEventBounce, EmailEventComplaint, EmailEventReject:
		return true
	default:
		return false
	}
}
