package sms_events

import (
	"time"

	"github.com/google/uuid"
)

// EventTypeMessagePhoneReceived is emitted when a new message is received by a mobile phone
const EventTypeMessagePhoneReceived = "message.phone.received"

// MessagePhoneReceivedPayload is the payload of the EventTypeMessagePhoneReceived event
type MessagePhoneReceivedPayload struct {
	MessageID uuid.UUID `json:"message_id"`
	UserID    string    `json:"user_id"`
	Owner     string    `json:"owner"`
	Encrypted bool      `json:"encrypted"`
	Contact   string    `json:"contact"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	SIM       string    `json:"sim"` // SIM1 or SIM2
}
