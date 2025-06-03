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
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/go-semver/semver"
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

	getEnvValue := func(cmdEnv []string, key string) string {
		for _, env := range cmdEnv {
			if strings.HasPrefix(env, key+"=") {
				return strings.TrimPrefix(env, key+"=")
			}
		}
		return ""
	}

	versionWithoutMSAL := semver.New(azureCLIVersionMSALRequirement.String())
	versionWithoutMSAL.Minor -= 1

	for name, tc := range map[string]struct {
		setEnvironment       func(t *testing.T)
		cliVersion           *semver.Version
		tokenEndpointURL     string
		expectedLoginCommand []string
		assertCommandEnv     require.ValueAssertionFunc
	}{
		"MSI": {
			setEnvironment: func(t *testing.T) {
				// This is required to avoid having a random generated secret.
				t.Setenv(msiEndpointEnvVarName, "https://azure-msi.teleport.dev/very-secret")
			},
			cliVersion:           versionWithoutMSAL,
			tokenEndpointURL:     "https://azure-msi.teleport.dev/very-secret",
			expectedLoginCommand: []string{"az", "login", "--identity", "--username", "dummy_azure_identity"},
			assertCommandEnv: func(t require.TestingT, val any, msgAndArgs ...any) {
				env := val.([]string)
				require.Equal(t, "https://azure-msi.teleport.dev/very-secret", getEnvValue(env, msiEndpointEnvVarName))
			},
		},
		"Identity": {
			setEnvironment: func(t *testing.T) {
				// This is required to avoid having a random generated secret.
				t.Setenv(identityEndpointEnvVarName, "https://azure-identity.teleport.dev")
				t.Setenv(identityHeaderEnvVarName, "very-secret")
			},
			cliVersion:           azureCLIVersionMSALRequirement,
			tokenEndpointURL:     "https://azure-identity.teleport.dev",
			expectedLoginCommand: []string{"az", "login", "--identity", "--resource-id", "dummy_azure_identity"},
			assertCommandEnv: func(t require.TestingT, val any, msgAndArgs ...any) {
				env := val.([]string)
				require.Equal(t, "https://azure-identity.teleport.dev", getEnvValue(env, identityEndpointEnvVarName))
				require.Equal(t, "very-secret", getEnvValue(env, identityHeaderEnvVarName))
			},
		},
	} {
		t.Run("With"+name, func(t *testing.T) {
			handleAzVersion := func(cmd *exec.Cmd) bool {
				if len(cmd.Args) > 0 && cmd.Args[1] == "version" {
					fmt.Fprintf(cmd.Stdout, `{ "azure-cli": "%s", "azure-cli-core": "%s", "azure-cli-telemetry": "1.1.0", "extensions": {} }`, tc.cliVersion.String(), tc.cliVersion.String())
					return true
				}
				return false
			}

			tc.setEnvironment(t)

			// Log into Teleport cluster.
			run([]string{"login", "--insecure", "--debug", "--proxy", proxyAddr.String()})

			// Log into the "azure-api" app.
			// Verify `tsh az login ...` gets called.
			run([]string{"app", "login", "--insecure", "--azure-identity", "dummy_azure_identity", "azure-api"},
				setCmdRunner(func(cmd *exec.Cmd) error {
					if handleAzVersion(cmd) {
						return nil
					}

					require.Equal(t, tc.expectedLoginCommand, cmd.Args[1:])
					return nil
				}))

			// Log into the "azure-api" app -- now with --debug flag.
			run([]string{"app", "login", "--insecure", "azure-api", "--debug"},
				setCmdRunner(func(cmd *exec.Cmd) error {
					if handleAzVersion(cmd) {
						return nil
					}

					require.Equal(t, append(tc.expectedLoginCommand, "--debug"), cmd.Args[1:])
					return nil
				}))

			// Run `tsh az vm ls`. Verify executed command and environment.
			run([]string{"az", "vm", "ls", "-g", "my-group"},
				setCmdRunner(func(cmd *exec.Cmd) error {
					if handleAzVersion(cmd) {
						return nil
					}

					require.Equal(t, []string{"az", "vm", "ls", "-g", "my-group"}, cmd.Args)

					require.Equal(t, filepath.Join(tmpHomePath, "azure/localhost/azure-api"), getEnvValue(cmd.Env, "AZURE_CONFIG_DIR"))
					require.Equal(t, filepath.Join(tmpHomePath, "keys/127.0.0.1/alice@example.com-app/localhost/azure-api-localca.pem"), getEnvValue(cmd.Env, "REQUESTS_CA_BUNDLE"))
					require.True(t, strings.HasPrefix(getEnvValue(cmd.Env, "HTTPS_PROXY"), "http://127.0.0.1:"))

					tc.assertCommandEnv(t, cmd.Env)

					// Validate MSI endpoint can be reached
					caPool, err := utils.NewCertPoolFromPath(getEnvValue(cmd.Env, "REQUESTS_CA_BUNDLE"))
					require.NoError(t, err)

					httpsProxy, err := url.Parse(getEnvValue(cmd.Env, "HTTPS_PROXY"))
					require.NoError(t, err)

					// Dial using the Azure token service to ensure it will be
					// reachable and handled when requested by the Azure CLI.
					client := &http.Client{
						Transport: &http.Transport{
							Proxy:           http.ProxyURL(httpsProxy),
							TLSClientConfig: &tls.Config{RootCAs: caPool},
						},
					}

					req, err := http.NewRequest("GET", tc.tokenEndpointURL, nil)
					require.NoError(t, err)

					// Given the missing params, the request should return error.
					resp, err := client.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close()
					require.NotNil(t, resp)
					require.Equal(t, http.StatusBadRequest, resp.StatusCode)

					return nil
				}))
		})
	}
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

