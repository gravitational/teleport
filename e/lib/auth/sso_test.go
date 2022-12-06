package auth

import (
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth"
)

func newTestTLSServer(t *testing.T) *auth.TestTLSServer {
	t.Helper()
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClock(),
	})
	require.NoError(t, err)

	srv, err := as.NewTestTLSServer()
	require.NoError(t, err)

	registerSAMLService(t, &SAMLAuthServiceConfig{Auth: as.AuthServer})
	registerOIDCService(t, &OIDCAuthServiceConfig{Auth: as.AuthServer})

	t.Cleanup(func() { require.NoError(t, srv.Close()) })
	return srv
}

func registerSAMLService(t *testing.T, cfg *SAMLAuthServiceConfig) *SAMLAuthService {
	t.Helper()
	sas, err := NewSAMLAuthService(cfg)
	require.NoError(t, err)
	cfg.Auth.SetSAMLService(sas)
	return sas
}

func registerOIDCService(t *testing.T, cfg *OIDCAuthServiceConfig) *OIDCAuthService {
	t.Helper()
	oas, err := NewOIDCAuthService(cfg)
	require.NoError(t, err)
	cfg.Auth.SetOIDCService(oas)
	return oas
}
