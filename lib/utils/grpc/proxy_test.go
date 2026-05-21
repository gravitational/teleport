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

package grpc_test

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	teletermv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	grpcutils "github.com/gravitational/teleport/lib/utils/grpc"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	m.Run()
}

// TestProxyBidiStream creates two gRPC services: one acting as a server and one
// as a proxy. The proxy uses [grpcutils.ProxyBidiStream] to proxy messages from
// the client and the server.
//
// Both services implement [teletermv1.TerminalServiceServer]. The server uses
// [fakeServerSvc] as its implementation, whereas the proxy uses [proxyService].
//
// The other tests in this file use the same setup. TestProxyBidiStream tests
// the happy path.
func TestProxyBidiStream(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t)

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient)
	ctx := t.Context()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(ctx)
	require.NoError(t, err)

	// Send a message.
	err = stream.Send(&teletermv1.ConnectToDesktopRequest{Data: []byte("hello")})
	require.NoError(t, err)

	// Receive the server's response.
	msg, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, []byte("ack"), msg.GetData())

	// Half-close and wait for the server to terminate the stream cleanly.
	err = stream.CloseSend()
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

// TestProxyBidiStream_HandlesServerReturningErr covers the case where the
// server errors on its first Recv. Before this regression test the proxy
// handler could deadlock instead of propagating the error.
func TestProxyBidiStream_HandlesServerReturningErr(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t)

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient)
	// Add a short timeout so if the proxy hangs (as it did before introducing
	// this regression test), the test doesn't wait for a whole minute to fail.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(ctx)
	require.NoError(t, err)

	// Empty input triggers the fake server to return an error on its first
	// Recv.
	err = stream.Send(&teletermv1.ConnectToDesktopRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.ErrorContains(t, err, "empty data")
}

// TestProxyBidiStream_PropagatesServerErrorAfterClientEOF asserts that a
// terminal error produced by the server *after* the client has half-closed
// (CloseSend) is still propagated through the proxy to the client.
//
// This exercises the handler path where forwardClientToServer returns first
// (normal CloseSend) and forwardServerToClient is the one that ends up carrying
// server's terminal status. A handler that treats forwardClientToServer as
// authoritative will finish and the client will see io.EOF instead of the real
// error, masking real server failures.
func TestProxyBidiStream_PropagatesServerErrorAfterClientEOF(t *testing.T) {
	t.Parallel()
	service, fakeServerSvcClient := newFakeServerSvc(t)
	service.postClientEOFErr = trace.AccessDenied("post-EOF validation failed")

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient)
	ctx := t.Context()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(ctx)
	require.NoError(t, err)

	err = stream.Send(&teletermv1.ConnectToDesktopRequest{Data: []byte("hello")})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)

	err = stream.CloseSend()
	require.NoError(t, err)

	// The client must see the server error, not a clean io.EOF.
	_, recvErr := stream.Recv()
	require.NotErrorIs(t, recvErr, io.EOF, "client saw clean EOF; server error was swallowed")
	require.ErrorContains(t, recvErr, "post-EOF validation failed")
}

// TestProxyBidiStream_ReturnsEOFWhenServerReturnsEarly asserts that when the
// server ends its handler cleanly (nil) *before* the client has half-closed,
// the proxy propagates that as io.EOF to the client rather than hanging or
// reshaping the server's nil into an error.
func TestProxyBidiStream_ReturnsEOFWhenServerReturnsEarly(t *testing.T) {
	t.Parallel()
	service, fakeServerSvcClient := newFakeServerSvc(t)
	service.returnAfterFirstResponse = true

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient)
	// Short timeout so a hang surfaces as a test failure rather than waiting
	// out the default go-test timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(ctx)
	require.NoError(t, err)

	err = stream.Send(&teletermv1.ConnectToDesktopRequest{Data: []byte("hello")})
	require.NoError(t, err)

	// Drain the first response the server sent before returning.
	_, err = stream.Recv()
	require.NoError(t, err)

	// Client sends the next message it would naturally send, not knowing the
	// server has already returned. Under the bug this reshapes the server's
	// clean completion into an error via a failed upstream Send; under the
	// fix the handler has already returned and the Send is irrelevant.
	//
	// At this point, the Send returns either nil if the trailer wasn't propagated
	// to the client yet or io.EOF if it was, so we skip asserting on err here.
	_ = stream.Send(&teletermv1.ConnectToDesktopRequest{Data: []byte("more")})

	// Server has returned nil. The client must see clean io.EOF, not a
	// proxy-reshaped error and not a hang (which would surface as a
	// DeadlineExceeded from ctx).
	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

