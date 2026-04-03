/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apimfa "github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/service"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

type noopTestRegisterCallback struct{}

func (noopTestRegisterCallback) Rollback() error { return nil }

func (noopTestRegisterCallback) Confirm() error { return nil }

func setupMFAAddTestUser(t *testing.T) (*service.TeleportProcess, types.User, string) {
	t.Helper()

	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	process, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithClusterName("root"),
		testserver.WithBootstrap(connector, alice),
	)
	require.NoError(t, err)

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)
	proxyHost, _, err := net.SplitHostPort(proxyAddr.Addr)
	require.NoError(t, err)
	_, err = process.GetAuthServer().UpsertAuthPreference(context.Background(), &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOn,
			Webauthn: &types.Webauthn{
				RPID: proxyHost,
			},
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, process.Close())
		assert.NoError(t, process.Wait())
	})

	tshHome, _ := mustLogin(t, process, alice, connector.GetName())
	return process, alice, tshHome
}

func setMockMFARegister() CliOption {
	device, err := mocku2f.Create()
	if err != nil {
		return func(*CLIConf) error {
			return err
		}
	}
	device.SetPasswordless()

	register := func(origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, error) {
		resp, err := device.SignCredentialCreation(origin, cc)
		if err != nil {
			return nil, err
		}
		return &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wantypes.CredentialCreationResponseToProto(resp),
			},
		}, nil
	}

	return func(cf *CLIConf) error {
		cf.WebauthnRegister = func(ctx context.Context, origin string, cc *wantypes.CredentialCreation, _ wancli.RegisterPrompt) (*proto.MFARegisterResponse, error) {
			return register(origin, cc)
		}
		cf.TouchIDRegister = func(origin string, cc *wantypes.CredentialCreation) (*proto.MFARegisterResponse, apimfa.RegistrationCallbacks, error) {
			resp, err := register(origin, cc)
			if err != nil {
				return nil, nil, err
			}
			return resp, noopTestRegisterCallback{}, nil
		}
		return nil
	}
}

func runMFAAddCommand(ctx context.Context, cmd *mfaAddCommand, opts ...CliOption) error {
	cf := &CLIConf{
		Context: ctx,
	}
	for _, opt := range opts {
		if err := opt(cf); err != nil {
			return err
		}
	}
	return cmd.run(cf)
}

func TestTshMFAAddOTP(t *testing.T) {
	process, alice, tshHome := setupMFAAddTestUser(t)

	oldStdin := prompt.Stdin()
	t.Cleanup(func() {
		prompt.SetStdin(oldStdin)
	})

	stdout := new(bytes.Buffer)
	secretRe := regexp.MustCompile(`Secret key:\s+([A-Z2-7]+)`)
	prompt.SetStdin(prompt.NewFakeReader().AddReply(func(context.Context) (string, error) {
		match := secretRe.FindStringSubmatch(stdout.String())
		if len(match) != 2 {
			return "", fmt.Errorf("TOTP secret not found in output")
		}

		code, err := totp.GenerateCode(match[1], time.Now())
		if err != nil {
			return "", err
		}
		return code, nil
	}))

	err := Run(context.Background(), []string{
		"mfa", "add",
		"--name", "otp-device",
		"--type", "TOTP",
	},
		setHomePath(tshHome),
		setCopyStdout(stdout),
	)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "MFA device \"otp-device\" added.")

	devices, err := process.GetAuthServer().Services.GetMFADevices(context.Background(), alice.GetName(), false)
	require.NoError(t, err)

	for _, device := range devices {
		if device.GetName() == "otp-device" {
			require.NotNil(t, device.GetTotp())
			return
		}
	}
	t.Fatal("OTP device was not found after tsh mfa add")
}

func TestTshMFAAddWebauthn(t *testing.T) {
	process, alice, tshHome := setupMFAAddTestUser(t)

	oldStdin := prompt.Stdin()
	t.Cleanup(func() {
		prompt.SetStdin(oldStdin)
	})
	prompt.SetStdin(prompt.NewFakeReader().AddString("NO"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	stdout := new(bytes.Buffer)
	err := Run(ctx, []string{
		"mfa", "add",
		"--name", "webauthn-device",
		"--type", "WEBAUTHN",
	},
		setHomePath(tshHome),
		setMockMFARegister(),
		setCopyStdout(stdout),
	)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "MFA device \"webauthn-device\" added.")

	devices, err := process.GetAuthServer().Services.GetMFADevices(context.Background(), alice.GetName(), false)
	require.NoError(t, err)

	for _, device := range devices {
		if device.GetName() == "webauthn-device" {
			require.NotNil(t, device.GetWebauthn())
			return
		}
	}
	t.Fatal("WebAuthn device was not found after tsh mfa add")
}

func TestTshMFAAddNativePlatformAuthenticator(t *testing.T) {
	process, alice, tshHome := setupMFAAddTestUser(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	stdout := new(bytes.Buffer)
	err := runMFAAddCommand(ctx, &mfaAddCommand{
		devName: "touchid-device",
		devType: "TOUCHID",
	},
		setHomePath(tshHome),
		setMockMFARegister(),
		setCopyStdout(stdout),
	)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "MFA device \"touchid-device\" added.")

	devices, err := process.GetAuthServer().Services.GetMFADevices(context.Background(), alice.GetName(), false)
	require.NoError(t, err)

	for _, device := range devices {
		if device.GetName() == "touchid-device" {
			require.NotNil(t, device.GetWebauthn())
			return
		}
	}
	t.Fatal("Native platform authenticator device was not found after tsh mfa add")
}
