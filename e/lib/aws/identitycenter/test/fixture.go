package test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/accesspoint"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

// Fixture holds resources for constructing and testing an
// IdentityCenter service
type Fixture struct {
	Ctx              context.Context
	Backend          backend.Backend
	Clock            clockwork.FakeClock
	Auth             *auth.Server
	SCIMClient       *scimsdk.ClientMock
	ICClient         *icsdk.ClientMock
	PluginService    *local.PluginsService
	PluginStatusSink common.StatusSink
}

type CacheArgs struct {
	Started bool
}

func initCache(srv *auth.Server, args CacheArgs) error {
	// TODO: See if we can trim the requirements for the cache down a bit.

	svces := srv.Services
	var err error
	srv.Cache, err = accesspoint.NewCache(accesspoint.Config{
		Context:      srv.CloseContext(),
		Setup:        cache.ForAuth,
		CacheName:    []string{teleport.ComponentAuth},
		EventsSystem: true,
		Unstarted:    !args.Started,

		Access:                  svces.Access,
		AccessLists:             svces.AccessLists,
		AccessMonitoringRules:   svces.AccessMonitoringRules,
		AppSession:              svces.Identity,
		Apps:                    svces.Apps,
		ClusterConfig:           svces.ClusterConfiguration,
		AutoUpdateService:       svces.AutoUpdateService,
		CrownJewels:             svces.CrownJewels,
		DatabaseObjects:         svces.DatabaseObjects,
		DatabaseServices:        svces.DatabaseServices,
		Databases:               svces.Databases,
		DiscoveryConfigs:        svces.DiscoveryConfigs,
		DynamicAccess:           svces.DynamicAccessExt,
		Events:                  svces.Events,
		IdentityCenter:          svces.IdentityCenter,
		Integrations:            svces.Integrations,
		KubeWaitingContainers:   svces.KubeWaitingContainer,
		Kubernetes:              svces.Kubernetes,
		Notifications:           svces.Notifications,
		Okta:                    svces.Okta,
		Presence:                svces.PresenceInternal,
		Provisioner:             svces.Provisioner,
		ProvisioningStates:      svces.ProvisioningStates,
		Restrictions:            svces.Restrictions,
		SAMLIdPServiceProviders: svces.SAMLIdPServiceProviders,
		SAMLIdPSession:          svces.Identity,
		SecReports:              svces.SecReports,
		SnowflakeSession:        svces.Identity,
		SPIFFEFederations:       svces.SPIFFEFederations,
		StaticHostUsers:         svces.StaticHostUser,
		Trust:                   svces.TrustInternal,
		UserGroups:              svces.UserGroups,
		UserTasks:               svces.UserTasks,
		UserLoginStates:         svces.UserLoginStates,
		Users:                   svces.Identity,
		WebSession:              svces.Identity.WebSessions(),
		WebToken:                svces.WebTokens(),
		WindowsDesktops:         svces.WindowsDesktops,
		DynamicWindowsDesktops:  svces.DynamicWindowsDesktops,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func WithCache(args CacheArgs) func(*auth.Server) error {
	return func(srv *auth.Server) error { return initCache(srv, args) }
}

func NewFixture(t *testing.T, opts ...auth.ServerOption) *Fixture {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: t.Name(),
	})
	require.NoError(t, err)

	opts = append(opts, auth.WithClock(clock))
	auth, err := auth.NewServer(&auth.InitConfig{
		Authority:              authority.New(),
		Backend:                backend,
		ClusterName:            clusterName,
		SkipPeriodicOperations: true,
		VersionStorage:         auth.NewFakeTeleportVersion(),
	}, opts...)
	require.NoError(t, err, "creating Auth server")
	t.Cleanup(func() { require.NoError(t, auth.Close()) })

	scimClient := scimsdk.NewSCIMClientMock()
	icClient := icsdk.NewClientMock(nil /* custom mock data */)

	pluginService := local.NewPluginsService(backend)

	fixture := &Fixture{
		Ctx:              ctx,
		Backend:          backend,
		Clock:            clock,
		Auth:             auth,
		SCIMClient:       scimClient,
		ICClient:         icClient,
		PluginService:    pluginService,
		PluginStatusSink: &integration.FakeStatusSink{},
	}

	return fixture
}
