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

package alpnproxy

import (
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAzureTokenMiddlewareHandleRequest(t *testing.T) {
	t.Parallel()
	for _, alg := range []cryptosuites.Algorithm{cryptosuites.RSA2048, cryptosuites.ECDSAP256} {
		for _, endpoint := range []struct {
			name              string
			endpoint          string
			resourceFieldName string
			secret            func(string) azureRequestModifier
		}{
			{name: "msi", endpoint: types.TeleportAzureMSIEndpoint, resourceFieldName: MSIResourceFieldName, secret: msiSecretModifier},
			{name: "identity", endpoint: types.TeleportAzureIdentityEndpoint, resourceFieldName: IdentityResourceFieldName, secret: identitySecretModifier},
		} {
			t.Run(alg.String()+"_"+endpoint.name, func(t *testing.T) {
				testAzureTokenMiddlewareHandleRequest(t, alg, endpoint.endpoint, endpoint.secret, endpoint.resourceFieldName)
			})
		}
	}
}

func testAzureTokenMiddlewareHandleRequest(t *testing.T, alg cryptosuites.Algorithm, endpoint string, endpointSecret func(string) azureRequestModifier, resourceFieldName string) {
	newPrivateKey := func() crypto.Signer {
		privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(alg)
		require.NoError(t, err)
		return privateKey
	}
	privateKey := newPrivateKey()
	m := &AzureTokenMiddleware{
		Identity: "azureTestIdentity",
		TenantID: "cafecafe-cafe-4aaa-cafe-cafecafecafe",
		ClientID: "decaffff-cafe-4aaa-cafe-cafecafecafe",
		Log:      utils.NewSlogLoggerForTests(),
		Clock:    clockwork.NewFakeClockAt(time.Date(2022, 1, 1, 9, 0, 0, 0, time.UTC)),
		Secret:   "my-secret",
	}
	require.NoError(t, m.CheckAndSetDefaults())

	tests := []struct {
		name           string
		url            string
		params         map[string]string
		headers        map[string]string
		privateKey     crypto.Signer
		secretFunc     azureRequestModifier
		expectedHandle bool
		expectedCode   int
		expectedBody   string
		verifyBody     func(t *testing.T, body []byte)
	}{
		{
			name:           "ignore non-msi requests",
			url:            "https://graph.windows.net/foo/bar/baz",
			privateKey:     privateKey,
			expectedHandle: false,
		},
		{
			name:           "invalid request, wrong secret",
			url:            endpoint,
			headers:        map[string]string{},
			secretFunc:     endpointSecret("bad-secret"),
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"invalid secret\"\n    }\n}",
		},
		{
			name:           "invalid request, missing secret",
			url:            endpoint,
			headers:        map[string]string{},
			secretFunc:     emptySecretMethod,
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"invalid secret\"\n    }\n}",
		},
		{
			name:           "invalid request, missing metadata",
			url:            endpoint,
			headers:        map[string]string{},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"expected Metadata header with value 'true'\"\n    }\n}",
		},
		{
			name:           "invalid request, bad metadata value",
			url:            endpoint,
			headers:        map[string]string{"Metadata": "false"},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"expected Metadata header with value 'true'\"\n    }\n}",
		},
		{
			name:           "invalid request, missing arguments",
			url:            endpoint,
			headers:        map[string]string{"Metadata": "true"},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"missing value for parameter 'resource'\"\n    }\n}",
		},
		{
			name:    "invalid request, missing resource",
			url:     endpoint,
			headers: map[string]string{"Metadata": "true"},
			params: map[string]string{
				resourceFieldName: "azureTestIdentity",
			},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"missing value for parameter 'resource'\"\n    }\n}",
		},
		{
			name:    "invalid request, missing identity",
			url:     endpoint,
			headers: map[string]string{"Metadata": "true"},
			params: map[string]string{
				"resource": "myresource",
			},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   fmt.Sprintf("{\n    \"error\": {\n        \"message\": \"unexpected value for parameter '%s': \"\n    }\n}", resourceFieldName),
		},
		{
			name:    "invalid request, wrong identity",
			url:     endpoint,
			headers: map[string]string{"Metadata": "true"},
			params: map[string]string{
				resourceFieldName: "azureTestWrongIdentity",
				"resource":        "myresource",
			},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     privateKey,
			expectedHandle: true,
			expectedCode:   400,
			expectedBody:   fmt.Sprintf("{\n    \"error\": {\n        \"message\": \"unexpected value for parameter '%s': azureTestWrongIdentity\"\n    }\n}", resourceFieldName),
		},
		{
			name:    "well-formatted request",
			url:     endpoint,
			headers: map[string]string{"Metadata": "true"},
			params: map[string]string{
				resourceFieldName: "azureTestIdentity",
				"resource":        "myresource",
			},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     privateKey,
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
						ClusterName: types.TeleportAzureMSIEndpoint,
					})
					require.NoError(t, err)
					return key.VerifyAzureToken(token)
				}

				claims, err := fromJWT(req.AccessToken, privateKey)
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
		{
			name:    "no private key set",
			url:     endpoint,
			headers: map[string]string{"Metadata": "true"},
			params: map[string]string{
				resourceFieldName: "azureTestIdentity",
				"resource":        "myresource",
			},
			secretFunc:     endpointSecret("my-secret"),
			privateKey:     nil,
			expectedHandle: true,
			expectedCode:   500,
			expectedBody:   "{\n    \"error\": {\n        \"message\": \"missing private key set in AzureTokenMiddleware\"\n    }\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.SetPrivateKey(tt.privateKey)

			params := url.Values{}
			for name, value := range tt.params {
				params.Set(name, value)
			}

			// prepare request
			req, err := http.NewRequest("GET", "https://"+tt.url+"?"+params.Encode(), strings.NewReader(""))
			require.NoError(t, err)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			if tt.secretFunc != nil {
				tt.secretFunc(req)
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

// azureRequestModifier modifies an Azure request.
type azureRequestModifier func(req *http.Request)

func msiSecretModifier(secret string) azureRequestModifier {
	return func(req *http.Request) {
		req.URL = req.URL.JoinPath(secret)
	}
}

func identitySecretModifier(secret string) azureRequestModifier {
	return func(req *http.Request) {
		req.Header.Add(IdentitySecretHeader, secret)
	}
}

func emptySecretMethod(_ *http.Request) {}
