package sesmonitor

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/priivacy-ai/agent-log-analyzer/internal/app"
)

type snsEnvelope struct {
	Type    string `json:"Type"`
	Message string `json:"Message"`
}

type sesEvent struct {
	EventType string `json:"eventType"`
	Mail      struct {
		Timestamp   string   `json:"timestamp"`
		MessageID   string   `json:"messageId"`
		Destination []string `json:"destination"`
	} `json:"mail"`
	Bounce struct {
		BounceType        string `json:"bounceType"`
		BouncedRecipients []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"bouncedRecipients"`
	} `json:"bounce"`
	Complaint struct {
		ComplaintFeedbackType string `json:"complaintFeedbackType"`
		ComplainedRecipients  []struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"complainedRecipients"`
	} `json:"complaint"`
}

func ParseSNSMessage(body []byte) ([]app.EmailDeliveryEvent, error) {
	var envelope snsEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if !strings.EqualFold(envelope.Type, "Notification") {
		return nil, errors.New("unsupported sns envelope type")
	}
	var raw sesEvent
	if err := json.Unmarshal([]byte(envelope.Message), &raw); err != nil {
		return nil, err
	}
	eventType, ok := mapEventType(raw.EventType)
	if !ok {
		return nil, errors.New("unsupported ses event type")
	}
	timestamp := parseEventTime(raw.Mail.Timestamp)
	recipients := recipientsForEvent(raw)
	if len(recipients) == 0 {
		return nil, errors.New("ses event has no recipients")
	}
	events := make([]app.EmailDeliveryEvent, 0, len(recipients))
	for _, recipient := range recipients {
		recipient = app.NormalizeEmail(recipient)
		if recipient == "" {
			continue
		}
		events = append(events, app.EmailDeliveryEvent{
			EmailHash: app.HashEmail(recipient),
			Type:      eventType,
			Source:    "ses_event",
			MessageID: raw.Mail.MessageID,
			Detail:    eventDetail(raw, eventType),
			CreatedAt: timestamp,
		})
	}
	if len(events) == 0 {
		return nil, errors.New("ses event has no valid recipients")
	}
	return events, nil
}

func mapEventType(raw string) (app.EmailEventType, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "send":
		return app.EmailEventSend, true
	case "delivery":
		return app.EmailEventDelivery, true
	case "bounce":
		return app.EmailEventBounce, true
	case "complaint":
		return app.EmailEventComplaint, true
	case "reject":
		return app.EmailEventReject, true
	case "renderingfailure", "rendering_failure":
		return app.EmailEventRenderingFailure, true
	case "deliverydelay", "delivery_delay":
		return app.EmailEventDeliveryDelay, true
	default:
		return "", false
	}
}

func recipientsForEvent(event sesEvent) []string {
	switch strings.ToLower(strings.TrimSpace(event.EventType)) {
	case "bounce":
		recipients := make([]string, 0, len(event.Bounce.BouncedRecipients))
		for _, recipient := range event.Bounce.BouncedRecipients {
			recipients = append(recipients, recipient.EmailAddress)
		}
		return recipients
	case "complaint":
		recipients := make([]string, 0, len(event.Complaint.ComplainedRecipients))
		for _, recipient := range event.Complaint.ComplainedRecipients {
			recipients = append(recipients, recipient.EmailAddress)
		}
		return recipients
	default:
		return event.Mail.Destination
	}
}

func eventDetail(event sesEvent, eventType app.EmailEventType) string {
	switch eventType {
	case app.EmailEventBounce:
		return boundedDetail(event.Bounce.BounceType)
	case app.EmailEventComplaint:
		return boundedDetail(event.Complaint.ComplaintFeedbackType)
	default:
		return ""
	}
}

func boundedDetail(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func parseEventTime(raw string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.UTC()
	}
	return time.Now().UTC()
}
