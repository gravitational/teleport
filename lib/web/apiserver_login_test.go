// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
)

func TestWebauthnLogin_ssh(t *testing.T) {
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

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

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

	// Prepare SSH key to be signed.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	sshPubKey, err := ssh.NewPublicKey(&privKey.PublicKey)
	require.NoError(t, err)
	sshPubKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)

	// 2nd login step: reply with signed challenged.
	finishResp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finish"), &client.AuthenticateSSHUserRequest{
		User:                      user,
		WebauthnChallengeResponse: assertionResp,
		PubKey:                    sshPubKeyBytes,
		TTL:                       24 * time.Hour,
	})
	require.NoError(t, err)
	loginResp := &auth.SSHLoginResponse{}
	require.NoError(t, json.Unmarshal(finishResp.Bytes(), loginResp))
	require.Equal(t, user, loginResp.Username)
	require.NotEmpty(t, loginResp.Cert)
	require.NotEmpty(t, loginResp.TLSCert)
	require.NotEmpty(t, loginResp.HostSigners)
}

func TestWebauthnLogin_web(t *testing.T) {
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

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

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
	sessionResp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finishsession"), &client.AuthenticateWebUserRequest{
		User:                      user,
		WebauthnAssertionResponse: assertionResp,
	})
	require.NoError(t, err)
	createSessionResp := &CreateSessionResponse{}
	require.NoError(t, json.Unmarshal(sessionResp.Bytes(), createSessionResp))
	require.NotEmpty(t, createSessionResp.TokenType)
	require.NotEmpty(t, createSessionResp.Token)
	require.NotEmpty(t, createSessionResp.TokenExpiresIn)
	require.NotEmpty(t, createSessionResp.SessionExpires.Unix())
}

func TestWebauthnLogin_webWithPrivateKeyEnabledError(t *testing.T) {
	ctx := context.Background()
	env := newWebPack(t, 1)
	authPref := &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		Webauthn: &types.Webauthn{
			RPID: env.server.TLS.ClusterName(),
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
	err = authServer.SetAuthPreference(ctx, cap)
	require.NoError(t, err)

	modules.SetTestModules(t, &modules.TestModules{
		MockAttestHardwareKey: func(_ context.Context, _ interface{}, policy keys.PrivateKeyPolicy, _ *keys.AttestationStatement, _ crypto.PublicKey, _ time.Duration) (keys.PrivateKeyPolicy, error) {
			return "", keys.NewPrivateKeyPolicyError(policy)
		},
	})

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	// 1st login step: request challenge.
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
	sessionResp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finishsession"), &client.AuthenticateWebUserRequest{
		User:                      user,
		WebauthnAssertionResponse: assertionResp,
	})
	require.Error(t, err)
	var resErr httpErrorResponse
	require.NoError(t, json.Unmarshal(sessionResp.Bytes(), &resErr))
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

	// Prepare SSH key to be signed.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	require.NoError(t, err)
	pubBytes := ssh.MarshalAuthorizedKey(pub)

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	tests := []struct {
		name  string
		login func(t *testing.T, assertionResp *wanlib.CredentialAssertionResponse)
	}{
		{
			name: "ssh",
			login: func(t *testing.T, assertionResp *wanlib.CredentialAssertionResponse) {
				ep := clt.Endpoint("webapi", "mfa", "login", "finish")
				sshResp, err := clt.PostJSON(ctx, ep, &client.AuthenticateSSHUserRequest{
					WebauthnChallengeResponse: assertionResp, // no username
					PubKey:                    pubBytes,
					TTL:                       24 * time.Hour,
				})
				require.NoError(t, err, "Passwordless authentication failed")
				loginResp := &auth.SSHLoginResponse{}
				require.NoError(t, json.Unmarshal(sshResp.Bytes(), loginResp))
				require.Equal(t, user, loginResp.Username)
			},
		},
		{
			name: "web",
			login: func(t *testing.T, assertionResp *wanlib.CredentialAssertionResponse) {
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
	err = authServer.SetAuthPreference(ctx, cap)
	require.NoError(t, err)

	// Create user.
	const user = "llama"
	const password = "password"
	env.proxies[0].createUser(ctx, t, user, "root", "password", "" /* otpSecret */, nil /* roles */)

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
