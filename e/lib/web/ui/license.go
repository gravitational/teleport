package ui

import (
	"time"

	"github.com/gravitational/reporting/types"
)

// LicenseCheckStatus is the license status
type LicenseCheckStatus struct {
	// Type is the notification type
	Type string `json:"type"`
	// Severity is the notification severity: info, warning or error
	Severity string `json:"severity"`
	// Text is the notification plain text
	Text string `json:"text"`
	// HTML is the notification HTML
	HTML string `json:"html"`
}

// GetLicenseResponse is the response to a GET license
type GetLicenseResponse struct {
	// PEM is the license PEM
	PEM string `json:"pem"`
	// Expiry is the license expiry date
	Expiry time.Time `json:"expiry"`
}

// NewLicenseCheckStatus creates houston status
func NewLicenseCheckStatus(licenseCheckResult *types.Heartbeat) *LicenseCheckStatus {
	messages := licenseCheckResult.Spec.Notifications
	if len(messages) == 0 {
		return nil
	}

	message := messages[0]
	return &LicenseCheckStatus{
		Text:     message.Text,
		HTML:     message.HTML,
		Type:     message.Type,
		Severity: message.Severity,
	}
}
