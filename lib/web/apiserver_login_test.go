/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"context"
	"crypto"
	"encoding/json"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestWebauthnLogin_ssh(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	clusterMFA := configureClusterForMFA(t, env, &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: env.server.TLS.ClusterName(),
		},
	})
	user := clusterMFA.User
	password := clusterMFA.Password
	device := clusterMFA.WebDev.Key

	// Prepare keys to be signed.
	sshKey, tlsKey, err := cryptosuites.GenerateUserSSHAndTLSKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(env.server.AuthServer.AuthServer))
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	tlsPubKeyBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                string
		pubKey              []byte
		sshPubKey           []byte
		tlsPubKey           []byte
		expectError         string
		expectSubjectSSHPub ssh.PublicKey
		expectSubjectTLSPub crypto.PublicKey
	}{
		{
			// TODO(nklaassen): DELETE IN 18.0.0 when all clients should be
			// using split keys.
			desc:                "single key",
			pubKey:              sshPubKeyBytes,
			expectSubjectSSHPub: sshPubKey,
			expectSubjectTLSPub: sshKey.Public(),
		},
		{
			desc:                "split keys",
			sshPubKey:           sshPubKeyBytes,
			tlsPubKey:           tlsPubKeyBytes,
			expectSubjectSSHPub: sshPubKey,
			expectSubjectTLSPub: tlsKey.Public(),
		},
		{
			desc:                "only SSH",
			sshPubKey:           sshPubKeyBytes,
			expectSubjectSSHPub: sshPubKey,
		},
		{
			desc:                "only TLS",
			tlsPubKey:           tlsPubKeyBytes,
			expectSubjectTLSPub: tlsKey.Public(),
		},
		{
			desc:        "no key",
			expectError: "'ssh_pub_key' or 'tls_pub_key' must be set",
		},
	} {
		// 1st login step: request challenge.
		ctx := context.Background()
		beginResp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "begin"), &client.MFAChallengeRequest{
			User: user,
			Pass: password,
		})
		require.NoError(t, err)
		authChallenge := &client.MFAAuthenticateChallenge{}
		require.NoError(t, json.Unmarshal(beginResp.Bytes(), authChallenge))
		require.NotNil(t, authChallenge.WebauthnChallenge)

		// Sign Webauthn challenge (requires user interaction in real-world
		// scenarios).
		assertionResp, err := device.SignAssertion("https://"+env.server.TLS.ClusterName(), authChallenge.WebauthnChallenge)
		require.NoError(t, err)

		// 2nd login step: reply with signed challenged.
		finishResp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finish"), &client.AuthenticateSSHUserRequest{
			User:                      user,
			WebauthnChallengeResponse: assertionResp,
			UserPublicKeys: client.UserPublicKeys{
				PubKey:    tc.pubKey,
				SSHPubKey: tc.sshPubKey,
				TLSPubKey: tc.tlsPubKey,
			},
			TTL: 24 * time.Hour,
		})
		if tc.expectError != "" {
			require.Error(t, err)
			require.ErrorContains(t, err, tc.expectError)
			return
		}
		require.NoError(t, err)

		loginResp := validateSSHLoginResponse(t, finishResp.Bytes(), tc.expectSubjectSSHPub, tc.expectSubjectTLSPub)
		require.Equal(t, user, loginResp.Username)
	}
}

func TestWebauthnLogin_web(t *testing.T) {
	env := newWebPack(t, 1)
	proxy := env.proxies[0]

	rpID := env.server.TLS.ClusterName()
	clusterMFA := configureClusterForMFA(t, env, &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: rpID,
		},
	})
	user := clusterMFA.User
	password := clusterMFA.Password
	device := clusterMFA.WebDev.Key

	ctx := context.Background()

	sessionResp, _ := loginWebMFA(ctx, t, loginWebMFAParams{
		webClient:     proxy.newClient(t),
		rpID:          rpID,
		user:          user,
		password:      password,
		authenticator: device,
	})

	// Run various additional response assertions.
	assert.NotEmpty(t, sessionResp.TokenType)
	assert.NotEmpty(t, sessionResp.Token)
	assert.NotEmpty(t, sessionResp.TokenExpiresIn)
	assert.NotEmpty(t, sessionResp.SessionExpires.Unix())
}

