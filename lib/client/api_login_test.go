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

package client_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/base32"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/defaults"
	dtauthntypes "github.com/gravitational/teleport/lib/devicetrust/authn/types"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestTeleportClient_Login_local(t *testing.T) {
	t.Parallel()

	type webauthnFunc func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error)

	waitForCancelFn := func(ctx context.Context) (string, error) {
		<-ctx.Done() // wait for timeout
		return "", ctx.Err()
	}
	noopWebauthnFn := func(_ *mocku2f.Key, _ []byte) webauthnFunc {
		return func(ctx context.Context, _ string, _ *wantypes.CredentialAssertion, _ wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
			<-ctx.Done() // wait for timeout
			return nil, ctx.Err()
		}
	}

	solveOTP := func(otpKey string, clock clockwork.Clock) func(ctx context.Context) (string, error) {
		return func(ctx context.Context) (string, error) {
			return totp.GenerateCode(otpKey, clock.Now())
		}
	}
	solveWebauthn := func(device *mocku2f.Key, _ []byte) webauthnFunc {
		return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
			ackTouch, err := prompt.PromptTouch()
			if err != nil {
				return nil, err
			}

			car, err := device.SignAssertion(origin, assertion)
			if err != nil {
				return nil, err
			}
			ackTouch()
			return &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: wantypes.CredentialAssertionResponseToProto(car),
				},
			}, nil
		}
	}
	solvePwdless := func(device *mocku2f.Key, webID []byte) webauthnFunc {
		return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
			resp, err := solveWebauthn(device, webID)(ctx, origin, assertion, prompt)
			if err == nil {
				resp.GetWebauthn().Response.UserHandle = webID
			}
			return resp, err
		}
	}

	const pin = "pin123"
	userPINFn := func(ctx context.Context) (string, error) {
		return pin, nil
	}
	solvePIN := func(device *mocku2f.Key, webID []byte) webauthnFunc {
		return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
			// Ask and verify the PIN. Usually the authenticator would verify the PIN,
			// but we are faking it here.
			got, err := prompt.PromptPIN()
			switch {
			case err != nil:
				return nil, err
			case got != pin:
				return nil, errors.New("invalid PIN")
			}

			resp, err := solveWebauthn(device, webID)(ctx, origin, assertion, prompt)
			if err != nil {
				return nil, err
			}
			return resp, nil
		}
	}

	tests := []struct {
		name                    string
		makeInputReader         func(pass, otpKey string, clock clockwork.Clock) *prompt.FakeReader
		makeSolveWebauthn       func(device *mocku2f.Key, webID []byte) webauthnFunc
		authConnector           string
		allowStdinHijack        bool
		preferOTP               bool
		hasTouchIDCredentials   bool
		authenticatorAttachment wancli.AuthenticatorAttachment
	}{
		{
			name: "OTP device login with hijack",
			makeInputReader: func(pass, otpKey string, clock clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(solveOTP(otpKey, clock))
			},
			makeSolveWebauthn: noopWebauthnFn,
			allowStdinHijack:  true,
		},
		{
			name: "Webauthn device login with hijack",
			makeInputReader: func(pass, _ string, _ clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(waitForCancelFn)
			},
			makeSolveWebauthn: solveWebauthn,
			allowStdinHijack:  true,
		},
		{
			name: "Webauthn device with PIN and hijack", // a bit hypothetical, but _could_ happen.
			makeInputReader: func(pass, _ string, _ clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(waitForCancelFn).
					AddReply(userPINFn)
			},

			makeSolveWebauthn: solvePIN,
			allowStdinHijack:  true,
		},
		{
			name: "OTP preferred",
			makeInputReader: func(pass, otpKey string, clock clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(solveOTP(otpKey, clock))
			},
			makeSolveWebauthn: func(_ *mocku2f.Key, _ []byte) webauthnFunc {
				return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
					panic("this should not be called")
				}
			},
			preferOTP: true,
		},
		{
			name: "Webauthn device login",
			makeInputReader: func(pass, _ string, _ clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(func(ctx context.Context) (string, error) {
						panic("this should not be called")
					})
			},
			makeSolveWebauthn: solveWebauthn,
		},
		{
			name: "passwordless login",
			makeInputReader: func(_, _ string, _ clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader() // no inputs
			},
			makeSolveWebauthn: solvePwdless,
			authConnector:     constants.PasswordlessConnector,
		},
		{
			name: "default to passwordless if registered",
			makeInputReader: func(_, _ string, _ clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader() // no inputs
			},
			makeSolveWebauthn:     solvePwdless,
			hasTouchIDCredentials: true,
		},
		{
			name: "cross-platform attachment doesn't default to passwordless",
			makeInputReader: func(pass, _ string, _ clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(func(ctx context.Context) (string, error) {
						panic("this should not be called")
					})
			},
			makeSolveWebauthn:       solveWebauthn,
			hasTouchIDCredentials:   true,
			authenticatorAttachment: wancli.AttachmentCrossPlatform,
		},
		{
			name: "local connector doesn't default to passwordless",
			makeInputReader: func(pass, _ string, _ clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(func(ctx context.Context) (string, error) {
						panic("this should not be called")
					})
			},
			makeSolveWebauthn:     solveWebauthn,
			authConnector:         constants.LocalConnector,
			hasTouchIDCredentials: true,
		},
		{
			name: "OTP preferred doesn't default to passwordless",
			makeInputReader: func(pass, otpKey string, clock clockwork.Clock) *prompt.FakeReader {
				return prompt.NewFakeReader().
					AddString(pass).
					AddReply(solveOTP(otpKey, clock))
			},
			makeSolveWebauthn: func(_ *mocku2f.Key, _ []byte) webauthnFunc {
				return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt) (*proto.MFAAuthenticateResponse, error) {
					panic("this should not be called")
				}
			},
			preferOTP:             true,
			hasTouchIDCredentials: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Start Teleport.
			clock := clockwork.NewFakeClockAt(time.Now())
			sa := newStandaloneTeleport(t, clock)
			username := sa.Username
			password := sa.Password
			webID := sa.WebAuthnID
			device := sa.Device
			otpKey := sa.OTPKey

			// Prepare client config.
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

			// Prepare the client proper.
			tc, err := client.NewClient(cfg)
			require.NoError(t, err)
			tc.AllowStdinHijack = test.allowStdinHijack
			tc.AuthConnector = test.authConnector
			tc.PreferOTP = test.preferOTP
			tc.AuthenticatorAttachment = test.authenticatorAttachment
			inputReader := test.makeInputReader(password, otpKey, clock)
			tc.StdinFunc = func() prompt.StdinReader { return inputReader }

			solveWebauthn := test.makeSolveWebauthn(device, webID)
			tc.WebauthnLogin = func(
				ctx context.Context,
				origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts,
			) (*proto.MFAAuthenticateResponse, string, error) {
				resp, err := solveWebauthn(ctx, origin, assertion, prompt)
				return resp, "", err
			}

			tc.HasTouchIDCredentialsFunc = func(_, _ string) bool {
				return test.hasTouchIDCredentials
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Test.
			clock.Advance(30 * time.Second)
			_, err = tc.Login(ctx)
			require.NoError(t, err)
		})
	}
}

