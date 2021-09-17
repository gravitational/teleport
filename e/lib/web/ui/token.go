package ui

import "time"

// NodeJoinToken contains node token fields for the UI.
type NodeJoinToken struct {
	//  ID is token ID.
	ID string `json:"id"`
	// Expiry is token expiration time.
	Expiry time.Time `json:"expiry,omitempty"`
}

// RecoveryToken describes RecoveryToken UI object.
type RecoveryToken struct {
	// TokenID is token ID
	TokenID string `json:"tokenId"`
	// User is user name associated with this token
	User string `json:"username"`
	// QRCode is a QR code value
	QRCode []byte `json:"qrCode,omitempty"`
	// IsRecoverPassword is a flag that indicates if user wanted to recover password
	// or second factor.
	IsRecoverPassword bool `json:"isRecoverPassword"`
	// IsApproved is a flag that determines if recovery token is type approved or not.
	IsApproved bool `json:"isApproved,omitempty"`
}
