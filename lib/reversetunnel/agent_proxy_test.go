package reversetunnel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProxiedServiceUpdater(t *testing.T) {
	proxies := NewConnectedProxyGetter()

	var expectIDs []string
	ids := proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)

	expectIDs = []string{}
	proxies.setProxiesIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)

	expectIDs = []string{"test1", "test2"}
	proxies.setProxiesIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)

	expectIDs = nil
	proxies.setProxiesIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)
}
