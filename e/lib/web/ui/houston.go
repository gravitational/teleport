package ui

import "github.com/gravitational/reporting/types"

// HoustonReport is houston report
type HoustonReport struct {
	// Type is the notification type
	Type string `json:"type"`
	// Severity is the notification severity: info, warning or error
	Severity string `json:"severity"`
	// Text is the notification plain text
	Text string `json:"text"`
	// HTML is the notification HTML
	HTML string `json:"html"`
}

// NewHoustonReport creates houston report
func NewHoustonReport(hb *types.Heartbeat) *HoustonReport {
	messages := hb.Spec.Notifications
	if len(messages) == 0 {
		return nil
	}

	message := messages[0]
	return &HoustonReport{
		Text:     message.Text,
		HTML:     message.HTML,
		Type:     message.Type,
		Severity: message.Severity,
	}
}
