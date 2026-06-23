package joinclient

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

func TestJoinNewIncludesProxyHintOnConnectionFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	// proxy_server without an explicit port defaults to the HTTP proxy listener port
	// (3080); join hints suggest :443 when that dial fails.
	_, err := joinNew(ctx, JoinParams{
		ProxyServer: utils.NetAddr{
			Addr:        "example." + defaults.CloudDomainSuffix,
			AddrNetwork: "tcp",
		},
		Log: slog.Default(),
	})
	require.Error(t, err)

	t.Logf("joinNew error: %v", err)
	require.Contains(t, err.Error(), "set proxy_server to example."+defaults.CloudDomainSuffix+":443")
}
