package alpnproxyauth

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
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
	require.Error(t, err)
	require.Equal(t, "empty auth servers list", err.Error())
}

func TestDialLocalAuthServerNoAvailableServers(t *testing.T) {
	server1, err := types.NewServer("s1", "auth", types.ServerSpecV2{Addr: "invalid:8000"})
	require.Nil(t, err)
	s := NewAuthProxyDialerService(nil, mockAuthGetter{servers: []types.Server{server1}})
	_, err = s.dialLocalAuthServer(context.Background())
	require.Error(t, err)
	require.Equal(t, "all auth servers unavailable: invalid:8000: dial tcp: lookup invalid: no such host", err.Error())
}

func TestDialLocalAuthServerAvailableServers(t *testing.T) {
	socket, err := net.Listen("tcp", "127.0.0.1:")
	require.Nil(t, err)
	defer socket.Close()
	server, err := types.NewServer("s1", "auth", types.ServerSpecV2{Addr: socket.Addr().String()})
	require.Nil(t, err)
	servers := []types.Server{server}
	for i := 0; i < 20; i++ {
		server, err := types.NewServer("s1", "auth", types.ServerSpecV2{Addr: "invalid2:8000"})
		require.Nil(t, err)
		servers = append(servers, server)
	}
	s := NewAuthProxyDialerService(nil, mockAuthGetter{servers: servers})
	_, err = s.dialLocalAuthServer(context.Background())
	require.Nil(t, err)
}
