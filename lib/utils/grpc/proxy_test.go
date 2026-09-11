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
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
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
	err = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build())
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

	err = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build())
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

// TestProxyBidiStream_PropagatesServerErrorMidStream asserts that a terminal
// error the server produces after messages have flowed both ways, while the
// client's send side is still open, reaches the client rather than being
// dropped while the proxy waits on the client's next message.
func TestProxyBidiStream_PropagatesServerErrorMidStream(t *testing.T) {
	t.Parallel()
	_, fakeServerSvcClient := newFakeServerSvc(t)

	lis := bufconn.Listen(1024)
	newProxyService(t, lis, fakeServerSvcClient)
	// Short timeout so a swallowed status surfaces as a test failure rather than
	// waiting out the default go-test timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	client := newProxyServiceClient(t, lis)
	stream, err := client.ConnectToDesktop(ctx)
	require.NoError(t, err)

	err = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build())
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)

	// Empty data makes the server return an error on its second Recv.
	err = stream.Send(&teletermv1.ConnectToDesktopRequest{})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorContains(t, err, "empty data")
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

	err = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build())
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
	_ = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("more")}.Build())

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

	for _, tc := range []struct {
		name      string
		proxyOpts []grpcutils.ProxyBidiStreamOption
		// wantDials is how many server streams the proxy opened. With a first
		// message timeout the failing Recv is the one that waits for the first
		// message, which happens before the proxy dials the server.
		wantDials int32
	}{
		{
			name:      "no first message timeout",
			wantDials: 1,
		},
		{
			name: "first message timeout",
			proxyOpts: []grpcutils.ProxyBidiStreamOption{
				grpcutils.WithFirstClientMessageTimeout(time.Minute, trace.LimitExceeded("no first message")),
			},
			wantDials: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, fakeServerSvcClient := newFakeServerSvc(t)

			lis := bufconn.Listen(1024)
			proxySvc := newProxyServiceWithOpts(t, lis, fakeServerSvcClient, tc.proxyOpts, grpc.MaxRecvMsgSize(64))

			client := newProxyServiceClient(t, lis)
			stream, err := client.ConnectToDesktop(t.Context())
			require.NoError(t, err)

			// Send a message larger than the proxy's MaxRecvMsgSize.
			err = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte(strings.Repeat("x", 256))}.Build())
			require.NoError(t, err)

			_, err = stream.Recv()
			assert.ErrorContains(t, err, "larger than max")
			assert.Equal(t, tc.wantDials, proxySvc.dials.Load())
		})
	}
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
	err = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte(strings.Repeat("x", 256))}.Build())
	require.NoError(t, err)

	_, err = stream.Recv()
	require.ErrorContains(t, err, "larger than max")
}

// TestProxyBidiStream_ForwardsMetadata asserts that the proxy passes the
// client's incoming metadata upstream to the server and forwards the server's
// response headers and trailers back to the client.
func TestProxyBidiStream_ForwardsMetadata(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		proxyOpts []grpcutils.ProxyBidiStreamOption
	}{
		{name: "no first message timeout"},
		{
			// With a first message timeout the proxy dials the server only once
			// the first message has arrived. The metadata attached to the call
			// must survive that.
			name: "first message timeout",
			proxyOpts: []grpcutils.ProxyBidiStreamOption{
				grpcutils.WithFirstClientMessageTimeout(time.Minute, trace.LimitExceeded("no first message")),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service, fakeServerSvcClient := newFakeServerSvc(t)
			service.echoMetadata = true

			lis := bufconn.Listen(1024)
			newProxyServiceWithOpts(t, lis, fakeServerSvcClient, tc.proxyOpts)

			// Short timeout so a hang (e.g. Header() never unblocks due to a regression)
			// surfaces as a test failure rather than waiting out the default go-test timeout.
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			// Attach metadata to the outgoing call so the proxy can forward it upstream.
			ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-test-key", "test-value"))

			client := newProxyServiceClient(t, lis)
			stream, err := client.ConnectToDesktop(ctx)
			require.NoError(t, err)

			err = stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build())
			require.NoError(t, err)

			// Receive the server's first response; by this point the server has already
			// called SendHeader so headers are available on the client stream.
			_, err = stream.Recv()
			require.NoError(t, err)

			headers, err := stream.Header()
			require.NoError(t, err)
			assert.Equal(t, []string{"test-value"}, headers.Get("x-test-key"),
				"response headers not forwarded through proxy")

			err = stream.CloseSend()
			require.NoError(t, err)

			_, err = stream.Recv()
			assert.ErrorIs(t, err, io.EOF)

			trailers := stream.Trailer()
			assert.Equal(t, []string{"test-value"}, trailers.Get("x-test-key"),
				"response trailers not forwarded through proxy")
		})
	}
}

