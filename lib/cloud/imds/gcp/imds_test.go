/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package gcp

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils"
)

func makeMetadataGetter(values map[string]string) MetadataGetter {
	return func(ctx context.Context, path string) (string, error) {
		value, ok := values[path]
		if ok {
			return value, nil
		}
		return "", trace.NotFound("no value for %v", path)
	}
}

type mockInstanceGetter struct {
	InstanceGetter
	instance    *Instance
	instanceErr error
	tags        map[string]string
	tagsErr     error
}

func (m *mockInstanceGetter) GetInstance(ctx context.Context, req *InstanceRequest) (*Instance, error) {
	return m.instance, m.instanceErr
}

func (m *mockInstanceGetter) GetInstanceTags(ctx context.Context, req *InstanceRequest) (map[string]string, error) {
	return m.tags, m.tagsErr
}

func TestIsInstanceMetadataAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		getMetadata MetadataGetter
		assert      require.BoolAssertionFunc
	}{
		{
			name: "not available",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "", trace.NotFound("")
			},
			assert: require.False,
		},
		{
			name: "not on gcp",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "non-numeric id", nil
			},
			assert: require.False,
		},
		{
			name: "zero ID",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "0", nil
			},
			assert: require.False,
		},
		{
			name: "on mocked gcp",
			getMetadata: func(ctx context.Context, path string) (string, error) {
				return "12345678", nil
			},
			assert: require.True,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &InstanceMetadataClient{
				getMetadata: tc.getMetadata,
			}
			tc.assert(t, client.IsAvailable(context.Background()))
		})
	}

	t.Run("on real gcp", func(t *testing.T) {
		if os.Getenv("TELEPORT_TEST_GCP") == "" {
			t.Skip("not on gcp")
		}
		client, err := NewInstanceMetadataClient(nil)
		require.NoError(t, err)
		require.True(t, client.IsAvailable(context.Background()))
	})
}

func TestGetTags(t *testing.T) {
	t.Parallel()

	defaultMetadataGetter := makeMetadataGetter(map[string]string{
		"project/project-id": "myproject",
		"instance/zone":      "myzone",
		"instance/name":      "myname",
		"instance/id":        "12345678",
	})
	defaultInstance := &Instance{
		ProjectID: "myproject",
		Zone:      "myzone",
		Name:      "myname",
		Labels: map[string]string{
			"foo": "bar",
		},
	}

	tests := []struct {
		name            string
		getMetadata     MetadataGetter
		instancesClient *mockInstanceGetter
		assertErr       require.ErrorAssertionFunc
		expectedTags    map[string]string
	}{
		{
			name:        "ok",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instance: defaultInstance,
				tags: map[string]string{
					"baz": "quux",
				},
			},
			assertErr: require.NoError,
			expectedTags: map[string]string{
				"label/foo": "bar",
				"tag/baz":   "quux",
			},
		},
		{
			name:        "not on gcp",
			getMetadata: makeMetadataGetter(nil),
			instancesClient: &mockInstanceGetter{
				instance: defaultInstance,
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), i...)
			},
		},
		{
			name:        "instance not found",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instanceErr: trace.NotFound(""),
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err), i...)
			},
		},
		{
			name:        "denied access to instance",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instanceErr: trace.AccessDenied(""),
				tags: map[string]string{
					"baz": "quux",
				},
			},
			assertErr: require.NoError,
			expectedTags: map[string]string{
				"tag/baz": "quux",
			},
		},
		{
			name:        "denied access to tags",
			getMetadata: defaultMetadataGetter,
			instancesClient: &mockInstanceGetter{
				instance: defaultInstance,
				tagsErr:  trace.AccessDenied(""),
			},
			assertErr: require.NoError,
			expectedTags: map[string]string{
				"label/foo": "bar",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &InstanceMetadataClient{
				getMetadata:    tc.getMetadata,
				instanceGetter: tc.instancesClient,
			}
			tags, err := client.GetTags(context.Background())
			tc.assertErr(t, err)
			require.Equal(t, tc.expectedTags, tags)
		})
	}
}

func TestGCPIsInstanceMetadataAvailableWithHTTPProxyEnv(t *testing.T) {
	imdsFakeServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/computeMetadata/v1/instance/id" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("12345678"))
		}
	}))

	// Azure IMDS local server is available at 169.254.169.254
	// https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
	//
	// To access IMDS, clients must never use a proxy, even if HTTP_PROXY env is set
	// https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service?tabs=linux#proxies
	// > IMDS is not intended to be used behind a proxy and doing so is unsupported.
	//
	// This test verifies that our client correctly ignores HTTP_PROXY env and can access IMDS server.
	// So, we start a local HTTP server to emulate the IMDS endpoint and set HTTP_PROXY env to point to a non-existent proxy server.
	// Then, the client is created and it should be able to access the fake IMDS server successfully, proving that it ignores the HTTP_PROXY env.
	//
	// The problem with the above setup is that Go's default HTTP proxy implementation ignores calls to localhost and any IP that is loopback, even if HTTP_PROXY is set.
	// See: https://cs.opensource.google/go/x/net/+/refs/tags/v0.55.0:http/httpproxy/proxy.go;l=185-186
	// So, while the code is correct we can't prove it with a test, because httptest.Server will always give us a localhost address.
	//
	// To work around this, we need to set the base url in the client to use a hostname that is not localhost.
	// Plan A is to find a non-loopback local interface and use its IP address as the server host.
	// The fallback is to use nip.io service which which resolves any hostname in the format <IP>.nip.io to IP.
	//
	// This way, we can prove that removing the defaults.DisableProxyFromEnvironment() when creating the HTTP client in imds.go causes the test to fail.
	// We also need to create a specific listener that binds to all interfaces because httptest.Server only binds to localhost.
	l, err := net.Listen("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	imdsFakeServer.Listener = l
	imdsFakeServer.Start()
	defer imdsFakeServer.Close()

	serverHost, err := testutils.NonLocalhostLocalInterfaceIP()
	if err != nil {
		t.Logf("failed to find a non-localhost usable network interface, using 127.0.0.1.nip.io: %v", err)
		serverHost = "127.0.0.1.nip.io"
	}

	fakeServerURL, err := url.Parse(imdsFakeServer.URL)
	require.NoError(t, err)
	serverHostWithPort := net.JoinHostPort(serverHost, fakeServerURL.Port())

	t.Setenv("HTTP_PROXY", "http://127.0.0.1:0")

	t.Setenv("GCE_METADATA_HOST", serverHostWithPort)

	client, err := NewInstanceMetadataClient(nil)
	require.NoError(t, err)
	require.True(t, client.IsAvailable(t.Context()), "instance metadata should be available even with HTTP_PROXY set")
}
