/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMFACommands_SSODevice(t *testing.T) {
	ctx := context.Background()

	authPref := types.DefaultAuthPreference()
	authPref.SetSecondFactors(
		types.SecondFactorType_SECOND_FACTOR_TYPE_OTP,
		types.SecondFactorType_SECOND_FACTOR_TYPE_WEBAUTHN,
		types.SecondFactorType_SECOND_FACTOR_TYPE_SSO,
	)
	authPref.SetWebauthn(&types.Webauthn{
		RPID: "localhost",
	})

	// Create an SSO user which can easily add MFA devices.
	connector := mockConnector(t)
	username := "user@google.com"
	user, err := types.NewUser(username)
	require.NoError(t, err)
	user.SetRoles([]string{teleport.PresetAccessRoleName})
	user.SetLogins(nil)
	userCreatedAt := time.Now()
	user.SetCreatedBy(types.CreatedBy{
		Time: userCreatedAt,
		Connector: &types.ConnectorRef{
			ID:   connector.GetName(),
			Type: connector.GetKind(),
		},
	})

	srv := testenv.MakeTestServer(t, testenv.WithAuthPreference(authPref), testenv.WithBootstrap(connector, user))
	auth := srv.GetAuthServer()
	proxyAddr, err := srv.ProxyWebAddr()
	require.NoError(t, err)

	tmpHomePath := t.TempDir()

	mfaList := func(t *testing.T) string {
		commandOutput := new(bytes.Buffer)
		err = Run(ctx,
			[]string{"mfa", "ls", "--format", teleport.JSON},
			setHomePath(tmpHomePath),
			setOverrideStdout(commandOutput),
		)
		require.NoError(t, err)
		return string(commandOutput.Bytes())
	}

	// Prepare a WebAuthn device.
	device, err := mocku2f.Create()
	require.NoError(t, err)
	webauthnLoginOpt := setupWebAuthnChallengeSolver(device, true /* success */)

	mfaAddWebAuthn := func(t *testing.T) string {
		commandOutput := new(bytes.Buffer)
		err = Run(ctx,
			[]string{"mfa", "add", "--name", "webauthn-device", "--type", "WEBAUTHN", "--allow-passwordless"},
			setHomePath(tmpHomePath),
			setOverrideStdout(commandOutput),
			webauthnLoginOpt,
		)
		require.NoError(t, err)
		return string(commandOutput.Bytes())
	}

	mfaRemove := func(t *testing.T, name string) string {
		commandOutput := new(bytes.Buffer)
		err = Run(ctx,
			[]string{"mfa", "rm", name},
			setHomePath(tmpHomePath),
			setOverrideStdout(commandOutput),
		)
		require.NoError(t, err)
		return string(commandOutput.Bytes())
	}

	// login.
	err = Run(ctx,
		[]string{"login", "-d", "--insecure", "--proxy", proxyAddr.String()},
		setHomePath(tmpHomePath),
		setMockSSOLogin(auth, user, connector.GetName()))
	require.NoError(t, err)

	// mfa ls should output no devices.
	out := mfaList(t)
	require.NoError(t, err)
	require.Equal(t, "null\n", out)

	// Add a webauthn device, it should show up in the list.
	out = mfaAddWebAuthn(t)
	require.Empty(t, out)

	out = mfaList(t)
	require.NoError(t, err)

	expectJSON, err := serializeMFADevices([]*types.MFADevice{}, teleport.JSON)
	require.Equal(t, expectJSON+"\n", out)

	// Enable MFA in the SSO connector, we should now see the device.
	connector.SetMFASettings(&types.OIDCConnectorMFASettings{
		Enabled:  true,
		ClientId: "mfa-client",
	})
	_, err = auth.UpsertOIDCConnector(ctx, connector)
	require.NoError(t, err)

	// The SSO device should look like this, but we don't add it directly.
	ssoDev, err := types.NewMFADevice(connector.GetDisplay(), connector.GetName(), userCreatedAt, &types.MFADevice_Sso{
		Sso: &types.SSOMFADevice{
			ConnectorId:   connector.GetName(),
			ConnectorType: connector.GetKind(),
		},
	})

	out = mfaList(t)
	require.NoError(t, err)

	expectJSON, err = serializeMFADevices([]*types.MFADevice{ssoDev}, teleport.JSON)
	require.Equal(t, expectJSON+"\n", out)

	// The SSO MFA device cannot be deleted.
	out = mfaRemove(t, connector.GetName())
	require.Empty(t, out)

	// Disabling MFA in the auth connector should remove the device for the user.
	connector.SetMFASettings(&types.OIDCConnectorMFASettings{Enabled: false})
	_, err = auth.UpsertOIDCConnector(ctx, connector)
	require.NoError(t, err)

	out = mfaList(t)
	require.NoError(t, err)
	require.Equal(t, "null\n", out)
}