func TestTeleportClient_DeviceLogin(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())
	sa := newStandaloneTeleport(t, clock)
	username := sa.Username
	password := sa.Password

	// Disable MFA. It makes testing easier.
	ctx := context.Background()
	authServer := sa.Auth.GetAuthServer()
	authPref, err := authServer.GetAuthPreference(ctx)
	require.NoError(t, err, "GetAuthPreference failed")
	authPref.SetType(constants.Local)
	authPref.SetSecondFactor(constants.SecondFactorOff)
	authPref.SetAllowPasswordless(false)
	authPref.SetAllowHeadless(false)
	_, err = authServer.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err, "UpsertAuthPreference failed")

	// Prepare client config, it won't change throughout the test.
	cfg := client.MakeDefaultConfig()
	cfg.Stdout = io.Discard
	cfg.Stderr = io.Discard
	cfg.Stdin = &bytes.Buffer{}
	cfg.Username = username
	cfg.HostLogin = username
	cfg.AddKeysToAgent = client.AddKeysToAgentNo
	cfg.WebProxyAddr = sa.ProxyWebAddr
	cfg.KeysDir = t.TempDir()
	cfg.InsecureSkipVerify = true

	teleportClient, err := client.NewClient(cfg)
	require.NoError(t, err, "NewClient failed")

	// Prepare prompt with the user password, for login, and reset it after tests.
	oldStdin := prompt.Stdin()
	t.Cleanup(func() { prompt.SetStdin(oldStdin) })
	prompt.SetStdin(prompt.NewFakeReader().AddString(password))

	// Login the current user and fetch a valid pair of certificates.
	keyRing, err := teleportClient.Login(ctx)
	require.NoError(t, err, "Login failed")

	// Sanity check we're generating EC keys.
	assert.IsType(t, ed25519.PrivateKey{}, keyRing.SSHPrivateKey.Signer)
	assert.IsType(t, &ecdsa.PrivateKey{}, keyRing.TLSPrivateKey.Signer)

	proxyClient, rootAuthClient, err := teleportClient.ConnectToRootCluster(ctx, keyRing)
	require.NoError(t, err, "Connecting to the root cluster failed")
	t.Cleanup(func() {
		require.NoError(t, rootAuthClient.Close())
		require.NoError(t, proxyClient.Close())
	})

	// Prepare "device aware" certificates from key.
	// In a real scenario these would be augmented certs.
	block, _ := pem.Decode(keyRing.TLSCert)
	require.NotNil(t, block, "Decode failed")
	validCerts := &devicepb.UserCertificates{
		X509Der:          block.Bytes,
		SshAuthorizedKey: keyRing.Cert,
	}

	t.Run("device login", func(t *testing.T) {
		// We need this because the running standalone process is not Enterprise.
		teleportClient.SetDTAttemptLoginIgnorePing(true)

		// validatingRunCeremony checks the parameters passed to dtAuthnRunCeremony
		// and returns validCerts on success.
		var runCeremonyCalls int
		validatingRunCeremony := func(_ context.Context, params *dtauthntypes.CeremonyRunParams) (*devicepb.UserCertificates, error) {
			runCeremonyCalls++
			switch {
			case params.DevicesClient == nil:
				return nil, errors.New("want non-nil DevicesClient")
			case params.Certs == nil:
				return nil, errors.New("want non-nil Certs")
			case params.SSHSigner == nil:
				return nil, errors.New("want non-nil SSHSigner")
			}
			return validCerts, nil
		}
		teleportClient.SetDTAuthnRunCeremony(validatingRunCeremony)

		// Sanity check that we can do authenticated actions before
		// AttemptDeviceLogin.
		authenticatedAction := func() error {
			// Any authenticated action would do.
			_, err := teleportClient.ListNodesWithFilters(ctx)
			return err
		}
		require.NoError(t, authenticatedAction(), "Authenticated action failed *before* AttemptDeviceLogin")

		// Test! Exercise DeviceLogin.
		got, err := teleportClient.DeviceLogin(ctx, &dtauthntypes.CeremonyRunParams{
			DevicesClient: rootAuthClient.DevicesClient(),
			Certs: &devicepb.UserCertificates{
				SshAuthorizedKey: keyRing.Cert,
			},
			SSHSigner: keyRing.SSHPrivateKey,
		})
		require.NoError(t, err, "DeviceLogin failed")
		require.Equal(t, validCerts, got, "DeviceLogin mismatch")
		assert.Equal(t, 1, runCeremonyCalls, `DeviceLogin didn't call dtAuthnRunCeremony()`)

		// Test! Exercise AttemptDeviceLogin.
		require.NoError(t,
			teleportClient.AttemptDeviceLogin(ctx, keyRing, rootAuthClient),
			"AttemptDeviceLogin failed")
		assert.Equal(t, 2, runCeremonyCalls, `AttemptDeviceLogin didn't call dtAuthnRunCeremony()`)

		// Verify that the "new" key was applied correctly.
		require.NoError(t, authenticatedAction(), "Authenticated action failed *after* AttemptDeviceLogin")
	})

	t.Run("attempt login respects ping", func(t *testing.T) {
		runCeremonyCalled := false
		teleportClient.SetDTAttemptLoginIgnorePing(false)
		teleportClient.SetDTAuthnRunCeremony(func(_ context.Context, _ *dtauthntypes.CeremonyRunParams) (*devicepb.UserCertificates, error) {
			runCeremonyCalled = true
			return nil, errors.New("dtAuthnRunCeremony called unexpectedly")
		})

		// Sanity check the Ping response.
		resp, err := teleportClient.Ping(ctx)
		require.NoError(t, err, "Ping failed")
		require.True(t, resp.Auth.DeviceTrust.Disabled, "Expected device trust to be disabled for Teleport OSS")

		// Test!
		// AttemptDeviceLogin should obey Ping and not attempt the ceremony.
		require.NoError(t,
			teleportClient.AttemptDeviceLogin(ctx, keyRing, rootAuthClient),
			"AttemptDeviceLogin failed")
		assert.False(t, runCeremonyCalled, "AttemptDeviceLogin called DeviceLogin/dtAuthnRunCeremony, despite the Ping response")
	})

	t.Run("device auto-enroll", func(t *testing.T) {
		// Setup:
		// - Ignore ping value for auto-enroll (only Enterprise can truly enable
		//   auto-enroll)
		// - RunCeremony only succeeds after AutoEnroll is called (simulate an
		//   unenrolled device)
		var enrolled bool
		var runCeremonyCalls, autoEnrollCalls int
		teleportClient.SetDTAutoEnrollIgnorePing(true)
		teleportClient.SetDTAuthnRunCeremony(func(_ context.Context, _ *dtauthntypes.CeremonyRunParams) (*devicepb.UserCertificates, error) {
			runCeremonyCalls++
			if !enrolled {
				return nil, errors.New("device not enrolled")
			}
			return validCerts, nil
		})
		teleportClient.SetDTAutoEnroll(func(_ context.Context, _ devicepb.DeviceTrustServiceClient) (*devicepb.Device, error) {
			autoEnrollCalls++
			enrolled = true
			return &devicepb.Device{
				Id: "mydevice",
			}, nil
		})

		clusterClient, err := teleportClient.ConnectToCluster(ctx)
		require.NoError(t, err)
		defer clusterClient.Close()

		rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
		require.NoError(t, err)
		defer rootAuthClient.Close()

		// Test!
		got, err := teleportClient.DeviceLogin(ctx, &dtauthntypes.CeremonyRunParams{
			DevicesClient: rootAuthClient.DevicesClient(),
			Certs: &devicepb.UserCertificates{
				SshAuthorizedKey: keyRing.Cert,
			},
			SSHSigner: keyRing.SSHPrivateKey,
		})
		require.NoError(t, err, "DeviceLogin failed")
		assert.Equal(t, got, validCerts, "DeviceLogin mismatch")
		assert.Equal(t, 2, runCeremonyCalls, "RunCeremony called an unexpected number of times")
		assert.Equal(t, 1, autoEnrollCalls, "AutoEnroll called an unexpected number of times")
	})
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
// function and reusing in other places.
func newStandaloneTeleport(t *testing.T, clock clockwork.Clock) *standaloneBundle {
	randomAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"}

	staticToken := uuid.New().String()

	// Prepare role and user.
	// Both resources are bootstrapped by the Auth Server below.
	const username = "llama"
	role, err := types.NewRole(username, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	user, err := types.NewUser("llama")
	require.NoError(t, err)
	user.AddRole(role.GetName())

	// Use os.MkdirTemp() instead of t.TempDir() for the DataDir.
	// The shorter temp paths avoid problems with long unix socket paths composed
	// using the DataDir.
	// See https://github.com/golang/go/issues/62614.
	makeDataDir := func() string {
		tempDir, err := os.MkdirTemp("", "teleport-test-")
		require.NoError(t, err, "os.MkdirTemp failed")
		t.Cleanup(func() { os.RemoveAll(tempDir) })
		return tempDir
	}

	// AuthServer setup.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.DataDir = makeDataDir()
	cfg.Hostname = "localhost"
	cfg.Clock = clock
	cfg.Logger = utils.NewSlogLoggerForTests()
	cfg.SetAuthServerAddress(randomAddr) // must be present
	cfg.Auth.Preference, err = types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
		SignatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
	})
	require.NoError(t, err)
	cfg.Auth.BootstrapResources = []types.Resource{role, user}
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
	cfg.Auth.ListenAddr = randomAddr
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = false
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.InstanceMetadataClient = imds.NewDisabledIMDSClient()
	authProcess := startAndWait(t, cfg, service.AuthTLSReady)
	t.Cleanup(func() { authProcess.Close() })
	authAddr, err := authProcess.AuthAddr()
	require.NoError(t, err)

	// Use the same clock on AuthServer, it doesn't appear to cascade from
	// configs.
	authServer := authProcess.GetAuthServer()
	authServer.SetClock(clock)

	// Initialize user's password and MFA.
	ctx := context.Background()
	const password = "supersecretpassword"
	token, err := authServer.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
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
	cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())
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
				Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
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
	cfg = servicecfg.MakeDefaultConfig()
	cfg.DataDir = makeDataDir()
	cfg.Hostname = "localhost"
	cfg.SetToken(staticToken)
	cfg.Clock = clock
	cfg.Logger = utils.NewSlogLoggerForTests()
	cfg.SetAuthServerAddress(*authAddr)
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = true
	cfg.Proxy.WebAddr = randomAddr
	cfg.Proxy.SSHAddr = randomAddr
	cfg.Proxy.ReverseTunnelListenAddr = randomAddr
	cfg.Proxy.DisableWebInterface = true
	cfg.SSH.Enabled = false
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.InstanceMetadataClient = imds.NewDisabledIMDSClient()
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

