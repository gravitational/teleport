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
	"crypto/tls"
	"net"
	"testing"

	clientapi "github.com/gravitational/teleport/api/client/proto"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/stretchr/testify/require"
)

// TestClientConn checks the client's connection caching capabilities
func TestClientConn(t *testing.T) {
	ca, err := newSelfSignedCA()
	require.NoError(t, err)

	client := setupClient(t, ca)
	server1 := setupServer(t, ca)
	server2 := setupServer(t, ca)
	go server1.Serve()
	go server2.Serve()
	t.Cleanup(func() {
		server1.Close()
		server2.Close()
		client.Close()
	})

	// start with a fresh client
	require.Equal(t, len(client.conns), 0)

	// dial first server and send a test data frame
	stream, cached, err := client.dial(context.TODO(), server1.config.Listener.Addr().String())
	require.NoError(t, err)
	require.False(t, cached)
	require.Equal(t, len(client.conns), 1)

	err = stream.Send(&clientapi.Frame{
		Message: &clientapi.Frame_Data{
			&clientapi.Data{Bytes: []byte("test")},
		},
	})
	require.NoError(t, err)

	// dial second server
	_, cached, err = client.dial(context.TODO(), server2.config.Listener.Addr().String())
	require.NoError(t, err)
	require.False(t, cached)
	require.Equal(t, len(client.conns), 2)

	// redial second server
	_, cached, err = client.dial(context.TODO(), server2.config.Listener.Addr().String())
	require.NoError(t, err)
	require.True(t, cached)
	require.Equal(t, len(client.conns), 2)

	// close second server
	// and attempt to redial it
	server2.Close()
	_, _, err = client.dial(context.TODO(), server2.config.Listener.Addr().String())
	require.Error(t, err)
	require.Equal(t, len(client.conns), 1)
}

// setupClients return a Client object.
func setupClient(t *testing.T, ca *tlsca.CertAuthority) *Client {
	tlsConf := certFromIdentity(t, ca, tlsca.Identity{
		Groups: []string{string(types.RoleProxy)},
	})

	client, err := NewClient(ClientConfig{
		AccessCache: &mockAccessCache{},
		TLSConfig:   tlsConf,
	})
	require.NoError(t, err)

	return client
}

// setupServer return a Server object.
func setupServer(t *testing.T, ca *tlsca.CertAuthority) *Server {
	tlsConf := certFromIdentity(t, ca, tlsca.Identity{
		Groups: []string{string(types.RoleProxy)},
	})

	getConfigForClient := func(chi *tls.ClientHelloInfo) (*tls.Config, error) {
		config := tlsConf.Clone()
		config.ClientAuth = tls.RequireAndVerifyClientCert
		config.ClientCAs = config.RootCAs
		return config, nil
	}

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	server, err := NewServer(ServerConfig{
		AccessCache:        &mockAccessCache{},
		Listener:           listener,
		TLSConfig:          tlsConf,
		ClusterDialer:      &mockClusterDialer{},
		getConfigForClient: getConfigForClient,
	})
	require.NoError(t, err)

	return server
}
