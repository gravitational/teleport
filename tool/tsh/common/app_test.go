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
	"net/http/httputil"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
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

// TestAppCommands tests the following basic app command functionality for registered root and leaf apps.
// - tsh app ls
// - tsh app login
// - tsh app config
func TestAppCommands(t *testing.T) {
	ctx := context.Background()

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

	rootServerOpts := []testserver.TestServerOptFunc{
		testserver.WithTestDefaults(t),
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
		testserver.WithTestDefaults(t),
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

	for _, loginCluster := range []string{"root", "leaf"} {
		t.Run(fmt.Sprintf("login %v", loginCluster), func(t *testing.T) {
			// Login to the cluster through the root proxy.
			tmpHomePath := t.TempDir()
			err = Run(context.Background(), []string{
				"login",
				"--insecure",
				"--proxy", rootProxyAddr.String(),
				loginCluster,
			}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuthServer, accessUser, connector.GetName()))
			require.NoError(t, err)

			for _, requireMFAType := range []types.RequireMFAType{
				types.RequireMFAType_OFF,
				types.RequireMFAType_SESSION,
			} {
				t.Run(fmt.Sprintf("require mfa %v", requireMFAType.String()), func(t *testing.T) {
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

					for _, app := range []struct {
						cluster string
						name    string
					}{
						{
							cluster: "root",
							name:    "rootapp",
						}, {
							cluster: "leaf",
							name:    "leafapp",
						},
					} {
						app := app
						t.Run(app.name, func(t *testing.T) {
							// List the apps in the app's cluster to ensure the app is listed.
							lsOut := new(bytes.Buffer)
							err = Run(context.Background(), []string{
								"app",
								"ls",
								"-v",
								"--format", "json",
								"--cluster", app.cluster,
							}, setHomePath(tmpHomePath), setOverrideStdout(lsOut))
							require.NoError(t, err)
							require.Contains(t, lsOut.String(), app.name)

							// Login to the app.
							err = Run(context.Background(), []string{
								"app",
								"login",
								app.name,
								"--cluster", app.cluster,
							}, setHomePath(tmpHomePath), webauthnLoginOpt)
							require.NoError(t, err)

							// Retrieve the app login config (private key, ca, and cert).
							confOut := new(bytes.Buffer)
							err = Run(context.Background(), []string{
								"app",
								"config",
								app.name,
								"--format", "json",
							}, setHomePath(tmpHomePath), setOverrideStdout(confOut), webauthnLoginOpt)
							require.NoError(t, err)

							// Verify that we can connect to the app using the generated app cert.
							var info appConfigInfo
							require.NoError(t, json.Unmarshal(confOut.Bytes(), &info))

							clientCert, err := tls.LoadX509KeyPair(info.Cert, info.Key)
							require.NoError(t, err)
							clt := &http.Client{
								Transport: &http.Transport{
									TLSClientConfig: &tls.Config{
										InsecureSkipVerify: true,
										Certificates:       []tls.Certificate{clientCert},
									},
								},
							}

							resp, err := clt.Get(fmt.Sprintf("https://%v", rootProxyAddr.Addr))
							require.NoError(t, err)

							respData, _ := httputil.DumpResponse(resp, true)
							t.Log(string(respData))

							require.Equal(t, 200, resp.StatusCode)
							require.Equal(t, app.name, resp.Header.Get("Server"))
							_ = resp.Body.Close()
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
			WebProxyAddr: "test-tp.teleport:8443",
		},
	}
	testProfile := &client.ProfileStatus{
		Username: "test-user",
		Dir:      "/test/dir",
	}
	testAppName := "test-tp"
	testAppPublicAddr := "test-tp.teleport"
	testCluster := "test-tp"

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
					WebProxyAddr: "test-tp.teleport:443",
				},
			},
			format:   appFormatURI,
			expected: "https://test-tp.teleport",
		},
		{
			name:     "format URI standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatURI,
			expected: "https://test-tp.teleport:8443",
		},
		{
			name:     "format CA",
			tc:       defaultTc,
			format:   appFormatCA,
			expected: "/test/dir/keys/cas/test-tp.pem",
		},
		{
			name:     "format cert",
			tc:       defaultTc,
			format:   appFormatCert,
			expected: "/test/dir/keys/test-user-app/test-tp-x509.pem",
		},
		{
			name:     "format key",
			tc:       defaultTc,
			format:   appFormatKey,
			expected: "/test/dir/keys/test-user",
		},
		{
			name:   "format curl standard non-standard HTTPS port",
			tc:     defaultTc,
			format: appFormatCURL,
			expected: `curl \
  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
  --key /test/dir/keys/test-user \
  https://test-tp.teleport:8443`,
		},
		{
			name:     "format insecure curl standard non-standard HTTPS port",
			tc:       defaultTc,
			format:   appFormatCURL,
			insecure: true,
			expected: `curl --insecure \
  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
  --key /test/dir/keys/test-user \
  https://test-tp.teleport:8443`,
		},
		{
			name:   "format JSON",
			tc:     defaultTc,
			format: appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443"
}
`,
		},
		{
			name:   "format YAML",
			tc:     defaultTc,
			format: appFormatYAML,
			expected: `ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
		{
			name:   "format default",
			tc:     defaultTc,
			format: "default",
			expected: `Name:      test-tp                                       
URI:       https://test-tp.teleport:8443                 
CA:        /test/dir/keys/cas/test-tp.pem                
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:       /test/dir/keys/test-user                      
`,
		},
		{
			name:   "empty format means default",
			tc:     defaultTc,
			format: "",
			expected: `Name:      test-tp                                       
URI:       https://test-tp.teleport:8443                 
CA:        /test/dir/keys/cas/test-tp.pem                
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:       /test/dir/keys/test-user                      
`,
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
			expected: `Name:      test-tp                                                                                                                                                        
URI:       https://test-tp.teleport:8443                                                                                                                                  
CA:        /test/dir/keys/cas/test-tp.pem                                                                                                                                 
Cert:      /test/dir/keys/test-user-app/test-tp-x509.pem                                                                                                                  
Key:       /test/dir/keys/test-user                                                                                                                                       
Azure Id:  /subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure 
`,
		},
		{
			name:          "azure JSON format",
			tc:            defaultTc,
			azureIdentity: "/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			format:        appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443",
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
ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
		// GCP
		{
			name:              "gcp default format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            "default",
			expected: `Name:                test-tp                                       
URI:                 https://test-tp.teleport:8443                 
CA:                  /test/dir/keys/cas/test-tp.pem                
Cert:                /test/dir/keys/test-user-app/test-tp-x509.pem 
Key:                 /test/dir/keys/test-user                      
GCP Service Account: dev@example-123456.iam.gserviceaccount.com    
`,
		},
		{
			name:              "gcp JSON format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatJSON,
			expected: `{
  "name": "test-tp",
  "uri": "https://test-tp.teleport:8443",
  "ca": "/test/dir/keys/cas/test-tp.pem",
  "cert": "/test/dir/keys/test-user-app/test-tp-x509.pem",
  "key": "/test/dir/keys/test-user",
  "curl": "curl \\\n  --cert /test/dir/keys/test-user-app/test-tp-x509.pem \\\n  --key /test/dir/keys/test-user \\\n  https://test-tp.teleport:8443",
  "gcp_service_account": "dev@example-123456.iam.gserviceaccount.com"
}
`,
		},
		{
			name:              "gcp YAML format",
			tc:                defaultTc,
			gcpServiceAccount: "dev@example-123456.iam.gserviceaccount.com",
			format:            appFormatYAML,
			expected: `ca: /test/dir/keys/cas/test-tp.pem
cert: /test/dir/keys/test-user-app/test-tp-x509.pem
curl: |-
  curl \
    --cert /test/dir/keys/test-user-app/test-tp-x509.pem \
    --key /test/dir/keys/test-user \
    https://test-tp.teleport:8443
gcp_service_account: dev@example-123456.iam.gserviceaccount.com
key: /test/dir/keys/test-user
name: test-tp
uri: https://test-tp.teleport:8443
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.tc.InsecureSkipVerify = test.insecure
			result, err := formatAppConfig(test.tc, testProfile, testAppName, testAppPublicAddr, test.format, testCluster, test.awsArn, test.azureIdentity, test.gcpServiceAccount)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}
