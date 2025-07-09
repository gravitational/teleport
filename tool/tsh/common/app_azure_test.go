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
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestAzure(t *testing.T) {
	lib.SetInsecureDevMode(true)
	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	user, azureRole := makeUserWithAzureRole(t)

	authProcess := testserver.MakeTestServer(
		t,
		testserver.WithClusterName(t, "localhost"),
		testserver.WithBootstrap(connector, user, azureRole),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name:  "azure-api",
					Cloud: types.CloudAzure,
				},
			}
		}),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	// helper function
	run := func(args []string, opts ...CliOption) {
		opts = append(opts, setHomePath(tmpHomePath))
		opts = append(opts, setMockSSOLogin(authServer, user, connector.GetName()))
		err := Run(context.Background(), args, opts...)
		require.NoError(t, err)
	}

	// set MSI_ENDPOINT along with secret
	t.Setenv("MSI_ENDPOINT", "https://azure-msi.teleport.dev/very-secret")

	// Log into Teleport cluster.
	run([]string{"login", "--insecure", "--debug", "--proxy", proxyAddr.String()})

	// Log into the "azure-api" app.
	// Verify `tsh az login ...` gets called.
	run([]string{"app", "login", "--insecure", "--azure-identity", "dummy_azure_identity", "azure-api"},
		setCmdRunner(func(cmd *exec.Cmd) error {
			require.Equal(t, []string{"az", "login", "--identity", "-u", "dummy_azure_identity"}, cmd.Args[1:])
			return nil
		}))

	// Log into the "azure-api" app -- now with --debug flag.
	run([]string{"app", "login", "--insecure", "azure-api", "--debug"},
		setCmdRunner(func(cmd *exec.Cmd) error {
			require.Equal(t, []string{"--debug", "az", "login", "--identity", "-u", "dummy_azure_identity"}, cmd.Args[1:])
			return nil
		}))

	// basic requests to verify we can dial proxy as expected
	// more comprehensive tests cover AzureMSIMiddleware directly
	requests := []struct {
		name         string
		url          string
		headers      map[string]string
		expectedCode int
		expectedBody []byte
		verifyBody   func(t *testing.T, body []byte)
	}{
		{
			name:         "incomplete request",
			url:          "https://azure-msi.teleport.dev/very-secret",
			headers:      nil,
			expectedCode: 400,
			expectedBody: []byte("{\n    \"error\": {\n        \"message\": \"expected Metadata header with value 'true'\"\n    }\n}"),
		},
		{
			name:         "well-formatted request",
			url:          "https://azure-msi.teleport.dev/very-secret?resource=myresource&msi_res_id=dummy_azure_identity",
			headers:      map[string]string{"Metadata": "true"},
			expectedCode: 200,
			verifyBody: func(t *testing.T, body []byte) {
				var req struct {
					AccessToken  string `json:"access_token"`
					ClientID     string `json:"client_id"`
					Resource     string `json:"resource"`
					TokenType    string `json:"token_type"`
					ExpiresIn    int    `json:"expires_in"`
					ExpiresOn    int    `json:"expires_on"`
					ExtExpiresIn int    `json:"ext_expires_in"`
					NotBefore    int    `json:"not_before"`
				}

				require.NoError(t, json.Unmarshal(body, &req))

				require.NotEmpty(t, req.AccessToken)
				require.NotEmpty(t, req.ClientID)
				require.Equal(t, "myresource", req.Resource)
				require.NotZero(t, req.ExpiresIn)
				require.NotZero(t, req.ExpiresOn)
				require.NotZero(t, req.ExtExpiresIn)
				require.NotZero(t, req.NotBefore)
			},
		},
	}

	// Run `tsh az vm ls`. Verify executed command and environment.
	run([]string{"az", "vm", "ls", "-g", "my-group"},
		setCmdRunner(func(cmd *exec.Cmd) error {
			require.Equal(t, []string{"az", "vm", "ls", "-g", "my-group"}, cmd.Args)

			getEnvValue := func(key string) string {
				for _, env := range cmd.Env {
					if strings.HasPrefix(env, key+"=") {
						return strings.TrimPrefix(env, key+"=")
					}
				}
				return ""
			}

			require.Equal(t, filepath.Join(tmpHomePath, "azure/localhost/azure-api"), getEnvValue("AZURE_CONFIG_DIR"))
			require.Equal(t, "https://azure-msi.teleport.dev/very-secret", getEnvValue("MSI_ENDPOINT"))
			require.Equal(t, filepath.Join(tmpHomePath, "keys/127.0.0.1/alice@example.com-app/localhost/azure-api-localca.pem"), getEnvValue("REQUESTS_CA_BUNDLE"))
			require.True(t, strings.HasPrefix(getEnvValue("HTTPS_PROXY"), "http://127.0.0.1:"))

			// Validate MSI endpoint can be reached
			caPool, err := utils.NewCertPoolFromPath(getEnvValue("REQUESTS_CA_BUNDLE"))
			require.NoError(t, err)

			httpsProxy, err := url.Parse(getEnvValue("HTTPS_PROXY"))
			require.NoError(t, err)

			client := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(httpsProxy),
					TLSClientConfig: &tls.Config{
						RootCAs: caPool,
					},
				},
			}

			for _, tc := range requests {
				t.Run(tc.name, func(t *testing.T) {
					req, err := http.NewRequest("GET", tc.url, nil)
					require.NoError(t, err)

					for k, v := range tc.headers {
						req.Header.Set(k, v)
					}

					resp, err := client.Do(req)
					require.NoError(t, err)

					require.Equal(t, tc.expectedCode, resp.StatusCode)
					body, err := io.ReadAll(resp.Body)
					require.NoError(t, err)
					require.NoError(t, resp.Body.Close())

					if tc.verifyBody != nil {
						tc.verifyBody(t, body)
					} else {
						require.Equal(t, tc.expectedBody, body)
					}
				})
			}

			return nil
		}))
}

