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

package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsEC2NodeID(t *testing.T) {
	// EC2 Node IDs are {AWS account ID}-{EC2 resource ID} eg:
	//   123456789012-i-1234567890abcdef0
	// AWS account ID is always a 12 digit number, see
	//   https://docs.aws.amazon.com/general/latest/gr/acct-identifiers.html
	// EC2 resource ID is i-{8 or 17 hex digits}, see
	//   https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/resource-ids.html
	testCases := []struct {
		name     string
		id       string
		expected bool
	}{
		{
			name:     "8 digit",
			id:       "123456789012-i-12345678",
			expected: true,
		},
		{
			name:     "17 digit",
			id:       "123456789012-i-1234567890abcdef0",
			expected: true,
		},
		{
			name:     "foo",
			id:       "foo",
			expected: false,
		},
		{
			name:     "uuid",
			id:       uuid.NewString(),
			expected: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, IsEC2NodeID(tc.id), tc.expected)
		})
	}
}

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
