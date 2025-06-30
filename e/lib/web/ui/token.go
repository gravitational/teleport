package ui

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

// OauthTokenResponse is the response from the OAuth token endpoint.
type OauthTokenResponse struct {
	// AccessToken is the token that authorizes and authenticates
	// the requests.
	AccessToken string `json:"access_token"`
	// TokenType is the type of token.
	// The Type method returns either this or "Bearer", the default.
	TokenType string `json:"token_type"`
	// ExpiresIn is the OAuth2 wire format "expires_in" field,
	// which specifies how many seconds later the token expires,
	// relative to an unknown time base approximately around "now".
	ExpiresIn int64 `json:"expires_in"`
}