func makeUserWithAzureRole(t *testing.T) (types.User, types.Role) {
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)

	role := services.NewPresetAccessRole()

	alice.SetRoles([]string{role.GetName()})
	alice.SetAzureIdentities([]string{
		"dummy_azure_identity",
		"other_dummy_azure_identity",
	})

	return alice, role
}

func Test_getAzureIdentityFromFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		requestedIdentity string
		profileIdentities []string
		want              string
		wantErr           require.ErrorAssertionFunc
	}{
		{
			name:              "no flag, use default identity",
			requestedIdentity: "",
			profileIdentities: []string{"default"},
			want:              "default",
			wantErr:           require.NoError,
		},
		{
			name:              "no flag, multiple possible identities",
			requestedIdentity: "",
			profileIdentities: []string{"id1", "id2"},
			want:              "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "multiple Azure identities available, choose one with --azure-identity flag")
			},
		},
		{
			name:              "no flag, no identities",
			requestedIdentity: "",
			profileIdentities: []string{},
			want:              "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "no Azure identities available, check your permissions")
			},
		},

		{
			name:              "exact match, one option",
			requestedIdentity: "id1",
			profileIdentities: []string{"id1"},
			want:              "id1",
			wantErr:           require.NoError,
		},
		{
			name:              "exact match, multiple options",
			requestedIdentity: "id1",
			profileIdentities: []string{"id1", "id2"},
			want:              "id1",
			wantErr:           require.NoError,
		},
		{
			name:              "no match, multiple options",
			requestedIdentity: "id3",
			profileIdentities: []string{"id1", "id2"},
			want:              "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "failed to find the identity matching \"id3\"")
			},
		},

		{
			name:              "different case, exact match, one option",
			requestedIdentity: "ID1",
			profileIdentities: []string{"id1"},
			want:              "id1",
			wantErr:           require.NoError,
		},

		{
			name:              "different case, exact match, one option, full identity",
			requestedIdentity: "/Subscriptions/0000000/ResourceGroups/MyGroup/Providers/MICROSOFT.ManagedIdentity/UserAssignedIdentities/ID1",
			profileIdentities: []string{"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1"},
			want:              "/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
			wantErr:           require.NoError,
		},
		{
			name:              "different case, exact match, multiple options",
			requestedIdentity: "ID1",
			profileIdentities: []string{"id1", "id2"},
			want:              "id1",
			wantErr:           require.NoError,
		},
		{
			name:              "different case, no match, multiple options",
			requestedIdentity: "ID3",
			profileIdentities: []string{"id1", "id2"},
			want:              "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "failed to find the identity matching \"ID3\"")
			},
		},

		{
			name:              "suffix match, one option",
			requestedIdentity: "id1",
			profileIdentities: []string{"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1"},
			want:              "/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
			wantErr:           require.NoError,
		},
		{
			name:              "suffix match, multiple options",
			requestedIdentity: "id1",
			profileIdentities: []string{
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id2",
			},
			want:    "/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
			wantErr: require.NoError,
		},
		{
			name:              "ambiguous suffix match",
			requestedIdentity: "id1",
			profileIdentities: []string{
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
				"/subscriptions/1111111/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
			},
			want: "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "provided identity \"id1\" is ambiguous, please specify full identity name")
			},
		},

		{
			name:              "different case, suffix match, one option",
			requestedIdentity: "ID1",
			profileIdentities: []string{"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1"},
			want:              "/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
			wantErr:           require.NoError,
		},
		{
			name:              "different case, suffix match, multiple options",
			requestedIdentity: "ID1",
			profileIdentities: []string{
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id2",
			},
			want:    "/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
			wantErr: require.NoError,
		},
		{
			name:              "different case, ambiguous suffix match",
			requestedIdentity: "ID1",
			profileIdentities: []string{
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
				"/subscriptions/1111111/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
			},
			want: "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "provided identity \"ID1\" is ambiguous, please specify full identity name")
			},
		},

		{
			name:              "no match, multiple options",
			requestedIdentity: "id3",
			profileIdentities: []string{
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id1",
				"/subscriptions/0000000/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/id2",
				"/subscriptions/1111111/resourcegroups/mygroup/providers/microsoft.managedidentity/userassignedidentities/idX",
			},
			want: "",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "failed to find the identity matching \"id3\"")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getAzureIdentityFromFlags(&CLIConf{AzureIdentity: tt.requestedIdentity}, &client.ProfileStatus{AzureIdentities: tt.profileIdentities})
			require.Equal(t, tt.want, result)
			tt.wantErr(t, err)
		})
	}
}

