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
	"io"
	"net"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"

	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	grpcinterceptors "github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/grpcproxy/clientaddr"
)

// proxyClientAddr is the address the proxy sees the test client at and forwards
// to the Auth Service.
var proxyClientAddr = &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 51234}

// TestService_forwardsClientAddr checks that every RPC reaches the Auth Service
// with the address of the connection in the client address header and ignores
// any address the client puts in the header itself.
func TestService_forwardsClientAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rpc  func(t *testing.T, ctx context.Context, client publicdevicepb.DeviceTrustServiceClient)
	}{
		{
			name: "CreatePairedDeviceEnrollToken",
			rpc: func(t *testing.T, ctx context.Context, client publicdevicepb.DeviceTrustServiceClient) {
				_, err := client.CreatePairedDeviceEnrollToken(ctx,
					publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
						EnrollPairingToken: "pairing-token",
					}.Build())
				require.NoError(t, err)
			},
		},
		{
			name: "EnrollDevice",
			rpc: func(t *testing.T, ctx context.Context, client publicdevicepb.DeviceTrustServiceClient) {
				stream, err := client.EnrollDevice(ctx)
				require.NoError(t, err)
				// The proxy opens the Auth Service stream only once the first message
				// arrives, so the reply is what proves the hop happened.
				require.NoError(t, stream.Send(
					publicdevicepb.EnrollDeviceRequest_builder{
						Init: publicdevicepb.EnrollDeviceInit_builder{}.Build(),
					}.Build()))
				_, err = stream.Recv()
				require.NoError(t, err)

				// Finish the ceremony so that the stream ends with fake's return rather
				// than with the test context.
				require.NoError(t, stream.Send(
					publicdevicepb.EnrollDeviceRequest_builder{
						IosChallengeResponse: publicdevicepb.IOSEnrollChallengeResponse_builder{}.Build(),
					}.Build()))
				_, err = stream.Recv()
				require.NoError(t, err)
				_, err = stream.Recv()
				require.ErrorIs(t, err, io.EOF)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeAuthService{
				createTokenResp: publicdevicepb.CreatePairedDeviceEnrollTokenResponse_builder{}.Build(),
			}
			client := newProxyClient(t, fake)

			// The header value set by the client must not survive the hop.
			const spoofedAddr = "198.51.100.1:1"
			ctx := metadata.AppendToOutgoingContext(t.Context(), clientaddr.Header, spoofedAddr)
			test.rpc(t, ctx, client)
			assert.Equal(t, []string{proxyClientAddr.String()}, fake.getLastClientAddrHeader(),
				"expected clientaddr.Header on the Auth Service side to be the peer address that the Proxy Service saw")
		})
	}
}

// TestService_rejectsANonTCPPeer checks that a proxy whose connections carry no
// TCP address fails the RPC itself instead of forwarding an address the Auth
// Service cannot parse.
func TestService_rejectsANonTCPPeer(t *testing.T) {
	t.Parallel()

	fake := &fakeAuthService{}
	authClient := fakeAuthClient{client: newGRPCClient(t, fake, nil /* remote */)}
	proxy, err := New(ServiceConfig{AuthClient: authClient})
	require.NoError(t, err)
	// When called with nil remote, newGRPCClient returns a client that reports
	// "bufconn" as its address.
	client := newGRPCClient(t, proxy, nil /* remote */)

	_, err = client.CreatePairedDeviceEnrollToken(t.Context(),
		publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{}.Build())
	assert.ErrorContains(t, err, "is not a TCP address")
	assert.Nil(t, fake.getLastReq(), "the Auth Service must not be called")
}

