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

package client_test

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	u2flib "github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	log "github.com/sirupsen/logrus"
)

func TestTeleportClient_Login_localMFALogin(t *testing.T) {
	// Silence logging during this test.
	lvl := log.GetLevel()
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetLevel(lvl)
	})
	log.SetOutput(io.Discard)
	log.SetLevel(log.PanicLevel)

	clock := clockwork.NewFakeClockAt(time.Now())
	sa := newStandaloneTeleport(t, clock)
	username := sa.Username
	password := sa.Password
	device := sa.Device
	otpKey := sa.OTPKey

	// Prepare client config, it won't change throughout the test.
	cfg := client.MakeDefaultConfig()
	cfg.Stdout = io.Discard
	cfg.Stderr = io.Discard
	cfg.Stdin = &bytes.Buffer{}
	cfg.Username = username
	cfg.HostLogin = username
	cfg.AddKeysToAgent = client.AddKeysToAgentNo
	// Replace "127.0.0.1" with "localhost". The proxy address becomes the origin
	// for Webauthn requests, and Webauthn doesn't take IP addresses.
	cfg.WebProxyAddr = strings.Replace(sa.ProxyWebAddr, "127.0.0.1", "localhost", 1 /* n */)
	cfg.KeysDir = t.TempDir()
	cfg.InsecureSkipVerify = true

	// Replace (and later reset) user-prompting functions.
	oldPwd, oldOTP, oldU2F, oldWAN := *client.PasswordFromConsoleFn, *client.PromptOTP, *client.PromptU2F, *client.PromptWebauthn
	t.Cleanup(func() {
		*client.PasswordFromConsoleFn = oldPwd
		*client.PromptOTP = oldOTP
		*client.PromptU2F = oldU2F
		*client.PromptWebauthn = oldWAN
	})
	*client.PasswordFromConsoleFn = func() (string, error) {
		return password, nil
	}

	// The loginMocks setup below is used to avoid data races when reading or
	// writing client.Prompt* pointers.
	// Tests are supposed to replace loginMocks functions instead.
	loginMocks := struct {
		promptOTP      func(ctx context.Context) (string, error)
		promptU2F      func(ctx context.Context, facet string, challenges ...u2flib.AuthenticateChallenge) (*u2flib.AuthenticateChallengeResponse, error)
		promptWebauthn func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error)
	}{}
	var loginMocksMU sync.RWMutex
	*client.PromptOTP = func(ctx context.Context, out io.Writer, in *prompt.ContextReader, question string) (string, error) {
		loginMocksMU.RLock()
		defer loginMocksMU.RUnlock()
		return loginMocks.promptOTP(ctx)
	}
	*client.PromptU2F = func(ctx context.Context, facet string, challenges ...u2flib.AuthenticateChallenge) (*u2flib.AuthenticateChallengeResponse, error) {
		loginMocksMU.RLock()
		defer loginMocksMU.RUnlock()
		return loginMocks.promptU2F(ctx, facet, challenges...)
	}
	*client.PromptWebauthn = func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error) {
		loginMocksMU.RLock()
		defer loginMocksMU.RUnlock()
		return loginMocks.promptWebauthn(ctx, origin, assertion)
	}

	promptOTPNoop := func(ctx context.Context) (string, error) {
		<-ctx.Done() // wait for timeout
		return "", ctx.Err()
	}
	promptWebauthnNoop := func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error) {
		<-ctx.Done() // wait for timeout
		return nil, ctx.Err()
	}

	solveOTP := func(ctx context.Context) (string, error) {
		return totp.GenerateCode(otpKey, clock.Now())
	}
	solveU2F := func(ctx context.Context, facet string, challenges ...u2flib.AuthenticateChallenge) (*u2flib.AuthenticateChallengeResponse, error) {
		kh := base64.RawURLEncoding.EncodeToString(device.KeyHandle)
		for _, c := range challenges {
			if kh == c.KeyHandle {
				return device.SignResponse(&c)
			}
		}
		return nil, trace.BadParameter("key handle now found")
	}
	solveWebauthn := func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error) {
		car, err := device.SignAssertion(origin, assertion)
		if err != nil {
			return nil, err
		}
		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wanlib.CredentialAssertionResponseToProto(car),
			},
		}, nil
	}

	ctx := context.Background()
	tests := []struct {
		name          string
		secondFactor  constants.SecondFactorType
		solveOTP      func(context.Context) (string, error)
		solveU2F      func(ctx context.Context, facet string, challenges ...u2flib.AuthenticateChallenge) (*u2flib.AuthenticateChallengeResponse, error)
		solveWebauthn func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error)
	}{
		{
			name:         "OK OTP device login",
			secondFactor: constants.SecondFactorOptional,
			solveOTP:     solveOTP,
			solveU2F: func(context.Context, string, ...u2flib.AuthenticateChallenge) (*u2flib.AuthenticateChallengeResponse, error) {
				panic("unused")
			},
			solveWebauthn: promptWebauthnNoop,
		},
		{
			name:         "OK Webauthn device login",
			secondFactor: constants.SecondFactorOptional,
			solveOTP:     promptOTPNoop,
			solveU2F: func(context.Context, string, ...u2flib.AuthenticateChallenge) (*u2flib.AuthenticateChallengeResponse, error) {
				panic("unused")
			},
			solveWebauthn: solveWebauthn,
		},
		{
			name:         "OK U2F device login",
			secondFactor: constants.SecondFactorU2F,
			solveOTP:     promptOTPNoop,
			solveU2F:     solveU2F,
			solveWebauthn: func(context.Context, string, *wanlib.CredentialAssertion) (*proto.MFAAuthenticateResponse, error) {
				panic("unused")
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			loginMocksMU.Lock()
			loginMocks.promptOTP = test.solveOTP
			loginMocks.promptU2F = test.solveU2F
			loginMocks.promptWebauthn = test.solveWebauthn
			loginMocksMU.Unlock()

			authServer := sa.Auth.GetAuthServer()
			pref, err := authServer.GetAuthPreference(ctx)
			require.NoError(t, err)
			if pref.GetSecondFactor() != test.secondFactor {
				pref.SetSecondFactor(test.secondFactor)
				require.NoError(t, authServer.SetAuthPreference(ctx, pref))
			}

			tc, err := client.NewClient(cfg)
			require.NoError(t, err)

			clock.Advance(30 * time.Second)
			_, err = tc.Login(ctx)
			require.NoError(t, err)
		})
	}
}

