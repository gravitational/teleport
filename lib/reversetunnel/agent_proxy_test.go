package reversetunnel

import (
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestProxiedServiceUpdater(t *testing.T) {
	proxies := NewProxiedServiceUpdater(clockwork.NewFakeClock())

	server, err := types.NewServer("test", types.KindNode, types.ServerSpecV2{})
	require.NoError(t, err)

	nonceID := proxies.nonceID

	tests := []struct {
		nonceID uint64
		nonce   uint64
		proxies []string
	}{
		{
			nonceID: nonceID,
			nonce:   1,
			proxies: []string{},
		},
		{
			nonceID: nonceID,
			nonce:   2,
			proxies: []string{"test1"},
		},
		{
			nonceID: nonceID,
			nonce:   3,
			proxies: []string{"test2"},
		},
		{
			nonceID: nonceID,
			nonce:   4,
			proxies: []string{"test3"},
		},
		{
			nonceID: nonceID,
			nonce:   4,
			proxies: []string{"test3"},
		},
	}

	for _, tc := range tests {
		proxies.setProxiesIDs(tc.proxies)
		proxies.Update(server)

		require.Equal(t, tc.proxies, server.GetProxyIDs())
		require.Equal(t, tc.nonceID, server.GetNonceID())
		require.Equal(t, tc.nonce, server.GetNonce())
	}
}