func TestService_CreatePairedDeviceEnrollToken(t *testing.T) {
	t.Parallel()

	t.Run("forwards the request and returns the response", func(t *testing.T) {
		fake := &fakeAuthService{
			createTokenResp: publicdevicepb.CreatePairedDeviceEnrollTokenResponse_builder{
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
		fake := &fakeAuthService{createTokenErr: trace.AccessDenied("denied")}
		client := newProxyClient(t, fake)

		_, err := client.CreatePairedDeviceEnrollToken(t.Context(),
			publicdevicepb.CreatePairedDeviceEnrollTokenRequest_builder{
				EnrollPairingToken: "pairing-token",
			}.Build())
		assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
	})
}

func TestService_EnrollDevice(t *testing.T) {
	t.Parallel()

	t.Run("forwards the stream in both directions", func(t *testing.T) {
		fake := &fakeAuthService{}
		client := newProxyClient(t, fake)

		stream, err := client.EnrollDevice(t.Context())
		require.NoError(t, err)

		init := publicdevicepb.EnrollDeviceRequest_builder{
			Init: publicdevicepb.EnrollDeviceInit_builder{
				Token:        "enroll-token",
				CredentialId: "credential-id",
				DeviceData: devicepb.DeviceCollectedData_builder{
					OsType:       devicepb.OSType_OS_TYPE_IOS,
					SerialNumber: "CXXXXXXXXX01",
				}.Build(),
				Ios: publicdevicepb.IOSEnrollPayload_builder{
					PublicKeyDer: []byte("public-key"),
				}.Build(),
			}.Build(),
		}.Build()
		require.NoError(t, stream.Send(init))

		resp, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, []byte("challenge"), resp.GetIosChallenge().GetChallenge())

		chalResp := publicdevicepb.EnrollDeviceRequest_builder{
			IosChallengeResponse: publicdevicepb.IOSEnrollChallengeResponse_builder{
				Signature: []byte("signed-challenge"),
			}.Build(),
		}.Build()
		require.NoError(t, stream.Send(chalResp))

		success, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, "device-id", success.GetSuccess().GetDevice().GetId())

		_, err = stream.Recv()
		require.ErrorIs(t, err, io.EOF)

		// The auth service saw exactly the messages the client sent.
		assert.Empty(t, cmp.Diff(
			[]*publicdevicepb.EnrollDeviceRequest{init, chalResp},
			fake.getEnrollReqs(),
			protocmp.Transform(),
		))
	})

	t.Run("propagates errors from the auth service", func(t *testing.T) {
		fake := &fakeAuthService{enrollErr: trace.AccessDenied("denied")}
		client := newProxyClient(t, fake)

		stream, err := client.EnrollDevice(t.Context())
		require.NoError(t, err)
		// The proxy opens the Auth Service stream only once the init message has
		// arrived, so the error cannot reach the client before it is sent.
		require.NoError(t, stream.Send(publicdevicepb.EnrollDeviceRequest_builder{
			Init: publicdevicepb.EnrollDeviceInit_builder{Token: "enroll-token"}.Build(),
		}.Build()))
		_, err = stream.Recv()
		assert.ErrorAs(t, err, new(*trace.AccessDeniedError))
	})

	t.Run("ends a stream whose client never sends init", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			fake := &fakeAuthService{}
			client := newProxyClient(t, fake)

			start := time.Now()
			stream, err := client.EnrollDevice(t.Context())
			require.NoError(t, err)

			_, err = recvWithTimeout(t, stream, devicetrust.PublicEnrollDeviceProxyTimeout+time.Minute)
			assert.ErrorIs(t, err, errEnrollDeviceFirstMessageTimeout)
			assert.Equal(t, devicetrust.PublicEnrollDeviceFirstMessageTimeout, time.Since(start))
		})
	})

	t.Run("ends a stream the auth service never ends", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			fake := &fakeAuthService{}
			client := newProxyClient(t, fake)

			start := time.Now()
			stream, err := client.EnrollDevice(t.Context())
			require.NoError(t, err)
			require.NoError(t, stream.Send(publicdevicepb.EnrollDeviceRequest_builder{
				Init: publicdevicepb.EnrollDeviceInit_builder{
					Token: "enroll-token",
					DeviceData: devicepb.DeviceCollectedData_builder{
						OsType:       devicepb.OSType_OS_TYPE_IOS,
						SerialNumber: "CXXXXXXXXX01",
					}.Build(),
				}.Build(),
			}.Build()))
			resp, err := stream.Recv()
			require.NoError(t, err)
			require.NotNil(t, resp.GetIosChallenge())

			// The client stalls and the fake Auth Service has no timeout of its
			// own, so the proxy's stream timeout is what ends the RPC.
			_, err = recvWithTimeout(t, stream, devicetrust.PublicEnrollDeviceProxyTimeout+time.Minute)
			assert.ErrorIs(t, err, errEnrollDeviceProxyTimeout)
			assert.Equal(t, devicetrust.PublicEnrollDeviceProxyTimeout, time.Since(start))
		})
	})
}

// fakeAuthService stands in for the auth-side public Device Trust service.
type fakeAuthService struct {
	publicdevicepb.UnimplementedDeviceTrustServiceServer

	createTokenResp *publicdevicepb.CreatePairedDeviceEnrollTokenResponse
	createTokenErr  error
	// enrollErr fails EnrollDevice before any message is read.
	enrollErr error

	mu                   sync.Mutex
	lastReq              *publicdevicepb.CreatePairedDeviceEnrollTokenRequest
	enrollReqs           []*publicdevicepb.EnrollDeviceRequest
	lastClientAddrHeader []string
}

func (f *fakeAuthService) CreatePairedDeviceEnrollToken(ctx context.Context, req *publicdevicepb.CreatePairedDeviceEnrollTokenRequest) (*publicdevicepb.CreatePairedDeviceEnrollTokenResponse, error) {
	f.recordClientAddr(ctx)
	f.mu.Lock()
	f.lastReq = req
	f.mu.Unlock()
	return f.createTokenResp, f.createTokenErr
}

