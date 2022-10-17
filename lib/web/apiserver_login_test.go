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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/client"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestU2FLogin(t *testing.T) {
	for _, sf := range []constants.SecondFactorType{
		constants.SecondFactorU2F,
		constants.SecondFactorOptional,
		constants.SecondFactorOn,
	} {
		sf := sf
		t.Run(fmt.Sprintf("second_factor_%s", sf), func(t *testing.T) {
			t.Parallel()
			testU2FLogin(t, sf)
		})
	}
}

func testU2FLogin(t *testing.T, sf constants.SecondFactorType) {
	env := newWebPack(t, 1)
	clusterMFA := configureClusterForMFA(t, env, &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: sf,
		U2F: &types.U2F{
			AppID:  "https://" + env.server.TLS.ClusterName(),
			Facets: []string{"https://" + env.server.TLS.ClusterName()},
		},
	})
	user := clusterMFA.User
	password := clusterMFA.Password
	device := clusterMFA.U2FDev.Key

	// normal login
	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)
	re, err := clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "signrequest"), client.MFAChallengeRequest{
		User: user,
		Pass: password,
	})
	require.NoError(t, err)
	var u2fSignReq u2f.AuthenticateChallenge
	require.NoError(t, json.Unmarshal(re.Bytes(), &u2fSignReq))

	u2fSignResp, err := device.SignResponse(&u2fSignReq)
	require.NoError(t, err)

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), &client.AuthenticateWebUserRequest{
		User:            user,
		U2FSignResponse: u2fSignResp,
	})
	require.NoError(t, err)

	// bad login: corrupted sign responses, should fail
	re, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "signrequest"), client.MFAChallengeRequest{
		User: user,
		Pass: password,
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &u2fSignReq))

	u2fSignResp, err = device.SignResponse(&u2fSignReq)
	require.NoError(t, err)

	// corrupted KeyHandle
	u2fSignRespCopy := u2fSignResp
	u2fSignRespCopy.KeyHandle = u2fSignRespCopy.KeyHandle + u2fSignRespCopy.KeyHandle
	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), &client.AuthenticateWebUserRequest{
		User:            user,
		U2FSignResponse: u2fSignRespCopy,
	})
	require.Error(t, err)

	// corrupted SignatureData
	u2fSignRespCopy = u2fSignResp
	u2fSignRespCopy.SignatureData = u2fSignRespCopy.SignatureData[:10] + u2fSignRespCopy.SignatureData[20:]

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), &client.AuthenticateWebUserRequest{
		User:            user,
		U2FSignResponse: u2fSignRespCopy,
	})
	require.Error(t, err)

	// corrupted ClientData
	u2fSignRespCopy = u2fSignResp
	u2fSignRespCopy.ClientData = u2fSignRespCopy.ClientData[:10] + u2fSignRespCopy.ClientData[20:]

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), &client.AuthenticateWebUserRequest{
		User:            user,
		U2FSignResponse: u2fSignRespCopy,
	})
	require.Error(t, err)

	// bad login: counter not increasing, should fail
	device.SetCounter(0)
	re, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "signrequest"), client.MFAChallengeRequest{
		User: user,
		Pass: password,
	})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(re.Bytes(), &u2fSignReq))

	u2fSignResp, err = device.SignResponse(&u2fSignReq)
	require.NoError(t, err)

	_, err = clt.PostJSON(context.Background(), clt.Endpoint("webapi", "u2f", "sessions"), &client.AuthenticateWebUserRequest{
		User:            user,
		U2FSignResponse: u2fSignResp,
	})
	require.Error(t, err)
}

func TestWebauthnLogin_ssh_u2fDevice(t *testing.T) {
	env := newWebPack(t, 1)
	clusterMFA := configureClusterForMFA(t, env, &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "https://" + env.server.TLS.ClusterName(),
			Facets: []string{"https://" + env.server.TLS.ClusterName()},
		},
		// Use default Webauthn configuration.
	})
	user := clusterMFA.User
	password := clusterMFA.Password
	device := clusterMFA.U2FDev.Key

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	// 1st login step: request challenge.
	ctx := context.Background()
	beginResp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "begin"), &client.MFAChallengeRequest{
		User: user,
		Pass: password,
	})
	require.NoError(t, err)
	authChallenge := &auth.MFAAuthenticateChallenge{}
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

func TestWebauthnLogin_web_u2fDevice(t *testing.T) {
	env := newWebPack(t, 1)
	clusterMFA := configureClusterForMFA(t, env, &types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOn,
		U2F: &types.U2F{
			AppID:  "https://" + env.server.TLS.ClusterName(),
			Facets: []string{"https://" + env.server.TLS.ClusterName()},
		},
		// Use default Webauthn configuration.
	})
	user := clusterMFA.User
	password := clusterMFA.Password
	device := clusterMFA.U2FDev.Key

	clt, err := client.NewWebClient(env.proxies[0].webURL.String(), roundtrip.HTTPClient(client.NewInsecureWebClient()))
	require.NoError(t, err)

	// 1st login step: request challenge.
	ctx := context.Background()
	beginResp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "begin"), &client.MFAChallengeRequest{
		User: user,
		Pass: password,
	})
	require.NoError(t, err)
	authChallenge := &auth.MFAAuthenticateChallenge{}
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

type configureMFAResp struct {
	User, Password string
	U2FDev         *auth.TestDevice
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
	u2fDev, err := auth.RegisterTestDevice(ctx, clt, "u2f", proto.DeviceType_DEVICE_TYPE_U2F, nil /* authenticator */)
	require.NoError(t, err)

	return &configureMFAResp{
		User:     user,
		Password: password,
		U2FDev:   u2fDev,
	}
}