// TestProxyBidiStream_StreamTimeout covers WithStreamTimeout: a client that
// keeps the stream open past the timeout gets the configured error, and the
// server stream is canceled so that its handler is released as well.
func TestProxyBidiStream_StreamTimeout(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const streamTimeout = time.Minute
		service, fakeServerSvcClient := newFakeServerSvc(t)
		lis := bufconn.Listen(1024)
		newProxyServiceWithOpts(t, lis, fakeServerSvcClient, []grpcutils.ProxyBidiStreamOption{
			grpcutils.WithStreamTimeout(streamTimeout, trace.LimitExceeded("stream timed out")),
		})
		client := newProxyServiceClient(t, lis)

		start := time.Now()
		stream, err := client.ConnectToDesktop(t.Context())
		require.NoError(t, err)
		require.NoError(t, stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build()))
		msg, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, []byte("ack"), msg.GetData())

		// The client stalls with the stream open.
		_, err = recvWithTimeout(t, stream, streamTimeout+time.Minute)
		assert.ErrorContains(t, err, "stream timed out")
		assert.Equal(t, streamTimeout, time.Since(start))

		// The deadline ended the server stream as well, which fails the server
		// handler's pending Recv.
		synctest.Wait()
		assert.Equal(t, int32(1), service.returned.Load())
	})
}

// TestProxyBidiStream_StreamTimeoutClientNotReading covers WithStreamTimeout on
// a client that stops reading. The proxy's Send to that client blocks on flow
// control once the client's window is full, so the deadline cannot surface
// through the forwarding goroutines and the handler has to observe it itself.
func TestProxyBidiStream_StreamTimeoutClientNotReading(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const streamTimeout = time.Minute
		service, fakeServerSvcClient := newFakeServerSvc(t)
		// 256 KiB in total, twice what the client's window and the proxy's send
		// queue absorb together.
		service.largeResponses = 16
		lis := bufconn.Listen(1024)
		proxySvc := newProxyServiceWithOpts(t, lis, fakeServerSvcClient, []grpcutils.ProxyBidiStreamOption{
			grpcutils.WithStreamTimeout(streamTimeout, trace.LimitExceeded("stream timed out")),
		})
		// A static window keeps gRPC from growing it as responses arrive, which
		// would make the point at which Send blocks depend on timing.
		client := newProxyServiceClient(t, lis, grpc.WithStaticStreamWindowSize(64*1024))

		stream, err := client.ConnectToDesktop(t.Context())
		require.NoError(t, err)
		require.NoError(t, stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build()))

		// The client never reads. Right before the timeout the handler is still
		// running, at the timeout it has returned.
		time.Sleep(streamTimeout - time.Nanosecond)
		synctest.Wait()
		require.Zero(t, proxySvc.returned.Load())
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		require.Equal(t, int32(1), proxySvc.returned.Load(), "the proxy handler did not return at the stream timeout")

		// Once the client reads again, it drains what the proxy managed to send
		// and then gets the timeout error.
		for err == nil {
			_, err = stream.Recv()
		}
		assert.ErrorContains(t, err, "stream timed out")
	})
}

