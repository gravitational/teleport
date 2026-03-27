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
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/internal/browsermfa"
	"github.com/gravitational/teleport/lib/auth/mfatypes"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/services"
)

const browserMFARedirectURL = "http://localhost:12345/callback?secret_key=test-key"

type testEnv struct {
	server       *authtest.Server
	auth         *auth.Server
	clock        *clockwork.FakeClock
	authPref     types.AuthPreference
	webauthnUser types.User
	webauthnDev  *types.MFADevice
}

func newBrowserMFATestEnv(t *testing.T) testEnv {
	t.Helper()
	ctx := t.Context()

	fakeClock := clockwork.NewFakeClock()
	testServer, err := authtest.NewTestServer(authtest.ServerConfig{
		Auth: authtest.AuthServerConfig{
			Dir:   t.TempDir(),
			Clock: fakeClock,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, testServer.Close()) })

	a := testServer.Auth()

	// Register a proxy server so getProxyPublicAddr returns a valid address.
	proxy, err := types.NewServer("test-proxy", types.KindProxy, types.ServerSpecV2{
		PublicAddrs: []string{"proxy.example.com:443"},
	})
	require.NoError(t, err)
	err = a.UpsertProxy(ctx, proxy)
	require.NoError(t, err)

	// Enable WebAuthn support.
	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorWebauthn,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		AllowCLIAuthViaBrowser: types.NewBoolOption(true),
	})
	require.NoError(t, err)
	_, err = a.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	// Create a user with a WebAuthn device.
	webauthnUser, _, err := authtest.CreateUserAndRole(a, "webauthn-user", []string{"role"}, nil)
	require.NoError(t, err)

	// Add a WebAuthn device for the webauthn user.
	webauthnDev, err := types.NewMFADevice("webauthn-device", "webauthn-device-id", fakeClock.Now(), &types.MFADevice_Webauthn{
		Webauthn: &types.WebauthnDevice{
			CredentialId:     []byte("credential-id"),
			PublicKeyCbor:    []byte("public-key"),
			AttestationType:  "none",
			Aaguid:           []byte("aaguid"),
			SignatureCounter: 0,
			ResidentKey:      false,
		},
	})
	require.NoError(t, err)
	err = a.UpsertMFADevice(ctx, webauthnUser.GetName(), webauthnDev)
	require.NoError(t, err)

	return testEnv{
		server:       testServer,
		auth:         a,
		clock:        fakeClock,
		authPref:     authPref,
		webauthnUser: webauthnUser,
		webauthnDev:  webauthnDev,
	}
}

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

	env := newBrowserMFATestEnv(t)
	a := env.auth
	username := env.webauthnUser.GetName()

	secretKey, err := secret.NewKey()
	require.NoError(t, err)

	rawID := env.webauthnDev.GetWebauthn().CredentialId
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
				session := &services.MFASessionData{
					RequestID:      requestID,
					Username:       "other-user", // mismatch username
					TSHRedirectURL: redirectURL,
					ConnectorID:    "test-connector",
					ConnectorType:  "test",
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
						AllowReuse:                  mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
						UserVerificationRequirement: "required",
					},
				}
				err := a.UpsertMFASessionData(ctx, session)
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
				session := &services.MFASessionData{
					RequestID:      requestID,
					Username:       username,
					TSHRedirectURL: "://invalid-url",
					ConnectorID:    "test-connector",
					ConnectorType:  "test",
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
				}
				err := a.UpsertMFASessionData(ctx, session)
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
				session := &services.MFASessionData{
					RequestID:      requestID,
					Username:       username,
					TSHRedirectURL: redirectURL,
					ConnectorID:    "test-connector",
					ConnectorType:  "test",
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
				}
				err := a.UpsertMFASessionData(ctx, session)
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