type standaloneBundle struct {
	AuthAddr, ProxyWebAddr string
	Username, Password     string
	Device                 *mocku2f.Key
	OTPKey                 string
	Auth, Proxy            *service.TeleportProcess
}

// TODO(codingllama): Consider refactoring newStandaloneTeleport into a public
//  function and reusing in other places.
func newStandaloneTeleport(t *testing.T, clock clockwork.Clock) *standaloneBundle {
	randomAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}

	// Silent logger and console.
	logger := utils.NewLoggerForTests()
	logger.SetLevel(log.PanicLevel)
	logger.SetOutput(io.Discard)
	console := io.Discard

	staticToken := uuid.New().String()

	user, err := types.NewUser("llama")
	require.NoError(t, err)
	role, err := types.NewRole(user.GetName(), types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{user.GetName()},
		},
	})
	require.NoError(t, err)

	// AuthServer setup.
	cfg := service.MakeDefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Hostname = "localhost"
	cfg.Clock = clock
	cfg.Console = console
	cfg.Log = logger
	cfg.AuthServers = []utils.NetAddr{randomAddr} // must be present
	cfg.Auth.Preference, err = types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		U2F: &types.U2F{
			AppID:  "localhost",
			Facets: []string{"https://localhost", "localhost"},
		},
	})
	require.NoError(t, err)
	cfg.Auth.Resources = []types.Resource{user, role}
	cfg.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles:   []types.SystemRole{types.RoleProxy},
				Expires: time.Now().Add(1 * time.Hour),
				Token:   staticToken,
			},
		},
	})
	require.NoError(t, err)
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	cfg.Auth.SSHAddr = randomAddr
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false
	authProcess := startAndWait(t, cfg, service.AuthTLSReady)
	t.Cleanup(func() { authProcess.Close() })
	authAddr, err := authProcess.AuthSSHAddr()
	require.NoError(t, err)

	// Use the same clock on AuthServer, it doesn't appear to cascade from
	// configs.
	authServer := authProcess.GetAuthServer()
	authServer.SetClock(clock)

	// Initialize user's password and MFA.
	ctx := context.Background()
	username := user.GetName()
	const password = "supersecretpassword"
	token, err := authServer.CreateResetPasswordToken(ctx, auth.CreateUserTokenRequest{
		Name: username,
	})
	require.NoError(t, err)
	tokenID := token.GetName()
	res, err := authServer.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:    tokenID,
		DeviceType: proto.DeviceType_DEVICE_TYPE_U2F,
	})
	require.NoError(t, err)
	device, err := mocku2f.Create()
	require.NoError(t, err)
	registerResp, err := device.RegisterResponse(&u2f.RegisterChallenge{
		Version:   res.GetU2F().GetVersion(),
		Challenge: res.GetU2F().GetChallenge(),
		AppID:     res.GetU2F().GetAppID(),
	})
	require.NoError(t, err)
	_, err = authServer.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     tokenID,
		NewPassword: []byte(password),
		NewMFARegisterResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_U2F{
				U2F: &proto.U2FRegisterResponse{
					RegistrationData: registerResp.RegistrationData,
					ClientData:       registerResp.ClientData,
				},
			},
		},
	})
	require.NoError(t, err)

	// Insert an OTP device.
	otpKey := base32.StdEncoding.EncodeToString([]byte("llamasrule"))
	otpDevice, err := services.NewTOTPDevice("otp", otpKey, clock.Now() /* addedAt */)
	require.NoError(t, err)
	require.NoError(t, authServer.UpsertMFADevice(ctx, username, otpDevice))

	// Proxy setup.
	cfg = service.MakeDefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Hostname = "localhost"
	cfg.Token = staticToken
	cfg.Clock = clock
	cfg.Console = console
	cfg.Log = logger
	cfg.AuthServers = []utils.NetAddr{*authAddr}
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = true
	cfg.Proxy.WebAddr = randomAddr
	cfg.Proxy.SSHAddr = randomAddr
	cfg.Proxy.ReverseTunnelListenAddr = randomAddr
	cfg.Proxy.DisableWebInterface = true
	cfg.SSH.Enabled = false
	proxyProcess := startAndWait(t, cfg, service.ProxyWebServerReady)
	t.Cleanup(func() { proxyProcess.Close() })
	proxyWebAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	return &standaloneBundle{
		AuthAddr:     authAddr.String(),
		ProxyWebAddr: proxyWebAddr.String(),
		Username:     username,
		Password:     password,
		Device:       device,
		OTPKey:       otpKey,
		Auth:         authProcess,
		Proxy:        proxyProcess,
	}
}

func startAndWait(t *testing.T, cfg *service.Config, eventName string) *service.TeleportProcess {
	instance, err := service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, instance.Start())

	eventC := make(chan service.Event, 1)
	instance.WaitForEvent(instance.ExitContext(), eventName, eventC)
	select {
	case <-eventC:
	case <-time.After(30 * time.Second):
		t.Fatal("Timed out waiting for teleport")
	}

	return instance
}
