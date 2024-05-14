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
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/client"
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

func testDummyAppConn(t require.TestingT, name string, addr string, tlsCerts ...tls.Certificate) {
	clt := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				Certificates:       tlsCerts,
			},
		},
	}

	resp, err := clt.Get(addr)
	assert.NoError(t, err)
	if err != nil {
		return
	}
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, name, resp.Header.Get("Server"))
	_ = resp.Body.Close()
}

// TestAppCommands tests the following basic app command functionality for registered root and leaf apps.
// - tsh app ls
// - tsh app login
// - tsh app config
// - tsh proxy app
func TestAppCommands(t *testing.T) {
	ctx := context.Background()

	testserver.WithResyncInterval(t, 0)

	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() {
		lib.SetInsecureDevMode(isInsecure)
	})

	localProxyPort := ports.Pop()

	accessUser, err := types.NewUser("access")
	require.NoError(t, err)
	accessUser.SetRoles([]string{"access"})

	user, err := user.Current()
	require.NoError(t, err)
	accessUser.SetLogins([]string{user.Name})
	connector := mockConnector(t)

	rootServerOpts := []testserver.TestServerOptFunc{
		testserver.WithBootstrap(connector, accessUser),
		testserver.WithClusterName(t, "root"),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Apps = servicecfg.AppsConfig{
				Enabled: true,
				Apps: []servicecfg.App{{
					Name: "rootapp",
					URI:  startDummyHTTPServer(t, "rootapp"),
				}},
			}
		}),
	}
	rootServer := testserver.MakeTestServer(t, rootServerOpts...)
	rootAuthServer := rootServer.GetAuthServer()
	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	leafServerOpts := []testserver.TestServerOptFunc{
		testserver.WithBootstrap(accessUser),
		testserver.WithClusterName(t, "leaf"),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Apps = servicecfg.AppsConfig{
				Enabled: true,
				Apps: []servicecfg.App{{
					Name: "leafapp",
					URI:  startDummyHTTPServer(t, "leafapp"),
				}},
			}
		}),
	}
	leafServer := testserver.MakeTestServer(t, leafServerOpts...)
	testserver.SetupTrustedCluster(ctx, t, rootServer, leafServer)

	// Set up user with MFA device for per session MFA tests below.
	origin := "https://127.0.0.1"
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()
	webauthnLoginOpt := setupWebAuthnChallengeSolver(device, true /* success */)

	_, err = rootAuthServer.UpsertAuthPreference(ctx, &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "127.0.0.1",
			},
		},
	})
	require.NoError(t, err)
	registerDeviceForUser(t, rootAuthServer, device, accessUser.GetName(), origin)

	// Used to login to a cluster through the root proxy.
	loginToCluster := func(t *testing.T, cluster string) string {
		loginPath := t.TempDir()
		err = Run(ctx, []string{
			"login",
			"--insecure",
			"--proxy", rootProxyAddr.String(),
			cluster,
		}, setHomePath(loginPath), setMockSSOLogin(rootAuthServer, accessUser, connector.GetName()))
		require.NoError(t, err)
		return loginPath
	}

	// Used to change per-session MFA requirement for test cases.
	setRequireMFA := func(t *testing.T, requireMFAType types.RequireMFAType) {
		_, err = rootAuthServer.UpsertAuthPreference(ctx, &types.AuthPreferenceV2{
			Spec: types.AuthPreferenceSpecV2{
				SecondFactor: constants.SecondFactorOptional,
				Webauthn: &types.Webauthn{
					RPID: "127.0.0.1",
				},
				RequireMFAType: requireMFAType,
			},
		})
		require.NoError(t, err)
		_, err = leafServer.GetAuthServer().UpsertAuthPreference(ctx, &types.AuthPreferenceV2{
			Spec: types.AuthPreferenceSpecV2{
				RequireMFAType: requireMFAType,
			},
		})
		require.NoError(t, err)
	}

	appTestCases := []struct {
		name    string
		cluster string
	}{
		{
			name:    "rootapp",
			cluster: "root",
		}, {
			name:    "leafapp",
			cluster: "leaf",
		},
	}

	for _, loginCluster := range []string{"root", "leaf"} {
		t.Run(fmt.Sprintf("login %v", loginCluster), func(t *testing.T) {
			loginPath := loginToCluster(t, loginCluster)

			// Run each test case twice to test with and without MFA.
			for _, requireMFAType := range []types.RequireMFAType{
				types.RequireMFAType_OFF,
				types.RequireMFAType_SESSION,
			} {
				t.Run(fmt.Sprintf("require mfa %v", requireMFAType.String()), func(t *testing.T) {
					setRequireMFA(t, requireMFAType)

					for _, app := range appTestCases {
						t.Run(fmt.Sprintf("login %v, app %v", loginCluster, app.name), func(t *testing.T) {
							// List the apps in the app's cluster to ensure the app is listed.
							t.Run("tsh app ls", func(t *testing.T) {
								lsOut := new(bytes.Buffer)
								err = Run(ctx, []string{
									"app",
									"ls",
									"-v",
									"--format", "json",
									"--cluster", app.cluster,
								}, setHomePath(loginPath), setOverrideStdout(lsOut))
								require.NoError(t, err)
								require.Contains(t, lsOut.String(), app.name)
							})

							// Test logging into the app and connecting.
							t.Run("tsh app login", func(t *testing.T) {
								err = Run(ctx, []string{
									"app",
									"login",
									app.name,
									"--cluster", app.cluster,
								}, setHomePath(loginPath), webauthnLoginOpt)
								require.NoError(t, err)

								// Retrieve the app login config (private key, ca, and cert).
								confOut := new(bytes.Buffer)
								err = Run(ctx, []string{
									"app",
									"config",
									app.name,
									"--cluster", app.cluster,
									"--format", "json",
								}, setHomePath(loginPath), setOverrideStdout(confOut))
								require.NoError(t, err)

								// Verify that we can connect to the app using the generated app cert.
								var info appConfigInfo
								require.NoError(t, json.Unmarshal(confOut.Bytes(), &info))

								clientCert, err := tls.LoadX509KeyPair(info.Cert, info.Key)
								require.NoError(t, err)

								testDummyAppConn(t, app.name, fmt.Sprintf("https://%v", rootProxyAddr.Addr), clientCert)

								// app logout.
								err = Run(ctx, []string{
									"app",
									"logout",
									"--cluster", app.cluster,
								}, setHomePath(loginPath))
								require.NoError(t, err)
							})

							// Test connecting to the app through a local proxy.
							t.Run("tsh proxy app", func(t *testing.T) {
								proxyCtx, proxyCancel := context.WithTimeout(ctx, time.Second)
								defer proxyCancel()

								go func() {
									Run(proxyCtx, []string{
										"--insecure",
										"proxy",
										"app",
										app.name,
										"--port", localProxyPort,
										"--cluster", app.cluster,
									}, setHomePath(loginPath), webauthnLoginOpt)
								}()

								require.EventuallyWithT(t, func(t *assert.CollectT) {
									testDummyAppConn(t, app.name, fmt.Sprintf("http://127.0.0.1:%v", localProxyPort))
								}, time.Second, 100*time.Millisecond)

								// proxy certs should not be saved to disk.
								err = Run(context.Background(), []string{
									"app",
									"config",
									app.name,
									"--cluster", app.cluster,
								}, setHomePath(loginPath))
								require.Error(t, err)
								require.True(t, trace.IsNotFound(err))
							})
						})
					}
				})
			}
		})
	}
}

