// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
package msgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyCredentials(t *testing.T) {
	resourceID := "foo"
	graphErr := &GraphError{}
	mux := http.NewServeMux()
	mux.Handle(fmt.Sprintf("GET /v1.0/applications(appId='%s')", resourceID),
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			graphResponse := &graphErrorResponse{
				Error: graphErr,
			}
			body, err := json.Marshal(graphResponse)
			if !assert.NoError(t, err) {
				return
			}

			w.WriteHeader(graphErr.StatusCode)
			_, err = w.Write(body)
			assert.NoError(t, err)
		}))
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(func() { srv.Close() })

	tests := []struct {
		name         string
		tokenErr     error
		graphErr     *GraphError
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name: "tenant not found",
			tokenErr: authErrorToSDKAuthFailedError(t, &AuthError{
				ErrorCode:        "invalid_request",
				ErrorDescription: "tenant not found",
				DiagCodes:        []int{DiagCodeTenantNotFound},
				StatusCode:       400,
			}),
			errAssertion: errorIs(ErrTenantNotFound),
		},
		{
			name: "invalid tenant identifier",
			tokenErr: authErrorToSDKAuthFailedError(t, &AuthError{
				ErrorCode:        "invalid_request",
				ErrorDescription: "tenant not found",
				DiagCodes:        []int{DiagCodeInvalidTenantIdentifier},
				StatusCode:       400,
			}),
			errAssertion: errorIs(ErrTenantNotFound),
		},
		{
			name: "invalid client ID",
			tokenErr: authErrorToSDKAuthFailedError(t, &AuthError{
				ErrorCode:        "unauthorized_client",
				ErrorDescription: "app not found",
				StatusCode:       400,
			}),
			errAssertion: errorIs(ErrInvalidCredentials),
		},
		{
			name: "invalid client secret",
			tokenErr: authErrorToSDKAuthFailedError(t, &AuthError{
				ErrorCode:        "invalid_client",
				ErrorDescription: "invalid client secret provided",
				StatusCode:       401,
			}),
			errAssertion: errorIs(ErrInvalidCredentials),
		},
		{
			name: "insufficient permissions",
			graphErr: &GraphError{
				StatusCode: 401,
				Code:       "InvalidAuthenticationToken",
				Message:    "invalid authentication token",
			},
			errAssertion: errorIs(ErrClientUnauthorized),
		},
		{
			name:         "unknown token error",
			tokenErr:     errors.New("something went wrong with token"),
			errAssertion: errorIsNotCredentialsError,
		},
		{
			name: "unknown graph error",
			graphErr: &GraphError{
				StatusCode: 400,
				Code:       "whoops",
				Message:    "something went wrong",
			},
			errAssertion: errorIsNotCredentialsError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// There's no way to clear an error from [fake.TokenCredential] once it's set, hence why
			// together with a client it needs to be created within each individual test.
			tokenProvider := &fake.TokenCredential{}
			client, err := NewClient(Config{
				TokenProvider: tokenProvider,
				HTTPClient:    newHTTPClient(srv),
			})
			require.NoError(t, err)

			if tt.tokenErr != nil {
				tokenProvider.SetError(tt.tokenErr)
			}
			graphErr = tt.graphErr

			err = client.VerifyCredentials(t.Context(), func(ctx context.Context, client *Client) error {
				_, err := client.GetApplication(ctx, resourceID)
				return err
			})
			tt.errAssertion(t, err)
		})
	}
}

func authErrorToSDKAuthFailedError(t *testing.T, authErr *AuthError) *azidentity.AuthenticationFailedError {
	b, err := json.Marshal(authErr)
	require.NoError(t, err)

	sdkErr := azidentity.AuthenticationFailedError{}
	sdkErr.RawResponse = &http.Response{
		Body:       io.NopCloser(bytes.NewReader(b)),
		StatusCode: authErr.StatusCode,
	}

	return &sdkErr
}

func errorIs(target error) require.ErrorAssertionFunc {
	return func(t require.TestingT, err error, _ ...any) {
		require.ErrorIs(t, err, target)
	}
}

func errorIsNotCredentialsError(t require.TestingT, err error, _ ...any) {
	require.Error(t, err)
	require.False(t, IsCredentialsError(err))
}