func TestCreateAuthenticateChallenge_BrowserMFARequestID(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	env := newBrowserMFATestEnv(t)
	a := env.auth

	password := []byte("test-password")
	require.NoError(t, a.UpsertPassword(env.webauthnUser.GetName(), password))

	userCredsRequest := &proto.CreateAuthenticateChallengeRequest_UserCredentials{
		UserCredentials: &proto.UserCredentials{
			Username: env.webauthnUser.GetName(),
			Password: password,
		},
	}

	tests := []struct {
		name           string
		setup          func(t *testing.T)
		request        *proto.CreateAuthenticateChallengeRequest
		checkError     func(t *testing.T, err error)
		wantExtensions *mfav1.ChallengeExtensions
	}{
		{
			name: "NOK invalid browser MFA request ID",
			request: &proto.CreateAuthenticateChallengeRequest{
				Request:             userCredsRequest,
				BrowserMFARequestID: "non-existent-id",
			},
			checkError: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*trace.AccessDeniedError), "CreateAuthenticateChallenge error mismatch")
				assert.ErrorContains(t, err, "invalid browser MFA request")
			},
		},
		{
			name: "NOK challenge extensions set with browser MFA request ID",
			request: &proto.CreateAuthenticateChallengeRequest{
				Request:             userCredsRequest,
				BrowserMFARequestID: "some-request-id",
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
			},
			checkError: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*trace.BadParameterError), "CreateAuthenticateChallenge error mismatch")
				assert.ErrorContains(t, err, "challenge extensions must not be set")
			},
		},
		{
			name: "OK browser MFA challenge extensions applied from MFA session",
			setup: func(t *testing.T) {
				session := &services.MFASessionData{
					RequestID:     "test-request-1",
					Username:      env.webauthnUser.GetName(),
					ConnectorID:   constants.BrowserMFA,
					ConnectorType: constants.BrowserMFA,
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
						AllowReuse:                  mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
						UserVerificationRequirement: "required",
					},
				}
				err := a.UpsertMFASessionData(ctx, session)
				require.NoError(t, err)
			},
			request: &proto.CreateAuthenticateChallengeRequest{
				Request:             userCredsRequest,
				BrowserMFARequestID: "test-request-1",
			},
			wantExtensions: &mfav1.ChallengeExtensions{
				Scope:                       mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				AllowReuse:                  mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_NO,
				UserVerificationRequirement: "required",
			},
		},
		{
			name: "NOK nil challenge extensions",
			setup: func(t *testing.T) {
				session := &services.MFASessionData{
					RequestID:           "test-request-2",
					Username:            env.webauthnUser.GetName(),
					ConnectorID:         constants.BrowserMFA,
					ConnectorType:       constants.BrowserMFA,
					ChallengeExtensions: nil,
				}
				err := a.UpsertMFASessionData(ctx, session)
				require.NoError(t, err)
			},
			request: &proto.CreateAuthenticateChallengeRequest{
				Request:             userCredsRequest,
				BrowserMFARequestID: "test-request-2",
			},
			checkError: func(t *testing.T, err error) {
				assert.ErrorAs(t, err, new(*trace.BadParameterError), "CreateAuthenticateChallenge error mismatch")
				assert.ErrorContains(t, err, "stored session lacks challenge extensions")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.setup != nil {
				tt.setup(t)
			}

			var gotExtensions *mfav1.ChallengeExtensions
			a.ObserveBrowserMFAChallengeExtensionsForTesting = func(ext *mfav1.ChallengeExtensions) {
				gotExtensions = ext
			}

			challenge, err := a.CreateAuthenticateChallenge(ctx, tt.request)

			if tt.checkError != nil {
				tt.checkError(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, challenge)
			assert.NotNil(t, challenge.WebauthnChallenge, "expected WebAuthn challenge to be present")

			if tt.wantExtensions != nil {
				require.Equal(t, tt.wantExtensions, gotExtensions)
			}
		})
	}
}

