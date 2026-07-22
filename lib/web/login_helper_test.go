// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package web

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base32"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/client"
)

// newOTPSharedSecret returns an OTP shared secret, encoded as a base32 string.
func newOTPSharedSecret() string {
	secret := uuid.NewString()
	return base32.StdEncoding.EncodeToString([]byte(secret))
}

type loginWebOTPParams struct {
	webClient      *TestWebClient
	clock          clockwork.Clock
	user, password string

	// otpSecret is the shared, base32-encoded OTP secret.
	// A new code is generated if provided.
	// Requires clock to the set.
	// If empty then no OTP is sent in the request.
	otpSecret string

	userAgent           string // Optional.
	overrideContentType string // Optional.
}

// DrainedHTTPResponse mimics an http.Response, but without a body.
type DrainedHTTPResponse struct {
	StatusCode int
	cookies    []*http.Cookie
}

// Cookies mimics http.Response.Cookies.
func (r *DrainedHTTPResponse) Cookies() []*http.Cookie {
	return r.cookies
}

// loginWebOTP logins the user using the /webapi/sessions/new endpoint.
//
// This is a lower-level utility for tests that want access to the returned
// unmarshaled CreateSessionResponse or HTTP response.
func loginWebOTP(t *testing.T, ctx context.Context, params loginWebOTPParams) (*CreateSessionResponse, *DrainedHTTPResponse) {
	httpResp, body, err := rawLoginWebOTP(ctx, params)
	require.NoError(t, err, "Login via OTP failed")
	require.Equal(t, http.StatusOK, httpResp.StatusCode, "Login via OTP failed (status mismatch)")

	sessionResp := &CreateSessionResponse{}
	require.NoError(t,
		json.Unmarshal(body, sessionResp),
		"Unmarshal failed")
	return sessionResp, httpResp
}

// rawLoginWebOTP is the raw variant of [loginWebOTP].
//
// This is a lower-level utility for tests that want access to the response body
// itself. Callers MUST check the response status themselves, a successful login
// is not guaranteed.
//
// Note that the response body is automatically drained into a []byte and
// closed.
func rawLoginWebOTP(ctx context.Context, params loginWebOTPParams) (resp *DrainedHTTPResponse, body []byte, err error) {
	webClient := params.webClient
	clock := params.clock

	var code string
	if params.otpSecret != "" {
		code, err = totp.GenerateCode(params.otpSecret, clock.Now())
		if err != nil {
			return nil, nil, trace.Wrap(err, "otp code generation")
		}
	}

	// Prepare request JSON body.
	reqBody, err := json.Marshal(&CreateSessionReq{
		User:              params.user,
		Pass:              params.password,
		SecondFactorToken: code,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err, "request marshal")
	}

	// Prepare HTTP request.
	url := webClient.Endpoint("webapi", "sessions", "web")
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, nil, trace.Wrap(err, "create HTTP request")
	}

	// Set assorted headers.
	req.Header.Set("Content-Type", cmp.Or(params.overrideContentType, "application/json"))
	if params.userAgent != "" {
		req.Header.Set("User-Agent", params.userAgent)
	}

	httpResp, err := webClient.HTTPClient().Do(req)
	if err != nil {
		return nil, nil, trace.Wrap(err, "do HTTP request")
	}

	// Drain body, then close, then handle error.
	body, err = io.ReadAll(httpResp.Body)
	_ = httpResp.Body.Close()
	if err != nil {
		return nil, nil, trace.Wrap(err, "reading body from response")
	}

	return &DrainedHTTPResponse{
		StatusCode: httpResp.StatusCode,
		cookies:    httpResp.Cookies(),
	}, body, nil
}

type loginWebMFAParams struct {
	webClient      *TestWebClient
	rpID           string
	user, password string
	authenticator  *mocku2f.Key
}

// loginWebMFA logins the user using /webapi/mfa/login/begin and
// /webapi/mfa/login/finishsession.
//
// This is a lower-level utility for tests that want access to the returned
// CreateSessionResponse.
func loginWebMFA(ctx context.Context, t *testing.T, params loginWebMFAParams) (*CreateSessionResponse, *DrainedHTTPResponse) {
	httpResp, body, err := rawLoginWebMFA(ctx, params)
	require.NoError(t, err, "Login via MFA failed")

	// Sanity check.
	require.Equal(t, http.StatusOK, httpResp.StatusCode, "Login via MFA failed (status mismatch)")

	sessionResp := &CreateSessionResponse{}
	require.NoError(t,
		json.Unmarshal(body, sessionResp),
		"Unmarshal failed")
	return sessionResp, httpResp
}

// rawLoginWebMFA is the raw variant of [loginWebMFA].
//
// This is a lower-level utility for tests that want access to the response body
// or error.
//
// Returns the acquired response, body and error from the last step, even when
// errored. Failures in previous steps return simply an error.
func rawLoginWebMFA(ctx context.Context, params loginWebMFAParams) (resp *DrainedHTTPResponse, body []byte, err error) {
	webClient := params.webClient

	beginResp, err := webClient.PostJSON(ctx, webClient.Endpoint("webapi", "mfa", "login", "begin"), &client.MFAChallengeRequest{
		User: params.user,
		Pass: params.password,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err, "begin step")
	}

	authChallenge := &client.MFAAuthenticateChallenge{}
	if err := json.Unmarshal(beginResp.Bytes(), authChallenge); err != nil {
		return nil, nil, trace.Wrap(err, "begin unmarshal")
	}
	if authChallenge.WebauthnChallenge == nil {
		// Avoid trace here, so it doesn't "match" anything.
		return nil, nil, errors.New("begin step returned nil WebauthnChallenge")
	}

	// Sign Webauthn challenge (requires user interaction in real-world
	// scenarios).
	key := params.authenticator
	assertionResp, err := key.SignAssertion("https://"+params.rpID, authChallenge.WebauthnChallenge)
	if err != nil {
		return nil, nil, trace.Wrap(err, "sign challenge")
	}

	// 2nd login step: reply with signed challenge.
	sessionResp, err := webClient.PostJSON(ctx, webClient.Endpoint("webapi", "mfa", "login", "finishsession"), &client.AuthenticateWebUserRequest{
		User:                      params.user,
		WebauthnAssertionResponse: assertionResp,
	})
	// Return everything we get from the last step, even if it errored.
	if sessionResp != nil {
		resp = &DrainedHTTPResponse{
			StatusCode: sessionResp.Code(),
			cookies:    sessionResp.Cookies(),
		}
		body = sessionResp.Bytes()
	}
	return resp, body, err
}