func TestWebauthnLogin_webWithPrivateKeyEnabledError(t *testing.T) {
	env := newWebPack(t, 1)
	proxy := env.proxies[0]
	ctx := context.Background()

	rpID := env.server.TLS.ClusterName()
	authPref := &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: rpID,
		},
	}

	// configureClusterForMFA will creates a user and a webauthn device,
	// so we will enable the private key policy afterwards.
	clusterMFA := configureClusterForMFA(t, env, authPref)
	user := clusterMFA.User
	password := clusterMFA.Password
	device := clusterMFA.WebDev.Key

	authPref.RequireMFAType = types.RequireMFAType_HARDWARE_KEY_TOUCH
	cap, err := types.NewAuthPreference(*authPref)
	require.NoError(t, err)
	authServer := env.server.Auth()
	_, err = authServer.UpsertAuthPreference(ctx, cap)
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		MockAttestationData: &keys.AttestationData{
			PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
		},
	})

	httpResp, body, err := rawLoginWebMFA(ctx, loginWebMFAParams{
		webClient:     proxy.newClient(t),
		rpID:          rpID,
		user:          user,
		password:      password,
		authenticator: device,
	})
	require.Error(t, err)
	// Make sure we failed in the last step.
	require.NotNil(t, httpResp, "HTTP response nil, did it fail in the finishsession step?")
	require.NotNil(t, body, "HTTP response body nil, did it fail in the finishsession step?")

	var resErr httpErrorResponse
	require.NoError(t, json.Unmarshal(body, &resErr))
	require.Contains(t, resErr.Error.Message, keys.PrivateKeyPolicyHardwareKeyTouch)
}

