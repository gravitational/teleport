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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func startDummyHTTPServer(t *testing.T, name string) string {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", name)
		_, _ = w.Write([]byte("hello"))
	}))

	srv.Start()

	t.Cleanup(func() {
		srv.Close()
	})

	return srv.URL
}

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
	mockSSOLoginWithCountAndAttestation := func(ctx context.Context, connectorID string, priv *keys.PrivateKey, protocol string) (*authclient.SSHLoginResponse, error) {
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

// TestHardwareKeyApp tests Hardware Key App flows.
func TestHardwareKeyApp(t *testing.T) {
	ctx := context.Background()

	testModules := &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.App: {Enabled: true},
			},
		},
	}
	modules.SetTestModules(t, testModules)

	testserver.WithResyncInterval(t, 0)

	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() {
		lib.SetInsecureDevMode(isInsecure)
	})

	accessUser, err := types.NewUser("access")
	require.NoError(t, err)
	accessUser.SetRoles([]string{"access"})

	user, err := user.Current()
	require.NoError(t, err)
	accessUser.SetLogins([]string{user.Name})
	connector := mockConnector(t)

	testServer := testserver.MakeTestServer(t,
		testserver.WithBootstrap(connector, accessUser),
		testserver.WithClusterName(t, "root"),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Apps = servicecfg.AppsConfig{
				Enabled: true,
				Apps: []servicecfg.App{{
					Name: "myapp",
					URI:  startDummyHTTPServer(t, "myapp"),
				}},
			}
		}),
	)
	authServer := testServer.GetAuthServer()
	proxyAddr, err := testServer.ProxyWebAddr()
	require.NoError(t, err)

	// Set up user with MFA device since app login requires MFA when
	// hardware key touch/pin is enabled.
	origin := "https://127.0.0.1"
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()

	_, err = authServer.UpsertAuthPreference(ctx, &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "127.0.0.1",
			},
		},
	})
	require.NoError(t, err)
	registerDeviceForUser(t, authServer, device, accessUser.GetName(), origin)

	// Login before adding hardware key requirement and verify we can connect to the app.
	tmpHomePath := t.TempDir()
	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, accessUser, connector.GetName()))
	require.NoError(t, err)

	err = Run(ctx, []string{
		"app",
		"login",
		"myapp",
		"--insecure",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	confOut := new(bytes.Buffer)
	err = Run(ctx, []string{
		"app",
		"config",
		"myapp",
		"--format", "json",
	}, setHomePath(tmpHomePath), setOverrideStdout(confOut))
	require.NoError(t, err)

	var info appConfigInfo
	require.NoError(t, json.Unmarshal(confOut.Bytes(), &info))

	clientCert, err := tls.LoadX509KeyPair(info.Cert, info.Key)
	require.NoError(t, err)

	resp := testDummyAppConn(t, fmt.Sprintf("https://%v", proxyAddr.Addr), clientCert)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "myapp", resp.Header.Get("Server"))
	resp.Body.Close()

	// Require hardware key touch for the user. The user's current app certs should fail.
	accessRole, err := authServer.GetRole(ctx, "access")
	require.NoError(t, err)
	accessRole.SetOptions(types.RoleOptions{
		RequireMFAType: types.RequireMFAType_HARDWARE_KEY_TOUCH,
	})
	_, err = authServer.UpsertRole(ctx, accessRole)
	require.NoError(t, err)

	testModules.MockAttestationData = nil

	resp = testDummyAppConn(t, fmt.Sprintf("https://%v", proxyAddr.Addr), clientCert)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()

	// App login fails without an attested hardware key login.
	err = Run(ctx, []string{
		"app",
		"login",
		"myapp",
		"--insecure",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, accessUser, connector.GetName()))
	require.Error(t, err)

	// Set MockAttestationData to attest the expected key policy and try again.
	testModules.MockAttestationData = &keys.AttestationData{
		PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
	}

	// App Login will still fail without MFA, since the app sessions will
	// only be attested as "web_session".
	webauthnLoginOpt := setupWebAuthnChallengeSolver(device, false /* success */)

	err = Run(ctx, []string{
		"app",
		"login",
		"myapp",
		"--insecure",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, accessUser, connector.GetName()), webauthnLoginOpt)
	require.Error(t, err)

	// App commands will succeed with MFA.
	webauthnLoginOpt = setupWebAuthnChallengeSolver(device, true /* success */)

	// Test App login success and connect.
	err = Run(ctx, []string{
		"app",
		"login",
		"myapp",
		"--insecure",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, accessUser, connector.GetName()), webauthnLoginOpt)
	require.NoError(t, err)

	confOut = new(bytes.Buffer)
	err = Run(ctx, []string{
		"app",
		"config",
		"myapp",
		"--format", "json",
	}, setHomePath(tmpHomePath), setOverrideStdout(confOut))
	require.NoError(t, err)

	require.NoError(t, json.Unmarshal(confOut.Bytes(), &info))

	clientCert, err = tls.LoadX509KeyPair(info.Cert, info.Key)
	require.NoError(t, err)

	resp = testDummyAppConn(t, fmt.Sprintf("https://%v", proxyAddr.Addr), clientCert)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "myapp", resp.Header.Get("Server"))
	resp.Body.Close()
}
