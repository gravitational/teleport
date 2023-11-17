// Copyright 2022 Gravitational, Inc
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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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

func setupService(t *testing.T) (*proxyService, proto.ProxyServiceClient) {
	server := grpc.NewServer()
	t.Cleanup(server.Stop)

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	proxyService := &proxyService{
		log: logrus.New(),
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
	service.clusterDialer = &mockClusterDialer{
		MockDialCluster: func(clusterName string, request DialParams) (net.Conn, error) {
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
