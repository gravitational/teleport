package jamf

import (
	"context"
	"errors"
	"net/http"

	"github.com/gravitational/trace"
)

// ErrMaxAuthnAttemptsReached is returned when too many authentication failures
// happen in sequence.
// Once the client reaches this state it won't recover.
var ErrMaxAuthnAttemptsReached = errors.New("max authentication attempts reached, are the credentials correct?")

// maxRepeatedAuthnFailures is the maximum number of repeated authn attempts.
// After this many attempts the client assumes the credentials themselves are
// invalid and stops trying.
const maxRepeatedAuthnFailures = 2

func (c *Client) doAuthnJSONRequest(req *http.Request, jsonResp any) error {
	allowRetry := true // One retry attempt allowed.
	for {
		token, err := c.createOrRenewCurrentToken(req.Context())
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

func (c *Client) createOrRenewCurrentToken(ctx context.Context) (string, error) {
	// Hold the lock until we get have a bearer token. This is fine - we don't
	// really expect the client to be used for loads of concurrent access, plus
	// it's a simple way to avoid bursting authn endpoints.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Have we failed authn too many times?
	if c.repeatedAuthnFailures >= maxRepeatedAuthnFailures {
		return "", trace.Wrap(ErrMaxAuthnAttemptsReached)
	}

	// Refresh/reacquire token.
	var err error
	if c.clientSecret != nil {
		err = c.renewClientSecretLocked(ctx)
	} else {
		err = c.renewUserPassLocked(ctx)
	}
	if err != nil {
		c.repeatedAuthnFailures++
		return "", trace.Wrap(err)
	}
	c.repeatedAuthnFailures = 0

	// Guaranteed non-nil by the success above.
	return c.currentToken.GetAccessToken(), nil
}
