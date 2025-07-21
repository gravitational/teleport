package jamf

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// AccessToken is a client bearer token acquired using API credentials.
// See https://developer.jamf.com/jamf-pro/docs/client-credentials.
type AccessToken struct {
	// AccessToken is the access token proper.
	AccessToken string `json:"access_token"`
	// Scope is the scope of the token.
	// Example: "api-role:1".
	Scope string `json:"scope"`
	// TokenType is the type of the token.
	// Example: "Bearer".
	TokenType string `json:"token_type"`
	// ExpiresIn is the expiration time of the token, in seconds, counting
	// from its creation.
	ExpiresIn int `json:"expires_in"`
	// Expires is the token expiration time, normalized to UTC.
	// Calculated using [ExpiresIn] and the [Client] clock.
	// This is not supplied by the Jamf API.
	Expires time.Time `json:"-"`
}

// GetAccessToken returns the access token proper.
func (t *AccessToken) GetAccessToken() string {
	if t == nil {
		return ""
	}
	return t.AccessToken
}

// GetExpires returns the token expiry time.
func (t *AccessToken) GetExpires() time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.Expires
}

type clientSecretCreds struct {
	clientID, clientSecret string
}

func (c *Client) renewClientSecretLocked(ctx context.Context) error {
	// We want the token to be valid for the next 5 seconds to avoid issues in case of time skew.
	// Typically these tokens expire in 1m.
	const minValidFor = 5 * time.Second
	if c.currentToken != nil && c.currentToken.GetExpires().Sub(c.nowUTC()) > minValidFor {
		return nil
	}

	// Acquire a new access token.
	newToken, err := c.postOauthToken(ctx, &postOauthTokenRequest{
		clientID:     c.clientSecret.clientID,
		clientSecret: c.clientSecret.clientSecret,
	})
	if err != nil {
		return trace.Wrap(err, "authentication against Jamf client credentials API failed")
	}
	c.currentToken = newToken
	return nil
}

type postOauthTokenRequest struct {
	clientID     string
	clientSecret string
}

// postOauthToken uses the clientID and secret to acquire an [AccessToken].
// See https://developer.jamf.com/jamf-pro/docs/client-credentials#access-tokens
func (c *Client) postOauthToken(ctx context.Context, req *postOauthTokenRequest) (*AccessToken, error) {
	form := url.Values{}
	form.Set("client_id", req.clientID)
	form.Set("client_secret", req.clientSecret)
	form.Set("grant_type", "client_credentials")

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/oauth/token"), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp := &AccessToken{}
	if err := c.doJSONRequest(postReq, resp); err != nil {
		return nil, trace.Wrap(err)
	}
	resp.Expires = c.nowUTC().Add(time.Duration(resp.ExpiresIn) * time.Second)
	return resp, nil
}
