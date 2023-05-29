package jamf

import "time"

// AuthToken is a client bearer token.
// See https://developer.jamf.com/jamf-pro/reference/post_v1-auth-token and
// https://developer.jamf.com/jamf-pro/reference/post_v1-auth-keep-alive.
type AuthToken struct {
	// Token is the bearer token for the API.
	Token string `json:"token"`
	// Expires is the token expiration time, normalized to UTC.
	Expires time.Time `json:"expires"`
}
