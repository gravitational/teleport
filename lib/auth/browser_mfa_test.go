// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/internal/browsermfa"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
)

func TestEncryptBrowserMFAResponse(t *testing.T) {
	t.Parallel()

	secretKey, err := secret.NewKey()
	require.NoError(t, err)

	webauthnResponse := &wantypes.CredentialAssertionResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   "test-credential-id",
				Type: "public-key",
			},
			RawID: []byte("test-raw-id"),
		},
		AssertionResponse: wantypes.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: []byte(`{"type":"webauthn.get","challenge":"test-challenge"}`),
			},
			AuthenticatorData: []byte("test-authenticator-data"),
			Signature:         []byte("test-signature"),
		},
	}

	tests := []struct {
		name              string
		redirectURL       string
		webauthnResponse  *wantypes.CredentialAssertionResponse
		assertError       func(t *testing.T, err error)
		assertRedirectURL func(t *testing.T, redirectURL string)
	}{
		{
			name:             "OK valid inputs",
			redirectURL:      "http://127.0.0.1:62972/callback?secret_key=" + secretKey.String(),
			webauthnResponse: webauthnResponse,
			assertError: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			assertRedirectURL: func(t *testing.T, redirectURL string) {
				// Parse the returned URL
				u, err := url.Parse(redirectURL)
				require.NoError(t, err)

				// Verify the response parameter exists
				response := u.Query().Get("response")
				require.NotEmpty(t, response, "response parameter should be present")

				// Verify we can decrypt the response
				plaintext, err := secretKey.Open([]byte(response))
				require.NoError(t, err)

				// Verify the decrypted content
				var loginResponse authclient.CLILoginResponse
				err = json.Unmarshal(plaintext, &loginResponse)
				require.NoError(t, err)
				require.NotNil(t, loginResponse.BrowserMFAWebauthnResponse)
				assert.Equal(t, webauthnResponse.ID, loginResponse.BrowserMFAWebauthnResponse.ID)
			},
		},
		{
			name:             "NOK missing secret_key",
			redirectURL:      "http://127.0.0.1:62972/callback",
			webauthnResponse: webauthnResponse,
			assertError: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, trace.IsBadParameter(err), "expected bad parameter error but got %v", err)
				assert.Contains(t, err.Error(), "missing secret_key")
			},
			assertRedirectURL: func(t *testing.T, redirectURL string) {
				require.Fail(t, "should not reach here, expected an error")
			},
		},
		{
			name:             "NOK invalid secret_key format",
			redirectURL:      "http://127.0.0.1:62972/callback?secret_key=invalid-not-hex",
			webauthnResponse: webauthnResponse,
			assertError: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "encoding/hex")
			},
			assertRedirectURL: func(t *testing.T, redirectURL string) {
				require.Fail(t, "should not reach here, expected an error")
			},
		},
		{
			name:             "NOK invalid URL",
			redirectURL:      "://invalid-url",
			webauthnResponse: webauthnResponse,
			assertError: func(t *testing.T, err error) {
				// This will fail during url.Parse
				require.Error(t, err)
			},
			assertRedirectURL: func(t *testing.T, redirectURL string) {
				require.Fail(t, "should not reach here, expected an error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redirectURL, err := url.Parse(tt.redirectURL)
			if err != nil {
				tt.assertError(t, err)
				return
			}

			result, err := browsermfa.EncryptBrowserMFAResponse(redirectURL, tt.webauthnResponse)
			tt.assertError(t, err)
			if err == nil {
				tt.assertRedirectURL(t, result)
			}
		})
	}
}