func Test_getMSISecret(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		want     string
		wantFunc func(t require.TestingT, result string)
		wantErr  require.ErrorAssertionFunc
	}{
		{
			name: "no env",
			env:  "",
			wantFunc: func(t require.TestingT, result string) {
				bytes, err := hex.DecodeString(result)
				require.NoError(t, err)
				require.Len(t, result, 2*10)
				require.Len(t, bytes, 10)
			},
			wantErr: require.NoError,
		},
		{
			name:    "MSI_ENDPOINT with secret",
			env:     "https://azure-msi.teleport.dev/mysecret",
			want:    "mysecret",
			wantErr: require.NoError,
		},
		{
			name: "MSI_ENDPOINT with invalid prefix",
			env:  "dummy",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, `MSI_ENDPOINT not empty, but doesn't start with "https://azure-msi.teleport.dev/" as expected`)
			},
		},
		{
			name: "MSI_ENDPOINT without secret",
			env:  "https://azure-msi.teleport.dev/",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "MSI secret cannot be empty")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MSI_ENDPOINT", tt.env)
			result, err := getMSISecret()
			tt.wantErr(t, err)
			if tt.wantFunc != nil {
				tt.wantFunc(t, result)
			} else {
				require.Equal(t, tt.want, result)
			}
		})
	}
}

func Test_formatAzureIdentities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		identities []string
		want       string
	}{
		{
			name:       "empty string",
			identities: nil,
			want:       "",
		},
		{
			name:       "empty string #2",
			identities: []string{},
			want:       "",
		},
		{
			name:       "one item",
			identities: []string{"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure"},
			want: `Available Azure identities                                                                                                                                     
-------------------------------------------------------------------------------------------------------------------------------------------------------------- 
/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure 
`,
		},
		{
			name: "multiple items, sorting",
			identities: []string{
				"/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
				"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure",
			},
			want: `Available Azure identities                                                                                                                                     
-------------------------------------------------------------------------------------------------------------------------------------------------------------- 
/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure 
/subscriptions/11111111-1111-1111-1111-111111111111/resourceGroups/my-resource-group/providers/Microsoft.ManagedIdentity/userAssignedIdentities/teleport-azure 
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatAzureIdentities(tt.identities))
		})
	}
}
