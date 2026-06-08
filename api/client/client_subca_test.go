// Copyright 2026 Gravitational, Inc.
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

package client

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
)

func TestClient_GetCertAuthorityOverride(t *testing.T) {
	t.Parallel()

	client := newClientForSubCATesting(t)

	override := caOverrides[0]
	overrideID := types.CertAuthorityOverrideID{
		ClusterName: override.Metadata.Name,
		CAType:      override.SubKind,
	}

	tests := []struct {
		name    string
		id      types.CertAuthorityOverrideID
		want    *subcav1.CertAuthorityOverride
		wantErr bool // always a NotFoundError
	}{
		{
			name: "ok",
			id:   overrideID,
			want: override,
		},
		{
			name: "default cluster",
			id: types.CertAuthorityOverrideID{
				ClusterName: "", // aka default
				CAType:      overrideID.CAType,
			},
			want: override,
		},
		{
			name: "wrong cluster",
			id: types.CertAuthorityOverrideID{
				ClusterName: "llama", // wrong
				CAType:      overrideID.CAType,
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := client.GetCertAuthorityOverride(t.Context(), test.id)
			if test.wantErr {
				assert.ErrorAs(t, err, new(*trace.NotFoundError), "GetCertAuthorityOverride error mismatch")
				return
			}
			require.NoError(t, err)

			if diff := cmp.Diff(test.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("GetCertAuthorityOverride mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestClient_ListCertAuthorityOverride(t *testing.T) {
	t.Parallel()

	client := newClientForSubCATesting(t)

	t.Run("ok", func(t *testing.T) {
		const pageSize = 0
		const pageToken = ""
		got, nextPageToken, err := client.ListCertAuthorityOverrides(t.Context(), pageSize, pageToken)
		require.NoError(t, err)
		assert.Empty(t, nextPageToken, "ListCertAuthorityOverrides returned unexpected nextPageToken")

		want := caOverrides
		if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
			t.Errorf("ListCertAuthorityOverrides mismatch (-want +got)\n%s", diff)
		}
	})
}

func newClientForSubCATesting(t *testing.T) *Client {
	// Use a bufconn as the gRPC listener.
	const bufSize = 10 // arbitrary
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		assert.NoError(t, lis.Close(), "bufconn.Listener.Close()")
	})

	// Create the gRPC server.
	s := grpc.NewServer(
		// Options below are similar to auth.GRPCServer.
		grpc.ChainStreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
		grpc.ChainUnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)

	// Take a defensive copy of overrides.
	overrides := make([]*subcav1.CertAuthorityOverride, len(caOverrides))
	copy(overrides, caOverrides)

	// Register services.
	subcav1.RegisterSubCAServiceServer(s, &fakeSubCAService{
		overrides: overrides,
	})

	// Start the gRPC server.
	var wg sync.WaitGroup
	wg.Go(func() {
		assert.NoError(t, s.Serve(lis), "grpc.Server.Serve()")
	})
	t.Cleanup(func() {
		s.GracefulStop()
		wg.Wait()
	})

	// Dial to the gRPC server using the bufconn.
	cc, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, cc.Close(), "grpc.ClientConn.Close()")
	})

	// Create the Client. We only set the fields required for SubCA testing.
	return &Client{
		conn: cc,
	}
}

var caOverrides = []*subcav1.CertAuthorityOverride{
	{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(types.DatabaseClientCA),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:     "zarquon",
			Revision: "1774a27d-977e-45a1-ac6a-dd8b7a8e0d8d",
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{},
	},
	{
		Kind:    types.KindCertAuthorityOverride,
		SubKind: string(types.WindowsCA),
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:     "zarquon",
			Revision: "511c8c23-03ea-44b2-81e5-62ddcd4d3445",
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{},
	},
}

// fakeSubCAService is a fake SubCAService implementation that returns
// hard-coded data.
type fakeSubCAService struct {
	subcav1.UnimplementedSubCAServiceServer

	overrides []*subcav1.CertAuthorityOverride
}

func (s *fakeSubCAService) GetCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.GetCertAuthorityOverrideRequest,
) (*subcav1.GetCertAuthorityOverrideResponse, error) {
	// Implement a simplistic Get. Input validation not performed.
	for _, o := range s.overrides {
		if o.SubKind == req.GetCaId().GetCaType() {
			return &subcav1.GetCertAuthorityOverrideResponse{
				CaOverride: o,
			}, nil
		}
	}
	return nil, trace.NotFound("ca override not found")
}

func (s *fakeSubCAService) ListCertAuthorityOverride(
	ctx context.Context,
	req *subcav1.ListCertAuthorityOverrideRequest,
) (*subcav1.ListCertAuthorityOverrideResponse, error) {
	// Input filters are completely disregarded.
	return &subcav1.ListCertAuthorityOverrideResponse{
		CaOverrides: s.overrides,
	}, nil
}