func TestFormatAppConfig(t *testing.T) {
	t.Parallel()

	defaultTc := &client.TeleportClient{
		Config: client.Config{
			SiteName:     "root",
			WebProxyAddr: "root.example.com:8443",
		},
	}
	testProfile := &client.ProfileStatus{
		Username: "alice",
		Dir:      "/test/dir",
	}

	testAppName := "test-app"
	testAppPublicAddr := "test-app.example.com"

	asciiRows := [][]string{
		{"Name:     ", testAppName},
		{"URI:", "https://test-app.example.com:8443"},
		{"CA:", "/test/dir/keys/cas/root.pem"},
		{"Cert:", "/test/dir/keys/alice-app/root/test-app-x509.pem"},
		{"Key:", "/test/dir/keys/alice"},
	}

	defaultFormatTable := asciitable.MakeTable(make([]string, 2), asciiRows...)
	defaultFormatTableAzure := asciitable.MakeTable(make([]string, 2), append(asciiRows, []string{"Azure Id:", "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"})...)
	defaultFormatTableGCP := asciitable.MakeTable(make([]string, 2), append(asciiRows, []string{"GCP Service Account:", "dev@example-123456.iam.gserviceaccount.com"})...)

	tests := []struct {
		name              string
		tc                *client.TeleportClient
		format            string
		awsArn            string
		azureIdentity     string
		gcpServiceAccount string
		insecure          bool
		expected          string
		wantErr           bool
	}{
		{
			name: "format URI standard HTTPS port",
			tc: &client.TeleportClient{
				Config: client.Config{
					SiteName:     "root",
					WebProxyAddr: "https://root.example.com:443",
				},
			},
			format:   appFormatURI,
			expected: "https://test-app.example.com",
		},
		{
			name:     "format URI standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatURI,
			expected: "https://test-app.example.com:8443",
		},
		{
			name:     "format CA",
			tc:       defaultTc,
			format:   appFormatCA,
			expected: "/test/dir/keys/cas/root.pem",
		},
		{
			name:     "format cert",
			tc:       defaultTc,
			format:   appFormatCert,
			expected: "/test/dir/keys/alice-app/root/test-app-x509.pem",
		},
		{
			name:     "format key",
			tc:       defaultTc,
			format:   appFormatKey,
			expected: "/test/dir/keys/alice",
		},
		{
			name:   "format curl standard non-standard HTTPS port",
			tc:     defaultTc,
			format: appFormatCURL,
			expected: `curl \
  --cert /test/dir/keys/alice-app/root/test-app-x509.pem \
  --key /test/dir/keys/alice \
  https://test-app.example.com:8443`,
		},
		{
			name:     "format insecure curl standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatCURL,
			insecure: true,
			expected: `curl --insecure \
  --cert /test/dir/keys/alice-app/root/test-app-x509.pem \
  --key /test/dir/keys/alice \
  https://test-app.example.com:8443`,
		},
		{
			name:   "format JSON",
			tc:     defaultTc,
			format: appFormatJSON,
			expected: `{
  "name": "test-app",
  "uri": "https://test-app.example.com:8443",
  "ca": "/test/dir/keys/cas/root.pem",
  "cert": "/test/dir/keys/alice-app/root/test-app-x509.pem",
  "key": "/test/dir/keys/alice",
  "curl": "curl \\\n  --cert /test/dir/keys/alice-app/root/test-app-x509.pem \\\n  --key /test/dir/keys/alice \\\n  https://test-app.example.com:8443"
}
`,
		},
		{
			name:   "format YAML",
			tc:     defaultTc,
			format: appFormatYAML,
			expected: `ca: /test/dir/keys/cas/root.pem
cert: /test/dir/keys/alice-app/root/test-app-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/alice-app/root/test-app-x509.pem \
    --key /test/dir/keys/alice \
    https://test-app.example.com:8443
key: /test/dir/keys/alice
name: test-app
uri: https://test-app.example.com:8443
`,
		},
		{
			name:     "format default",
			tc:       defaultTc,
			format:   "default",
			expected: defaultFormatTable.AsBuffer().String(),
		},
		{
			name:     "empty format means default",
			tc:       defaultTc,
			format:   "",
			expected: defaultFormatTable.AsBuffer().String(),
		},
		{
			name:    "reject invalid format",
			tc:      defaultTc,
			format:  "invalid",
			wantErr: true,
		},
		// Azure
		{
			name:          "azure default format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        "default",
			expected:      defaultFormatTableAzure.AsBuffer().String(),
		},
		{
			name:          "azure JSON format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        appFormatJSON,
			expected: `{
  "name": "test-app",
  "uri": "https://test-app.example.com:8443",
  "ca": "/test/dir/keys/cas/root.pem",
  "cert": "/test/dir/keys/alice-app/root/test-app-x509.pem",
  "key": "/test/dir/keys/alice",
  "curl": "curl \\\n  --cert /test/dir/keys/alice-app/root/test-app-x509.pem \\\n  --key /test/dir/keys/alice \\\n  https://test-app.example.com:8443",
  "azure_identity": "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"
}
`,
		},
		{
			name:          "azure YAML format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        appFormatYAML,
			expected: `azure_identity: /subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure
ca: /test/dir/keys/cas/root.pem
cert: /test/dir/keys/alice-app/root/test-app-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/alice-app/root/test-app-x509.pem \
    --key /test/dir/keys/alice \
    https://test-app.example.com:8443
key: /test/dir/keys/alice
name: test-app
uri: https://test-app.example.com:8443
`,
		},
		// GCP
		{
			name:              "gcp default format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            "default",
			expected:          defaultFormatTableGCP.AsBuffer().String(),
		},
		{
			name:              "gcp JSON format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatJSON,
			expected: `{
  "name": "test-app",
  "uri": "https://test-app.example.com:8443",
  "ca": "/test/dir/keys/cas/root.pem",
  "cert": "/test/dir/keys/alice-app/root/test-app-x509.pem",
  "key": "/test/dir/keys/alice",
  "curl": "curl \\\n  --cert /test/dir/keys/alice-app/root/test-app-x509.pem \\\n  --key /test/dir/keys/alice \\\n  https://test-app.example.com:8443",
  "gcp_service_account": "dev@example-123456.iam.gserviceaccount.com"
}
`,
		},
		{
			name:              "gcp YAML format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatYAML,
			expected: `ca: /test/dir/keys/cas/root.pem
cert: /test/dir/keys/alice-app/root/test-app-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/alice-app/root/test-app-x509.pem \
    --key /test/dir/keys/alice \
    https://test-app.example.com:8443
gcp_service_account: dev@example-123456.iam.gserviceaccount.com
key: /test/dir/keys/alice
name: test-app
uri: https://test-app.example.com:8443
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.tc.InsecureSkipVerify = test.insecure
			routeToApp := proto.RouteToApp{
				Name:              testAppName,
				PublicAddr:        testAppPublicAddr,
				ClusterName:       "root",
				AWSRoleARN:        test.awsArn,
				AzureIdentity:     test.azureIdentity,
				GCPServiceAccount: test.gcpServiceAccount,
			}
			result, err := formatAppConfig(test.tc, testProfile, routeToApp, test.format)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}
