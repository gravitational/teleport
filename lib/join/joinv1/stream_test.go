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

package joinv1

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/lib/join/internal/messages"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestStreamErrorsWhenPeerDisconnects(t *testing.T) {
	t.Run("client sees server disconnect", func(t *testing.T) {
		// Start a server that, for each stream, receives the first message
		// from the client then returns an error to terminate the stream.
		server := newTestJoinServer(t, testMessageServerFunc(func(stream messages.ServerStream) error {
			_, err := stream.Recv()
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.ConnectionProblem(nil, "server disconnected")
		}))

		// Make a client connected to the test server.
		client := newTestJoinClient(t, server)

		// Initiate the join stream and send a single message, which should succeed.
		stream, err := client.Join(t.Context())
		require.NoError(t, err)
		require.NoError(t, stream.Send(&messages.ClientInit{
			TokenName:  "test-token",
			SystemRole: "node",
		}))

		// Make sure the next Recv gets the error from the server.
		_, err = stream.Recv()
		require.ErrorContains(t, err, "server disconnected", "expected an error on Recv from the disconnected server")

		// A subsequent Send should get the same error.
		err = stream.Send(&messages.GivingUp{Msg: "server disconnected"})
		require.ErrorContains(t, err, "server disconnected", "expected an error on Send to the disconnected server")
	})

	t.Run("server sees client disconnect", func(t *testing.T) {
		// Start a server that, for each stream, receives the first message
		// from the client then waits for a second message from the client.
		serverErr := make(chan error, 1)
		server := newTestJoinServer(t, testMessageServerFunc(func(stream messages.ServerStream) error {
			_, err := stream.Recv()
			if err != nil {
				serverErr <- err
				return trace.Wrap(err)
			}

			_, err = stream.Recv()
			serverErr <- err
			return trace.Wrap(err)
		}))

		// Make a client connected to the test server.
		client := newTestJoinClient(t, server)

		// Initiate a join stream that can be canceled by the context, causing
		// the client to terminate the stream.
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		stream, err := client.Join(ctx)
		require.NoError(t, err)

		// Send a single message, which should succeed.
		require.NoError(t, stream.Send(&messages.ClientInit{
			TokenName:  "test-token",
			SystemRole: "node",
		}))

		// Cancel the context to terminate the stream.
		cancel()

		// Wait for the server to observe the error and send it on the channel.
		select {
		case err := <-serverErr:
			require.Error(t, err, "expected the server to see an error on Recv from the disconnected client")
		case <-time.After(10 * time.Second):
			t.Fatal("server did not see client disconnect")
		}
	})
}

type testMessageServerFunc func(messages.ServerStream) error

func (f testMessageServerFunc) Join(stream messages.ServerStream) error {
	return f(stream)
}

func newTestJoinServer(t *testing.T, server messageServer) *bufconn.Listener {
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	RegisterJoinServiceServer(grpcServer, server)

	testutils.RunTestBackgroundTask(t.Context(), t, &testutils.TestBackgroundTask{
		Name: "test gRPC join server",
		Task: func(ctx context.Context) error {
			return grpcServer.Serve(listener)
		},
		Terminate: func() error {
			grpcServer.Stop()
			return nil
		},
	})

	return listener
}

func newTestJoinClient(t *testing.T, server *bufconn.Listener) *Client {
	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return server.DialContext(ctx)
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	return NewClientFromConn(conn)
}
