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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	log "github.com/sirupsen/logrus"
)

func TestTeleportClient_Login_local(t *testing.T) {
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
	webID := sa.WebAuthnID
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

	// Reset functions after tests.
	oldStdin, oldWebauthn := prompt.Stdin(), *client.PromptWebauthn
	t.Cleanup(func() {
		prompt.SetStdin(oldStdin)
		*client.PromptWebauthn = oldWebauthn
	})

	waitForCancelFn := func(ctx context.Context) (string, error) {
		<-ctx.Done() // wait for timeout
		return "", ctx.Err()
	}
	noopWebauthnFn := func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
		<-ctx.Done() // wait for timeout
		return nil, ctx.Err()
	}

	solveOTP := func(ctx context.Context) (string, error) {
		return totp.GenerateCode(otpKey, clock.Now())
	}
	solveWebauthn := func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
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
	solvePwdless := func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
		resp, err := solveWebauthn(ctx, origin, assertion, prompt)
		if err == nil {
			resp.GetWebauthn().Response.UserHandle = webID
		}
		return resp, err
	}

	const pin = "pin123"
	pinC := make(chan struct{})
	userPINFn := func(ctx context.Context) (string, error) {
		<-pinC // Do not answer until we get the WebAuthn prompt.
		return pin, nil
	}
	solvePIN := func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
		go func() {
			// Wait a bit so prompt.PromptPIN() happens first.
			// It's important because it signals cancellation of the OTP read.
			time.Sleep(100 * time.Millisecond)
			pinC <- struct{}{}
		}()

		// Ask and verify the PIN. Usually the authenticator would verify the PIN,
		// but we are faking it here.
		got, err := prompt.PromptPIN()
		switch {
		case err != nil:
			return nil, err
		case got != pin:
			return nil, errors.New("invalid PIN")
		}
		prompt.PromptTouch() // Realistically, this would happen too.
		return solveWebauthn(ctx, origin, assertion, prompt)
	}

	ctx := context.Background()
	tests := []struct {
		name          string
		secondFactor  constants.SecondFactorType
		inputReader   *prompt.FakeReader
		solveWebauthn func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error)
		pwdless       bool
	}{
		{
			name:          "OTP device login",
			secondFactor:  constants.SecondFactorOptional,
			inputReader:   prompt.NewFakeReader().AddString(password).AddReply(solveOTP),
			solveWebauthn: noopWebauthnFn,
		},
		{
			name:          "Webauthn device login",
			secondFactor:  constants.SecondFactorOptional,
			inputReader:   prompt.NewFakeReader().AddString(password).AddReply(waitForCancelFn),
			solveWebauthn: solveWebauthn,
		},
		{
			name:          "Webauthn device with PIN", // a bit hypothetical, but _could_ happen.
			secondFactor:  constants.SecondFactorOptional,
			inputReader:   prompt.NewFakeReader().AddString(password).AddReply(waitForCancelFn).AddReply(userPINFn),
			solveWebauthn: solvePIN,
		},
		{
			name:          "passwordless login",
			secondFactor:  constants.SecondFactorOptional,
			inputReader:   prompt.NewFakeReader(), // no inputs
			solveWebauthn: solvePwdless,
			pwdless:       true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			prompt.SetStdin(test.inputReader)
			*client.PromptWebauthn = func(ctx context.Context, origin, _ string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, string, error) {
				resp, err := test.solveWebauthn(ctx, origin, assertion, prompt)
				return resp, "", err
			}

			authServer := sa.Auth.GetAuthServer()
			pref, err := authServer.GetAuthPreference(ctx)
			require.NoError(t, err)
			if pref.GetSecondFactor() != test.secondFactor {
				pref.SetSecondFactor(test.secondFactor)
				require.NoError(t, authServer.SetAuthPreference(ctx, pref))
			}

			tc, err := client.NewClient(cfg)
			require.NoError(t, err)
			tc.Passwordless = test.pwdless

			clock.Advance(30 * time.Second)
			_, err = tc.Login(ctx)
			require.NoError(t, err)
		})
	}
}

type standaloneBundle struct {
	AuthAddr, ProxyWebAddr string
	Username, Password     string
	WebAuthnID             []byte
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
	role, err := types.NewRoleV3(user.GetName(), types.RoleSpecV5{
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
		Webauthn: &types.Webauthn{
			RPID: "localhost",
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
		TokenID:     tokenID,
		DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	require.NoError(t, err)
	cc := wanlib.CredentialCreationFromProto(res.GetWebauthn())
	webID := cc.Response.User.ID
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()
	const origin = "https://localhost"
	ccr, err := device.SignCredentialCreation(origin, cc)
	require.NoError(t, err)
	_, err = authServer.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID:     tokenID,
		NewPassword: []byte(password),
		NewMFARegisterResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
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
		WebAuthnID:   webID,
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
