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
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/stretchr/testify/require"
)

// TestClientConn checks the client's connection caching capabilities
func TestClientConn(t *testing.T) {
	ca, err := newSelfSignedCA()
	require.NoError(t, err)

	client, _ := setupClient(t, ca, ca, types.RoleProxy)
	server1, _ := setupServer(t, ca, ca, types.RoleProxy)
	server2, _ := setupServer(t, ca, ca, types.RoleProxy)

	// start with a fresh client
	require.Equal(t, len(client.conns), 0)

	// dial first server and send a test data frame
	stream, cached, err := client.dial(context.TODO(), server1.config.Listener.Addr().String())
	require.NoError(t, err)
	require.False(t, cached)
	require.NotNil(t, stream)
	require.Equal(t, len(client.conns), 1)
	require.NoError(t, sendMsg(stream))
	stream.CloseSend()

	// dial second server
	stream, cached, err = client.dial(context.TODO(), server2.config.Listener.Addr().String())
	require.NoError(t, err)
	require.False(t, cached)
	require.NotNil(t, stream)
	require.Equal(t, len(client.conns), 2)
	stream.CloseSend()

	// redial second server
	stream, cached, err = client.dial(context.TODO(), server2.config.Listener.Addr().String())
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream)
	require.Equal(t, len(client.conns), 2)
	stream.CloseSend()

	// close second server
	// and attempt to redial it
	server2.Shutdown()
	stream, cached, err = client.dial(context.TODO(), server2.config.Listener.Addr().String())
	require.Error(t, err)
	require.Nil(t, stream)
	require.Equal(t, len(client.conns), 1)
}

func TestCAChange(t *testing.T) {
	clientCA, err := newSelfSignedCA()
	require.NoError(t, err)

	serverCA, err := newSelfSignedCA()
	require.NoError(t, err)

	client, clientTLSConfig := setupClient(t, clientCA, serverCA, types.RoleProxy)
	server, serverTLSConfig := setupServer(t, serverCA, clientCA, types.RoleProxy)

	// dial server and send a test data frame
	stream1, cached, err := client.dial(context.TODO(), server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.False(t, cached)
	require.NotNil(t, stream1)

	require.NoError(t, sendMsg(stream1))

	// server ca rotated
	newServerCA, err := newSelfSignedCA()
	require.NoError(t, err)

	newServerTLSConfig := certFromIdentity(t, newServerCA, tlsca.Identity{
		Groups: []string{string(types.RoleProxy)},
	})

	*serverTLSConfig = *newServerTLSConfig

	// existing connection should still be working
	stream1, cached, err = client.dial(context.TODO(), server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.True(t, cached)
	require.NotNil(t, stream1)
	require.NoError(t, sendMsg(stream1))

	// new connection should fail because client tls config still references old
	// RootCAs.
	stream2, err := client.newConnection(context.TODO(), server.config.Listener.Addr().String())
	require.Error(t, err)
	require.Nil(t, stream2)

	// new connection should succeed because client references new RootCAs
	*serverCA = *newServerCA
	stream3, err := client.newConnection(context.TODO(), server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, stream3)
	require.NoError(t, sendMsg(stream3))
	stream3.CloseSend()

	// for good measure, original stream should still be working
	require.NoError(t, sendMsg(stream1))

	// client ca rotated
	newClientCA, err := newSelfSignedCA()
	require.NoError(t, err)

	newClientTLSConfig := certFromIdentity(t, newClientCA, tlsca.Identity{
		Groups: []string{string(types.RoleProxy)},
	})

	*clientTLSConfig = *newClientTLSConfig

	// new connection should fail because server tls config still references old
	// ClientCAs.
	stream4, err := client.newConnection(context.TODO(), server.config.Listener.Addr().String())
	require.Error(t, err)
	require.Nil(t, stream4)

	// new connection should succeed because client references new RootCAs
	*clientCA = *newClientCA
	stream5, err := client.newConnection(context.TODO(), server.config.Listener.Addr().String())
	require.NoError(t, err)
	require.NotNil(t, stream5)
	require.NoError(t, sendMsg(stream5))
	stream5.CloseSend()

	// and one final time, original stream should still be working
	require.NoError(t, sendMsg(stream1))
	stream1.CloseSend()
}