func TestAuthenticate_passwordless(t *testing.T) {
	env := newWebPack(t, 1)
	clusterMFA := configureClusterForMFA(t, env, &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: env.server.TLS.ClusterName(),
		},
	})
	user := clusterMFA.User
	device := clusterMFA.WebDev.Key

	// Fake a passwordless device. Typically this would require a separate
	// registration, but because we use fake devices we can get away with it.
	device.SetPasswordless()

	// Fetch the WebAuthn User Handle. In a real-world scenario the device stores
	// the handle alongside the credentials during registration.
	ctx := context.Background()
	authServer := env.server.Auth()
	wla, err := authServer.GetWebauthnLocalAuth(ctx, user)
	require.NoError(t, err)
	userHandle := wla.UserID

	// Prepare keys to be signed.
	sshKey, tlsKey, err := cryptosuites.GenerateUserSSHAndTLSKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(env.server.AuthServer.AuthServer))
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	tlsPubKeyBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	tests := []struct {
		name  string
		login func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse)
	}{
		{
			// TODO(nklaassen): DELETE IN 18.0.0 when all clients should be
			// using split keys.
			name: "ssh single key",
			login: func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse) {
				ep := clt.Endpoint("webapi", "mfa", "login", "finish")
				sshResp, err := clt.PostJSON(ctx, ep, &client.AuthenticateSSHUserRequest{
					WebauthnChallengeResponse: assertionResp, // no username
					UserPublicKeys: client.UserPublicKeys{
						PubKey: sshPubKeyBytes,
					},
					TTL: 24 * time.Hour,
				})
				require.NoError(t, err, "Passwordless authentication failed")
				loginResp := validateSSHLoginResponse(t, sshResp.Bytes(), sshPubKey, sshKey.Public())
				require.Equal(t, user, loginResp.Username)
			},
		},
		{
			name: "ssh split keys",
			login: func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse) {
				ep := clt.Endpoint("webapi", "mfa", "login", "finish")
				sshResp, err := clt.PostJSON(ctx, ep, &client.AuthenticateSSHUserRequest{
					WebauthnChallengeResponse: assertionResp, // no username
					UserPublicKeys: client.UserPublicKeys{
						SSHPubKey: sshPubKeyBytes,
						TLSPubKey: tlsPubKeyBytes,
					},
					TTL: 24 * time.Hour,
				})
				require.NoError(t, err, "Passwordless authentication failed")
				loginResp := validateSSHLoginResponse(t, sshResp.Bytes(), sshPubKey, tlsKey.Public())
				require.Equal(t, user, loginResp.Username)
			},
		},
		{
			name: "ssh ssh key only",
			login: func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse) {
				ep := clt.Endpoint("webapi", "mfa", "login", "finish")
				sshResp, err := clt.PostJSON(ctx, ep, &client.AuthenticateSSHUserRequest{
					WebauthnChallengeResponse: assertionResp, // no username
					UserPublicKeys: client.UserPublicKeys{
						SSHPubKey: sshPubKeyBytes,
					},
					TTL: 24 * time.Hour,
				})
				require.NoError(t, err, "Passwordless authentication failed")
				loginResp := validateSSHLoginResponse(t, sshResp.Bytes(), sshPubKey, nil)
				require.Equal(t, user, loginResp.Username)
			},
		},
		{
			name: "ssh tls key only",
			login: func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse) {
				ep := clt.Endpoint("webapi", "mfa", "login", "finish")
				sshResp, err := clt.PostJSON(ctx, ep, &client.AuthenticateSSHUserRequest{
					WebauthnChallengeResponse: assertionResp, // no username
					UserPublicKeys: client.UserPublicKeys{
						TLSPubKey: tlsPubKeyBytes,
					},
					TTL: 24 * time.Hour,
				})
				require.NoError(t, err, "Passwordless authentication failed")
				loginResp := validateSSHLoginResponse(t, sshResp.Bytes(), nil, tlsKey.Public())
				require.Equal(t, user, loginResp.Username)
			},
		},
		{
			name: "web",
			login: func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse) {
				ep := clt.Endpoint("webapi", "mfa", "login", "finishsession")
				sessionResp, err := clt.PostJSON(ctx, ep, &client.AuthenticateWebUserRequest{
					WebauthnAssertionResponse: assertionResp, // no username
				})
				require.NoError(t, err, "Passwordless authentication failed")
				createSessionResp := &CreateSessionResponse{}
				require.NoError(t, json.Unmarshal(sessionResp.Bytes(), createSessionResp))
				require.NotEmpty(t, createSessionResp.TokenType)
				require.NotEmpty(t, createSessionResp.Token)
				require.NotEmpty(t, createSessionResp.TokenExpiresIn)
				require.NotEmpty(t, createSessionResp.SessionExpires.Unix())
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Request passwordless challenge.
			ep := clt.Endpoint("webapi", "mfa", "login", "begin")
			beginResp, err := clt.PostJSON(ctx, ep, &client.MFAChallengeRequest{
				Passwordless: true, // no username and password
			})
			require.NoError(t, err, "Failed to create passwordless challenge")
			mfaChallenge := &client.MFAAuthenticateChallenge{}
			require.NoError(t, json.Unmarshal(beginResp.Bytes(), mfaChallenge))
			require.NotNil(t, mfaChallenge.WebauthnChallenge, "Want non-nil WebAuthn challenge")

			// Sign challenge and set user handle.
			origin := "https://" + env.server.TLS.ClusterName()
			assertionResp, err := device.SignAssertion(origin, mfaChallenge.WebauthnChallenge)
			require.NoError(t, err)
			assertionResp.AssertionResponse.UserHandle = userHandle

			// Complete passwordless login.
			test.login(t, assertionResp)
		})
	}

	// Test a couple of config-mismatch scenarios.
	// They progressively alter the cluster's auth preference.

	t.Run("allow_passwordless=false", func(t *testing.T) {
		// Set allow_passwordless=false
		authPref, err := authServer.GetAuthPreference(ctx)
		require.NoError(t, err, "GetAuthPreference failed")
		authPref.SetAllowPasswordless(false)
		_, err = authServer.UpsertAuthPreference(ctx, authPref)
		require.NoError(t, err, "UpsertAuthPreference failed")

		// GET /webapi/mfa/login/begin.
		ep := clt.Endpoint("webapi", "mfa", "login", "begin")
		_, err = clt.PostJSON(ctx, ep, &client.MFAChallengeRequest{
			Passwordless: true, // no username and password
		})
		assert.ErrorIs(t, err, types.ErrPasswordlessDisabledBySettings, "/webapi/mfa/login/begin error mismatch")
	})

	t.Run("webauthn disabled", func(t *testing.T) {
		authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOTP, // disable webauthn
		})
		require.NoError(t, err, "NewAuthPreference failed")
		_, err = authServer.UpsertAuthPreference(ctx, authPref)
		require.NoError(t, err, "UpsertAuthPreference failed")

		// GET /webapi/mfa/login/begin.
		ep := clt.Endpoint("webapi", "mfa", "login", "begin")
		_, err = clt.PostJSON(ctx, ep, &client.MFAChallengeRequest{
			Passwordless: true, // no username and password
		})
		assert.ErrorIs(t, err, types.ErrPasswordlessRequiresWebauthn, "/webapi/mfa/login/begin error mismatch")
	})
}

