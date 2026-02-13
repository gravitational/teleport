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

package azure

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tc.handler)
			clt := tc.client(t, server)
			tc.assertion(t, clt.IsAvailable(t.Context()))
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

type mockIMDS struct {
	t              *testing.T
	versionsCalled bool
	lastAPIVersion string

	mu sync.Mutex
}

func (m *mockIMDS) status() (versionsCalled bool, lastAPIVersion string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.versionsCalled, m.lastAPIVersion
}

func newMockIMDS(t *testing.T, overrides map[string]http.Handler) (*mockIMDS, *httptest.Server) {
	t.Helper()

	m := &mockIMDS{t: t}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		defer m.mu.Unlock()

		m.lastAPIVersion = r.URL.Query().Get("api-version")

		if r.URL.Path == "/versions" {
			m.versionsCalled = true
		}

		// /versions doesn't require api-version; all other endpoints do
		if r.URL.Path != "/versions" && m.lastAPIVersion == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Bad request. api-version is invalid or was not specified in the request.","newest-versions":["2023-07-01","2021-02-01"]}`))
			return
		}

		if overrides != nil {
			handler, ok := overrides[r.URL.Path]
			if ok {
				handler.ServeHTTP(w, r)
				return
			}
		}

		responses := map[string]string{
			"/versions":                  `{"apiVersions":["2021-02-01","2023-07-01"]}`,
			"/instance/compute":          `{"resourceId":"/subscriptions/test","location":"eastus"}`,
			"/instance/compute/tagsList": `[{"name":"foo","value":"bar"}]`,
			"/attested/document":         `{"signature":"test"}`,
			"/identity/oauth2/token":     `{"access_token":"test-token"}`,
		}

		if body, ok := responses[r.URL.Path]; ok {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))

	t.Cleanup(srv.Close)

	return m, srv
}

func TestGetInstanceInfo(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		statusCode           int
		body                 []byte
		expectedInstanceInfo *InstanceInfo
		wantErr              string
	}{
		{
			name:       "with resource ID",
			statusCode: http.StatusOK,
			body:       []byte(`{"resourceId":"test-id"}`),
			expectedInstanceInfo: &InstanceInfo{
				ResourceID: "test-id",
			},
			wantErr: "",
		},
		{
			name:       "all fields",
			statusCode: http.StatusOK,
			body: []byte(`{"resourceId":"test-id", "location":"eastus", "resourceGroupName":"TestGroup", ` +
				`"subscriptionId": "5187AF11-3581-4AB6-A654-59405CD40C44", "vmId":"ED7DAC09-6E73-447F-BD18-AF4D1196C1E4"}`),
			expectedInstanceInfo: &InstanceInfo{
				ResourceID:        "test-id",
				Location:          "eastus",
				ResourceGroupName: "TestGroup",
				SubscriptionID:    "5187AF11-3581-4AB6-A654-59405CD40C44",
				VMID:              "ED7DAC09-6E73-447F-BD18-AF4D1196C1E4",
			},
			wantErr: "",
		},
		{
			name:       "request error",
			statusCode: http.StatusNotFound,
			wantErr:    "not found",
		},
		{
			name:       "empty body returns an error",
			statusCode: http.StatusOK,
			wantErr:    "error found in #0 byte",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, server := newMockIMDS(t, map[string]http.Handler{
				"/instance/compute": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.statusCode)
					w.Write(tc.body)
				}),
			})

			client := NewInstanceMetadataClient(WithBaseURL(server.URL))
			instanceInfo, err := client.GetInstanceInfo(t.Context())
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expectedInstanceInfo, instanceInfo)
			} else {
				require.Nil(t, instanceInfo)
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestGetInstanceID(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		statusCode         int
		body               []byte
		expectedResourceID string
		errAssertion       require.ErrorAssertionFunc
	}{
		{
			name:               "with resource ID",
			statusCode:         http.StatusOK,
			body:               []byte(`{"resourceId":"test-id"}`),
			expectedResourceID: "test-id",
			errAssertion:       require.NoError,
		},
		{
			name:         "with error",
			statusCode:   http.StatusOK,
			body:         []byte(`{"error":"test-error"}`),
			errAssertion: require.Error,
		},
		{
			name:       "request error",
			statusCode: http.StatusNotFound,
			errAssertion: func(tt require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "not found")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, server := newMockIMDS(t, map[string]http.Handler{
				"/instance/compute": http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tc.statusCode)
					w.Write(tc.body)
				}),
			})

			client := NewInstanceMetadataClient(WithBaseURL(server.URL))
			resourceID, err := client.GetID(t.Context())
			tc.errAssertion(t, err)
			require.Equal(t, tc.expectedResourceID, resourceID)
		})
	}
}

func TestMethodsEnsureInitialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		call func(ctx context.Context, c *InstanceMetadataClient) error
	}{
		{"GetInstanceInfo", func(ctx context.Context, c *InstanceMetadataClient) error {
			_, err := c.GetInstanceInfo(ctx)
			return err
		}},
		{"GetID", func(ctx context.Context, c *InstanceMetadataClient) error {
			_, err := c.GetID(ctx)
			return err
		}},
		{"GetTags", func(ctx context.Context, c *InstanceMetadataClient) error {
			_, err := c.GetTags(ctx)
			return err
		}},
		{"GetAttestedData", func(ctx context.Context, c *InstanceMetadataClient) error {
			_, err := c.GetAttestedData(ctx, "")
			return err
		}},
		{"GetAccessToken", func(ctx context.Context, c *InstanceMetadataClient) error {
			_, err := c.GetAccessToken(ctx, "")
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mock, srv := newMockIMDS(t, nil)
			defer srv.Close()

			client := NewInstanceMetadataClient(WithBaseURL(srv.URL))
			require.Empty(t, client.GetAPIVersion(), "client should start uninitialized")

			err := tc.call(t.Context(), client)
			require.NoError(t, err)
			versionsCalled, lastAPIVersion := mock.status()
			require.True(t, versionsCalled, "should call /versions to initialize")
			require.Equal(t, "2023-07-01", lastAPIVersion, "should use negotiated api-version")
			require.Equal(t, "2023-07-01", client.GetAPIVersion(), "client should be initialized")
		})
	}
}
