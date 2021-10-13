package ui

import "time"

// AccountRecoveryCodesMetadata describes AccountRecoveryCodesMetadata UI object.
type AccountRecoveryCodesMetadata struct {
	// Created is when the codes were created.
	Created *time.Time `json:"created,omitempty"`
}