// TestPasswordlessProhibitedForSSO is rather similar to
// lib/auth.TestPasswordlessProhibitedForSSO, but here our main concern is that
// error messages aren't obfuscated along the way.
func TestPasswordlessProhibitedForSSO(t *testing.T) {
	env := newWebPack(t, 1)

	testServer := env.server
	authServer := testServer.Auth()
	proxyServer := env.proxies[0]
	clock := env.clock

	// Prepare user and default devices.
	mfa := configureClusterForMFA(t, env, &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: testServer.ClusterName(),
		},
	})
	user := mfa.User
	ctx := context.Background()

	// Register a passwordless device.
	userClient, err := testServer.NewClient(auth.TestUser(user))
	require.NoError(t, err, "NewClient failed")
	pwdlessDev, err := auth.RegisterTestDevice(
		ctx, userClient, "pwdless", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, mfa.WebDev, auth.WithPasswordless())
	require.NoError(t, err, "RegisterTestDevice failed")

	// Update the user so it looks like an SSO user.
	_, err = authServer.UpdateAndSwapUser(ctx, user, false /* withSecrets */, func(u types.User) (changed bool, err error) {
		u.SetCreatedBy(types.CreatedBy{
			Connector: &types.ConnectorRef{
				Type:     constants.Github,
				ID:       "github",
				Identity: user,
			},
			Time: clock.Now(),
			User: types.UserRef{
				Name: teleport.UserSystem,
			},
		})
		return true, nil
	})
	require.NoError(t, err, "UpdateAndSwapUser failed")

	// Prepare keys to be signed.
	sshKey, tlsKey, err := cryptosuites.GenerateUserSSHAndTLSKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(env.server.AuthServer.AuthServer))
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	tlsPubKeyBytes, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	webClient, err := client.NewWebClient(
		proxyServer.webURL.String(),
		roundtrip.HTTPClient(client.NewInsecureWebClient()),
	)
	require.NoError(t, err, "NewWebClient failed")

	tests := []struct {
		name  string
		login func(chalResp *wantypes.CredentialAssertionResponse) error
	}{
		{
			name: "web",
			login: func(chalResp *wantypes.CredentialAssertionResponse) error {
				ep := webClient.Endpoint("webapi", "mfa", "login", "finishsession")
				_, err := webClient.PostJSON(ctx, ep, &client.AuthenticateWebUserRequest{
					WebauthnAssertionResponse: chalResp,
				})
				return err
			},
		},
		{
			name: "ssh",
			login: func(chalResp *wantypes.CredentialAssertionResponse) error {
				ep := webClient.Endpoint("webapi", "mfa", "login", "finish")
				_, err := webClient.PostJSON(ctx, ep, &client.AuthenticateSSHUserRequest{
					WebauthnChallengeResponse: chalResp,
					UserPublicKeys: client.UserPublicKeys{
						SSHPubKey: sshPubKeyBytes,
						TLSPubKey: tlsPubKeyBytes,
					},
					TTL: 12 * time.Hour,
				})
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Issue passwordless challenge.
			ep := webClient.Endpoint("webapi", "mfa", "login", "begin")
			beginResp, err := webClient.PostJSON(ctx, ep, &client.MFAChallengeRequest{
				Passwordless: true,
			})
			require.NoError(t, err, "POST /webapi/mfa/login/begin")
			mfaChallenge := &client.MFAAuthenticateChallenge{}
			require.NoError(t, json.Unmarshal(beginResp.Bytes(), mfaChallenge), "Unmarshal MFA challenge failed")

			// Sign it.
			origin := "https://" + testServer.ClusterName()
			chalResp, err := pwdlessDev.Key.SignAssertion(origin, mfaChallenge.WebauthnChallenge)
			require.NoError(t, err, "SignAssertion failed")

			// Login and verify that the passwordless/SSO error was not obfuscated.
			err = test.login(chalResp)
			assert.ErrorIs(t, err, types.ErrPassswordlessLoginBySSOUser, "Login error mismatch")
		})
	}
}

