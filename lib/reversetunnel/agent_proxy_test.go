package reversetunnel

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func TestProxiedServiceUpdater(t *testing.T) {
	proxies := NewProxiedServiceUpdater()

	server, err := types.NewServer("test", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)

	tests := []struct {
		proxies []string
	}{
		{
			proxies: []string{},
		},
		{
			proxies: []string{"test1"},
		},
		{
			proxies: []string{"test2"},
		},
		{
			proxies: []string{"test3"},
		},
		{
			proxies: []string{"test3"},
		},
	}

	for _, tc := range tests {
		proxies.setProxiesIDs(tc.proxies)
		proxies.Update(server)

		require.Equal(t, tc.proxies, server.GetProxyIDs())
	}
}
