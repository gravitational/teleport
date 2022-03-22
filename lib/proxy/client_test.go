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

package proxy

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/stretchr/testify/require"
)

// TestClientConn checks the client's connection caching capabilities
func TestClientConn(t *testing.T) {
	ca := newSelfSignedCA(t)

	client, _ := setupClient(t, ca, ca, types.RoleProxy)
	_, _, def1 := setupServer(t, "s1", ca, ca, types.RoleProxy)
	server2, _, def2 := setupServer(t, "s2", ca, ca, types.RoleProxy)

	// simulate watcher finding two servers
	err := client.updateConnections([]types.Server{def1, def2})
	require.NoError(t, err)
	require.Len(t, client.conns, 2)

	// dial first server and send a test data frame
	stream, cached, err := client.dial([]string{"s1"})
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream)
	require.NoError(t, sendMsg(stream))
	stream.CloseSend()

	// dial second server
	stream, cached, err = client.dial([]string{"s2"})
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream)
	stream.CloseSend()

	// redial second server
	stream, cached, err = client.dial([]string{"s2"})
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream)
	stream.CloseSend()

	// close second server
	// and attempt to redial it
	server2.Shutdown()
	stream, cached, err = client.dial([]string{"s2"})
	require.Error(t, err)
	require.False(t, cached)
	require.Nil(t, stream)
}

// TestClientUpdate checks the client's watcher update behaviour
func TestClientUpdate(t *testing.T) {
	ca := newSelfSignedCA(t)

	client, _ := setupClient(t, ca, ca, types.RoleProxy)
	_, _, def1 := setupServer(t, "s1", ca, ca, types.RoleProxy)
	server2, _, def2 := setupServer(t, "s2", ca, ca, types.RoleProxy)

	// watcher finds two servers
	err := client.updateConnections([]types.Server{def1, def2})
	require.NoError(t, err)
	require.Len(t, client.conns, 2)
	require.Contains(t, client.conns, "s1")
	require.Contains(t, client.conns, "s2")

	s1, _, err := client.dial([]string{"s1"})
	require.NoError(t, err)
	require.NotNil(t, s1)
	require.NoError(t, sendMsg(s1))
	s2, _, err := client.dial([]string{"s2"})
	require.NoError(t, err)
	require.NotNil(t, s2)
	require.NoError(t, sendMsg(s2))

	// watcher finds one of the two servers
	err = client.updateConnections([]types.Server{def1})
	require.NoError(t, err)
	require.Len(t, client.conns, 1)
	require.Contains(t, client.conns, "s1")
	require.NoError(t, sendMsg(s1)) // stream is not broken across updates
	require.NoError(t, sendMsg(s2)) // stream is not forcefully closed. ClientConn waits for a graceful shutdown before it closes.

	s2.CloseSend()

	// watcher finds two servers with one broken connection
	server2.Shutdown()
	err = client.updateConnections([]types.Server{def1, def2})
	require.NoError(t, err) // server2 is in a transient failure state but not reported as an error
	require.Len(t, client.conns, 2)
	require.Contains(t, client.conns, "s1")
	require.NoError(t, sendMsg(s1)) // stream is still going strong
	_, _, err = client.dial([]string{"s2"})
	require.Error(t, err) // can't dial server2, obviously

	// peer address change
	_, _, def3 := setupServer(t, "s1", ca, ca, types.RoleProxy)
	err = client.updateConnections([]types.Server{def3})
	require.NoError(t, err)
	require.Len(t, client.conns, 1)
	require.Contains(t, client.conns, "s1")
	require.NoError(t, sendMsg(s1)) // stream is not forcefully closed. ClientConn waits for a graceful shutdown before it closes.
	s3, _, err := client.dial([]string{"s1"})
	require.NoError(t, err)
	require.NotNil(t, s3)
	require.NoError(t, sendMsg(s3)) // new stream is working

	s1.CloseSend()
	s3.CloseSend()
}

func TestCAChange(t *testing.T) {
	clientCA := newSelfSignedCA(t)
	serverCA := newSelfSignedCA(t)

	client, clientTLSConfig := setupClient(t, clientCA, serverCA, types.RoleProxy)
	server, serverTLSConfig, serverDef := setupServer(t, "s1", serverCA, clientCA, types.RoleProxy)

	err := client.updateConnections([]types.Server{serverDef})
	require.NoError(t, err)
	require.Len(t, client.conns, 1)

	// dial server and send a test data frame
	ogStream, cached, err := client.dial([]string{"s1"})
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, ogStream)

	require.NoError(t, sendMsg(ogStream))
	ogStream.CloseSend()

	// server ca rotated
	newServerCA := newSelfSignedCA(t)

	newServerTLSConfig := certFromIdentity(t, newServerCA, tlsca.Identity{
		Groups: []string{string(types.RoleProxy)},
	})

	*serverTLSConfig = *newServerTLSConfig

	// existing connection should still be working
	ogStream, cached, err = client.dial([]string{"s1"})
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, ogStream)
	require.NoError(t, sendMsg(ogStream))

	// new connection should fail because client tls config still references old
	// RootCAs.
	conn, err := client.connect("s1", server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, conn)
	stream, err := client.startStream(conn)
	require.Error(t, err)
	require.Nil(t, stream)

	// new connection should succeed because client references new RootCAs
	*serverCA = *newServerCA
	conn, err = client.connect("s1", server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, conn)
	stream, err = client.startStream(conn)
	require.NoError(t, err)
	require.NoError(t, sendMsg(stream))
	stream.CloseSend()

	// for good measure, original stream should still be working
	require.NoError(t, sendMsg(ogStream))

	// client ca rotated
	newClientCA := newSelfSignedCA(t)

	newClientTLSConfig := certFromIdentity(t, newClientCA, tlsca.Identity{
		Groups: []string{string(types.RoleProxy)},
	})

	*clientTLSConfig = *newClientTLSConfig

	// new connection should fail because server tls config still references old
	// ClientCAs.
	conn, err = client.connect("s1", server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, conn)
	stream, err = client.startStream(conn)
	require.Error(t, err)
	require.Nil(t, stream)

	// new connection should succeed because client references new RootCAs
	*clientCA = *newClientCA
	conn, err = client.connect("s1", server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, conn)
	stream, err = client.startStream(conn)
	require.NoError(t, err)
	require.NoError(t, sendMsg(stream))
	stream.CloseSend()

	// and one final time, original stream should still be working
	require.NoError(t, sendMsg(ogStream))
	ogStream.CloseSend()
}