// TestProxyBidiStream_FirstClientMessageTimeout covers
// WithFirstClientMessageTimeout on a client that opens a stream and sends
// nothing.
func TestProxyBidiStream_FirstClientMessageTimeout(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const firstMessageTimeout = 10 * time.Second
		service, fakeServerSvcClient := newFakeServerSvc(t)
		lis := bufconn.Listen(1024)
		proxySvc := newProxyServiceWithOpts(t, lis, fakeServerSvcClient, []grpcutils.ProxyBidiStreamOption{
			grpcutils.WithFirstClientMessageTimeout(firstMessageTimeout, trace.LimitExceeded("no first message")),
		})
		client := newProxyServiceClient(t, lis)

		start := time.Now()
		stream, err := client.ConnectToDesktop(t.Context())
		require.NoError(t, err)

		_, err = recvWithTimeout(t, stream, firstMessageTimeout+time.Minute)
		assert.ErrorContains(t, err, "no first message")
		assert.Equal(t, firstMessageTimeout, time.Since(start))

		synctest.Wait()
		assert.Equal(t, int32(0), proxySvc.dials.Load(), "server must not be dialed before the first message")
		assert.Equal(t, int32(0), service.returned.Load())
	})
}

// TestProxyBidiStream_FirstClientMessageInTime checks that a first message that
// arrives in time is forwarded, and that the first message timeout is over once
// it has. The stream should stay usable well past it.
func TestProxyBidiStream_FirstClientMessageInTime(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		const firstMessageTimeout = 10 * time.Second
		_, fakeServerSvcClient := newFakeServerSvc(t)
		lis := bufconn.Listen(1024)
		proxySvc := newProxyServiceWithOpts(t, lis, fakeServerSvcClient, []grpcutils.ProxyBidiStreamOption{
			grpcutils.WithFirstClientMessageTimeout(firstMessageTimeout, trace.LimitExceeded("no first message")),
		})
		client := newProxyServiceClient(t, lis)

		stream, err := client.ConnectToDesktop(t.Context())
		require.NoError(t, err)
		require.NoError(t, stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("hello")}.Build()))
		msg, err := stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, []byte("ack"), msg.GetData())
		assert.Equal(t, int32(1), proxySvc.dials.Load())

		// Well past the first message timeout the stream still works.
		time.Sleep(2 * firstMessageTimeout)
		require.NoError(t,
			stream.Send(teletermv1.ConnectToDesktopRequest_builder{Data: []byte("again")}.Build()),
			"expected the stream to stay usable past the first message timeout")
		msg, err = stream.Recv()
		require.NoError(t, err)
		assert.Equal(t, []byte("ack"), msg.GetData())

		require.NoError(t, stream.CloseSend())
		_, err = stream.Recv()
		assert.ErrorIs(t, err, io.EOF)
	})
}

// TestProxyBidiStream_HalfCloseBeforeFirstMessage checks that with a first
// message timeout, a client that half-closes without sending anything still
// gets the server's verdict: the proxy dials the server, which sees io.EOF, and
// its terminal error reaches the client.
func TestProxyBidiStream_HalfCloseBeforeFirstMessage(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		service, fakeServerSvcClient := newFakeServerSvc(t)
		service.postClientEOFErr = trace.BadParameter("nothing uploaded")
		lis := bufconn.Listen(1024)
		proxySvc := newProxyServiceWithOpts(t, lis, fakeServerSvcClient, []grpcutils.ProxyBidiStreamOption{
			grpcutils.WithFirstClientMessageTimeout(10*time.Second, trace.LimitExceeded("no first message")),
		})
		client := newProxyServiceClient(t, lis)

		stream, err := client.ConnectToDesktop(t.Context())
		require.NoError(t, err)
		require.NoError(t, stream.CloseSend())

		_, err = recvWithTimeout(t, stream, time.Minute)
		assert.ErrorContains(t, err, "nothing uploaded")
		assert.Equal(t, int32(1), proxySvc.dials.Load())
	})
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
	t.Cleanup(func() { _ = client.Close() })

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

	// returned counts ConnectToDesktop calls that have returned, so that tests
	// can check that the proxy releases the server handler when it ends a stream
	// on its own.
	returned atomic.Int32

	// largeResponses, if positive, makes ConnectToDesktop answer each request
	// with this many 16 KiB responses instead of the ack. Once a client that
	// stopped reading has 64 KiB of them in its flow control window and the
	// proxy has queued another 64 KiB for that stream, the proxy's Send to that
	// client blocks.
	largeResponses int
}