// TestProxyBidiStream_SurfacesClientRecvError asserts that when client.Recv
// on the proxy side fails with a non-EOF error, the proxy returns that
// specific error instead of the Canceled artifact produced by a naive design
// that cancels the server stream and then returns whatever server.Recv yields.
//
// To trigger this, we set a tiny MaxRecvMsgSize on the proxy's gRPC server and
// have the client send a message exceeding it. The proxy's client.Recv returns
// a ResourceExhausted status error; the handler must propagate it.
func TestProxyBidiStream_SurfacesClientRecvError(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t)

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient, grpc.MaxRecvMsgSize(64))

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(t.Context())
	require.NoError(t, err)

	// Send a message larger than the proxy's MaxRecvMsgSize.
	err = stream.Send(&teletermv1.ConnectToDesktopRequest{Data: []byte(strings.Repeat("x", 256))})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorContains(t, err, "larger than max")
}

// TestProxyBidiStream_SurfacesServerSendError asserts that when server.Send on
// the proxy side fails with a non-EOF error (locally generated, e.g. the
// outbound message exceeds MaxCallSendMsgSize on the proxy's upstream
// connection), the proxy returns that specific error rather than masking it as
// Canceled.
//
// To trigger this, we dial the fake server with a tiny MaxCallSendMsgSize so
// that the proxy's server.Send fails whenever the client-forwarded message is
// larger than that limit.
func TestProxyBidiStream_SurfacesServerSendError(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t,
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(64)),
	)

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient)

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(t.Context())
	require.NoError(t, err)

	// The proxy accepts this message (its server-side MaxRecvMsgSize is the
	// default 4MB), then tries to forward it to the fake server whose upstream
	// connection caps sends at 64 bytes, triggering a local ResourceExhausted on
	// the proxy's server.Send.
	err = stream.Send(&teletermv1.ConnectToDesktopRequest{Data: []byte(strings.Repeat("x", 256))})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorContains(t, err, "larger than max")
}

// TestProxyBidiStream_ForwardsMetadata asserts that the proxy passes the
// client's incoming metadata upstream to the server and forwards the server's
// response headers and trailers back to the client.
func TestProxyBidiStream_ForwardsMetadata(t *testing.T) {
	t.Parallel()
	service, fakeServerSvcClient := newFakeServerSvc(t)
	service.echoMetadata = true

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient)

	// Short timeout so a hang (e.g. Header() never unblocks due to a regression)
	// surfaces as a test failure rather than waiting out the default go-test timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	// Attach metadata to the outgoing call so the proxy can forward it upstream.
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-test-key", "test-value"))

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(ctx)
	require.NoError(t, err)

	err = stream.Send(&teletermv1.ConnectToDesktopRequest{Data: []byte("hello")})
	require.NoError(t, err)

	// Receive the server's first response; by this point the server has already
	// called SendHeader so headers are available on the client stream.
	_, err = stream.Recv()
	require.NoError(t, err)

	headers, err := stream.Header()
	require.NoError(t, err)
	require.Equal(t, []string{"test-value"}, headers.Get("x-test-key"),
		"response headers not forwarded through proxy")

	err = stream.CloseSend()
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)

	trailers := stream.Trailer()
	require.Equal(t, []string{"test-value"}, trailers.Get("x-test-key"),
		"response trailers not forwarded through proxy")
}

func newFakeServerSvc(t *testing.T, clientOpts ...grpc.DialOption) (*fakeServerSvc, teletermv1.TerminalServiceClient) {
	lis := bufconn.Listen(1024)
	server := grpc.NewServer()
	service := &fakeServerSvc{}
	teletermv1.RegisterTerminalServiceServer(server, service)
	go func() {
		err := server.Serve(lis)
		require.NoError(t, err)
	}()
	t.Cleanup(server.GracefulStop)

	opts := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	}, clientOpts...)
	client, err := grpc.NewClient("passthrough:///bufconn", opts...)
	require.NoError(t, err)

	return service, teletermv1.NewTerminalServiceClient(client)
}

