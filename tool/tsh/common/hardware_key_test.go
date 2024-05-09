//go:build pivtest

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
	"context"
	"fmt"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestHardwareKeyLogin tests Hardware Key login and relogin flows.
func TestHardwareKeyLogin(t *testing.T) {
	ctx := context.Background()

	testModules := &modules.TestModules{TestBuildType: modules.BuildEnterprise}
	modules.SetTestModules(t, testModules)

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	aliceRole, err := types.NewRole("alice", types.RoleSpecV6{})
	require.NoError(t, err)
	alice.SetRoles([]string{aliceRole.GetName()})

	testServer := testserver.MakeTestServer(t, testserver.WithBootstrap(connector, alice, aliceRole), func(o *testserver.TestServersOpts) {
		o.ConfigFuncs = append(o.ConfigFuncs, func(cfg *servicecfg.Config) {
			// TODO (Joerger): This test fails to propagate hardware key policy errors from Proxy SSH connections
			// for unknown reasons unless Multiplex mode is on. I could not reproduce these errors on a live
			// cluster, so the issue likely lies with the test server setup. Perhaps the test certs generated
			// are not 1-to-1 with live certs.
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		})
	})
	authServer := testServer.GetAuthServer()

	proxyAddr, err := testServer.ProxyWebAddr()
	require.NoError(t, err)

	// mock SSO login and count the number of login attempts.
	var lastLoginCount int
	mockSSOLogin := mockSSOLogin(authServer, alice)
	mockSSOLoginWithCountAndAttestation := func(ctx context.Context, connectorID string, priv *keys.PrivateKey, protocol string) (*auth.SSHLoginResponse, error) {
		lastLoginCount++

		// Set MockAttestationData to attest the expected key policy and reset it after login.
		testModules.MockAttestationData = &keys.AttestationData{
			PrivateKeyPolicy: priv.GetPrivateKeyPolicy(),
		}
		defer func() {
			testModules.MockAttestationData = nil
		}()

		return mockSSOLogin(ctx, connectorID, priv, protocol)
	}
	setMockSSOLogin := setMockSSOLoginCustom(mockSSOLoginWithCountAndAttestation, connector.GetName())

	t.Run("cap", func(t *testing.T) {
		setRequireMFAType := func(t *testing.T, requireMFAType types.RequireMFAType) {
			// Set require MFA type in the cluster auth preference.
			_, err := authServer.UpsertAuthPreference(ctx, &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					RequireMFAType: requireMFAType,
				},
			})
			require.NoError(t, err)
		}

		t.Cleanup(func() {
			setRequireMFAType(t, types.RequireMFAType_OFF)
		})

		// login should use the private key policy reported by the proxy without
		// needing to retry hardware key login.
		setRequireMFAType(t, types.RequireMFAType_HARDWARE_KEY_TOUCH)
		tmpHomePath := t.TempDir()
		err = Run(context.Background(), []string{
			"login",
			"--debug",
			"--insecure",
			"--proxy", proxyAddr.String(),
		}, setHomePath(tmpHomePath), setMockSSOLogin)
		require.NoError(t, err)
		assert.Equal(t, 1, lastLoginCount, "expected one login attempt but got %v", lastLoginCount)
		lastLoginCount = 0 // reset login count

		// Upgrading the auth preference requireMFAType should trigger relogin
		// on the next command run.
		setRequireMFAType(t, types.RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN)
		err = Run(context.Background(), []string{
			"ls",
			"--debug",
			"--insecure",
			"--proxy", proxyAddr.String(),
		}, setHomePath(tmpHomePath), setMockSSOLogin)
		require.NoError(t, err)
		assert.Equal(t, 1, lastLoginCount, "expected one login attempt but got %v", lastLoginCount)
		lastLoginCount = 0 // reset login count
	})

	t.Run("role", func(t *testing.T) {
		setRequireMFAType := func(t *testing.T, requireMFAType types.RequireMFAType) {
			// Set require MFA type in the user's role.
			aliceRole.SetOptions(types.RoleOptions{
				RequireMFAType: requireMFAType,
			})
			_, err = authServer.UpsertRole(ctx, aliceRole)
			require.NoError(t, err)
		}

		t.Cleanup(func() {
			setRequireMFAType(t, types.RequireMFAType_OFF)
		})

		// login should initially fail using the private key policy reported by the proxy (off),
		// then trigger a retry with the hardware key policy parsed from the error.
		setRequireMFAType(t, types.RequireMFAType_HARDWARE_KEY_TOUCH)
		tmpHomePath := t.TempDir()
		err = Run(context.Background(), []string{
			"login",
			"--debug",
			"--insecure",
			"--proxy", proxyAddr.String(),
		}, setHomePath(tmpHomePath), setMockSSOLogin)
		require.NoError(t, err)
		assert.Equal(t, 2, lastLoginCount, "expected two login attempts but got %v", lastLoginCount)
		lastLoginCount = 0 // reset login count

		// Upgrading the auth preference requireMFAType should trigger relogin
		// on the next command run.
		setRequireMFAType(t, types.RequireMFAType_HARDWARE_KEY_TOUCH_AND_PIN)
		err = Run(context.Background(), []string{
			"ls",
			"--debug",
			"--insecure",
			"--proxy", proxyAddr.String(),
		}, setHomePath(tmpHomePath), setMockSSOLogin)
		require.NoError(t, err)
		assert.Equal(t, 1, lastLoginCount, "expected one login attempt but got %v", lastLoginCount)
		lastLoginCount = 0 // reset login count
	})
}

