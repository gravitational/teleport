package jamf

import (
	"context"
	"net/http"
	"time"

	"github.com/gravitational/trace"
)

// AuthToken is a client bearer token.
// See https://developer.jamf.com/jamf-pro/reference/post_v1-auth-token and
// https://developer.jamf.com/jamf-pro/reference/post_v1-auth-keep-alive.
type AuthToken struct {
	// Token is the bearer token for the API.
	Token string `json:"token"`
	// Expires is the token expiration time, normalized to UTC.
	Expires time.Time `json:"expires"`
}

func (c *Client) renewUserPassLocked(ctx context.Context) error {
	// Refresh deadline for user/pass bearer token.
	// Typically these tokens expire in 20m.
	const tokenRefreshDeadline = 5 * time.Minute

	t := c.currentToken
	timeLeft := time.Duration(-1)
	if t != nil {
		timeLeft = t.Expires.Sub(c.nowUTC())
	}
	if timeLeft >= tokenRefreshDeadline {
		return nil
	}

	// Attempt token refresh.
	if timeLeft > 0 {
		newToken, err := c.postAuthKeepAlive(ctx, &authKeepAliveRequest{
			Token: t.Token,
		})
		// OK, successfully refreshed.
		if err == nil {
			c.currentToken = newToken
			return nil
		}
		// NOK, try to acquire a fresh token.
		c.logger.WarnContext(ctx,
			"Jamf API: Failed to refresh AuthToken",
			"error", err,
		)
	}

	// Attempt to acquire a fresh token.
	newToken, err := c.postAuthToken(ctx, &authTokenRequest{
		Username: c.username,
		Password: c.password,
	})
	if err != nil {
		return trace.Wrap(err, "authentication against Jamf API failed")
	}

	c.currentToken = newToken
	return nil
}

type authTokenRequest struct {
	Username string
	Password string
}

// postAuthToken exchanges user/password for a bearer token.
// See https://developer.jamf.com/jamf-pro/reference/post_v1-auth-token.
func (c *Client) postAuthToken(ctx context.Context, req *authTokenRequest) (*AuthToken, error) {
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/v1/auth/token"), nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	postReq.SetBasicAuth(req.Username, req.Password)

	resp := &AuthToken{}
	if err := c.doJSONRequest(postReq, resp); err != nil {
		return nil, trace.Wrap(err)
	}
	resp.Expires = resp.Expires.UTC()
	return resp, nil
}

type authKeepAliveRequest struct {
	Token string
}

// postAuthKeepAlive refreshes a still-valid bearer token.
// See https://developer.jamf.com/jamf-pro/reference/post_v1-auth-keep-alive.
func (c *Client) postAuthKeepAlive(ctx context.Context, req *authKeepAliveRequest) (*AuthToken, error) {
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/v1/auth/keep-alive"), nil /* body */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	postReq.Header.Add("Authorization", "Bearer "+req.Token)

	resp := &AuthToken{}
	if err := c.doJSONRequest(postReq, resp); err != nil {
		return nil, trace.Wrap(err)
	}
	resp.Expires = resp.Expires.UTC()
	return resp, nil
}
