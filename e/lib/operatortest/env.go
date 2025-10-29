// Package operatortest is meant for testing the Teleport Kubernetes Operator
// against an enterprise cluster, for any resource types which require an
// enterprise cluster to function and cannot be tested in the OSS repo.
package operatortest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client"
	eauth "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
)

// startAuthServer starts an enterprise auth server which will be cleaned up at
// the end of the test. It returns an admin client for the auth server.
func startAuthServer(t *testing.T) *client.Client {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.OIDC: {Enabled: true},
				entitlements.SAML: {Enabled: true},
			},
			AdvancedAccessWorkflows: true,
		},
	})
	authServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
		// Disable the retry interval to make tests unblock when
		// RunWhileLocked is called.
		RunWhileLockedRetryInterval: -1 * time.Millisecond,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})

	authPlugin, err := eauth.NewPlugin(eauth.Config{
		License: eauth.ValidLicense{},
	})
	require.NoError(t, err)

	registry := plugin.NewRegistry()
	registry.Add(authPlugin)

	server, err := authtest.NewTestTLSServer(authtest.TLSServerConfig{
		APIConfig: &auth.APIConfig{
			PluginRegistry:   registry,
			AuthServer:       authServer.AuthServer,
			Authorizer:       authServer.Authorizer,
			AuditLog:         authServer.AuditLog,
			Emitter:          authServer.AuditLog,
			ScopedAuthorizer: authServer.ScopedAuthorizer,
		},
		AuthServer:    authServer,
		AcceptedUsage: authServer.AcceptedUsage,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, server.Close())
	})

	authClient, err := server.NewClient(authtest.TestAdmin())
	require.NoError(t, err)

	return authClient.APIClient
}
