/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAzure(t *testing.T) {
	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	user, azureRole := makeUserWithAzureRole(t)

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, user, azureRole))
	makeTestApplicationServer(t, authProcess, proxyProcess, service.App{
		Name:  "azure-api",
		Cloud: types.CloudAzure,
	})

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// helper function
	run := func(args []string, opts ...cliOption) {
		opts = append(opts, setHomePath(tmpHomePath))
		opts = append(opts, func(cf *CLIConf) error {
			cf.mockSSOLogin = mockSSOLogin(t, authServer, user)
			return nil
		})
		err := Run(context.Background(), args, opts...)
		require.NoError(t, err)
	}

	// Log into Teleport cluster.
	run([]string{"login", "--insecure", "--debug", "--auth", connector.GetName(), "--proxy", proxyAddr.String()})

	// Log into the "azure-api" app.
	// Verify `tsh az login ...` gets called.
	run([]string{"app", "login", "azure-api"},
		setCmdRunner(func(cmd *exec.Cmd) error {
			require.Equal(t, []string{"az", "login", "--identity", "-u", "dummy_azure_identity"}, cmd.Args[1:])
			return nil
		}))

	// Log into the "azure-api" app -- now with --debug flag.
	run([]string{"app", "login", "azure-api", "--debug"},
		setCmdRunner(func(cmd *exec.Cmd) error {
			require.Equal(t, []string{"--debug", "az", "login", "--identity", "-u", "dummy_azure_identity"}, cmd.Args[1:])
			return nil
		}))

	t.Setenv("HOME", "/user/home/dir")

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
			url:          "https://azure-msi.teleport.dev",
			headers:      nil,
			expectedCode: 400,
			expectedBody: []byte("{\n    \"error\": {\n        \"message\": \"expected Metadata header with value 'true'\"\n    }\n}"),
		},
		{
			name:         "well-formatted request",
			url:          "https://azure-msi.teleport.dev?resource=myresource&msi_res_id=dummy_azure_identity",
			headers:      map[string]string{"Metadata": "true"},
			expectedCode: 200,
			verifyBody: func(t *testing.T, body []byte) {
				var req struct {
					AccessToken  string `json:"access_token"`
					ClientId     string `json:"client_id"`
					Resource     string `json:"resource"`
					TokenType    string `json:"token_type"`
					ExpiresIn    int    `json:"expires_in"`
					ExpiresOn    int    `json:"expires_on"`
					ExtExpiresIn int    `json:"ext_expires_in"`
					NotBefore    int    `json:"not_before"`
				}

				require.NoError(t, json.Unmarshal(body, &req))

				require.NotEmpty(t, req.AccessToken)
				require.NotEmpty(t, req.ClientId)
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

			require.Equal(t, "/user/home/dir/.azure-teleport-azure-api", getEnvValue("AZURE_CONFIG_DIR"))
			require.Equal(t, "https://azure-msi.teleport.dev", getEnvValue("MSI_ENDPOINT"))
			require.Equal(t, path.Join(tmpHomePath, "keys/127.0.0.1/alice@example.com-app/localhost/azure-api-localca.pem"), getEnvValue("REQUESTS_CA_BUNDLE"))
			require.True(t, strings.HasPrefix(getEnvValue("HTTPS_PROXY"), "http://127.0.0.1:"))
			require.True(t, strings.HasPrefix(getEnvValue("HTTP_PROXY"), "http://127.0.0.1:"))

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
	alice.SetAzureIdentities([]string{"dummy_azure_identity"})

	return alice, role
}
