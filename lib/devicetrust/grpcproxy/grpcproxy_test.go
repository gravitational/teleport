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
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"

	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	grpcinterceptors "github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/devicetrust"
)

// testClientAddr stands in for the address of the device calling the proxy.
// bufconn reports a placeholder peer address, so the proxy's inbound server has
// a real one injected instead.
var testClientAddr = &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 4242}

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

	t.Run("forwards the address the calling device connected from", func(t *testing.T) {
		fake := &fakeAuthService{}
		client := newProxyClient(t, fake)

		_, err := client.CreatePairedDeviceEnrollToken(t.Context(),
			publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
				EnrollPairingToken: "pairing-token",
			}.Build())
		require.NoError(t, err)

		got, err := devicetrust.ForwardedClientSrcAddrFromContext(
			metadata.NewIncomingContext(t.Context(), fake.getLastMD()))
		require.NoError(t, err)
		assert.Equal(t, testClientAddr, got)
	})

	// The device picking its own source address would defeat IP pinning, so what
	// it sends must not reach the auth service.
	t.Run("ignores a source address supplied by the calling device", func(t *testing.T) {
		fake := &fakeAuthService{}
		client := newProxyClient(t, fake)

		spoofed := devicetrust.WithForwardedClientSrcAddr(t.Context(),
			&net.TCPAddr{IP: net.ParseIP("198.51.100.7"), Port: 1})
		_, err := client.CreatePairedDeviceEnrollToken(spoofed,
			publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
				EnrollPairingToken: "pairing-token",
			}.Build())
		require.NoError(t, err)

		// A single value, so the spoofed address neither won nor turned the
		// forwarded address ambiguous.
		got, err := devicetrust.ForwardedClientSrcAddrFromContext(
			metadata.NewIncomingContext(t.Context(), fake.getLastMD()))
		require.NoError(t, err)
		assert.Equal(t, testClientAddr, got)
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
	lastMD  metadata.MD
}

func (f *fakeAuthService) CreatePairedDeviceEnrollToken(ctx context.Context, req *publicdevicepb.CreatePairedDeviceEnrollTokenRequest) (*publicdevicepb.CreatePairedDeviceEnrollTokenResponse, error) {
	f.mu.Lock()
	f.lastReq = req
	f.lastMD, _ = metadata.FromIncomingContext(ctx)
	f.mu.Unlock()
	return f.resp, f.err
}

func (f *fakeAuthService) getLastReq() *publicdevicepb.CreatePairedDeviceEnrollTokenRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastReq
}

func (f *fakeAuthService) getLastMD() metadata.MD {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastMD
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

	return newGRPCClient(t, proxy, withPeerAddr(testClientAddr))
}

// withPeerAddr reports addr as the peer address to the handler, standing in for
// the address a device would connect from over a real transport.
func withPeerAddr(addr net.Addr) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(peer.NewContext(ctx, &peer.Peer{Addr: addr}), req)
	}
}

// newGRPCClient serves svc on an in-memory bufconn listener and returns a client
// dialed over it. bufconn keeps the transport off the real network so tests can
// run inside a synctest bubble.
func newGRPCClient(t *testing.T, svc publicdevicepb.DeviceTrustServiceServer, extra ...grpc.UnaryServerInterceptor) publicdevicepb.DeviceTrustServiceClient {
	t.Helper()

	lis := bufconn.Listen(1024)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			append([]grpc.UnaryServerInterceptor{grpcinterceptors.GRPCServerUnaryErrorInterceptor}, extra...)...),
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
