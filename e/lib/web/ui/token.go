package ui

import "time"

// NodeJoinToken contains node token fields for the UI.
type NodeJoinToken struct {
	//  ID is token ID.
	ID string `json:"id"`
	// Expiry is token expiration time.
	Expiry time.Time `json:"expiry,omitempty"`
}
