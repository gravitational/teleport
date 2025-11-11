/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package peer

import (
	"context"
	"log/slog"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	peerdial "github.com/gravitational/teleport/lib/proxy/peer/dial"
)

type mockClusterDialer struct {
	MockDialCluster func(string, peerdial.DialParams) (net.Conn, error)
}

func (m *mockClusterDialer) Dial(clusterName string, request peerdial.DialParams) (net.Conn, error) {
	if m.MockDialCluster == nil {
		return nil, trace.NotImplemented("")
	}
	return m.MockDialCluster(clusterName, request)
}

func setupService(t *testing.T) (*proxyService, proto.ProxyServiceClient) {
	server := grpc.NewServer()
	t.Cleanup(server.Stop)

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	proxyService := &proxyService{
		log: slog.Default(),
	}
	proto.RegisterProxyServiceServer(server, proxyService)

	go server.Serve(listener)

	conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	client := proto.NewProxyServiceClient(conn)
	return proxyService, client
}

func TestInvalidFirstFrame(t *testing.T) {
	_, client := setupService(t)
	stream, err := client.DialNode(context.Background())
	require.NoError(t, err)

	err = stream.Send(&proto.Frame{
		Message: &proto.Frame_Data{},
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err, "expected invalid dial request")
}

func TestSendReceive(t *testing.T) {
	service, client := setupService(t)
	stream, err := client.DialNode(context.Background())
	require.NoError(t, err)

	dialRequest := &proto.DialRequest{
		NodeID:      "test-id.test-cluster",
		TunnelType:  types.NodeTunnel,
		Source:      &proto.NetAddr{},
		Destination: &proto.NetAddr{},
	}

	local, remote := net.Pipe()
	service.dialer = &mockClusterDialer{
		MockDialCluster: func(clusterName string, request peerdial.DialParams) (net.Conn, error) {
			require.Equal(t, "test-cluster", clusterName)
			require.Equal(t, dialRequest.TunnelType, request.ConnType)
			require.Equal(t, dialRequest.NodeID, request.ServerID)

			return remote, nil
		},
	}

	send := []byte("ping")
	recv := []byte("pong")

	err = stream.Send(&proto.Frame{Message: &proto.Frame_DialRequest{
		DialRequest: dialRequest,
	}})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		send := append(send, byte(i))
		err = stream.Send(&proto.Frame{Message: &proto.Frame_Data{Data: &proto.Data{
			Bytes: send,
		}}})
		require.NoError(t, err)

		b := make([]byte, len(send))
		local.Read(b)
		require.Equal(t, send, b, "unexpected bytes sent")

		recv := append(recv, byte(i))
		local.Write(recv)
		msg, err := stream.Recv()
		require.NoError(t, err)
		require.Equal(t, recv, msg.GetData().Bytes, "unexpected bytes received")
	}
}

func TestSplitServerID(t *testing.T) {
	tests := []struct {
		serverID          string
		expectServerID    string
		expectClusterName string
		assertErr         require.ErrorAssertionFunc
	}{
		{
			"id.localhost",
			"id",
			"localhost",
			require.NoError,
		},
		{
			"id",
			"id",
			"",
			require.NoError,
		},
		{
			"id.teleport.example.com",
			"id",
			"teleport.example.com",
			require.NoError,
		},
		{
			"",
			"",
			"",
			require.Error,
		},
	}

	for _, tc := range tests {
		id, cluster, err := splitServerID(tc.serverID)
		require.Equal(t, tc.expectServerID, id)
		require.Equal(t, tc.expectClusterName, cluster)
		tc.assertErr(t, err)
	}
}
