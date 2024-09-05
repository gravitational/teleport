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
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/types"
	peerv0 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/proxy/peer/v0"
)

type mockClusterDialer struct {
	MockDialCluster func(string, DialParams) (net.Conn, error)
}

func (m *mockClusterDialer) Dial(clusterName string, request DialParams) (net.Conn, error) {
	if m.MockDialCluster == nil {
		return nil, trace.NotImplemented("")
	}
	return m.MockDialCluster(clusterName, request)
}

func setupService(t *testing.T) (*proxyService, peerv0.ProxyServiceClient) {
	server := grpc.NewServer()
	t.Cleanup(server.Stop)

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	proxyService := &proxyService{
		log: logrus.New(),
	}
	peerv0.RegisterProxyServiceServer(server, proxyService)

	go server.Serve(listener)

	conn, err := grpc.Dial(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	client := peerv0.NewProxyServiceClient(conn)
	return proxyService, client
}

func TestInvalidFirstFrame(t *testing.T) {
	_, client := setupService(t)
	stream, err := client.DialNode(context.Background())
	require.NoError(t, err)

	err = stream.Send(&peerv0.DialNodeRequest{
		Message: &peerv0.DialNodeRequest_Data{Data: &peerv0.Data{}},
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.Error(t, err, "expected invalid dial request")
}

func TestSendReceive(t *testing.T) {
	service, client := setupService(t)
	stream, err := client.DialNode(context.Background())
	require.NoError(t, err)

	dialRequest := &peerv0.DialRequest{
		NodeId:      "test-id.test-cluster",
		TunnelType:  string(types.NodeTunnel),
		Source:      &peerv0.NetAddr{},
		Destination: &peerv0.NetAddr{},
	}

	local, remote := net.Pipe()
	service.clusterDialer = &mockClusterDialer{
		MockDialCluster: func(clusterName string, request DialParams) (net.Conn, error) {
			require.Equal(t, "test-cluster", clusterName)
			require.Equal(t, dialRequest.TunnelType, request.ConnType)
			require.Equal(t, dialRequest.NodeId, request.ServerID)

			return remote, nil
		},
	}

	send := []byte("ping")
	recv := []byte("pong")

	err = stream.Send(&peerv0.DialNodeRequest{
		Message: &peerv0.DialNodeRequest_DialRequest{DialRequest: dialRequest},
	})
	require.NoError(t, err)

	_, err = stream.Recv()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		send := append(send, byte(i))
		err = stream.Send(&peerv0.DialNodeRequest{
			Message: &peerv0.DialNodeRequest_Data{Data: &peerv0.Data{
				Bytes: send,
			}},
		})
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
