package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestReconnectPathIncludesProxyHint(t *testing.T) {
	t.Parallel()

	// Build a realistic connector so newClient executes the reconnect path
	// without short-circuiting on missing identity material.
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

	connector, err := newConnector(tlsServer.Identity, tlsServer.Identity)
	require.NoError(t, err)

	supervisor, err := NewSupervisor("test", slog.Default(), clockwork.NewRealClock())
	require.NoError(t, err)

	cfg := &servicecfg.Config{
		Version: defaults.TeleportConfigVersionV3,
		ProxyServer: utils.NetAddr{
			Addr:        "example." + defaults.CloudDomainSuffix,
			AddrNetwork: "tcp",
		},
	}
	process := &TeleportProcess{
		Supervisor: supervisor,
		Clock:      clockwork.NewRealClock(),
		Config:     cfg,
		logger:     slog.Default(),
		resolver: func(ctx context.Context) (*utils.NetAddr, types.ProxyListenerMode, error) {
			return nil, types.ProxyListenerMode_Separate, context.DeadlineExceeded
		},
	}

	_, _, err = process.newClient(connector)
	require.Error(t, err)
	t.Logf("newClient reconnect error: %v", err)
	require.Contains(t, err.Error(), "set proxy_server to \"example."+defaults.CloudDomainSuffix+":443\"")
}
