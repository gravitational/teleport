package auth

import (
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

func newTestTLSServer(t *testing.T, license License, m *modulestest.Modules, opts ...authtest.TestTLSServerOption) *authtest.TLSServer {
	t.Helper()
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:     t.TempDir(),
		Clock:   clockwork.NewFakeClock(),
		Modules: m,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

	srv, err := as.NewTestTLSServer(opts...)
	require.NoError(t, err)

	registerSAMLService(t, &SAMLAuthServiceConfig{Auth: as.AuthServer, License: license})
	registerOIDCService(t, &OIDCAuthServiceConfig{Auth: as.AuthServer, License: license})

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
