/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package alpnproxyauth

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
)

type mockAuthGetter struct {
	servers []types.Server
}

func (m mockAuthGetter) GetClusterName(...services.MarshalOption) (types.ClusterName, error) {
	return nil, nil
}

func (m mockAuthGetter) GetAuthServers() ([]types.Server, error) {
	return m.servers, nil
}

func TestDialLocalAuthServerNoServers(t *testing.T) {
	s := NewAuthProxyDialerService(nil, mockAuthGetter{servers: []types.Server{}})
	_, err := s.dialLocalAuthServer(context.Background())
	require.Error(t, err, "dialLocalAuthServer expected to fail")
	require.Equal(t, "empty auth servers list", err.Error())
}

func TestDialLocalAuthServerNoAvailableServers(t *testing.T) {
	server1, err := types.NewServer("s1", "auth", types.ServerSpecV2{Addr: "invalid:8000"})
	require.NoError(t, err)
	s := NewAuthProxyDialerService(nil, mockAuthGetter{servers: []types.Server{server1}})
	_, err = s.dialLocalAuthServer(context.Background())
	require.Error(t, err, "dialLocalAuthServer expected to fail")
	require.Contains(t, err.Error(), "invalid:8000:")
}

func TestDialLocalAuthServerAvailableServers(t *testing.T) {
	socket, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err)
	defer socket.Close()
	server, err := types.NewServer("s1", "auth", types.ServerSpecV2{Addr: socket.Addr().String()})
	require.NoError(t, err)
	servers := []types.Server{server}
	// multiple invalid servers to minimize chance that we select good one first try
	for i := 0; i < 20; i++ {
		server, err := types.NewServer("s1", "auth", types.ServerSpecV2{Addr: "invalid2:8000"})
		require.NoError(t, err)
		servers = append(servers, server)
	}
	s := NewAuthProxyDialerService(nil, mockAuthGetter{servers: servers})
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = s.dialLocalAuthServer(ctx)
	require.NoError(t, err)
}
