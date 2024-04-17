package jamf

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gravitational/trace"
)

const (
	// maxRepeatedAuthnFailures is the maximum number of repeated authn attempts.
	// After this many attempts the client assumes the credentials themselves are
	// invalid and stops trying.
	maxRepeatedAuthnFailures = 2

	// tokenRefreshDeadline is the deadline after which a bearer token refresh is
	// attempted.
	// The token expiration time must be <= to the deadline for it to happen.
	tokenRefreshDeadline = 5 * time.Minute
)

// ErrMaxAuthnAttemptsReached is returned when too many authentication failures
// happen in sequence.
// Once the client reaches this state it won't recover.
var ErrMaxAuthnAttemptsReached = errors.New("max authentication attempts reached, are the credentials correct?")

// AuthToken is a client bearer token.
// See https://developer.jamf.com/jamf-pro/reference/post_v1-auth-token and
// https://developer.jamf.com/jamf-pro/reference/post_v1-auth-keep-alive.
type AuthToken struct {
	// Token is the bearer token for the API.
	Token string `json:"token"`
	// Expires is the token expiration time, normalized to UTC.
	Expires time.Time `json:"expires"`
}

func (c *Client) doAuthnJSONRequest(req *http.Request, jsonResp any) error {
	allowRetry := true // One retry attempt allowed.
	for {
		token, err := c.createOrRenewAuthToken(req.Context())
		if err != nil {
			return trace.Wrap(err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		err = c.doJSONRequest(req, jsonResp)
		if err == nil || !allowRetry {
			return trace.Wrap(err)
		}

		// If we got a 401 attempt a single token renewal.
		// This may happen if our existing auth token got invalidated.
		apiErr := &APIError{}
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
			return trace.Wrap(err)
		}
		c.logger.WarnContext(req.Context(), "Jamf API: Existing auth token invalidated, attempting renewal")

		allowRetry = false
		c.mu.Lock()
		c.currentToken = nil
		c.mu.Unlock()
	}
}

func (c *Client) createOrRenewAuthToken(ctx context.Context) (string, error) {
	// Hold the lock until we get have a bearer token. This is fine - we don't
	// really expect the client to be used for loads of concurrent access, plus
	// it's a simple way to avoid bursting authn endpoints.
	c.mu.Lock()
	defer c.mu.Unlock()

	t := c.currentToken
	timeLeft := time.Duration(-1)
	if t != nil {
		timeLeft = t.Expires.Sub(c.nowUTC())
	}
	if timeLeft >= tokenRefreshDeadline {
		return t.Token, nil
	}

	// Attempt token refresh.
	if timeLeft > 0 {
		newToken, err := c.postAuthKeepAlive(ctx, &authKeepAliveRequest{
			Token: t.Token,
		})
		// OK, successfully refreshed.
		if err == nil {
			c.currentToken = newToken
			c.repeatedAuthnFailures = 0
			return c.currentToken.Token, nil
		}
		// NOK, try to acquire a fresh token.
		c.logger.WarnContext(ctx,
			"Jamf API: Failed to refresh AuthToken",
			"error", err,
		)
	}

	// Have we failed authn too many times?
	if c.repeatedAuthnFailures >= maxRepeatedAuthnFailures {
		return "", trace.Wrap(ErrMaxAuthnAttemptsReached)
	}

	// Attempt to acquire a fresh token.
	newToken, err := c.postAuthToken(ctx, &authTokenRequest{
		Username: c.username,
		Password: c.password,
	})
	if err != nil {
		c.repeatedAuthnFailures++
		return "", trace.Wrap(err, "authentication against Jamf API failed")
	}
	c.currentToken = newToken
	c.repeatedAuthnFailures = 0
	return c.currentToken.Token, nil
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