func TestAuthenticate_rateLimiting(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		burst int
		fn    func(clt *client.WebClient) error
	}{
		{
			name:  "/webapi/mfa/login/begin",
			burst: defaults.LimiterBurst,
			fn: func(clt *client.WebClient) error {
				ep := clt.Endpoint("webapi", "mfa", "login", "begin")
				_, err := clt.PostJSON(ctx, ep, &client.MFAChallengeRequest{})
				return err
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Use a separate webPack per test, so limits won't influence one another.
			env := newWebPack(t, 1)
			clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
			require.NoError(t, err)

			for i := 0; i < test.burst; i++ {
				err := test.fn(clt)
				require.False(t, trace.IsLimitExceeded(err), "got err = %v, want non-LimitExceeded", err)
			}

			err = test.fn(clt)
			require.True(t, trace.IsLimitExceeded(err), "got err = %v, want LimitExceeded", err)
		})
	}
}

func TestAuthenticate_deviceWebToken(t *testing.T) {
	pack := newWebPack(t, 1 /* numProxies */)
	authServer := pack.server.AuthServer.AuthServer
	proxy := pack.proxies[0]
	clock := pack.clock

	// Mimic a valid DeviceWebToken, regardless of any parameters.
	wantToken := &types.DeviceWebToken{
		Id:    "this is an opaque token ID",
		Token: "this is an opaque token Token",
	}
	authServer.SetCreateDeviceWebTokenFunc(func(context.Context, *devicepb.DeviceWebToken) (*devicepb.DeviceWebToken, error) {
		return &devicepb.DeviceWebToken{
			Id:    wantToken.Id,
			Token: wantToken.Token,
		}, nil
	})

	ctx := context.Background()

	t.Run("login using OTP", func(t *testing.T) {
		// Create a user with password + OTP.
		const user = "llama1"
		const pass = "mysupersecretpassword!!1!"
		otpSecret := newOTPSharedSecret()
		proxy.createUser(ctx, t, user, user, pass, otpSecret, nil /* roles */)

		sessionResp, _ := loginWebOTP(t, ctx, loginWebOTPParams{
			webClient: proxy.newClient(t),
			clock:     clock,
			user:      user,
			password:  pass,
			otpSecret: otpSecret,
		})
		assert.Equal(t, wantToken, sessionResp.DeviceWebToken, "WebSession DeviceWebToken mismatch")
	})

	t.Run("login using WebAuthn", func(t *testing.T) {
		rpID := pack.server.TLS.ClusterName()
		mfaResp := configureClusterForMFA(t, pack, &types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: rpID,
			},
		})

		sessionResp, _ := loginWebMFA(ctx, t, loginWebMFAParams{
			webClient:     proxy.newClient(t),
			rpID:          rpID,
			user:          mfaResp.User,
			password:      mfaResp.Password,
			authenticator: mfaResp.WebDev.Key,
		})
		assert.Equal(t, wantToken, sessionResp.DeviceWebToken, "WebSession DeviceWebToken mismatch")
	})
}

type configureMFAResp struct {
	User, Password string
	WebDev         *auth.TestDevice
}

func configureClusterForMFA(t *testing.T, env *webPack, spec *types.AuthPreferenceSpecV2) *configureMFAResp {
	t.Helper()
	ctx := context.Background()

	// Configure cluster auth preferences.
	cap, err := types.NewAuthPreference(*spec)
	require.NoError(t, err)
	authServer := env.server.Auth()
	_, err = authServer.UpsertAuthPreference(ctx, cap)
	require.NoError(t, err)

	// Create user.
	const user = "llama"
	const password = "password1234"
	env.proxies[0].createUser(ctx, t, user, "root", "password1234", "" /* otpSecret */, nil /* roles */)

	// Register device.
	clt, err := env.server.NewClient(auth.TestUser(user))
	require.NoError(t, err)
	webDev, err := auth.RegisterTestDevice(ctx, clt, "webauthn", proto.DeviceType_DEVICE_TYPE_WEBAUTHN, nil /* authenticator */)
	require.NoError(t, err)

	return &configureMFAResp{
		User:     user,
		Password: password,
		WebDev:   webDev,
	}
}

