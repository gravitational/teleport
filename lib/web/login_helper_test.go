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
	"context"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

// newOTPSharedSecret returns an OTP shared secret, encoded as a base32 string.
func newOTPSharedSecret() string {
	return base32.StdEncoding.EncodeToString([]byte("supersecretsecret!!1!"))
}

type loginWebOTPParams struct {
	webClient      *TestWebClient
	clock          clockwork.Clock
	user, password string
	otpSecret      string // base32-encoded shared OTP secret
}

// loginWebOTP logins the user using the /webapi/sessions/new endpoint.
//
// This is a lower-level utility for tests that want access to the returned
// CreateSessionResponse.
func loginWebOTP(t *testing.T, ctx context.Context, params loginWebOTPParams) *CreateSessionResponse {
	webClient := params.webClient
	clock := params.clock

	code, err := totp.GenerateCode(params.otpSecret, clock.Now())
	require.NoError(t, err, "GenerateCode failed")

	// Prepare request JSON body.
	reqBody, err := json.Marshal(&CreateSessionReq{
		User:              params.user,
		Pass:              params.password,
		SecondFactorToken: code,
	})
	require.NoError(t, err, "Marshal failed")

	// Prepare request with CSRF token.
	url := webClient.Endpoint("webapi", "sessions", "web")
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	require.NoError(t, err, "NewRequestWithContext failed")
	const csrfToken = "2ebcb768d0090ea4368e42880c970b61865c326172a4a2343b645cf5d7f20992"
	addCSRFCookieToReq(req, csrfToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrf.HeaderName, csrfToken)

	resp, err := webClient.HTTPClient().Do(req)
	require.NoError(t, err, "Do failed")

	// Drain body, then close, then handle error.
	sessionResp := &CreateSessionResponse{}
	err = json.NewDecoder(resp.Body).Decode(sessionResp)
	_ = resp.Body.Close()
	require.NoError(t, err, "Unmarshal failed")

	return sessionResp
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
func loginWebMFA(ctx context.Context, t *testing.T, params loginWebMFAParams) *CreateSessionResponse {
	webClient := params.webClient

	beginResp, err := webClient.PostJSON(ctx, webClient.Endpoint("webapi", "mfa", "login", "begin"), &client.MFAChallengeRequest{
		User: params.user,
		Pass: params.password,
	})
	require.NoError(t, err)

	authChallenge := &client.MFAAuthenticateChallenge{}
	require.NoError(t, json.Unmarshal(beginResp.Bytes(), authChallenge))
	require.NotNil(t, authChallenge.WebauthnChallenge)

	// Sign Webauthn challenge (requires user interaction in real-world
	// scenarios).
	key := params.authenticator
	assertionResp, err := key.SignAssertion("https://"+params.rpID, authChallenge.WebauthnChallenge)
	require.NoError(t, err)

	// 2nd login step: reply with signed challenge.
	sessionResp, err := webClient.PostJSON(ctx, webClient.Endpoint("webapi", "mfa", "login", "finishsession"), &client.AuthenticateWebUserRequest{
		User:                      params.user,
		WebauthnAssertionResponse: assertionResp,
	})
	require.NoError(t, err)
	createSessionResp := &CreateSessionResponse{}
	require.NoError(t, json.Unmarshal(sessionResp.Bytes(), createSessionResp))
	return createSessionResp
}
