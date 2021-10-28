package ui

import "time"

// AccountRecoveryCode describes AccountRecoveryCode UI object.
type AccountRecoveryCode struct {
	// Codes are user's new recovery codes.
	Codes []string `json:"codes,omitempty"`
	// Created is when the codes were created.
	Created *time.Time `json:"created,omitempty"`
}