// TestCreateSSHCert tests the login endpoint /webapi/ssh/certs with different
// options for subject SSH and TLS keys.
// TODO(Joerger): DELETE IN v18.0.0 when 2fa-less login endpoint is deprecated.
func TestCreateSSHCert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pack := newWebPack(t, 1)
	proxy := pack.proxies[0]

	const (
		user      = "alice"
		login     = "root"
		pass      = "password1234"
		otpSecret = ""
	)
	var roles []types.Role

	proxy.createUser(ctx, t, user, login, pass, otpSecret, roles)
	clt := proxy.newClient(t)

	sshKey, tlsKey, err := cryptosuites.GenerateUserSSHAndTLSKey(ctx, cryptosuites.GetCurrentSuiteFromAuthPreference(pack.server.AuthServer.AuthServer))
	require.NoError(t, err)

	sshPub, err := ssh.NewPublicKey(sshKey.Public())
	require.NoError(t, err)
	sshPubKey := ssh.MarshalAuthorizedKey(sshPub)
	tlsPubKey, err := keys.MarshalPublicKey(tlsKey.Public())
	require.NoError(t, err)

	for _, tc := range []struct {
		desc                string
		pubKey              []byte
		sshPubKey           []byte
		tlsPubKey           []byte
		expectError         string
		expectSubjectSSHPub ssh.PublicKey
		expectSubjectTLSPub crypto.PublicKey
	}{
		{
			// TODO(nklaassen): DELETE IN 18.0.0 when all clients should be
			// using split keys.
			desc:                "single key",
			pubKey:              sshPubKey,
			expectSubjectSSHPub: sshPub,
			expectSubjectTLSPub: sshKey.Public(),
		},
		{
			desc:                "split keys",
			sshPubKey:           sshPubKey,
			tlsPubKey:           tlsPubKey,
			expectSubjectSSHPub: sshPub,
			expectSubjectTLSPub: tlsKey.Public(),
		},
		{
			desc:                "only SSH",
			sshPubKey:           sshPubKey,
			expectSubjectSSHPub: sshPub,
		},
		{
			desc:                "only TLS",
			tlsPubKey:           tlsPubKey,
			expectSubjectTLSPub: tlsKey.Public(),
		},
		{
			desc:        "no key",
			expectError: "'ssh_pub_key' or 'tls_pub_key' must be set",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			req := &client.CreateSSHCertReq{
				User:     user,
				Password: pass,
				UserPublicKeys: client.UserPublicKeys{
					PubKey:    tc.pubKey,
					SSHPubKey: tc.sshPubKey,
					TLSPubKey: tc.tlsPubKey,
				},
			}
			resp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "ssh", "certs"), req)
			if tc.expectError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.expectError)
				return
			}
			require.NoError(t, err)

			validateSSHLoginResponse(t, resp.Bytes(), tc.expectSubjectSSHPub, tc.expectSubjectTLSPub)
		})
	}
}

func validateSSHLoginResponse(t *testing.T, resp []byte, expectedSubjectSSHPub ssh.PublicKey, expectedSubjectTLSPub crypto.PublicKey) *authclient.SSHLoginResponse {
	t.Helper()

	var loginResp authclient.SSHLoginResponse
	require.NoError(t, json.Unmarshal(resp, &loginResp))
	assert.NotEmpty(t, loginResp.Username)
	assert.NotEmpty(t, loginResp.HostSigners)

	if expectedSubjectSSHPub != nil {
		if sshCert, err := apisshutils.ParseCertificate(loginResp.Cert); assert.NoError(t, err) {
			assert.Equal(t, expectedSubjectSSHPub, sshCert.Key)
		}
	} else {
		assert.Empty(t, loginResp.Cert)
	}

	if expectedSubjectTLSPub != nil {
		if tlsCert, err := tlsca.ParseCertificatePEM(loginResp.TLSCert); assert.NoError(t, err) {
			assert.Equal(t, expectedSubjectTLSPub, tlsCert.PublicKey)
		}
	} else {
		assert.Empty(t, loginResp.TLSCert)
	}

	return &loginResp
}
