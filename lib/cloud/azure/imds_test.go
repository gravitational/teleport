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
package azure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestAzureIsInstanceMetadataAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		handler   http.HandlerFunc
		client    func(t *testing.T, server *httptest.Server) *InstanceMetadataClient
		assertion require.BoolAssertionFunc
	}{
		{
			name: "not available",
			client: func(t *testing.T, server *httptest.Server) *InstanceMetadataClient {
				server.Close()
				clt := NewInstanceMetadataClient(WithBaseURL(server.URL))

				return clt
			},
			handler:   func(w http.ResponseWriter, r *http.Request) {},
			assertion: require.False,
		},
		{
			name: "mistake some other service for instance metadata",
			client: func(t *testing.T, server *httptest.Server) *InstanceMetadataClient {
				clt := NewInstanceMetadataClient(WithBaseURL(server.URL))

				return clt
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hello there!"))
			},
			assertion: require.False,
		},
		{
			name:    "on azure",
			handler: func(w http.ResponseWriter, r *http.Request) {},
			client: func(t *testing.T, server *httptest.Server) *InstanceMetadataClient {
				if os.Getenv("TELEPORT_TEST_AZURE") == "" {
					t.Skip("not on azure")
				}
				clt := NewInstanceMetadataClient()
				return clt
			},
			assertion: require.True,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			server := httptest.NewServer(tc.handler)
			clt := tc.client(t, server)
			tc.assertion(t, clt.IsAvailable(ctx))
		})
	}
}

func TestSelectVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		versions        []string
		minimumVersion  string
		expectedVersion string
	}{
		{
			name:            "exact match",
			versions:        []string{"2020-03-04", "2021-06-08"},
			minimumVersion:  "2021-06-08",
			expectedVersion: "2021-06-08",
		},
		{
			name:            "more recent version",
			versions:        []string{"2020-03-04", "2021-06-08"},
			minimumVersion:  "2020-06-08",
			expectedVersion: "2021-06-08",
		},
		{
			name:           "no match",
			versions:       []string{"2020-03-04", "2021-06-08"},
			minimumVersion: "2022-10-12",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			version, err := selectVersion(tc.versions, tc.minimumVersion)
			if tc.expectedVersion == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedVersion, version)
			}
		})
	}
}

func TestParseMetadataClientError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		code        int
		body        []byte
		wantErr     func(error) bool
		wantMessage string
	}{
		{
			name: "ok",
			code: http.StatusOK,
		},
		{
			name:        "error message",
			code:        http.StatusNotFound,
			body:        []byte(`{"error": "test message"}`),
			wantErr:     trace.IsNotFound,
			wantMessage: "test message",
		},
		{
			name:    "non-JSON response",
			code:    http.StatusNotFound,
			body:    []byte("<html>some html junk</html>"),
			wantErr: trace.IsNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := parseMetadataClientError(tc.code, tc.body)
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}

			require.True(t, tc.wantErr(err))
			if tc.wantMessage != "" {
				require.ErrorContains(t, err, tc.wantMessage)
			}
		})
	}
}

func TestGetInstanceID(t *testing.T) {
	for _, tc := range []struct {
		name               string
		stausCode          int
		body               []byte
		expectedResourceID string
		errAssertion       require.ErrorAssertionFunc
	}{
		{
			name:               "with resource ID",
			stausCode:          http.StatusOK,
			body:               []byte(`{"resourceId":"test-id"}`),
			expectedResourceID: "test-id",
			errAssertion:       require.NoError,
		},
		{
			name:         "with error",
			stausCode:    http.StatusOK,
			body:         []byte(`{"error":"test-error"}`),
			errAssertion: require.Error,
		},
		{
			name:         "request error",
			stausCode:    http.StatusNotFound,
			errAssertion: require.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write(tc.body)
			}))

			client := NewInstanceMetadataClient(WithBaseURL(server.URL))
			resourceID, err := client.GetID(context.Background())
			tc.errAssertion(t, err)
			require.Equal(t, tc.expectedResourceID, resourceID)
		})
	}
}

func TestGetInstanceMetadata(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		expectedInstanceMetadata *types.InstanceMetadata
		errAssertion             require.ErrorAssertionFunc
	}{
		{
			name:                     "not implemented should return empty struct",
			expectedInstanceMetadata: &types.InstanceMetadata{},
			errAssertion:             require.NoError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))

			client := NewInstanceMetadataClient(WithBaseURL(server.URL))

			instanceMetadata, err := client.GetInstanceMetadata(context.Background())
			tc.errAssertion(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tc.expectedInstanceMetadata, instanceMetadata)
		})
	}
}