type fakeServerSvc struct {
	teletermv1.UnimplementedTerminalServiceServer

	// postClientEOFErr, if non-nil, is returned by ConnectToDesktop after it gets
	// EOF from the client, modeling the server producing a terminal error during
	// post-upload processing (after the client has already half-closed).
	postClientEOFErr error

	// returnAfterFirstResponse, if true, makes ConnectToDesktop return nil right
	// after sending its first response, without waiting for any further client
	// input or for the client to half-close. It models a server that ends the
	// stream early while the client is still mid-conversation.
	returnAfterFirstResponse bool

	// echoMetadata, if true, makes ConnectToDesktop read the incoming metadata
	// from the stream context and echo it back as both response headers and
	// trailers. Used to verify the proxy forwards metadata in both directions.
	echoMetadata bool
}

// ConnectToDesktop does NOT implement the semantics of the real
// ConnectToDesktop RPC. The RPC is borrowed purely for its bidi-stream shape so
// the tests in this file can exercise ProxyBidiStream without introducing a
// custom test-only proto.
//
// Contract used by the tests:
//   - Every request must populate data with a non-empty payload. An empty data
//     triggers a trace.BadParameter return.
//   - Every response carries data = "ack".
func (f *fakeServerSvc) ConnectToDesktop(stream teletermv1.TerminalService_ConnectToDesktopServer) error {
	if f.echoMetadata {
		if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
			stream.SetTrailer(md)
			if err := stream.SendHeader(md); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if f.postClientEOFErr != nil {
				return f.postClientEOFErr
			}
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
		if len(req.GetData()) == 0 {
			return trace.BadParameter("empty data")
		}
		if err := stream.Send(&teletermv1.ConnectToDesktopResponse{Data: []byte("ack")}); err != nil {
			return trace.Wrap(err)
		}
		if f.returnAfterFirstResponse {
			return nil
		}
	}
}

// newProxyService creates a gRPC server under lis and registers in it a gRPC
// service that proxies ConnectToDesktop calls to the server using
// [grpcutils.ProxyBidiStream]. Callers may supply extra grpc.ServerOptions to
// drive specific fault scenarios.
func newProxyService(t *testing.T, lis net.Listener, client teletermv1.TerminalServiceClient, opts ...grpc.ServerOption) {
	t.Helper()

	s := grpc.NewServer(opts...)
	t.Cleanup(s.GracefulStop)

	proxySvc := &proxyService{
		serverSvcClient: client,
	}

	teletermv1.RegisterTerminalServiceServer(s, proxySvc)

	go func() {
		err := s.Serve(lis)
		require.NoError(t, err)
	}()
}

type proxyService struct {
	teletermv1.UnimplementedTerminalServiceServer

	serverSvcClient teletermv1.TerminalServiceClient
}

// ConnectToDesktop forwards every client request (whose data is non-empty by
// contract) to the upstream server and every response (carrying data = "ack" by
// contract) back to the client, using ProxyBidiStream.
//
// ConnectToDesktop does NOT implement the semantics of the real
// ConnectToDesktop RPC. See the godoc for [fakeServerSvc.ConnectToDesktop].
//
// client goes from a client to the proxy. From that point of view, the proxy
// is a server for the client.
// server from getServer goes from the proxy to the server. From that point of
// view, the proxy is a client of the server.
func (p *proxyService) ConnectToDesktop(client teletermv1.TerminalService_ConnectToDesktopServer) error {
	getServer := func(ctx context.Context) (teletermv1.TerminalService_ConnectToDesktopClient, error) {
		return p.serverSvcClient.ConnectToDesktop(ctx)
	}
	err := grpcutils.ProxyBidiStream(logtest.NewLogger(), client, getServer)
	return trace.Wrap(err)
}

func newProxyServiceClient(t *testing.T, lis *bufconn.Listener) teletermv1.TerminalServiceClient {
	t.Helper()
	clientConn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(
			func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			},
		),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return teletermv1.NewTerminalServiceClient(clientConn)
}