func TestBrowserMFAChallengeCreation(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	env := newBrowserMFATestEnv(t)
	a := env.auth

	// Create a standard user without MFA devices.
	standardUser, _, err := authtest.CreateUserAndRole(a, "standard", []string{"role"}, nil)
	require.NoError(t, err)

	// Create a fake SAML user with SSO MFA enabled who shouldn't get Browser MFA challenge
	// because they don't have webauthn
	samlUser, samlRole, err := authtest.CreateUserAndRole(a, "saml-user", []string{"role"}, nil)
	require.NoError(t, err)

	// Create a fake SAML user with SSO MFA enabled and a webauthn device, who will get Browser MFA
	samlUserWithWebauthn, samlWebauthnRole, err := authtest.CreateUserAndRole(a, "saml-webauthn-user", []string{"role"}, nil)
	require.NoError(t, err)
	err = a.UpsertMFADevice(ctx, samlUserWithWebauthn.GetName(), env.webauthnDev)
	require.NoError(t, err)

	samlConnector, err := types.NewSAMLConnector("saml", types.SAMLConnectorSpecV2{
		AssertionConsumerService: "http://localhost:65535/acs",
		Issuer:                   "test",
		SSO:                      "https://localhost:65535/sso",
		AttributesToRoles: []types.AttributeMapping{
			{Name: "groups", Value: "admin", Roles: []string{samlRole.GetName(), samlWebauthnRole.GetName()}},
		},
		MFASettings: &types.SAMLConnectorMFASettings{
			Enabled: true,
			Issuer:  "test",
			Sso:     "https://localhost:65535/sso",
		},
	})
	require.NoError(t, err)
	_, err = a.UpsertSAMLConnector(ctx, samlConnector)
	require.NoError(t, err)

	samlUser.SetCreatedBy(types.CreatedBy{
		Time: env.clock.Now(),
		Connector: &types.ConnectorRef{
			ID:   samlConnector.GetName(),
			Type: samlConnector.GetKind(),
		},
	})
	_, err = a.UpsertUser(ctx, samlUser)
	require.NoError(t, err)

	loginExt := &mfav1.ChallengeExtensions{
		Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
	}

	for _, tt := range []struct {
		name             string
		username         string
		setup            func(t *testing.T)
		challengeRequest *proto.CreateAuthenticateChallengeRequest
		checkError       func(t *testing.T, err error)
		assertChallenge  func(t *testing.T, chal *proto.MFAAuthenticateChallenge)
	}{
		{
			name:     "NOK user without WebAuthn devices",
			username: standardUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions:      loginExt,
				BrowserMFATSHRedirectURL: browserMFARedirectURL,
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge) {
				assert.Nil(t, chal.BrowserMFAChallenge, "should not return Browser MFA challenge for user without WebAuthn devices")
			},
		},
		{
			name:     "NOK BrowserMFATSHRedirectURL not provided",
			username: env.webauthnUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions:      loginExt,
				BrowserMFATSHRedirectURL: "",
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge) {
				assert.Nil(t, chal.BrowserMFAChallenge, "should not return Browser MFA challenge when BrowserMFATSHRedirectURL is empty")
			},
		},
		{
			name:     "NOK Browser authentication disabled by auth preference",
			username: env.webauthnUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions:      loginExt,
				BrowserMFATSHRedirectURL: browserMFARedirectURL,
			},
			setup: func(t *testing.T) {
				// Disable Browser MFA
				env.authPref.SetAllowCLIAuthViaBrowser(false)
				_, err = a.UpsertAuthPreference(ctx, env.authPref)
				require.NoError(t, err)
				t.Cleanup(func() {
					env.authPref.SetAllowCLIAuthViaBrowser(true)
					_, err = a.UpsertAuthPreference(ctx, env.authPref)
					assert.NoError(t, err)
				})
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge) {
				assert.Nil(t, chal.BrowserMFAChallenge, "should not return Browser MFA challenge when AllowCLIAuthViaBrowser is false")
			},
		},
		{
			name:     "NOK SSO MFA user without webauthn should not get Browser MFA",
			username: samlUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions:      loginExt,
				BrowserMFATSHRedirectURL: browserMFARedirectURL,
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge) {
				assert.Nil(t, chal.BrowserMFAChallenge, "SSO MFA users should not get Browser MFA challenge when webauthn not available")
			},
		},
		{
			name:     "OK SSO MFA user gets Browser MFA when webauthn available",
			username: samlUserWithWebauthn.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions:      loginExt,
				BrowserMFATSHRedirectURL: browserMFARedirectURL,
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge) {
				assert.NotNil(t, chal.BrowserMFAChallenge, "expected Browser MFA challenge to be returned")
				assert.NotEmpty(t, chal.BrowserMFAChallenge.RequestId, "request ID should be generated")

				sd, err := a.GetSSOMFASessionData(ctx, chal.BrowserMFAChallenge.RequestId)
				require.NoError(t, err)
				assert.Equal(t, &services.MFASessionData{
					RequestID:      chal.BrowserMFAChallenge.RequestId,
					Username:       samlUserWithWebauthn.GetName(),
					ConnectorID:    constants.BrowserMFA,
					ConnectorType:  constants.BrowserMFA,
					TSHRedirectURL: browserMFARedirectURL,
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
					Payload: &mfatypes.SessionIdentifyingPayload{},
				}, sd)
			},
		},
		{
			name:     "OK WebAuthn user gets Browser MFA challenge",
			username: env.webauthnUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions:      loginExt,
				BrowserMFATSHRedirectURL: browserMFARedirectURL,
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, chal.BrowserMFAChallenge, "expected Browser MFA challenge to be returned")
				assert.NotEmpty(t, chal.BrowserMFAChallenge.RequestId, "request ID should be generated")

				// Find MFA session data tied to the challenge.
				sd, err := a.GetSSOMFASessionData(ctx, chal.BrowserMFAChallenge.RequestId)
				require.NoError(t, err)
				assert.Equal(t, &services.MFASessionData{
					RequestID:      chal.BrowserMFAChallenge.RequestId,
					Username:       env.webauthnUser.GetName(),
					ConnectorID:    constants.BrowserMFA,
					ConnectorType:  constants.BrowserMFA,
					TSHRedirectURL: browserMFARedirectURL,
					ChallengeExtensions: &mfatypes.ChallengeExtensions{
						Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
					},
					Payload: &mfatypes.SessionIdentifyingPayload{},
				}, sd)
			},
		},
		{
			name:     "OK allow reuse",
			username: env.webauthnUser.GetName(),
			challengeRequest: &proto.CreateAuthenticateChallengeRequest{
				ChallengeExtensions: &mfav1.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
				BrowserMFATSHRedirectURL: browserMFARedirectURL,
			},
			assertChallenge: func(t *testing.T, chal *proto.MFAAuthenticateChallenge) {
				require.NotNil(t, chal.BrowserMFAChallenge, "expected Browser MFA challenge to be returned")

				// We should find MFA session data tied to the challenge by request ID.
				sd, err := a.GetSSOMFASessionData(ctx, chal.BrowserMFAChallenge.RequestId)
				require.NoError(t, err)
				assert.Equal(t, mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES, sd.ChallengeExtensions.AllowReuse)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			userClient, err := env.server.NewClient(authtest.TestUser(tt.username))
			require.NoError(t, err)

			if tt.setup != nil {
				tt.setup(t)
			}

			chal, err := userClient.CreateAuthenticateChallenge(ctx, tt.challengeRequest)

			if tt.checkError != nil {
				require.Error(t, err)
				tt.checkError(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, chal)
			if tt.assertChallenge != nil {
				tt.assertChallenge(t, chal)
			}
		})
	}
}

