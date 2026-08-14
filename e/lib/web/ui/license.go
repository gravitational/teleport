package ui

import (
	"time"
)

// GetLicenseResponse is the response to a GET license
type GetLicenseResponse struct {
	// PEM is the license PEM
	PEM string `json:"pem"`
	// Expiry is the license expiry date
	Expiry time.Time `json:"expiry"`
}
