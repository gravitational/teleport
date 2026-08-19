// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package grpcproxy

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

	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	grpcinterceptors "github.com/gravitational/teleport/api/utils/grpc/interceptors"
)

func TestService_CreatePairedDeviceEnrollToken(t *testing.T) {
	t.Parallel()

	t.Run("forwards the request and returns the response", func(t *testing.T) {
		fake := &fakeAuthService{
			resp: publicdevicepb.CreatePairedDeviceEnrollTokenResponse_builder{
				DeviceEnrollToken: devicepb.DeviceEnrollToken_builder{Token: "enroll-token"}.Build(),
			}.Build(),
		}
		client := newProxyClient(t, fake)

		req := publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
			EnrollPairingToken: "pairing-token",
			DeviceData: devicepb.DeviceCollectedData_builder{
				OsType:       devicepb.OSType_OS_TYPE_IOS,
				SerialNumber: "CXXXXXXXXX01",
			}.Build(),
		}.Build()

		resp, err := client.CreatePairedDeviceEnrollToken(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, "enroll-token", resp.GetDeviceEnrollToken().GetToken())

		assert.Empty(t, cmp.Diff(req, fake.getLastReq(), protocmp.Transform()))
	})

	t.Run("propagates errors from the auth service", func(t *testing.T) {
		fake := &fakeAuthService{err: trace.AccessDenied("denied")}
		client := newProxyClient(t, fake)

		_, err := client.CreatePairedDeviceEnrollToken(t.Context(),
			publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
				EnrollPairingToken: "pairing-token",
			}.Build())
		assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
	})
}

// fakeAuthService stands in for the auth-side public Device Trust service.
type fakeAuthService struct {
	publicdevicepb.UnimplementedDeviceTrustServiceServer

	resp *publicdevicepb.CreatePairedDeviceEnrollTokenResponse
	err  error

	mu      sync.Mutex
	lastReq *publicdevicepb.CreatePairedDeviceEnrollTokenRequest
}

func (f *fakeAuthService) CreatePairedDeviceEnrollToken(_ context.Context, req *publicdevicepb.CreatePairedDeviceEnrollTokenRequest) (*publicdevicepb.CreatePairedDeviceEnrollTokenResponse, error) {
	f.mu.Lock()
	f.lastReq = req
	f.mu.Unlock()
	return f.resp, f.err
}

func (f *fakeAuthService) getLastReq() *publicdevicepb.CreatePairedDeviceEnrollTokenRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastReq
}

// fakeAuthClient adapts a public Device Trust client to the [AuthClient] interface.
type fakeAuthClient struct {
	client publicdevicepb.DeviceTrustServiceClient
}

func (c fakeAuthClient) PublicDevicesClient() publicdevicepb.DeviceTrustServiceClient {
	return c.client
}

// newProxyClient stands up the fake auth service, the proxy in front of it, and
// returns a client connected to the proxy.
func newProxyClient(t *testing.T, authSvc publicdevicepb.DeviceTrustServiceServer) publicdevicepb.DeviceTrustServiceClient {
	t.Helper()

	authClient := fakeAuthClient{client: newGRPCClient(t, authSvc)}

	proxy, err := New(ServiceConfig{AuthClient: authClient})
	require.NoError(t, err)

	return newGRPCClient(t, proxy)
}

// newGRPCClient serves svc on an in-memory bufconn listener and returns a client
// dialed over it. bufconn keeps the transport off the real network so tests can
// run inside a synctest bubble.
func newGRPCClient(t *testing.T, svc publicdevicepb.DeviceTrustServiceServer) publicdevicepb.DeviceTrustServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcinterceptors.GRPCServerUnaryErrorInterceptor),
	)
	publicdevicepb.RegisterDeviceTrustServiceServer(server, svc)
	go func() {
		assert.NoError(t, server.Serve(lis))
	}()
	t.Cleanup(func() {
		server.Stop()
		assert.NoError(t, lis.Close())
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcinterceptors.GRPCClientUnaryErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return publicdevicepb.NewDeviceTrustServiceClient(conn)
}