func TestBrowserMFAChallenge_Validation(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	env := newBrowserMFATestEnv(t)
	a := env.auth

	for _, tt := range []struct {
		name             string
		sd               *services.MFASessionData
		requestID        string
		checkError       func(t *testing.T, err error)
		assertValidation func(t *testing.T, sd *services.MFASessionData)
	}{
		{
			name:      "NOK session data not found",
			sd:        nil,
			requestID: "nonexistent-request",
			checkError: func(t *testing.T, err error) {
				require.Error(t, err, "should fail when session data not found")
			},
		},
		{
			name: "OK session data retrieved correctly",
			sd: &services.MFASessionData{
				RequestID:      "request1",
				Username:       env.webauthnUser.GetName(),
				ConnectorID:    constants.BrowserMFA,
				ConnectorType:  constants.BrowserMFA,
				TSHRedirectURL: browserMFARedirectURL,
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN,
				},
			},
			requestID: "request1",
			assertValidation: func(t *testing.T, sd *services.MFASessionData) {
				require.NotNil(t, sd)
				assert.Equal(t, "request1", sd.RequestID)
				assert.Equal(t, env.webauthnUser.GetName(), sd.Username)
				assert.Equal(t, constants.BrowserMFA, sd.ConnectorID)
				assert.Equal(t, constants.BrowserMFA, sd.ConnectorType)
				assert.Equal(t, browserMFARedirectURL, sd.TSHRedirectURL)
				assert.Equal(t, mfav1.ChallengeScope_CHALLENGE_SCOPE_LOGIN, sd.ChallengeExtensions.Scope)
			},
		},
		{
			name: "OK session data with allow reuse",
			sd: &services.MFASessionData{
				RequestID:      "request2",
				Username:       env.webauthnUser.GetName(),
				ConnectorID:    constants.BrowserMFA,
				ConnectorType:  constants.BrowserMFA,
				TSHRedirectURL: browserMFARedirectURL,
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope:      mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
					AllowReuse: mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES,
				},
			},
			requestID: "request2",
			assertValidation: func(t *testing.T, sd *services.MFASessionData) {
				require.NotNil(t, sd)
				assert.Equal(t, mfav1.ChallengeAllowReuse_CHALLENGE_ALLOW_REUSE_YES, sd.ChallengeExtensions.AllowReuse)
			},
		},
		{
			name: "OK session data with admin action scope",
			sd: &services.MFASessionData{
				RequestID:      "request3",
				Username:       env.webauthnUser.GetName(),
				ConnectorID:    constants.BrowserMFA,
				ConnectorType:  constants.BrowserMFA,
				TSHRedirectURL: browserMFARedirectURL,
				ChallengeExtensions: &mfatypes.ChallengeExtensions{
					Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION,
				},
			},
			requestID: "request3",
			assertValidation: func(t *testing.T, sd *services.MFASessionData) {
				require.NotNil(t, sd)
				assert.Equal(t, mfav1.ChallengeScope_CHALLENGE_SCOPE_ADMIN_ACTION, sd.ChallengeExtensions.Scope)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.sd != nil {
				err := a.UpsertMFASessionData(ctx, tt.sd)
				require.NoError(t, err)
			}

			sd, err := a.GetSSOMFASessionData(ctx, tt.requestID)

			if tt.checkError != nil {
				require.Error(t, err)
				tt.checkError(t, err)
				return
			}

			require.NoError(t, err)
			if tt.assertValidation != nil {
				tt.assertValidation(t, sd)
			}
		})
	}
}