func TestCompleteBrowserMFAChallenge(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	testServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testServer.Close()) })

	a := testServer.Auth()

	username := "test-user"
	_, _, err = authtest.CreateUserAndRole(a, username, []string{"role"}, nil)
	require.NoError(t, err)

	secretKey, err := secret.NewKey()
	require.NoError(t, err)

	rawID := []byte("test-raw-id")
	webauthnResponse := &wantypes.CredentialAssertionResponse{
		PublicKeyCredential: wantypes.PublicKeyCredential{
			Credential: wantypes.Credential{
				ID:   base64.RawURLEncoding.EncodeToString(rawID),
				Type: "public-key",
			},
			RawID: rawID,
		},
		AssertionResponse: wantypes.AuthenticatorAssertionResponse{
			AuthenticatorResponse: wantypes.AuthenticatorResponse{
				ClientDataJSON: []byte(`{"type":"webauthn.get","challenge":"test-challenge"}`),
			},
			AuthenticatorData: []byte("test-authenticator-data"),
			Signature:         []byte("test-signature"),
		},
	}

	tests := []struct {
		name         string
		setupSession func(t *testing.T) string
		assertError  require.ErrorAssertionFunc
		assertResult func(t *testing.T, result string)
	}{
		{
			name: "NOK missing MFA session",
			setupSession: func(t *testing.T) string {
				return "non-existent-request-id"
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name: "NOK username mismatch",
			setupSession: func(t *testing.T) string {
				requestID := uuid.NewString()
				redirectURL := "http://127.0.0.1:62972/callback?secret_key=" + secretKey.String()
				session := &services.SSOMFASessionData{
					RequestID:         requestID,
					Username:          "other-user", // mismatch username
					ClientRedirectURL: redirectURL,
					ConnectorID:       "test-connector",
					ConnectorType:     "test",
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
				}
				err := a.UpsertSSOMFASessionData(ctx, session)
				require.NoError(t, err)
				return requestID
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				assert.True(t, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		},
		{
			name: "NOK invalid redirect URL in session",
			setupSession: func(t *testing.T) string {
				requestID := uuid.NewString()
				session := &services.SSOMFASessionData{
					RequestID:         requestID,
					Username:          username,
					ClientRedirectURL: "://invalid-url",
					ConnectorID:       "test-connector",
					ConnectorType:     "test",
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
				}
				err := a.UpsertSSOMFASessionData(ctx, session)
				require.NoError(t, err)
				return requestID
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
			},
		},
		{
			name: "OK valid response",
			setupSession: func(t *testing.T) string {
				requestID := uuid.NewString()
				redirectURL := "http://127.0.0.1:62972/callback?secret_key=" + secretKey.String()
				session := &services.SSOMFASessionData{
					RequestID:         requestID,
					Username:          username,
					ClientRedirectURL: redirectURL,
					ConnectorID:       "test-connector",
					ConnectorType:     "test",
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
				}
				err := a.UpsertSSOMFASessionData(ctx, session)
				require.NoError(t, err)
				return requestID
			},
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				require.NoError(t, err)
			},
			assertResult: func(t *testing.T, result string) {
				u, err := url.Parse(result)
				require.NoError(t, err)
				assert.Equal(t, "127.0.0.1:62972", u.Host)
				assert.Equal(t, "/callback", u.Path)

				response := u.Query().Get("response")
				require.NotEmpty(t, response, "response parameter should be present")

				plaintext, err := secretKey.Open([]byte(response))
				require.NoError(t, err)

				var loginResponse authclient.CLILoginResponse
				err = json.Unmarshal(plaintext, &loginResponse)
				require.NoError(t, err)
				require.NotNil(t, loginResponse.BrowserMFAWebauthnResponse)
				assert.Equal(t, webauthnResponse.ID, loginResponse.BrowserMFAWebauthnResponse.ID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestID := tt.setupSession(t)
			userCtx := authz.ContextWithUser(ctx, authtest.TestUserWithRoles(username, []string{"role"}).I)
			result, err := a.CompleteBrowserMFAChallenge(
				userCtx,
				requestID,
				wantypes.CredentialAssertionResponseToProto(webauthnResponse),
			)
			tt.assertError(t, err)
			if tt.assertResult != nil {
				tt.assertResult(t, result)
			}
		})
	}
}
