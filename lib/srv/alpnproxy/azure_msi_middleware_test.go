// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alpnproxy

import (
	"crypto"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAzureMSIMiddlewareHandleRequest(t *testing.T) {
	newPrivateKey := func() crypto.Signer {
		_, privateBytes, err := jwt.GenerateKeyPair()
		require.NoError(t, err)
		privateKey, err := utils.ParsePrivateKey(privateBytes)
		require.NoError(t, err)
		return privateKey
	}

	m := &AzureMSIMiddleware{
		Identity: "azureTestIdentity",
		TenantID: "cafecafe-cafe-4aaa-cafe-cafecafecafe",
		ClientID: "decaffff-cafe-4aaa-cafe-cafecafecafe",
		Log:      logrus.WithField(trace.Component, "msi"),
		Clock:    clockwork.NewFakeClockAt(time.Date(2022, 1, 1, 9, 0, 0, 0, time.UTC)),
		Key:      newPrivateKey(),
		Secret:   "my-secret",
	}
	require.NoError(t, m.CheckAndSetDefaults())

	tests := []struct {
		name           string
		url            string
		headers        map[string]string
		expectedHandle bool
		expectedCode   int
		expectedBody   string
		verifyBody     func(t *testing.T, body []byte)
	}{
		{
			name:           "ignore non-msi requests",
			url:            "https://graph.windows.net/foo/bar/baz",
			expectedHandle: false,
		},
		{
			name:           "invalid request, wrong secret",
			url:            "https://azure-msi.teleport.dev/bad-secret",
			headers:        nil,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"invalid secret\"\n    }\n}",
		},
		{
			name:           "invalid request, missing secret",
			url:            "https://azure-msi.teleport.dev",
			headers:        nil,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"invalid secret\"\n    }\n}",
		},
		{
			name:           "invalid request, missing metadata",
			url:            "https://azure-msi.teleport.dev/my-secret",
			headers:        nil,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"expected Metadata header with value 'true'\"\n    }\n}",
		},
		{
			name:           "invalid request, bad metadata value",
			url:            "https://azure-msi.teleport.dev/my-secret",
			headers:        map[string]string{"Metadata": "false"},
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"expected Metadata header with value 'true'\"\n    }\n}",
		},
		{
			name:           "invalid request, missing arguments",
			url:            "https://azure-msi.teleport.dev/my-secret",
			headers:        map[string]string{"Metadata": "true"},
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"missing value for parameter 'resource'\"\n    }\n}",
		},
		{
			name:           "invalid request, missing resource",
			url:            "https://azure-msi.teleport.dev/my-secret?msi_res_id=azureTestIdentity",
			headers:        map[string]string{"Metadata": "true"},
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"missing value for parameter 'resource'\"\n    }\n}",
		},
		{
			name:           "invalid request, missing identity",
			url:            "https://azure-msi.teleport.dev/my-secret?resource=myresource",
			headers:        map[string]string{"Metadata": "true"},
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"unexpected value for parameter 'msi_res_id': \"\n    }\n}",
		},
		{
			name:           "invalid request, wrong identity",
			url:            "https://azure-msi.teleport.dev/my-secret?resource=myresource&msi_res_id=azureTestWrongIdentity",
			headers:        map[string]string{"Metadata": "true"},
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"unexpected value for parameter 'msi_res_id': azureTestWrongIdentity\"\n    }\n}",
		},
		{
			name:           "well-formatted request",
			url:            "https://azure-msi.teleport.dev/my-secret?resource=myresource&msi_res_id=azureTestIdentity",
			headers:        map[string]string{"Metadata": "true"},
			expectedHandle: true,
			expectedCode:   200,
			verifyBody: func(t *testing.T, body []byte) {
				type request struct {
					AccessToken  string `json:"access_token"`
					ClientID     string `json:"client_id"`
					Resource     string `json:"resource"`
					TokenType    string `json:"token_type"`
					ExpiresIn    int    `json:"expires_in"`
					ExpiresOn    int    `json:"expires_on"`
					ExtExpiresIn int    `json:"ext_expires_in"`
					NotBefore    int    `json:"not_before"`
				}
				var req request
				require.NoError(t, json.Unmarshal(body, &req))

				expected := request{
					ClientID:     "decaffff-cafe-4aaa-cafe-cafecafecafe",
					Resource:     "myresource",
					TokenType:    "Bearer",
					ExpiresIn:    31536000,
					ExpiresOn:    1672563600,
					ExtExpiresIn: 31536000,
					NotBefore:    1641027590,
				}

				fromJWT := func(token string, pk crypto.Signer) (*jwt.AzureTokenClaims, error) {
					key, err := jwt.New(&jwt.Config{
						Clock:       m.Clock,
						PrivateKey:  pk,
						Algorithm:   defaults.ApplicationTokenAlgorithm,
						ClusterName: types.TeleportAzureMSIEndpoint,
					})
					require.NoError(t, err)
					return key.VerifyAzureToken(token)
				}

				claims, err := fromJWT(req.AccessToken, m.Key)
				require.NoError(t, err)
				require.Equal(t, jwt.AzureTokenClaims{
					TenantID: "cafecafe-cafe-4aaa-cafe-cafecafecafe",
					Resource: "myresource",
				}, *claims)

				// verify that verification fails with different private key
				_, err = fromJWT(req.AccessToken, newPrivateKey())
				require.Error(t, err)

				require.Equal(t, expected.ClientID, req.ClientID)
				require.Equal(t, expected.Resource, req.Resource)
				require.Equal(t, expected.TokenType, req.TokenType)
				require.Equal(t, expected.ExpiresIn, req.ExpiresIn)
				require.Equal(t, expected.ExpiresOn, req.ExpiresOn)
				require.Equal(t, expected.ExtExpiresIn, req.ExtExpiresIn)
				require.Equal(t, expected.NotBefore, req.NotBefore)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// prepare request
			req, err := http.NewRequest("GET", tt.url, strings.NewReader(""))
			require.NoError(t, err)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			recorder := httptest.NewRecorder()

			// run handler
			handled := m.HandleRequest(recorder, req)
			require.Equal(t, tt.expectedHandle, handled)
			if !handled {
				// skip the rest of test
				return
			}

			// check results
			resp := recorder.Result()
			require.Equal(t, tt.expectedCode, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			if tt.verifyBody != nil {
				tt.verifyBody(t, body)
			} else {
				require.Equal(t, tt.expectedBody, string(body))
			}
		})
	}
}