func startAndWait(t *testing.T, cfg *servicecfg.Config, eventName string) *service.TeleportProcess {
	instance, err := service.NewTeleport(cfg)
	require.NoError(t, err)
	require.NoError(t, instance.Start())

	_, err = instance.WaitForEventTimeout(30*time.Second, eventName)
	require.NoError(t, err, "timed out waiting for teleport")

	return instance
}

func TestRetryWithRelogin(t *testing.T) {
	clock := clockwork.NewFakeClockAt(time.Now())
	sa := newStandaloneTeleport(t, clock)

	cfg := client.MakeDefaultConfig()
	cfg.Username = sa.Username
	cfg.HostLogin = sa.Username
	cfg.WebProxyAddr = sa.ProxyWebAddr
	cfg.KeysDir = t.TempDir()
	cfg.InsecureSkipVerify = true
	cfg.AllowStdinHijack = true

	tc, err := client.NewClient(cfg)
	require.NoError(t, err)

	errorOnTry := func(counter *int, failedTry int) func() error {
		return func() error {
			*counter++
			if *counter == failedTry {
				return errors.New("ssh: cert has expired")
			}
			return nil
		}
	}

	t.Run("Does not try login if function succeeds on the first run", func(t *testing.T) {
		calledTimes := 0

		err = client.RetryWithRelogin(context.Background(), tc, errorOnTry(&calledTimes, -1))

		require.NoError(t, err)
		require.Equal(t, 1, calledTimes)
	})
	t.Run("Runs 'beforeLoginHook' before login, if it's present", func(t *testing.T) {
		calledTimes := 0

		err = client.RetryWithRelogin(context.Background(), tc, errorOnTry(&calledTimes, 1), client.WithBeforeLoginHook(
			func() error {
				return errors.New("hook called")
			}))

		require.ErrorContains(t, err, "hook called")
		require.Equal(t, 1, calledTimes)
	})

	t.Run("Successful retry after login", func(t *testing.T) {
		solveOTP := func(ctx context.Context) (string, error) {
			return totp.GenerateCode(sa.OTPKey, clock.Now())
		}
		oldStdin := prompt.Stdin()
		t.Cleanup(func() {
			prompt.SetStdin(oldStdin)
		})
		prompt.SetStdin(prompt.NewFakeReader().AddString(sa.Password).AddReply(solveOTP))
		calledTimes := 0

		err = client.RetryWithRelogin(context.Background(), tc, errorOnTry(&calledTimes, 1))

		require.NoError(t, err)
		require.Equal(t, 2, calledTimes)
	})
}
