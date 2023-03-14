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
package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/require"
)

func TestEC2IsInstanceMetadataAvailable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		handler   http.HandlerFunc
		client    func(t *testing.T, ctx context.Context, server *httptest.Server) *InstanceMetadataClient
		assertion require.BoolAssertionFunc
	}{
		{
			name: "not available",
			client: func(t *testing.T, ctx context.Context, server *httptest.Server) *InstanceMetadataClient {
				clt, err := NewInstanceMetadataClient(ctx, WithIMDSClient(imds.New(imds.Options{Endpoint: server.URL})))
				require.NoError(t, err)
				return clt
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			assertion: require.False,
		},
		{
			name: "mistake some other service for instance metadata",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hello there!"))
			},
			client: func(t *testing.T, ctx context.Context, server *httptest.Server) *InstanceMetadataClient {
				clt, err := NewInstanceMetadataClient(ctx, WithIMDSClient(imds.New(imds.Options{Endpoint: server.URL})))
				require.NoError(t, err)

				return clt
			},
			assertion: require.False,
		},
		{
			name: "response with old id format",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("i-1a2b3c4d"))
			},
			client: func(t *testing.T, ctx context.Context, server *httptest.Server) *InstanceMetadataClient {
				clt, err := NewInstanceMetadataClient(ctx, WithIMDSClient(imds.New(imds.Options{Endpoint: server.URL})))
				require.NoError(t, err)

				return clt
			},
			assertion: require.True,
		},
		{
			name: "response with new id format",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("i-1234567890abcdef0"))
			},
			client: func(t *testing.T, ctx context.Context, server *httptest.Server) *InstanceMetadataClient {
				clt, err := NewInstanceMetadataClient(ctx, WithIMDSClient(imds.New(imds.Options{Endpoint: server.URL})))
				require.NoError(t, err)

				return clt
			},
			assertion: require.True,
		},
		{
			name:    "on ec2",
			handler: func(w http.ResponseWriter, r *http.Request) {},
			client: func(t *testing.T, ctx context.Context, server *httptest.Server) *InstanceMetadataClient {
				if os.Getenv("TELEPORT_TEST_EC2") == "" {
					t.Skip("not on EC2")
				}
				clt, err := NewInstanceMetadataClient(ctx)
				require.NoError(t, err)
				return clt
			},
			assertion: require.True,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(tt.handler)
			t.Cleanup(server.Close)

			ctx := context.Background()
			clt := tt.client(t, ctx, server)
			tt.assertion(t, clt.IsAvailable(ctx))
		})
	}
}

func TestGetInstanceID(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name         string
		stausCode    int
		body         []byte
		expectedID   string
		errAssertion require.ErrorAssertionFunc
	}{
		{
			name:         "with ID",
			stausCode:    http.StatusOK,
			body:         []byte(`i-1234567890abcdef0`),
			expectedID:   "i-1234567890abcdef0",
			errAssertion: require.NoError,
		},
		{
			name:         "with old ID format",
			stausCode:    http.StatusOK,
			body:         []byte(`i-1a2b3c4d`),
			expectedID:   "i-1a2b3c4d",
			errAssertion: require.NoError,
		},
		{
			name:         "with empty ID",
			stausCode:    http.StatusOK,
			body:         []byte{},
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

			client, err := NewInstanceMetadataClient(ctx, WithIMDSClient(imds.New(imds.Options{Endpoint: server.URL})))
			require.NoError(t, err)
			id, err := client.GetID(context.Background())
			tc.errAssertion(t, err)
			require.Equal(t, tc.expectedID, id)
		})
	}
}
