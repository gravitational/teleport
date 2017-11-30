package ui

import "github.com/gravitational/reporting/types"

// HoustonStatus is houston status
type HoustonStatus struct {
	// Type is the notification type
	Type string `json:"type"`
	// Severity is the notification severity: info, warning or error
	Severity string `json:"severity"`
	// Text is the notification plain text
	Text string `json:"text"`
	// HTML is the notification HTML
	HTML string `json:"html"`
}

// NewHoustonStatus creates houston status
func NewHoustonStatus(hb *types.Heartbeat) *HoustonStatus {
	messages := hb.Spec.Notifications
	if len(messages) == 0 {
		return nil
	}

	message := messages[0]
	return &HoustonStatus{
		Text:     message.Text,
		HTML:     message.HTML,
		Type:     message.Type,
		Severity: message.Severity,
	}
}