// TestHardwareKeySSH tests Hardware Key SSH flows.
func TestHardwareKeySSH(t *testing.T) {
	ctx := context.Background()

	testModules := &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	}
	modules.SetTestModules(t, testModules)

	connector := mockConnector(t)

	user, err := user.Current()
	require.NoError(t, err)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	aliceRole, err := types.NewRole("alice", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{user.Name},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	require.NoError(t, err)
	alice.SetRoles([]string{aliceRole.GetName()})

	testServer := testserver.MakeTestServer(t,
		testserver.WithBootstrap(connector, alice, aliceRole),
		testserver.WithClusterName(t, "my-cluster"),
		func(o *testserver.TestServersOpts) {
			o.ConfigFuncs = append(o.ConfigFuncs, func(cfg *servicecfg.Config) {
				// TODO (Joerger): This test fails to propagate hardware key policy errors from Proxy SSH connections
				// for unknown reasons unless Multiplex mode is on. I could not reproduce these errors on a live
				// cluster, so the issue likely lies with the test server setup. Perhaps the test certs generated
				// are not 1-to-1 with live certs.
				cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			})
		})
	authServer := testServer.GetAuthServer()
	proxyAddr, err := testServer.ProxyWebAddr()
	require.NoError(t, err)

	agentless := testserver.CreateAgentlessNode(t, authServer, "my-cluster", "agentless-node")

	// Login before adding hardware key requirement
	tmpHomePath := t.TempDir()
	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	// Require hardware key touch for alice.
	aliceRole.SetOptions(types.RoleOptions{
		RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
	})
	_, err = authServer.UpsertRole(ctx, aliceRole)
	require.NoError(t, err)

	tests := []struct {
		name       string
		targetHost string
	}{
		{
			name:       "regular node",
			targetHost: testServer.Config.Hostname,
		},
		{
			name:       "agentless node",
			targetHost: agentless.GetHostname(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testModules.MockAttestationData = nil
			tmpHomePath = t.TempDir()
			// SSH fails without an attested hardware key login.
			err = Run(ctx, []string{
				"ssh",
				"--insecure",
				fmt.Sprintf("%s@%s", user.Name, tc.targetHost),
				"echo", "test",
			}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
			require.Error(t, err)

			// Set MockAttestationData to attest the expected key policy and try again.
			testModules.MockAttestationData = &keys.AttestationData{
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
			}

			err = Run(ctx, []string{
				"login",
				"--insecure",
				"--proxy", proxyAddr.String(),
			}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
			require.NoError(t, err)

			err = Run(ctx, []string{
				"ssh",
				"--insecure",
				fmt.Sprintf("%s@%s", user.Name, tc.targetHost),
				"echo", "test",
			}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
			require.NoError(t, err)
		})
	}
}
