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
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/connectivity"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/proxy/peer/internal"
	"github.com/gravitational/teleport/lib/utils"
)

// TestClientConn checks the client's connection caching capabilities
func TestClientConn(t *testing.T) {
	ca := newSelfSignedCA(t)

	client := setupClient(t, ca, newAtomicCA(ca), types.RoleProxy)
	_, def1 := setupServer(t, "s1", ca, ca, types.RoleProxy)
	server2, def2 := setupServer(t, "s2", ca, ca, types.RoleProxy)

	// simulate watcher finding two servers
	err := client.updateConnections([]types.Server{def1, def2})
	require.NoError(t, err)
	require.Len(t, client.conns, 2)

	// dial first server and send a test data frame
	stream, cached, err := client.dial([]string{"s1"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream)
	stream.Close()

	// dial second server
	stream, cached, err = client.dial([]string{"s2"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream)
	stream.Close()

	// redial second server
	stream, cached, err = client.dial([]string{"s2"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream)
	stream.Close()

	// close second server
	// and attempt to redial it
	server2.Shutdown()
	stream, cached, err = client.dial([]string{"s2"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.Error(t, err)
	require.True(t, cached)
	require.Nil(t, stream)
}

// TestClientUpdate checks the client's watcher update behavior
func TestClientUpdate(t *testing.T) {
	ca := newSelfSignedCA(t)

	client := setupClient(t, ca, newAtomicCA(ca), types.RoleProxy)
	_, def1 := setupServer(t, "s1", ca, ca, types.RoleProxy)
	server2, def2 := setupServer(t, "s2", ca, ca, types.RoleProxy)

	// watcher finds two servers
	err := client.updateConnections([]types.Server{def1, def2})
	require.NoError(t, err)
	require.Len(t, client.conns, 2)
	require.Contains(t, client.conns, "s1")
	require.Contains(t, client.conns, "s2")

	s1, _, err := client.dial([]string{"s1"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.NoError(t, err)
	require.NotNil(t, s1)
	s2, _, err := client.dial([]string{"s2"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.NoError(t, err)
	require.NotNil(t, s2)

	// watcher finds one of the two servers
	err = client.updateConnections([]types.Server{def1})
	require.NoError(t, err)
	require.Len(t, client.conns, 1)
	require.Contains(t, client.conns, "s1")
	sendMsg(t, s1) // stream is not broken across updates
	sendMsg(t, s2) // stream is not forcefully closed. ClientConn waits for a graceful shutdown before it closes.

	s2.Close()

	// watcher finds two servers with one broken connection
	server2.Shutdown()
	err = client.updateConnections([]types.Server{def1, def2})
	require.NoError(t, err) // server2 is in a transient failure state but not reported as an error
	require.Len(t, client.conns, 2)
	require.Contains(t, client.conns, "s1")
	sendMsg(t, s1) // stream is still going strong
	_, _, err = client.dial([]string{"s2"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.Error(t, err) // can't dial server2, obviously

	// peer address change
	_, def3 := setupServer(t, "s1", ca, ca, types.RoleProxy)
	err = client.updateConnections([]types.Server{def3})
	require.NoError(t, err)
	require.Len(t, client.conns, 1)
	require.Contains(t, client.conns, "s1")
	sendMsg(t, s1) // stream is not forcefully closed. ClientConn waits for a graceful shutdown before it closes.
	s3, _, err := client.dial([]string{"s1"}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.NoError(t, err)
	require.NotNil(t, s3)

	s1.Close()
	s3.Close()
}

func TestCAChange(t *testing.T) {
	clientCA := newSelfSignedCA(t)
	serverCA := newSelfSignedCA(t)
	currentServerCA := newAtomicCA(serverCA)

	client := setupClient(t, clientCA, currentServerCA, types.RoleProxy)
	server, ts := setupServer(t, "s1", serverCA, clientCA, types.RoleProxy)
	t.Cleanup(func() { server.Close() })

	// dial server and send a test data frame
	const supportsQUICFalse = false
	conn, err := client.connect(connectParams{
		peerID:       "s1",
		peerAddr:     ts.GetPeerAddr(),
		peerHost:     "s1",
		peerGroup:    "",
		supportsQUIC: supportsQUICFalse,
	})
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.IsType(t, (*grpcClientConn)(nil), conn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := proto.NewProxyServiceClient(conn.(*grpcClientConn).cc).DialNode(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)

	// rotate server ca
	require.NoError(t, server.Close())
	newServerCA := newSelfSignedCA(t)
	server2, ts := setupServer(t, "s1", newServerCA, clientCA, types.RoleProxy)
	t.Cleanup(func() { server2.Close() })

	// new connection should fail because client tls config still references old
	// RootCAs.
	conn, err = client.connect(connectParams{
		peerID:       "s1",
		peerAddr:     ts.GetPeerAddr(),
		peerHost:     "s1",
		peerGroup:    "",
		supportsQUIC: supportsQUICFalse,
	})
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.IsType(t, (*grpcClientConn)(nil), conn)
	stream, err = proto.NewProxyServiceClient(conn.(*grpcClientConn).cc).DialNode(ctx)
	require.Error(t, err)
	require.Nil(t, stream)

	// new connection should succeed because client tls config references new
	// RootCAs.
	currentServerCA.Store(newServerCA)

	conn, err = client.connect(connectParams{
		peerID:       "s1",
		peerAddr:     ts.GetPeerAddr(),
		peerHost:     "s1",
		peerGroup:    "",
		supportsQUIC: supportsQUICFalse,
	})
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.IsType(t, (*grpcClientConn)(nil), conn)
	stream, err = proto.NewProxyServiceClient(conn.(*grpcClientConn).cc).DialNode(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)
}

func TestBackupClient(t *testing.T) {
	ca := newSelfSignedCA(t)
	client := setupClient(t, ca, newAtomicCA(ca), types.RoleProxy)
	dialCalled := false

	// Force the first client connection to fail.
	_, def1 := setupServer(t, "s1", ca, ca, types.RoleProxy, func(c *ServerConfig) {
		c.service = &mockProxyService{
			mockDialNode: func(stream proto.ProxyService_DialNodeServer) error {
				dialCalled = true
				return trace.NotFound("tunnel not found")
			},
		}
	})
	_, def2 := setupServer(t, "s2", ca, ca, types.RoleProxy)

	err := client.updateConnections([]types.Server{def1, def2})
	require.NoError(t, err)
	waitForGRPCConns(t, client.conns, time.Second*2)

	_, _, err = client.dial([]string{def1.GetName(), def2.GetName()}, "", &utils.NetAddr{}, &utils.NetAddr{}, "")
	require.NoError(t, err)
	require.True(t, dialCalled)
}

func waitForGRPCConns(t *testing.T, conns map[string]internal.ClientConn, d time.Duration) {
	require.Eventually(t, func() bool {
		for _, conn := range conns {
			// panic if we hit a non-grpc client conn
			if conn.(*grpcClientConn).cc.GetState() != connectivity.Ready {
				return false
			}
		}
		return true
	}, d, time.Millisecond*5)
}
