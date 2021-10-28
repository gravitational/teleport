package ui

import "time"

// RecoveryCodes describes RecoveryCodes UI object.
type RecoveryCodes struct {
	// Codes are user's new recovery codes.
	Codes []string `json:"codes,omitempty"`
	// Created is when the codes were created.
	Created *time.Time `json:"created,omitempty"`
}