// ConnectToDesktop does NOT implement the semantics of the real
// ConnectToDesktop RPC. The RPC is borrowed purely for its bidi-stream shape so
// the tests in this file can exercise ProxyBidiStream without introducing a
// custom test-only proto.
//
// Contract used by the tests:
//   - Every request must populate data with a non-empty payload. An empty data
//     triggers a trace.BadParameter return.
//   - Every response carries data = "ack", unless largeResponses is set.
func (f *fakeServerSvc) ConnectToDesktop(stream teletermv1.TerminalService_ConnectToDesktopServer) error {
	defer f.returned.Add(1)
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
		if f.largeResponses > 0 {
			data := make([]byte, 16*1024)
			for range f.largeResponses {
				if err := stream.Send(teletermv1.ConnectToDesktopResponse_builder{Data: data}.Build()); err != nil {
					return trace.Wrap(err)
				}
			}
			continue
		}
		if err := stream.Send(teletermv1.ConnectToDesktopResponse_builder{Data: []byte("ack")}.Build()); err != nil {
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
	newProxyServiceWithOpts(t, lis, client, nil, opts...)
}

// newProxyServiceWithOpts is newProxyService with ProxyBidiStream options, for
// the tests that exercise them. It returns the service so that tests can read
// its counters.
func newProxyServiceWithOpts(t *testing.T, lis net.Listener, client teletermv1.TerminalServiceClient,
	proxyOpts []grpcutils.ProxyBidiStreamOption, opts ...grpc.ServerOption,
) *proxyService {
	t.Helper()

	s := grpc.NewServer(opts...)
	t.Cleanup(s.GracefulStop)

	proxySvc := &proxyService{
		serverSvcClient: client,
		opts:            proxyOpts,
	}

	teletermv1.RegisterTerminalServiceServer(s, proxySvc)

	go func() {
		err := s.Serve(lis)
		require.NoError(t, err)
	}()
	return proxySvc
}

type proxyService struct {
	teletermv1.UnimplementedTerminalServiceServer

	serverSvcClient teletermv1.TerminalServiceClient
	// opts are passed to every ProxyBidiStream call.
	opts []grpcutils.ProxyBidiStreamOption
	// dials counts the server streams the proxy opened.
	dials atomic.Int32
	// returned counts ConnectToDesktop calls that have returned, for tests whose
	// client cannot observe the end of the stream.
	returned atomic.Int32
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
	defer p.returned.Add(1)
	getServer := func(ctx context.Context) (teletermv1.TerminalService_ConnectToDesktopClient, error) {
		p.dials.Add(1)
		return p.serverSvcClient.ConnectToDesktop(ctx)
	}
	err := grpcutils.ProxyBidiStream(logtest.NewLogger(), client, getServer, p.opts...)
	return trace.Wrap(err)
}

func newProxyServiceClient(t *testing.T, lis *bufconn.Listener, opts ...grpc.DialOption) teletermv1.TerminalServiceClient {
	t.Helper()
	clientConn, err := grpc.NewClient(
		"passthrough:///bufconn",
		append([]grpc.DialOption{
			grpc.WithContextDialer(
				func(ctx context.Context, _ string) (net.Conn, error) {
					return lis.DialContext(ctx)
				},
			),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}, opts...)...,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.Close() })
	return teletermv1.NewTerminalServiceClient(clientConn)
}

// recvWithTimeout receives from stream in a goroutine and fails the test if
// nothing arrives within timeout, so that a proxy that never ends the stream
// fails the test instead of hanging it. Meant for synctest bubbles, where the
// timeout costs no wall time.
func recvWithTimeout(t *testing.T, stream teletermv1.TerminalService_ConnectToDesktopClient, timeout time.Duration) (*teletermv1.ConnectToDesktopResponse, error) {
	t.Helper()
	type result struct {
		msg *teletermv1.ConnectToDesktopResponse
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		msg, err := stream.Recv()
		resCh <- result{msg: msg, err: err}
	}()
	select {
	case res := <-resCh:
		return res.msg, res.err
	case <-time.After(timeout):
		t.Fatal("timed out waiting for the proxy to end the stream")
		return nil, nil
	}
}
