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

	grpcutils "github.com/gravitational/teleport/lib/utils/grpc"
	footest "github.com/gravitational/teleport/lib/utils/grpc/test"
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
// Both services implement [footest.FooServiceServer]. The server uses
// [fakeServerSvc] as its implementation, whereas the proxy uses [proxyService].
//
// The other tests in this file use the same setup. TestProxyBidiStream tests
// the happy path.
func TestProxyBidiStream(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t)

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	ctx := t.Context()

	client := newProxyServiceClient(t, lis)
	stream, err := client.Session(ctx)
	require.NoError(t, err)

	// Send a message.
	err = stream.Send(&footest.SessionRequest{Input: "hello"})
	require.NoError(t, err)

	// Receive the server's response.
	msg, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, "ack", msg.GetOutput())

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

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	// Add a short timeout so if the proxy hangs (as it did before introducing
	// this regression test), the test doesn't wait for a whole minute to fail.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client := newProxyServiceClient(t, lis)
	stream, err := client.Session(ctx)
	require.NoError(t, err)

	// Empty input triggers the fake server to return an error on its first
	// Recv.
	err = stream.Send(&footest.SessionRequest{})
	require.NoError(t, err)
	_, err = stream.Recv()
	require.ErrorContains(t, err, "empty input")
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

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	ctx := t.Context()

	client := newProxyServiceClient(t, lis)
	stream, err := client.Session(ctx)
	require.NoError(t, err)

	err = stream.Send(&footest.SessionRequest{Input: "hello"})
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

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)
	// Short timeout so a hang surfaces as a test failure rather than waiting
	// out the default go-test timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client := newProxyServiceClient(t, lis)
	stream, err := client.Session(ctx)
	require.NoError(t, err)

	err = stream.Send(&footest.SessionRequest{Input: "hello"})
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
	_ = stream.Send(&footest.SessionRequest{Input: "more"})

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

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient, grpc.MaxRecvMsgSize(64))

	client := newProxyServiceClient(t, lis)
	stream, err := client.Session(t.Context())
	require.NoError(t, err)

	// Send a message larger than the proxy's MaxRecvMsgSize.
	err = stream.Send(&footest.SessionRequest{Input: strings.Repeat("x", 256)})
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

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	newProxyService(t, lis, fakeServerSvcClient)

	client := newProxyServiceClient(t, lis)
	stream, err := client.Session(t.Context())
	require.NoError(t, err)

	// The proxy accepts this message (its server-side MaxRecvMsgSize is the
	// default 4MB), then tries to forward it to the fake server whose upstream
	// connection caps sends at 64 bytes, triggering a local ResourceExhausted on
	// the proxy's server.Send.
	err = stream.Send(&footest.SessionRequest{Input: strings.Repeat("x", 256)})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorContains(t, err, "larger than max")
}

func newFakeServerSvc(t *testing.T, clientOpts ...grpc.DialOption) (*fakeServerSvc, footest.FooServiceClient) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	service := &fakeServerSvc{}
	footest.RegisterFooServiceServer(server, service)
	go func() {
		err := server.Serve(lis)
		require.NoError(t, err)
	}()
	t.Cleanup(server.GracefulStop)

	opts := append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, clientOpts...)
	client, err := grpc.NewClient(lis.Addr().String(), opts...)
	require.NoError(t, err)

	return service, footest.NewFooServiceClient(client)
}

type fakeServerSvc struct {
	footest.UnimplementedFooServiceServer

	// postClientEOFErr, if non-nil, is returned by Session after it gets EOF
	// from the client, modeling the server producing a terminal error during
	// post-upload processing (after the client has already half-closed).
	postClientEOFErr error

	// returnAfterFirstResponse, if true, makes Session return nil right after
	// sending its first response, without waiting for any further client input
	// or for the client to half-close. It models a server that ends the
	// stream early while the client is still mid-conversation.
	returnAfterFirstResponse bool
}

func (f *fakeServerSvc) Session(stream footest.FooService_SessionServer) error {
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
		if len(req.GetInput()) == 0 {
			return trace.BadParameter("empty input")
		}
		if err := stream.Send(&footest.SessionResponse{Output: "ack"}); err != nil {
			return trace.Wrap(err)
		}
		if f.returnAfterFirstResponse {
			return nil
		}
	}
}

// newProxyService creates a gRPC server under lis and registers in it a gRPC
// service that proxies Session calls to the server using
// [grpcutils.ProxyBidiStream]. Callers may supply extra grpc.ServerOptions to
// drive specific fault scenarios.
func newProxyService(t *testing.T, lis net.Listener, client footest.FooServiceClient, opts ...grpc.ServerOption) {
	t.Helper()

	s := grpc.NewServer(opts...)
	t.Cleanup(s.GracefulStop)

	proxySvc := &proxyService{
		serverSvcClient: client,
	}

	footest.RegisterFooServiceServer(s, proxySvc)

	go func() {
		err := s.Serve(lis)
		require.NoError(t, err)
	}()
}

type proxyService struct {
	footest.UnimplementedFooServiceServer

	serverSvcClient footest.FooServiceClient
}

// Session is the gRPC handler in the proxy.
// client goes from a client to the proxy. From that point of view, the proxy is
// a server for the client.
// server from getServer goes from the proxy to the server. From that point of
// view, the proxy is a client of the server.
func (p *proxyService) Session(client footest.FooService_SessionServer) error {
	getServer := func(ctx context.Context) (footest.FooService_SessionClient, error) {
		return p.serverSvcClient.Session(ctx)
	}
	err := grpcutils.ProxyBidiStream(logtest.NewLogger(), client, getServer)
	return trace.Wrap(err)
}

func newProxyServiceClient(t *testing.T, lis net.Listener) footest.FooServiceClient {
	t.Helper()
	clientConn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	return footest.NewFooServiceClient(clientConn)
}
