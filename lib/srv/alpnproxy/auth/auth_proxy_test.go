package alpnproxyauth

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	//ctx := context.Background()
	_, err = s.dialLocalAuthServer(ctx)
	require.NoError(t, err)
}
