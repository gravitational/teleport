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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib/csrf"
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
		MockAttestationData: &keys.AttestationData{
			PrivateKeyPolicy: keys.PrivateKeyPolicyNone,
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
		login func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse)
	}{
		{
			name: "ssh",
			login: func(t *testing.T, assertionResp *wantypes.CredentialAssertionResponse) {
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

		sessionResp := loginWebOTP(t, ctx, loginWebOTPParams{
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

		sessionResp := loginWebMFA(ctx, t, loginWebMFAParams{
			webClient:     proxy.newClient(t),
			rpID:          rpID,
			user:          mfaResp.User,
			password:      mfaResp.Password,
			authenticator: mfaResp.WebDev.Key,
		})
		assert.Equal(t, wantToken, sessionResp.DeviceWebToken, "WebSession DeviceWebToken mismatch")
	})
}

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