// EnrollDevice runs a scripted happy-path ceremony: init in, challenge out,
// challenge response in, success out.
func (f *fakeAuthService) EnrollDevice(stream publicdevicepb.DeviceTrustService_EnrollDeviceServer) error {
	f.recordClientAddr(stream.Context())

	if f.enrollErr != nil {
		return f.enrollErr
	}

	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	f.recordEnrollReq(req)
	if req.GetInit() == nil {
		return trace.BadParameter("expected EnrollDeviceInit")
	}

	if err := stream.Send(publicdevicepb.EnrollDeviceResponse_builder{
		IosChallenge: publicdevicepb.IOSEnrollChallenge_builder{
			Challenge: []byte("challenge"),
		}.Build(),
	}.Build()); err != nil {
		return trace.Wrap(err)
	}

	req, err = stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	f.recordEnrollReq(req)
	if req.GetIosChallengeResponse() == nil {
		return trace.BadParameter("expected IOSEnrollChallengeResponse")
	}

	return trace.Wrap(stream.Send(publicdevicepb.EnrollDeviceResponse_builder{
		Success: publicdevicepb.EnrollDeviceSuccess_builder{
			Device: devicepb.Device_builder{Id: "device-id"}.Build(),
		}.Build(),
	}.Build()))
}

func (f *fakeAuthService) recordEnrollReq(req *publicdevicepb.EnrollDeviceRequest) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enrollReqs = append(f.enrollReqs, req)
}

func (f *fakeAuthService) getEnrollReqs() []*publicdevicepb.EnrollDeviceRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.enrollReqs)
}

func (f *fakeAuthService) getLastReq() *publicdevicepb.CreatePairedDeviceEnrollTokenRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastReq
}

func (f *fakeAuthService) recordClientAddr(ctx context.Context) {
	md, _ := metadata.FromIncomingContext(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastClientAddrHeader = md.Get(clientaddr.Header)
}

func (f *fakeAuthService) getLastClientAddrHeader() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.lastClientAddrHeader)
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

	authClient := fakeAuthClient{client: newGRPCClient(t, authSvc, nil /* remote */)}

	proxy, err := New(ServiceConfig{AuthClient: authClient})
	require.NoError(t, err)

	return newGRPCClient(t, proxy,
		// The proxy forwards the address it sees on the connection, so its listener
		// reports the test client at proxyClientAddr.
		proxyClientAddr)
}

// newGRPCClient serves svc on an in-memory bufconn listener and returns a client
// dialed over it. bufconn keeps the transport off the real network so tests can
// run inside a synctest bubble.
//
// A non-nil remote makes the server see every connection as coming from that
// address.
func newGRPCClient(t *testing.T, svc publicdevicepb.DeviceTrustServiceServer, remote net.Addr) publicdevicepb.DeviceTrustServiceClient {
	t.Helper()

	buf := bufconn.Listen(1024)
	var lis net.Listener = buf
	if remote != nil {
		lis = &remoteAddrListener{Listener: buf, remote: remote}
	}
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(grpcinterceptors.GRPCServerUnaryErrorInterceptor),
		grpc.ChainStreamInterceptor(grpcinterceptors.GRPCServerStreamErrorInterceptor),
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
			return buf.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcinterceptors.GRPCClientUnaryErrorInterceptor),
		grpc.WithChainStreamInterceptor(grpcinterceptors.GRPCClientStreamErrorInterceptor),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return publicdevicepb.NewDeviceTrustServiceClient(conn)
}

// remoteAddrListener makes accepted connections report a fixed remote address,
// which the gRPC server exposes as the peer of every RPC on them.
type remoteAddrListener struct {
	net.Listener
	remote net.Addr
}

func (l *remoteAddrListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &remoteAddrConn{Conn: conn, remote: l.remote}, nil
}

type remoteAddrConn struct {
	net.Conn
	remote net.Addr
}

func (c *remoteAddrConn) RemoteAddr() net.Addr { return c.remote }

// recvWithTimeout receives from stream in a goroutine and fails the test if
// nothing arrives within timeout, so that a proxy that never ends the stream
// fails the test instead of hanging it. Meant for synctest bubbles, where the
// timeout costs no wall time.
func recvWithTimeout(t *testing.T, stream publicdevicepb.DeviceTrustService_EnrollDeviceClient, timeout time.Duration) (*publicdevicepb.EnrollDeviceResponse, error) {
	t.Helper()
	type result struct {
		resp *publicdevicepb.EnrollDeviceResponse
		err  error
	}
	resCh := make(chan result, 1)
	go func() {
		resp, err := stream.Recv()
		resCh <- result{resp: resp, err: err}
	}()
	select {
	case res := <-resCh:
		return res.resp, res.err
	case <-time.After(timeout):
		t.Fatal("timed out waiting for the proxy to end the stream")
		return nil, nil
	}
}
