package reversetunnel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectedProxyGetter(t *testing.T) {
	proxies := NewConnectedProxyGetter()

	var expectIDs []string
	ids := proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)

	expectIDs = []string{}
	proxies.setProxyIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)

	expectIDs = []string{"test1", "test2"}
	proxies.setProxyIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)

	expectIDs = nil
	proxies.setProxyIDs(expectIDs)
	ids = proxies.GetProxyIDs()
	require.Equal(t, expectIDs, ids)
}
