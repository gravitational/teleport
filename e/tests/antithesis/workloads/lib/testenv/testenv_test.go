package testenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvDefaults(t *testing.T) {
	// Ensure these are not set already.
	t.Setenv("TELEPORT_CREDS_DIR", "")
	t.Setenv("TELEPORT_PROXY_ADDR", "")
	t.Setenv("TELEPORT_IDENTITY_FILE", "")

	require.Equal(t, DefaultCredsDir, CredsDir())
	require.Equal(t, DefaultProxyAddr, ProxyAddr())
	require.Equal(t, "/creds/admin/identity", IdentityPath("admin"))
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("TELEPORT_CREDS_DIR", "/tmp/creds")
	t.Setenv("TELEPORT_PROXY_ADDR", "proxy.example.com:443")

	require.Equal(t, "/tmp/creds", CredsDir())
	require.Equal(t, "proxy.example.com:443", ProxyAddr())
	require.Equal(t, "/tmp/creds/admin/identity", IdentityPath("admin"))

	t.Setenv("TELEPORT_IDENTITY_FILE", "/tmp/admin.pem")
	require.Equal(t, "/tmp/admin.pem", IdentityPath("admin"))
}
