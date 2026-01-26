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
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/internal"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
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
				var loginResponse authclient.SSHLoginResponse
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

			result, err := internal.EncryptBrowserMFAResponse(redirectURL, tt.webauthnResponse)
			tt.assertError(t, err)
			if err == nil {
				tt.assertRedirectURL(t, result)
			}
		})
	}
}

func TestValidateBrowserMFAChallengeErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fakeClock := clockwork.NewFakeClock()
	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: fakeClock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	testServer, err := testAuthServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testServer.Close()) })

	a := testServer.Auth()

	// Enable WebAuthn
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type: constants.Local,
		SecondFactors: []types.SecondFactorType{
			types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
		},
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	_, err = a.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a test user with WebAuthn device for error test cases
	username := "test-user"
	_, _, err = authtest.CreateUserAndRole(a, username, []string{"role"}, nil)
	require.NoError(t, err)

	// Register a mock WebAuthn device for the user
	device, err := types.NewMFADevice("webauthn-device", uuid.NewString(), fakeClock.Now(), &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:     []byte("test-credential-id"),
			PublicKeyCbor:    []byte("test-public-key"),
			AttestationType:  "none",
			SignatureCounter: 0,
		},
	})
	require.NoError(t, err)
	err = a.UpsertMFADevice(ctx, username, device)
	require.NoError(t, err)

	// Create a valid secret key for the redirect URL
	secretKey, err := secret.NewKey()
	require.NoError(t, err)

	errorTests := []struct {
		name             string
		webauthnResponse *wantypes.CredentialAssertionResponse
		setupSession     func(t *testing.T) string
		assertError      func(t *testing.T, err error)
	}{
		{
			name: "NOK missing MFA session",
			webauthnResponse: &wantypes.CredentialAssertionResponse{
				PublicKeyCredential: wantypes.PublicKeyCredential{
					Credential: wantypes.Credential{
						ID:   "test-credential-id",
						Type: "public-key",
					},
				},
			},
			setupSession: func(t *testing.T) string {
				return "non-existent-request-id"
			},
			assertError: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.True(t, trace.IsNotFound(err), "expected not found error but got %v", err)
			},
		},
		{
			name: "NOK invalid redirect URL in session",
			webauthnResponse: &wantypes.CredentialAssertionResponse{
				PublicKeyCredential: wantypes.PublicKeyCredential{
					Credential: wantypes.Credential{
						ID:   "test-credential-id",
						Type: "public-key",
					},
				},
			},
			setupSession: func(t *testing.T) string {
				requestID := uuid.NewString()
				session := &services.SSOMFASessionData{
					RequestID:         requestID,
					Username:          username,
					ClientRedirectURL: "://invalid-url", // Invalid URL
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
			assertError: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "NOK invalid webauthn response",
			webauthnResponse: &wantypes.CredentialAssertionResponse{
				PublicKeyCredential: wantypes.PublicKeyCredential{
					Credential: wantypes.Credential{
						ID:   "wrong-credential-id",
						Type: "public-key",
					},
					RawID: []byte("wrong-credential-id"),
				},
				AssertionResponse: wantypes.AuthenticatorAssertionResponse{
					AuthenticatorResponse: wantypes.AuthenticatorResponse{
						ClientDataJSON: []byte(`{"type":"webauthn.get","challenge":"wrong-challenge"}`),
					},
					AuthenticatorData: []byte("wrong-data"),
					Signature:         []byte("wrong-signature"),
				},
			},
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
			assertError: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to validate browser MFA response")
			},
		},
	}

	// Run error test cases
	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			requestID := tt.setupSession(t)
			_, err := a.ValidateBrowserMFAChallenge(ctx, requestID, wantypes.CredentialAssertionResponseToProto(tt.webauthnResponse))
			tt.assertError(t, err)
		})
	}

	t.Run("OK valid webauthn response", func(t *testing.T) {
		// Create a real WebAuthn user and device
		validUsername := "webauthn-user"
		_, _, err := authtest.CreateUserAndRole(a, validUsername, []string{"role"}, nil)
		require.NoError(t, err)

		webKey, err := mocku2f.Create()
		require.NoError(t, err)
		webKey.PreferRPID = true
		webKey.SetCounter(10)

		const origin = "https://localhost"
		webConfig, err := authPref.GetWebauthn()
		require.NoError(t, err)

		webRegistration := &wanlib.RegistrationFlow{
			Webauthn: webConfig,
			Identity: a.Services,
		}

		// Register the device
		cc, err := webRegistration.Begin(ctx, validUsername, false /* passwordless */)
		require.NoError(t, err)
		ccr, err := webKey.SignCredentialCreation(origin, cc)
		require.NoError(t, err)
		_, err = webRegistration.Finish(ctx, wanlib.RegisterResponse{
			User:             validUsername,
			DeviceName:       "webauthn-device",
			CreationResponse: ccr,
		})
		require.NoError(t, err)

		// Now create a login challenge
		webLogin := &wanlib.LoginFlow{
			Webauthn: webConfig,
			Identity: a.Services,
		}

		assertion, err := webLogin.Begin(ctx, wanlib.BeginParams{
			User: validUsername,
			ChallengeExtensions: &mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
		})
		require.NoError(t, err)

		// Sign the assertion with the device
		assertionResp, err := webKey.SignAssertion(origin, assertion)
		require.NoError(t, err)

		// Create an SSO MFA session with the challenge
		validRequestID := uuid.NewString()
		validSecretKey, err := secret.NewKey()
		require.NoError(t, err)
		validRedirectURL := "http://127.0.0.1:62972/callback?secret_key=" + validSecretKey.String()

		session := &services.SSOMFASessionData{
			RequestID:         validRequestID,
			Username:          validUsername,
			ClientRedirectURL: validRedirectURL,
			ConnectorID:       "test-connector",
			ConnectorType:     "test",
			ChallengeExtensions: &mfatypes.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
			},
		}
		err = a.UpsertSSOMFASessionData(ctx, session)
		require.NoError(t, err)

		// Call ValidateBrowserMFAChallenge with the valid response
		result, err := a.ValidateBrowserMFAChallenge(ctx, validRequestID, wantypes.CredentialAssertionResponseToProto(assertionResp))
		require.NoError(t, err)
		require.NotEmpty(t, result)

		// Verify the result is a valid redirect URL
		u, err := url.Parse(result)
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:62972", u.Host)
		assert.Equal(t, "/callback", u.Path)

		// Verify the response parameter exists and can be decrypted
		response := u.Query().Get("response")
		require.NotEmpty(t, response, "response parameter should be present")

		// Decrypt and verify the response
		plaintext, err := validSecretKey.Open([]byte(response))
		require.NoError(t, err)

		var loginResponse authclient.SSHLoginResponse
		err = json.Unmarshal(plaintext, &loginResponse)
		require.NoError(t, err)
		require.NotNil(t, loginResponse.BrowserMFAWebauthnResponse)
		assert.Equal(t, assertionResp.ID, loginResponse.BrowserMFAWebauthnResponse.ID)
	})
}