func Test_getAzureTokenSecret(t *testing.T) {
	tests := []struct {
		name             string
		msiEndpoint      string
		identityHeader   string
		identityEndpoint string
		want             string
		wantFunc         func(t require.TestingT, result string)
		wantErr          require.ErrorAssertionFunc
	}{
		{
			name:        "no env",
			msiEndpoint: "",
			wantFunc: func(t require.TestingT, result string) {
				bytes, err := hex.DecodeString(result)
				require.NoError(t, err)
				require.Len(t, result, 2*10)
				require.Len(t, bytes, 10)
			},
			wantErr: require.NoError,
		},
		{
			name:        "MSI_ENDPOINT with secret",
			msiEndpoint: "https://" + types.TeleportAzureMSIEndpoint + "/mysecret",
			want:        "mysecret",
			wantErr:     require.NoError,
		},
		{
			name:        "MSI_ENDPOINT with invalid prefix",
			msiEndpoint: "dummy",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, `"MSI_ENDPOINT" environment variable not empty, but doesn't start with "https://azure-msi.teleport.dev/" as expected`)
			},
		},
		{
			name:        "MSI_ENDPOINT without secret",
			msiEndpoint: "https://" + types.TeleportAzureMSIEndpoint + "/",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "MSI secret cannot be empty")
			},
		},
		{
			name:             "IDENTITY_HEADER and IDENTITY_ENDPOINT present",
			identityHeader:   "secret",
			identityEndpoint: "https://" + types.TeleportAzureIdentityEndpoint,
			want:             "secret",
			wantErr:          require.NoError,
		},
		{
			name:           "IDENTITY_HEADER present without endpoint",
			identityHeader: "secret",
			wantErr: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, `IDENTITY_HEADER`)
			},
		},
		{
			name:             "Identity and MSI present, identity takes precedence",
			identityHeader:   "secret",
			identityEndpoint: "https://" + types.TeleportAzureIdentityEndpoint,
			msiEndpoint:      "https://azure-msi.teleport.dev/different-secret",
			want:             "secret",
			wantErr:          require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(msiEndpointEnvVarName, tt.msiEndpoint)
			t.Setenv(identityHeaderEnvVarName, tt.identityHeader)
			t.Setenv(identityEndpointEnvVarName, tt.identityEndpoint)
			result, err := getAzureTokenSecret()
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
