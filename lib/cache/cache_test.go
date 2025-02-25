/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package cache

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	update "github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/api/types/clusterconfig"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/kubewaitingcontainer"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/srv/db/common/databaseobject"
	"github.com/gravitational/teleport/lib/utils"
)

const eventBufferSize = 1024

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestNodesDontCacheHighVolumeResources verifies that resources classified as "high volume" aren't
// cached by nodes.
func TestNodesDontCacheHighVolumeResources(t *testing.T) {
	for _, kind := range ForNode(Config{}).Watches {
		require.False(t, isHighVolumeResource(kind.Kind), "resource=%q", kind.Kind)
	}
}

// testPack contains pack of
// services used for test run
type testPack struct {
	dataDir      string
	backend      *backend.Wrapper
	eventsC      chan Event
	cache        *Cache
	cacheBackend backend.Backend

	eventsS        *proxyEvents
	trustS         services.Trust
	provisionerS   services.Provisioner
	clusterConfigS services.ClusterConfiguration

	usersS                  services.UsersService
	accessS                 services.Access
	dynamicAccessS          services.DynamicAccessCore
	presenceS               services.Presence
	appSessionS             services.AppSession
	snowflakeSessionS       services.SnowflakeSession
	samlIdPSessionsS        services.SAMLIdPSession //nolint:revive // Because we want this to be IdP.
	restrictions            services.Restrictions
	apps                    services.Apps
	kubernetes              services.Kubernetes
	databases               services.Databases
	databaseServices        services.DatabaseServices
	webSessionS             types.WebSessionInterface
	webTokenS               types.WebTokenInterface
	windowsDesktops         services.WindowsDesktops
	dynamicWindowsDesktops  services.DynamicWindowsDesktops
	samlIDPServiceProviders services.SAMLIdPServiceProviders
	userGroups              services.UserGroups
	okta                    services.Okta
	integrations            services.Integrations
	userTasks               services.UserTasks
	discoveryConfigs        services.DiscoveryConfigs
	userLoginStates         services.UserLoginStates
	secReports              services.SecReports
	accessLists             services.AccessLists
	kubeWaitingContainers   services.KubeWaitingContainer
	notifications           services.Notifications
	accessMonitoringRules   services.AccessMonitoringRules
	crownJewels             services.CrownJewels
	databaseObjects         services.DatabaseObjects
	spiffeFederations       *local.SPIFFEFederationService
	staticHostUsers         services.StaticHostUser
	autoUpdateService       services.AutoUpdateService
	provisioningStates      services.ProvisioningStates
	identityCenter          services.IdentityCenter
	pluginStaticCredentials *local.PluginStaticCredentialsService
	gitServers              services.GitServers
	workloadIdentity        *local.WorkloadIdentityService
}

// testFuncs are functions to support testing an object in a cache.
type testFuncs[T types.Resource] struct {
	newResource    func(string) (T, error)
	create         func(context.Context, T) error
	list           func(context.Context) ([]T, error)
	cacheGet       func(context.Context, string) (T, error)
	cacheList      func(context.Context) ([]T, error)
	update         func(context.Context, T) error
	deleteAll      func(context.Context) error
	changeResource func(T)
}

// testFuncs153 are functions to support testing an RFD153-style resource in a cache.
type testFuncs153[T types.Resource153] struct {
	newResource func(string) (T, error)
	create      func(context.Context, T) error
	list        func(context.Context) ([]T, error)
	cacheGet    func(context.Context, string) (T, error)
	cacheList   func(context.Context) ([]T, error)
	update      func(context.Context, T) error
	delete      func(context.Context, string) error
	deleteAll   func(context.Context) error
}

func (t *testPack) Close() {
	var errors []error
	if t.backend != nil {
		errors = append(errors, t.backend.Close())
	}
	if t.cache != nil {
		errors = append(errors, t.cache.Close())
	}
	if err := trace.NewAggregate(errors...); err != nil {
		slog.WarnContext(context.Background(), "Failed to close", "error", err)
	}
}

func newPackForAuth(t *testing.T) *testPack {
	return newTestPack(t, ForAuth)
}

func newTestPack(t *testing.T, setupConfig SetupConfigFn) *testPack {
	pack, err := newPack(t.TempDir(), setupConfig)
	require.NoError(t, err)
	return pack
}

func newTestPackWithoutCache(t *testing.T) *testPack {
	pack, err := newPackWithoutCache(t.TempDir())
	require.NoError(t, err)
	return pack
}

type packCfg struct {
	memoryBackend bool
	ignoreKinds   []types.WatchKind
}

type packOption func(cfg *packCfg)

func memoryBackend(bool) packOption {
	return func(cfg *packCfg) {
		cfg.memoryBackend = true
	}
}

// ignoreKinds specifies the list of kinds that should be removed from the watch request by eventsProxy
// to simulate cache resource type rejection due to version incompatibility.
func ignoreKinds(kinds []types.WatchKind) packOption {
	return func(cfg *packCfg) {
		cfg.ignoreKinds = kinds
	}
}

// newPackWithoutCache returns a new test pack without creating cache
func newPackWithoutCache(dir string, opts ...packOption) (*testPack, error) {
	ctx := context.Background()
	var cfg packCfg
	for _, opt := range opts {
		opt(&cfg)
	}

	p := &testPack{
		dataDir: dir,
	}
	var bk backend.Backend
	var err error
	if cfg.memoryBackend {
		bk, err = memory.New(memory.Config{
			Context: ctx,
			Mirror:  true,
		})
	} else {
		bk, err = lite.NewWithConfig(ctx, lite.Config{
			Path:             p.dataDir,
			PollStreamPeriod: 200 * time.Millisecond,
		})
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.backend = backend.NewWrapper(bk)

	p.cacheBackend, err = memory.New(
		memory.Config{
			Context: ctx,
			Mirror:  true,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.eventsC = make(chan Event, eventBufferSize)

	clusterConfig, err := local.NewClusterConfigurationService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	idService, err := local.NewTestIdentityService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dynamicWindowsDesktopService, err := local.NewDynamicWindowsDesktopService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.trustS = local.NewCAService(p.backend)
	p.clusterConfigS = clusterConfig
	p.provisionerS = local.NewProvisioningService(p.backend)
	p.eventsS = newProxyEvents(local.NewEventsService(p.backend), cfg.ignoreKinds)
	p.presenceS = local.NewPresenceService(p.backend)
	p.usersS = idService
	p.accessS = local.NewAccessService(p.backend)
	p.dynamicAccessS = local.NewDynamicAccessService(p.backend)
	p.appSessionS = idService
	p.webSessionS = idService.WebSessions()
	p.snowflakeSessionS = idService
	p.samlIdPSessionsS = idService
	p.webTokenS = idService.WebTokens()
	p.restrictions = local.NewRestrictionsService(p.backend)
	p.apps = local.NewAppService(p.backend)
	p.kubernetes = local.NewKubernetesService(p.backend)
	p.databases = local.NewDatabasesService(p.backend)
	p.databaseServices = local.NewDatabaseServicesService(p.backend)
	p.windowsDesktops = local.NewWindowsDesktopService(p.backend)
	p.dynamicWindowsDesktops = dynamicWindowsDesktopService
	p.samlIDPServiceProviders, err = local.NewSAMLIdPServiceProviderService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.userGroups, err = local.NewUserGroupService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oktaSvc, err := local.NewOktaService(p.backend, p.backend.Clock())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.okta = oktaSvc

	igSvc, err := local.NewIntegrationsService(p.backend, local.WithIntegrationsServiceCacheMode(true))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.integrations = igSvc

	userTasksSvc, err := local.NewUserTasksService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.userTasks = userTasksSvc

	dcSvc, err := local.NewDiscoveryConfigService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.discoveryConfigs = dcSvc

	ulsSvc, err := local.NewUserLoginStateService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.userLoginStates = ulsSvc

	secReportsSvc, err := local.NewSecReportsService(p.backend, p.backend.Clock())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.secReports = secReportsSvc

	accessListsSvc, err := local.NewAccessListService(p.backend, p.backend.Clock())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.accessLists = accessListsSvc
	accessMonitoringRuleService, err := local.NewAccessMonitoringRulesService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.accessMonitoringRules = accessMonitoringRuleService

	crownJewelsSvc, err := local.NewCrownJewelsService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.crownJewels = crownJewelsSvc

	spiffeFederationsSvc, err := local.NewSPIFFEFederationService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.spiffeFederations = spiffeFederationsSvc

	workloadIdentitySvc, err := local.NewWorkloadIdentityService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.workloadIdentity = workloadIdentitySvc

	databaseObjectsSvc, err := local.NewDatabaseObjectService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.databaseObjects = databaseObjectsSvc

	kubeWaitingContSvc, err := local.NewKubeWaitingContainerService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.kubeWaitingContainers = kubeWaitingContSvc
	notificationsSvc, err := local.NewNotificationsService(p.backend, p.backend.Clock())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.notifications = notificationsSvc

	staticHostUserService, err := local.NewStaticHostUserService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.staticHostUsers = staticHostUserService

	p.autoUpdateService, err = local.NewAutoUpdateService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.provisioningStates, err = local.NewProvisioningStateService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.identityCenter, err = local.NewIdentityCenterService(local.IdentityCenterServiceConfig{
		Backend: p.backend,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.pluginStaticCredentials, err = local.NewPluginStaticCredentialsService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.gitServers, err = local.NewGitServerService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return p, nil
}

// newPack returns a new test pack or fails the test on error
func newPack(dir string, setupConfig func(c Config) Config, opts ...packOption) (*testPack, error) {
	ctx := context.Background()
	p, err := newPackWithoutCache(dir, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.cache, err = New(setupConfig(Config{
		Context:                 ctx,
		Backend:                 p.cacheBackend,
		Events:                  p.eventsS,
		ClusterConfig:           p.clusterConfigS,
		Provisioner:             p.provisionerS,
		Trust:                   p.trustS,
		Users:                   p.usersS,
		Access:                  p.accessS,
		DynamicAccess:           p.dynamicAccessS,
		Presence:                p.presenceS,
		AppSession:              p.appSessionS,
		WebSession:              p.webSessionS,
		WebToken:                p.webTokenS,
		SnowflakeSession:        p.snowflakeSessionS,
		SAMLIdPSession:          p.samlIdPSessionsS,
		Restrictions:            p.restrictions,
		Apps:                    p.apps,
		Kubernetes:              p.kubernetes,
		DatabaseServices:        p.databaseServices,
		Databases:               p.databases,
		WindowsDesktops:         p.windowsDesktops,
		DynamicWindowsDesktops:  p.dynamicWindowsDesktops,
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		UserTasks:               p.userTasks,
		DiscoveryConfigs:        p.discoveryConfigs,
		UserLoginStates:         p.userLoginStates,
		SecReports:              p.secReports,
		AccessLists:             p.accessLists,
		KubeWaitingContainers:   p.kubeWaitingContainers,
		Notifications:           p.notifications,
		AccessMonitoringRules:   p.accessMonitoringRules,
		CrownJewels:             p.crownJewels,
		SPIFFEFederations:       p.spiffeFederations,
		DatabaseObjects:         p.databaseObjects,
		StaticHostUsers:         p.staticHostUsers,
		AutoUpdateService:       p.autoUpdateService,
		ProvisioningStates:      p.provisioningStates,
		IdentityCenter:          p.identityCenter,
		PluginStaticCredentials: p.pluginStaticCredentials,
		GitServers:              p.gitServers,
		WorkloadIdentity:        p.workloadIdentity,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 p.eventsC,
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	select {
	case event := <-p.eventsC:

		if event.Type != WatcherStarted {
			return nil, trace.CompareFailed("%q != %q %s", event.Type, WatcherStarted, event)
		}
	case <-time.After(time.Second):
		return nil, trace.ConnectionProblem(nil, "wait for the watcher to start")
	}
	return p, nil
}

// TestCA tests certificate authorities
func TestCA(t *testing.T) {
	t.Parallel()

	p := newPackForAuth(t)
	t.Cleanup(p.Close)
	ctx := context.Background()

	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(ca, out, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	err = p.trustS.DeleteCertAuthority(ctx, ca.GetID())
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetCertAuthority(ctx, ca.GetID(), false)
	require.True(t, trace.IsNotFound(err))
}

// TestWatchers tests watchers connected to the cache,
// verifies that all watchers of the cache will be closed
// if the underlying watcher to the target backend is closed
func TestWatchers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	w, err := p.cache.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{
		{
			Kind: types.KindCertAuthority,
			Filter: types.CertAuthorityFilter{
				types.HostCA: "example.com",
				types.UserCA: types.Wildcard,
			}.IntoMap(),
		},
		{
			Kind: types.KindAccessRequest,
			Filter: map[string]string{
				"user": "alice",
			},
		},
	}})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, w.Close())
	})

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Timeout waiting for event.")
	}

	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpPut, e.Type)
		require.Equal(t, types.KindCertAuthority, e.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for event.")
	}

	// create an access request that matches the supplied filter
	req, err := services.NewAccessRequest("alice", "dictator")
	require.NoError(t, err)

	req, err = p.dynamicAccessS.CreateAccessRequestV2(ctx, req)
	require.NoError(t, err)

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpPut, e.Type)
		require.Equal(t, types.KindAccessRequest, e.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for event.")
	}

	require.NoError(t, p.dynamicAccessS.DeleteAccessRequest(ctx, req.GetName()))

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpDelete, e.Type)
		require.Equal(t, types.KindAccessRequest, e.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for event.")
	}

	// create an access request that does not match the supplied filter
	req2, err := services.NewAccessRequest("bob", "dictator")
	require.NoError(t, err)

	// create and then delete the non-matching request.
	req2, err = p.dynamicAccessS.CreateAccessRequestV2(ctx, req2)
	require.NoError(t, err)
	require.NoError(t, p.dynamicAccessS.DeleteAccessRequest(ctx, req2.GetName()))

	// because our filter did not match the request, the create event should never
	// have been created, meaning that the next event on the pipe is the delete
	// event (which cannot be filtered out because username is not visible inside
	// a delete event).
	select {
	case e := <-w.Events():
		require.Equal(t, types.OpDelete, e.Type)
		require.Equal(t, types.KindAccessRequest, e.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for event.")
	}

	// this ca will not be matched by our filter, so the same reasoning applies
	// as we upsert it and delete it
	filteredCa := suite.NewTestCA(types.HostCA, "example.net")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, filteredCa))
	require.NoError(t, p.trustS.DeleteCertAuthority(ctx, filteredCa.GetID()))

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpDelete, e.Type)
		require.Equal(t, types.KindCertAuthority, e.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for event.")
	}

	// event has arrived, now close the watchers
	p.backend.CloseWatchers()

	// make sure watcher has been closed
	select {
	case <-w.Done():
	case <-time.After(time.Second):
		t.Fatalf("Timeout waiting for close event.")
	}
}

func TestNodeCAFiltering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	require.NoError(t, err)
	err = p.cache.clusterConfigCache.UpsertClusterName(clusterName)
	require.NoError(t, err)

	nodeCacheBackend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeCacheBackend.Close()) })

	// this mimics a cache for a node pulling events from the auth server via WatchEvents
	nodeCache, err := New(ForNode(Config{
		Events:                  p.cache,
		Trust:                   p.cache.trustCache,
		ClusterConfig:           p.cache.clusterConfigCache,
		Provisioner:             p.cache.provisionerCache,
		Users:                   p.cache.usersCache,
		Access:                  p.cache.accessCache,
		DynamicAccess:           p.cache.dynamicAccessCache,
		Presence:                p.cache.presenceCache,
		Restrictions:            p.cache.restrictionsCache,
		Apps:                    p.cache.appsCache,
		Kubernetes:              p.cache.kubernetesCache,
		Databases:               p.cache.databasesCache,
		DatabaseServices:        p.cache.databaseServicesCache,
		AppSession:              p.cache.appSessionCache,
		WebSession:              p.cache.webSessionCache,
		WebToken:                p.cache.webTokenCache,
		WindowsDesktops:         p.cache.windowsDesktopsCache,
		DynamicWindowsDesktops:  p.cache.dynamicWindowsDesktopsCache,
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		StaticHostUsers:         p.staticHostUsers,
		Backend:                 nodeCacheBackend,
	}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeCache.Close()) })

	cacheWatcher, err := nodeCache.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{
		{
			Kind:   types.KindCertAuthority,
			Filter: map[string]string{"host": "example.com", "user": "*"},
		},
	}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cacheWatcher.Close()) })

	fetchEvent := func() types.Event {
		var ev types.Event
		select {
		case ev = <-cacheWatcher.Events():
		case <-time.After(time.Second * 5):
			t.Fatal("watcher timeout")
		}
		return ev
	}
	require.Equal(t, types.OpInit, fetchEvent().Type)

	// upsert and delete a local host CA, we expect to see a Put and a Delete event
	localCA := suite.NewTestCA(types.HostCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, localCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(ctx, localCA.GetID()))

	ev := fetchEvent()
	require.Equal(t, types.OpPut, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.com", ev.Resource.GetName())

	ev = fetchEvent()
	require.Equal(t, types.OpDelete, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.com", ev.Resource.GetName())

	// upsert and delete a nonlocal host CA, we expect to only see the Delete event
	nonlocalCA := suite.NewTestCA(types.HostCA, "example.net")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, nonlocalCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(ctx, nonlocalCA.GetID()))

	ev = fetchEvent()
	require.Equal(t, types.OpDelete, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.net", ev.Resource.GetName())

	// whereas we expect to see the Put and Delete for a trusted *user* CA
	trustedUserCA := suite.NewTestCA(types.UserCA, "example.net")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, trustedUserCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(ctx, trustedUserCA.GetID()))

	ev = fetchEvent()
	require.Equal(t, types.OpPut, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.net", ev.Resource.GetName())

	ev = fetchEvent()
	require.Equal(t, types.OpDelete, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.net", ev.Resource.GetName())
}

func waitForRestart(t *testing.T, eventsC <-chan Event) {
	expectEvent(t, eventsC, WatcherStarted)
}

func drainEvents(eventsC <-chan Event) {
	for {
		select {
		case <-eventsC:
		default:
			return
		}
	}
}

func expectEvent(t *testing.T, eventsC <-chan Event, expectedEvent string) {
	timeC := time.After(5 * time.Second)
	for {
		select {
		case event := <-eventsC:
			if event.Type == expectedEvent {
				return
			}
		case <-timeC:
			t.Fatalf("Timeout waiting for expected event: %s", expectedEvent)
		}
	}
}

func unexpectedEvent(t *testing.T, eventsC <-chan Event, unexpectedEvent string) {
	timeC := time.After(time.Second)
	for {
		select {
		case event := <-eventsC:
			if event.Type == unexpectedEvent {
				t.Fatalf("Received unexpected event: %s", unexpectedEvent)
			}
		case <-timeC:
			return
		}
	}
}

func expectNextEvent(t *testing.T, eventsC <-chan Event, expectedEvent string, skipEvents ...string) {
	timeC := time.After(5 * time.Second)
	for {
		// wait for watcher to restart
		select {
		case event := <-eventsC:
			if slices.Contains(skipEvents, event.Type) {
				continue
			}
			require.Equal(t, expectedEvent, event.Type)
			return
		case <-timeC:
			t.Fatalf("Timeout waiting for expected event: %s", expectedEvent)
		}
	}
}

// TestCompletenessInit verifies that flaky backends don't cause
// the cache to return partial results during init.
func TestCompletenessInit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const caCount = 100
	const inits = 20
	p := newTestPackWithoutCache(t)
	t.Cleanup(p.Close)

	// put lots of CAs in the backend
	for i := 0; i < caCount; i++ {
		ca := suite.NewTestCA(types.UserCA, fmt.Sprintf("%d.example.com", i))
		require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))
	}

	for i := 0; i < inits; i++ {
		var err error

		p.cacheBackend, err = memory.New(
			memory.Config{
				Context: ctx,
				Mirror:  true,
			})
		require.NoError(t, err)

		// simulate bad connection to auth server
		p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
		p.eventsS.closeWatchers()

		p.cache, err = New(ForAuth(Config{
			Context:                 ctx,
			Backend:                 p.cacheBackend,
			Events:                  p.eventsS,
			ClusterConfig:           p.clusterConfigS,
			Provisioner:             p.provisionerS,
			Trust:                   p.trustS,
			Users:                   p.usersS,
			Access:                  p.accessS,
			DynamicAccess:           p.dynamicAccessS,
			Presence:                p.presenceS,
			AppSession:              p.appSessionS,
			WebSession:              p.webSessionS,
			SnowflakeSession:        p.snowflakeSessionS,
			SAMLIdPSession:          p.samlIdPSessionsS,
			WebToken:                p.webTokenS,
			Restrictions:            p.restrictions,
			Apps:                    p.apps,
			Kubernetes:              p.kubernetes,
			DatabaseServices:        p.databaseServices,
			Databases:               p.databases,
			WindowsDesktops:         p.windowsDesktops,
			DynamicWindowsDesktops:  p.dynamicWindowsDesktops,
			SAMLIdPServiceProviders: p.samlIDPServiceProviders,
			UserGroups:              p.userGroups,
			Okta:                    p.okta,
			Integrations:            p.integrations,
			UserTasks:               p.userTasks,
			DiscoveryConfigs:        p.discoveryConfigs,
			UserLoginStates:         p.userLoginStates,
			SecReports:              p.secReports,
			AccessLists:             p.accessLists,
			KubeWaitingContainers:   p.kubeWaitingContainers,
			Notifications:           p.notifications,
			AccessMonitoringRules:   p.accessMonitoringRules,
			CrownJewels:             p.crownJewels,
			DatabaseObjects:         p.databaseObjects,
			SPIFFEFederations:       p.spiffeFederations,
			StaticHostUsers:         p.staticHostUsers,
			AutoUpdateService:       p.autoUpdateService,
			ProvisioningStates:      p.provisioningStates,
			WorkloadIdentity:        p.workloadIdentity,
			MaxRetryPeriod:          200 * time.Millisecond,
			IdentityCenter:          p.identityCenter,
			PluginStaticCredentials: p.pluginStaticCredentials,
			EventsC:                 p.eventsC,
			GitServers:              p.gitServers,
		}))
		require.NoError(t, err)

		p.backend.SetReadError(nil)

		cas, err := p.cache.GetCertAuthorities(ctx, types.UserCA, false)
		// we don't actually care whether the cache ever fully constructed
		// the CA list.  for the purposes of this test, we just care that it
		// doesn't return the CA list *unless* it was successfully constructed.
		if err == nil {
			require.Len(t, cas, caCount)
		} else {
			require.True(t, trace.IsConnectionProblem(err))
		}

		require.NoError(t, p.cache.Close())
		p.cache = nil
		require.NoError(t, p.cacheBackend.Close())
		p.cacheBackend = nil
	}
}

// TestCompletenessReset verifies that flaky backends don't cause
// the cache to return partial results during reset.
func TestCompletenessReset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	const caCount = 100
	const resets = 20
	p := newTestPackWithoutCache(t)
	t.Cleanup(p.Close)

	// put lots of CAs in the backend
	for i := 0; i < caCount; i++ {
		ca := suite.NewTestCA(types.UserCA, fmt.Sprintf("%d.example.com", i))
		require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))
	}

	var err error
	p.cache, err = New(ForAuth(Config{
		Context:                 ctx,
		Backend:                 p.cacheBackend,
		Events:                  p.eventsS,
		ClusterConfig:           p.clusterConfigS,
		Provisioner:             p.provisionerS,
		Trust:                   p.trustS,
		Users:                   p.usersS,
		Access:                  p.accessS,
		DynamicAccess:           p.dynamicAccessS,
		Presence:                p.presenceS,
		AppSession:              p.appSessionS,
		WebSession:              p.webSessionS,
		SnowflakeSession:        p.snowflakeSessionS,
		SAMLIdPSession:          p.samlIdPSessionsS,
		WebToken:                p.webTokenS,
		Restrictions:            p.restrictions,
		Apps:                    p.apps,
		Kubernetes:              p.kubernetes,
		DatabaseServices:        p.databaseServices,
		Databases:               p.databases,
		WindowsDesktops:         p.windowsDesktops,
		DynamicWindowsDesktops:  p.dynamicWindowsDesktops,
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		UserTasks:               p.userTasks,
		DiscoveryConfigs:        p.discoveryConfigs,
		UserLoginStates:         p.userLoginStates,
		SecReports:              p.secReports,
		AccessLists:             p.accessLists,
		KubeWaitingContainers:   p.kubeWaitingContainers,
		Notifications:           p.notifications,
		AccessMonitoringRules:   p.accessMonitoringRules,
		CrownJewels:             p.crownJewels,
		DatabaseObjects:         p.databaseObjects,
		SPIFFEFederations:       p.spiffeFederations,
		StaticHostUsers:         p.staticHostUsers,
		AutoUpdateService:       p.autoUpdateService,
		ProvisioningStates:      p.provisioningStates,
		IdentityCenter:          p.identityCenter,
		PluginStaticCredentials: p.pluginStaticCredentials,
		WorkloadIdentity:        p.workloadIdentity,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 p.eventsC,
		GitServers:              p.gitServers,
	}))
	require.NoError(t, err)

	// verify that CAs are immediately available
	cas, err := p.cache.GetCertAuthorities(ctx, types.UserCA, false)
	require.NoError(t, err)
	require.Len(t, cas, caCount)

	for i := 0; i < resets; i++ {
		// simulate bad connection to auth server
		p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
		p.eventsS.closeWatchers()
		p.backend.SetReadError(nil)

		// load CAs while connection is bad
		cas, err := p.cache.GetCertAuthorities(ctx, types.UserCA, false)
		// we don't actually care whether the cache ever fully constructed
		// the CA list.  for the purposes of this test, we just care that it
		// doesn't return the CA list *unless* it was successfully constructed.
		if err == nil {
			require.Len(t, cas, caCount)
		} else {
			require.True(t, trace.IsConnectionProblem(err))
		}
	}
}

// TestInitStrategy verifies that cache uses expected init strategy
// of serving backend state when init is taking too long.
func TestInitStrategy(t *testing.T) {
	t.Parallel()

	for i := 0; i < utils.GetIterations(); i++ {
		initStrategy(t)
	}
}

/*
goos: linux
goarch: amd64
pkg: github.com/gravitational/teleport/lib/cache
cpu: Intel(R) Core(TM) i9-10885H CPU @ 2.40GHz
BenchmarkGetMaxNodes-16     	       1	1029199093 ns/op
*/
func BenchmarkGetMaxNodes(b *testing.B) {
	benchGetNodes(b, 1_000_000)
}

func benchGetNodes(b *testing.B, nodeCount int) {
	p, err := newPack(b.TempDir(), ForAuth, memoryBackend(true))
	require.NoError(b, err)
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	createErr := make(chan error, 1)

	go func() {
		for i := 0; i < nodeCount; i++ {
			server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
			_, err := p.presenceS.UpsertNode(ctx, server)
			if err != nil {
				createErr <- err
				return
			}
		}
	}()

	timeout := time.After(time.Second * 90)

	for i := 0; i < nodeCount; i++ {
		select {
		case event := <-p.eventsC:
			require.Equal(b, EventProcessed, event.Type)
		case err := <-createErr:
			b.Fatalf("failed to create node: %v", err)
		case <-timeout:
			b.Fatalf("timeout waiting for event, progress=%d", i)
		}
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		nodes, err := p.cache.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(b, err)
		require.Len(b, nodes, nodeCount)
	}
}

/*
goos: linux
goarch: amd64
pkg: github.com/gravitational/teleport/lib/cache
cpu: Intel(R) Core(TM) i7-8550U CPU @ 1.80GHz
BenchmarkListResourcesWithSort-8               1        2351035036 ns/op
*/
func BenchmarkListResourcesWithSort(b *testing.B) {
	p, err := newPack(b.TempDir(), ForAuth, memoryBackend(true))
	require.NoError(b, err)
	defer p.Close()

	ctx := context.Background()

	count := 100000
	for i := 0; i < count; i++ {
		server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
		// Set some static and dynamic labels.
		server.Metadata.Labels = map[string]string{"os": "mac", "env": "prod", "country": "us", "tier": "frontend"}
		server.Spec.CmdLabels = map[string]types.CommandLabelV2{
			"version": {Result: "v8"},
			"time":    {Result: "now"},
		}
		_, err := p.presenceS.UpsertNode(ctx, server)
		require.NoError(b, err)

		select {
		case event := <-p.eventsC:
			require.Equal(b, EventProcessed, event.Type)
		case <-time.After(200 * time.Millisecond):
			b.Fatalf("timeout waiting for event, iteration=%d", i)
		}
	}

	b.ResetTimer()

	for _, limit := range []int32{100, 1_000, 10_000, 100_000} {
		for _, totalCount := range []bool{true, false} {
			b.Run(fmt.Sprintf("limit=%d,needTotal=%t", limit, totalCount), func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					resp, err := p.cache.ListResources(ctx, proto.ListResourcesRequest{
						ResourceType: types.KindNode,
						Namespace:    apidefaults.Namespace,
						SortBy: types.SortBy{
							IsDesc: true,
							Field:  types.ResourceSpecHostname,
						},
						// Predicate is the more expensive filter.
						PredicateExpression: `search("mac", "frontend") && labels.version == "v8"`,
						Limit:               limit,
						NeedTotalCount:      totalCount,
					})
					require.NoError(b, err)
					require.Len(b, resp.Resources, int(limit))
				}
			})
		}
	}
}

// TestListResources_NodesTTLVariant verifies that the custom ListNodes impl that we fallback to when
// using ttl-based caching works as expected.
func TestListResources_NodesTTLVariant(t *testing.T) {
	t.Parallel()

	const nodeCount = 100
	const pageSize = 10
	var err error

	ctx := context.Background()

	p, err := newPackWithoutCache(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(p.Close)

	p.cache, err = New(ForAuth(Config{
		Context:                 ctx,
		Backend:                 p.cacheBackend,
		Events:                  p.eventsS,
		ClusterConfig:           p.clusterConfigS,
		Provisioner:             p.provisionerS,
		Trust:                   p.trustS,
		Users:                   p.usersS,
		Access:                  p.accessS,
		DynamicAccess:           p.dynamicAccessS,
		Presence:                p.presenceS,
		AppSession:              p.appSessionS,
		WebSession:              p.webSessionS,
		WebToken:                p.webTokenS,
		SnowflakeSession:        p.snowflakeSessionS,
		SAMLIdPSession:          p.samlIdPSessionsS,
		Restrictions:            p.restrictions,
		Apps:                    p.apps,
		Kubernetes:              p.kubernetes,
		DatabaseServices:        p.databaseServices,
		Databases:               p.databases,
		WindowsDesktops:         p.windowsDesktops,
		DynamicWindowsDesktops:  p.dynamicWindowsDesktops,
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		UserTasks:               p.userTasks,
		DiscoveryConfigs:        p.discoveryConfigs,
		UserLoginStates:         p.userLoginStates,
		SecReports:              p.secReports,
		AccessLists:             p.accessLists,
		KubeWaitingContainers:   p.kubeWaitingContainers,
		Notifications:           p.notifications,
		AccessMonitoringRules:   p.accessMonitoringRules,
		CrownJewels:             p.crownJewels,
		DatabaseObjects:         p.databaseObjects,
		SPIFFEFederations:       p.spiffeFederations,
		StaticHostUsers:         p.staticHostUsers,
		AutoUpdateService:       p.autoUpdateService,
		ProvisioningStates:      p.provisioningStates,
		IdentityCenter:          p.identityCenter,
		PluginStaticCredentials: p.pluginStaticCredentials,
		WorkloadIdentity:        p.workloadIdentity,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 p.eventsC,
		neverOK:                 true, // ensure reads are never healthy
		GitServers:              p.gitServers,
	}))
	require.NoError(t, err)

	for i := 0; i < nodeCount; i++ {
		server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
		_, err := p.presenceS.UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	time.Sleep(time.Second * 2)

	allNodes, err := p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, allNodes, nodeCount)

	var resources []types.ResourceWithLabels
	var listResourcesStartKey string
	sortBy := types.SortBy{
		Field:  types.ResourceMetadataName,
		IsDesc: true,
	}
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := p.cache.ListResources(ctx, proto.ListResourcesRequest{
			Namespace:    apidefaults.Namespace,
			ResourceType: types.KindNode,
			StartKey:     listResourcesStartKey,
			Limit:        int32(pageSize),
			SortBy:       sortBy,
		})
		assert.NoError(t, err)

		resources = append(resources, resp.Resources...)
		listResourcesStartKey = resp.NextKey
		assert.Len(t, resources, nodeCount)
	}, 5*time.Second, 100*time.Millisecond)

	servers, err := types.ResourcesWithLabels(resources).AsServers()
	require.NoError(t, err)
	fieldVals, err := types.Servers(servers).GetFieldVals(sortBy.Field)
	require.NoError(t, err)
	require.IsDecreasing(t, fieldVals)
}

func initStrategy(t *testing.T) {
	ctx := context.Background()
	p := newTestPackWithoutCache(t)
	t.Cleanup(p.Close)

	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is out"))
	var err error
	p.cache, err = New(ForAuth(Config{
		Context:                 ctx,
		Backend:                 p.cacheBackend,
		Events:                  p.eventsS,
		ClusterConfig:           p.clusterConfigS,
		Provisioner:             p.provisionerS,
		Trust:                   p.trustS,
		Users:                   p.usersS,
		Access:                  p.accessS,
		DynamicAccess:           p.dynamicAccessS,
		Presence:                p.presenceS,
		AppSession:              p.appSessionS,
		SnowflakeSession:        p.snowflakeSessionS,
		SAMLIdPSession:          p.samlIdPSessionsS,
		WebSession:              p.webSessionS,
		WebToken:                p.webTokenS,
		Restrictions:            p.restrictions,
		Apps:                    p.apps,
		Kubernetes:              p.kubernetes,
		DatabaseServices:        p.databaseServices,
		Databases:               p.databases,
		WindowsDesktops:         p.windowsDesktops,
		DynamicWindowsDesktops:  p.dynamicWindowsDesktops,
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		UserTasks:               p.userTasks,
		DiscoveryConfigs:        p.discoveryConfigs,
		UserLoginStates:         p.userLoginStates,
		SecReports:              p.secReports,
		AccessLists:             p.accessLists,
		KubeWaitingContainers:   p.kubeWaitingContainers,
		Notifications:           p.notifications,
		AccessMonitoringRules:   p.accessMonitoringRules,
		CrownJewels:             p.crownJewels,
		DatabaseObjects:         p.databaseObjects,
		SPIFFEFederations:       p.spiffeFederations,
		StaticHostUsers:         p.staticHostUsers,
		AutoUpdateService:       p.autoUpdateService,
		ProvisioningStates:      p.provisioningStates,
		IdentityCenter:          p.identityCenter,
		PluginStaticCredentials: p.pluginStaticCredentials,
		WorkloadIdentity:        p.workloadIdentity,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 p.eventsC,
		GitServers:              p.gitServers,
	}))
	require.NoError(t, err)

	_, err = p.cache.GetCertAuthorities(ctx, types.UserCA, false)
	require.True(t, trace.IsConnectionProblem(err))

	ca := suite.NewTestCA(types.UserCA, "example.com")
	// NOTE 1: this could produce event processed
	// below, based on whether watcher restarts to get the event
	// or not, which is normal, but has to be accounted below
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))
	p.backend.SetReadError(nil)

	// wait for watcher to restart
	waitForRestart(t, p.eventsC)

	normalizeCA := func(ca types.CertAuthority) types.CertAuthority {
		ca = ca.Clone()
		ca.SetExpiry(time.Time{})
		types.RemoveCASecrets(ca)
		return ca
	}
	_ = normalizeCA

	out, err := p.cache.GetCertAuthority(ctx, ca.GetID(), false)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(normalizeCA(ca), normalizeCA(out), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// fail again, make sure last recent data is still served
	// on errors
	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
	p.eventsS.closeWatchers()
	// wait for the watcher to fail
	// there could be optional event processed event,
	// see NOTE 1 above
	expectNextEvent(t, p.eventsC, WatcherFailed, EventProcessed, Reloading)

	// backend is out, but old value is available
	out2, err := p.cache.GetCertAuthority(ctx, ca.GetID(), false)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(normalizeCA(ca), normalizeCA(out2), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	// add modification and expect the resource to recover
	ca.SetRoleMap(types.RoleMap{types.RoleMapping{Remote: "test", Local: []string{"local-test"}}})
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))

	// now, recover the backend and make sure the
	// service is back and the new value has propagated
	p.backend.SetReadError(nil)

	// wait for watcher to restart successfully; ignoring any failed
	// attempts which occurred before backend became healthy again.
	expectEvent(t, p.eventsC, WatcherStarted)

	// new value is available now
	out, err = p.cache.GetCertAuthority(ctx, ca.GetID(), false)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(normalizeCA(ca), normalizeCA(out), cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

// TestRecovery tests error recovery scenario
func TestRecovery(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	// event has arrived, now close the watchers
	watchers := p.eventsS.getWatchers()
	require.Len(t, watchers, 1)
	p.eventsS.closeWatchers()

	// wait for watcher to restart
	waitForRestart(t, p.eventsC)

	// add modification and expect the resource to recover
	ca2 := suite.NewTestCA(types.UserCA, "example2.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca2))

	// wait for watcher to receive an event
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetCertAuthority(context.Background(), ca2.GetID(), false)
	require.NoError(t, err)
	types.RemoveCASecrets(ca2)
	require.Empty(t, cmp.Diff(ca2, out, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

// TestTokens tests static and dynamic tokens
func TestTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Token:   "static1",
				Roles:   types.SystemRoles{types.RoleAuth, types.RoleNode},
				Expires: time.Now().UTC().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	err = p.clusterConfigS.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetStaticTokens()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(staticTokens, out, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	expires := time.Now().Add(10 * time.Hour).Truncate(time.Second).UTC()
	token, err := types.NewProvisionToken("token", types.SystemRoles{types.RoleAuth, types.RoleNode}, expires)
	require.NoError(t, err)

	err = p.provisionerS.UpsertToken(ctx, token)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	tout, err := p.cache.GetToken(ctx, token.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(token, tout, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	err = p.provisionerS.DeleteToken(ctx, token.GetName())
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetToken(ctx, token.GetName())
	require.True(t, trace.IsNotFound(err))
}

func TestAuthPreference(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	authPref, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		AllowLocalAuth:  types.NewBoolOption(true),
		MessageOfTheDay: "test MOTD",
	})
	require.NoError(t, err)
	authPref, err = p.clusterConfigS.UpsertAuthPreference(ctx, authPref)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterAuthPreference, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outAuthPref, err := p.cache.GetAuthPreference(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outAuthPref, authPref, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestClusterNetworkingConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	netConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout:        types.Duration(time.Minute),
		ClientIdleTimeoutMessage: "test idle timeout message",
	})
	require.NoError(t, err)
	_, err = p.clusterConfigS.UpsertClusterNetworkingConfig(ctx, netConfig)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterNetworkingConfig, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outNetConfig, err := p.cache.GetClusterNetworkingConfig(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outNetConfig, netConfig, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestSessionRecordingConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode:                types.RecordAtProxySync,
		ProxyChecksHostKeys: types.NewBoolOption(true),
	})
	require.NoError(t, err)
	_, err = p.clusterConfigS.UpsertSessionRecordingConfig(ctx, recConfig)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindSessionRecordingConfig, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outRecConfig, err := p.cache.GetSessionRecordingConfig(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outRecConfig, recConfig, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestClusterAuditConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
		AuditEventsURI: []string{"dynamodb://audit_table_name", "file:///home/log"},
	})
	require.NoError(t, err)
	err = p.clusterConfigS.SetClusterAuditConfig(ctx, auditConfig)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterAuditConfig, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outAuditConfig, err := p.cache.GetClusterAuditConfig(ctx)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outAuditConfig, auditConfig, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

func TestClusterName(t *testing.T) {
	t.Parallel()

	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	require.NoError(t, err)
	err = p.clusterConfigS.SetClusterName(clusterName)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindClusterName, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	outName, err := p.cache.GetClusterName()
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(outName, clusterName, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

// TestUsers tests caching of users
func TestUsers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.User]{
		newResource: func(name string) (types.User, error) {
			return types.NewUser("bob")
		},
		create: func(ctx context.Context, user types.User) error {
			_, err := p.usersS.UpsertUser(ctx, user)
			return err
		},
		list: func(ctx context.Context) ([]types.User, error) {
			return p.usersS.GetUsers(ctx, false)
		},
		cacheList: func(ctx context.Context) ([]types.User, error) {
			return p.cache.GetUsers(ctx, false)
		},
		update: func(ctx context.Context, user types.User) error {
			_, err := p.usersS.UpdateUser(ctx, user)
			return err
		},
		deleteAll: func(ctx context.Context) error {
			return p.usersS.DeleteAllUsers(ctx)
		},
	})
}

// TestRoles tests caching of roles
func TestRoles(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForNode)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Role]{
		newResource: func(name string) (types.Role, error) {
			return types.NewRole("role1", types.RoleSpecV6{
				Options: types.RoleOptions{
					MaxSessionTTL: types.Duration(time.Hour),
				},
				Allow: types.RoleConditions{
					Logins:     []string{"root", "bob"},
					NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
				},
				Deny: types.RoleConditions{},
			})
		},
		create: func(ctx context.Context, role types.Role) error {
			_, err := p.accessS.UpsertRole(ctx, role)
			return err
		},
		list:      p.accessS.GetRoles,
		cacheGet:  p.cache.GetRole,
		cacheList: p.cache.GetRoles,
		update: func(ctx context.Context, role types.Role) error {
			_, err := p.accessS.UpsertRole(ctx, role)
			return err
		},
		deleteAll: func(ctx context.Context) error {
			return p.accessS.DeleteAllRoles(ctx)
		},
	})
}

// TestTunnelConnections tests tunnel connections caching
func TestTunnelConnections(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	clusterName := "example.com"
	testResources(t, p, testFuncs[types.TunnelConnection]{
		newResource: func(name string) (types.TunnelConnection, error) {
			return types.NewTunnelConnection(name, types.TunnelConnectionSpecV2{
				ClusterName:   clusterName,
				ProxyName:     "p1",
				LastHeartbeat: time.Now().UTC(),
			})
		},
		create: modifyNoContext(p.trustS.UpsertTunnelConnection),
		list: func(ctx context.Context) ([]types.TunnelConnection, error) {
			return p.trustS.GetTunnelConnections(clusterName)
		},
		cacheList: func(ctx context.Context) ([]types.TunnelConnection, error) {
			return p.cache.GetTunnelConnections(clusterName)
		},
		update: modifyNoContext(p.trustS.UpsertTunnelConnection),
		deleteAll: func(ctx context.Context) error {
			return p.trustS.DeleteAllTunnelConnections()
		},
	})
}

// TestNodes tests nodes cache
func TestNodes(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Server]{
		newResource: func(name string) (types.Server, error) {
			return suite.NewServer(types.KindNode, name, "127.0.0.1:2022", apidefaults.Namespace), nil
		},
		create: withKeepalive(p.presenceS.UpsertNode),
		list: func(ctx context.Context) ([]types.Server, error) {
			return p.presenceS.GetNodes(ctx, apidefaults.Namespace)
		},
		cacheGet: func(ctx context.Context, name string) (types.Server, error) {
			return p.cache.GetNode(ctx, apidefaults.Namespace, name)
		},
		cacheList: func(ctx context.Context) ([]types.Server, error) {
			return p.cache.GetNodes(ctx, apidefaults.Namespace)
		},
		update: withKeepalive(p.presenceS.UpsertNode),
		deleteAll: func(ctx context.Context) error {
			return p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
		},
	})
}

// TestProxies tests proxies cache
func TestProxies(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Server]{
		newResource: func(name string) (types.Server, error) {
			return suite.NewServer(types.KindProxy, name, "127.0.0.1:2022", apidefaults.Namespace), nil
		},
		create: p.presenceS.UpsertProxy,
		list: func(_ context.Context) ([]types.Server, error) {
			return p.presenceS.GetProxies()
		},
		cacheList: func(_ context.Context) ([]types.Server, error) {
			return p.cache.GetProxies()
		},
		update: p.presenceS.UpsertProxy,
		deleteAll: func(_ context.Context) error {
			return p.presenceS.DeleteAllProxies()
		},
	})
}

// TestAuthServers tests auth servers cache
func TestAuthServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Server]{
		newResource: func(name string) (types.Server, error) {
			return suite.NewServer(types.KindAuthServer, name, "127.0.0.1:2022", apidefaults.Namespace), nil
		},
		create: p.presenceS.UpsertAuthServer,
		list: func(_ context.Context) ([]types.Server, error) {
			return p.presenceS.GetAuthServers()
		},
		cacheList: func(_ context.Context) ([]types.Server, error) {
			return p.cache.GetAuthServers()
		},
		update: p.presenceS.UpsertAuthServer,
		deleteAll: func(_ context.Context) error {
			return p.presenceS.DeleteAllAuthServers()
		},
	})
}

// TestRemoteClusters tests remote clusters caching
func TestRemoteClusters(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.RemoteCluster]{
		newResource: func(name string) (types.RemoteCluster, error) {
			return types.NewRemoteCluster(name)
		},
		create: func(ctx context.Context, rc types.RemoteCluster) error {
			_, err := p.trustS.CreateRemoteCluster(ctx, rc)
			return err
		},
		list: func(ctx context.Context) ([]types.RemoteCluster, error) {
			return p.trustS.GetRemoteClusters(ctx)
		},
		cacheGet: func(ctx context.Context, name string) (types.RemoteCluster, error) {
			return p.cache.GetRemoteCluster(ctx, name)
		},
		cacheList: func(ctx context.Context) ([]types.RemoteCluster, error) {
			return p.cache.GetRemoteClusters(ctx)
		},
		update: func(ctx context.Context, rc types.RemoteCluster) error {
			_, err := p.trustS.UpdateRemoteCluster(ctx, rc)
			return err
		},
		deleteAll: func(ctx context.Context) error {
			return p.trustS.DeleteAllRemoteClusters(ctx)
		},
	})
}

// TestKubernetes tests that CRUD operations on kubernetes clusters resources are
// replicated from the backend to the cache.
func TestKubernetes(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.KubeCluster]{
		newResource: func(name string) (types.KubeCluster, error) {
			return types.NewKubernetesClusterV3(types.Metadata{
				Name: name,
			}, types.KubernetesClusterSpecV3{})
		},
		create:    p.kubernetes.CreateKubernetesCluster,
		list:      p.kubernetes.GetKubernetesClusters,
		cacheGet:  p.cache.GetKubernetesCluster,
		cacheList: p.cache.GetKubernetesClusters,
		update:    p.kubernetes.UpdateKubernetesCluster,
		deleteAll: p.kubernetes.DeleteAllKubernetesClusters,
	})
}

// TestApplicationServers tests that CRUD operations on app servers are
// replicated from the backend to the cache.
func TestApplicationServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.AppServer]{
		newResource: func(name string) (types.AppServer, error) {
			app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{URI: "localhost"})
			require.NoError(t, err)
			return types.NewAppServerV3FromApp(app, "host", uuid.New().String())
		},
		create: withKeepalive(p.presenceS.UpsertApplicationServer),
		list: func(ctx context.Context) ([]types.AppServer, error) {
			return p.presenceS.GetApplicationServers(ctx, apidefaults.Namespace)
		},
		cacheList: func(ctx context.Context) ([]types.AppServer, error) {
			return p.cache.GetApplicationServers(ctx, apidefaults.Namespace)
		},
		update: withKeepalive(p.presenceS.UpsertApplicationServer),
		deleteAll: func(ctx context.Context) error {
			return p.presenceS.DeleteAllApplicationServers(ctx, apidefaults.Namespace)
		},
	})
}

// TestKubernetesServers tests that CRUD operations on kube servers are
// replicated from the backend to the cache.
func TestKubernetesServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.KubeServer]{
		newResource: func(name string) (types.KubeServer, error) {
			app, err := types.NewKubernetesClusterV3(types.Metadata{Name: name}, types.KubernetesClusterSpecV3{})
			require.NoError(t, err)
			return types.NewKubernetesServerV3FromCluster(app, "host", uuid.New().String())
		},
		create: withKeepalive(p.presenceS.UpsertKubernetesServer),
		list: func(ctx context.Context) ([]types.KubeServer, error) {
			return p.presenceS.GetKubernetesServers(ctx)
		},
		cacheList: func(ctx context.Context) ([]types.KubeServer, error) {
			return p.cache.GetKubernetesServers(ctx)
		},
		update: withKeepalive(p.presenceS.UpsertKubernetesServer),
		deleteAll: func(ctx context.Context) error {
			return p.presenceS.DeleteAllKubernetesServers(ctx)
		},
	})
}

// TestApps tests that CRUD operations on application resources are
// replicated from the backend to the cache.
func TestApps(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Application]{
		newResource: func(name string) (types.Application, error) {
			return types.NewAppV3(types.Metadata{
				Name: "foo",
			}, types.AppSpecV3{
				URI: "localhost",
			})
		},
		create:    p.apps.CreateApp,
		list:      p.apps.GetApps,
		cacheGet:  p.cache.GetApp,
		cacheList: p.cache.GetApps,
		update:    p.apps.UpdateApp,
		deleteAll: p.apps.DeleteAllApps,
	})
}

func mustCreateDatabase(t *testing.T, name, protocol, uri string) *types.DatabaseV3 {
	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name: name,
		},
		types.DatabaseSpecV3{
			Protocol: protocol,
			URI:      uri,
		},
	)
	require.NoError(t, err)
	return database
}

// TestDatabaseServers tests that CRUD operations on database servers are
// replicated from the backend to the cache.
func TestDatabaseServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.DatabaseServer]{
		newResource: func(name string) (types.DatabaseServer, error) {
			return types.NewDatabaseServerV3(types.Metadata{
				Name: name,
			}, types.DatabaseServerSpecV3{
				Database: mustCreateDatabase(t, name, defaults.ProtocolPostgres, "localhost:5432"),
				Hostname: "localhost",
				HostID:   uuid.New().String(),
			})
		},
		create: withKeepalive(p.presenceS.UpsertDatabaseServer),
		list: func(ctx context.Context) ([]types.DatabaseServer, error) {
			return p.presenceS.GetDatabaseServers(ctx, apidefaults.Namespace)
		},
		cacheList: func(ctx context.Context) ([]types.DatabaseServer, error) {
			return p.cache.GetDatabaseServers(ctx, apidefaults.Namespace)
		},
		update: withKeepalive(p.presenceS.UpsertDatabaseServer),
		deleteAll: func(ctx context.Context) error {
			return p.presenceS.DeleteAllDatabaseServers(ctx, apidefaults.Namespace)
		},
	})
}

// TestDatabaseServices tests that CRUD operations on DatabaseServices are
// replicated from the backend to the cache.
func TestDatabaseServices(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.DatabaseService]{
		newResource: func(name string) (types.DatabaseService, error) {
			return types.NewDatabaseServiceV1(types.Metadata{
				Name: uuid.NewString(),
			}, types.DatabaseServiceSpecV1{
				ResourceMatchers: []*types.DatabaseResourceMatcher{
					{Labels: &types.Labels{"env": []string{"prod"}}},
				},
			})
		},
		create: withKeepalive(p.databaseServices.UpsertDatabaseService),
		list: func(ctx context.Context) ([]types.DatabaseService, error) {
			listServicesResp, err := p.presenceS.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: types.KindDatabaseService,
				Limit:        apidefaults.DefaultChunkSize,
			})
			require.NoError(t, err)
			return types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
		},
		cacheList: func(ctx context.Context) ([]types.DatabaseService, error) {
			listServicesResp, err := p.cache.ListResources(ctx, proto.ListResourcesRequest{
				ResourceType: types.KindDatabaseService,
				Limit:        apidefaults.DefaultChunkSize,
			})
			require.NoError(t, err)
			return types.ResourcesWithLabels(listServicesResp.Resources).AsDatabaseServices()
		},
		update:    withKeepalive(p.databaseServices.UpsertDatabaseService),
		deleteAll: p.databaseServices.DeleteAllDatabaseServices,
	})
}

// TestDatabases tests that CRUD operations on database resources are
// replicated from the backend to the cache.
func TestDatabases(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Database]{
		newResource: func(name string) (types.Database, error) {
			return types.NewDatabaseV3(types.Metadata{
				Name: name,
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			})
		},
		create:    p.databases.CreateDatabase,
		list:      p.databases.GetDatabases,
		cacheGet:  p.cache.GetDatabase,
		cacheList: p.cache.GetDatabases,
		update:    p.databases.UpdateDatabase,
		deleteAll: p.databases.DeleteAllDatabases,
	})
}

// TestSAMLIdPServiceProviders tests that CRUD operations on SAML IdP service provider resources are
// replicated from the backend to the cache.
func TestSAMLIdPServiceProviders(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.SAMLIdPServiceProvider]{
		newResource: func(name string) (types.SAMLIdPServiceProvider, error) {
			return types.NewSAMLIdPServiceProvider(
				types.Metadata{
					Name: name,
				},
				types.SAMLIdPServiceProviderSpecV1{
					EntityDescriptor: testEntityDescriptor,
					EntityID:         "IAMShowcase",
				})
		},
		create: p.samlIDPServiceProviders.CreateSAMLIdPServiceProvider,
		list: func(ctx context.Context) ([]types.SAMLIdPServiceProvider, error) {
			results, _, err := p.samlIDPServiceProviders.ListSAMLIdPServiceProviders(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetSAMLIdPServiceProvider,
		cacheList: func(ctx context.Context) ([]types.SAMLIdPServiceProvider, error) {
			results, _, err := p.cache.ListSAMLIdPServiceProviders(ctx, 0, "")
			return results, err
		},
		update:    p.samlIDPServiceProviders.UpdateSAMLIdPServiceProvider,
		deleteAll: p.samlIDPServiceProviders.DeleteAllSAMLIdPServiceProviders,
	})
}

// TestUserGroups tests that CRUD operations on user group resources are
// replicated from the backend to the cache.
func TestUserGroups(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.UserGroup]{
		newResource: func(name string) (types.UserGroup, error) {
			return types.NewUserGroup(
				types.Metadata{
					Name: name,
				}, types.UserGroupSpecV1{},
			)
		},
		create: p.userGroups.CreateUserGroup,
		list: func(ctx context.Context) ([]types.UserGroup, error) {
			results, _, err := p.userGroups.ListUserGroups(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetUserGroup,
		cacheList: func(ctx context.Context) ([]types.UserGroup, error) {
			results, _, err := p.cache.ListUserGroups(ctx, 0, "")
			return results, err
		},
		update:    p.userGroups.UpdateUserGroup,
		deleteAll: p.userGroups.DeleteAllUserGroups,
	})
}

// TestLocks tests that CRUD operations on lock resources are
// replicated from the backend to the cache.
func TestLocks(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Lock]{
		newResource: func(name string) (types.Lock, error) {
			return types.NewLock(
				name,
				types.LockSpecV2{
					Target: types.LockTarget{
						Role: "target-role",
					},
				},
			)
		},
		create: p.accessS.UpsertLock,
		list: func(ctx context.Context) ([]types.Lock, error) {
			results, err := p.accessS.GetLocks(ctx, false)
			return results, err
		},
		cacheGet: p.cache.GetLock,
		cacheList: func(ctx context.Context) ([]types.Lock, error) {
			results, err := p.cache.GetLocks(ctx, false)
			return results, err
		},
		update:    p.accessS.UpsertLock,
		deleteAll: p.accessS.DeleteAllLocks,
	})
}

// TestOktaImportRules tests that CRUD operations on Okta import rule resources are
// replicated from the backend to the cache.
func TestOktaImportRules(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForOkta)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.OktaImportRule]{
		newResource: func(name string) (types.OktaImportRule, error) {
			return types.NewOktaImportRule(
				types.Metadata{
					Name: name,
				},
				types.OktaImportRuleSpecV1{
					Mappings: []*types.OktaImportRuleMappingV1{
						{
							Match: []*types.OktaImportRuleMatchV1{
								{
									AppIDs: []string{"yes"},
								},
							},
							AddLabels: map[string]string{
								"label1": "value1",
							},
						},
						{
							Match: []*types.OktaImportRuleMatchV1{
								{
									GroupIDs: []string{"yes"},
								},
							},
							AddLabels: map[string]string{
								"label1": "value1",
							},
						},
					},
				},
			)
		},
		create: func(ctx context.Context, resource types.OktaImportRule) error {
			_, err := p.okta.CreateOktaImportRule(ctx, resource)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]types.OktaImportRule, error) {
			results, _, err := p.okta.ListOktaImportRules(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetOktaImportRule,
		cacheList: func(ctx context.Context) ([]types.OktaImportRule, error) {
			results, _, err := p.cache.ListOktaImportRules(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, resource types.OktaImportRule) error {
			_, err := p.okta.UpdateOktaImportRule(ctx, resource)
			return trace.Wrap(err)
		},
		deleteAll: p.okta.DeleteAllOktaImportRules,
	})
}

// TestOktaAssignments tests that CRUD operations on Okta import rule resources are
// replicated from the backend to the cache.
func TestOktaAssignments(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForOkta)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.OktaAssignment]{
		newResource: func(name string) (types.OktaAssignment, error) {
			return types.NewOktaAssignment(
				types.Metadata{
					Name: name,
				},
				types.OktaAssignmentSpecV1{
					User: "test-user@test.user",
					Targets: []*types.OktaAssignmentTargetV1{
						{
							Type: types.OktaAssignmentTargetV1_APPLICATION,
							Id:   "123456",
						},
						{
							Type: types.OktaAssignmentTargetV1_GROUP,
							Id:   "234567",
						},
					},
				},
			)
		},
		create: func(ctx context.Context, resource types.OktaAssignment) error {
			_, err := p.okta.CreateOktaAssignment(ctx, resource)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]types.OktaAssignment, error) {
			results, _, err := p.okta.ListOktaAssignments(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetOktaAssignment,
		cacheList: func(ctx context.Context) ([]types.OktaAssignment, error) {
			results, _, err := p.cache.ListOktaAssignments(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, resource types.OktaAssignment) error {
			_, err := p.okta.UpdateOktaAssignment(ctx, resource)
			return trace.Wrap(err)
		},
		deleteAll: p.okta.DeleteAllOktaAssignments,
	})
}

// TestIntegrations tests that CRUD operations on integrations resources are
// replicated from the backend to the cache.
func TestIntegrations(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.Integration]{
		newResource: func(name string) (types.Integration, error) {
			return types.NewIntegrationAWSOIDC(
				types.Metadata{Name: name},
				&types.AWSOIDCIntegrationSpecV1{
					RoleARN: "arn:aws:iam::123456789012:role/OpsTeam",
				},
			)
		},
		create: func(ctx context.Context, i types.Integration) error {
			_, err := p.integrations.CreateIntegration(ctx, i)
			return err
		},
		list: func(ctx context.Context) ([]types.Integration, error) {
			results, _, err := p.integrations.ListIntegrations(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetIntegration,
		cacheList: func(ctx context.Context) ([]types.Integration, error) {
			results, _, err := p.cache.ListIntegrations(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, i types.Integration) error {
			_, err := p.integrations.UpdateIntegration(ctx, i)
			return err
		},
		deleteAll: p.integrations.DeleteAllIntegrations,
	})
}

// TestUserTasks tests that CRUD operations on user notification resources are
// replicated from the backend to the cache.
func TestUserTasks(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*usertasksv1.UserTask]{
		newResource: func(name string) (*usertasksv1.UserTask, error) {
			return newUserTasks(t), nil
		},
		create: func(ctx context.Context, item *usertasksv1.UserTask) error {
			_, err := p.userTasks.CreateUserTask(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*usertasksv1.UserTask, error) {
			items, _, err := p.userTasks.ListUserTasks(ctx, 0, "", &usertasksv1.ListUserTasksFilters{})
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*usertasksv1.UserTask, error) {
			items, _, err := p.userTasks.ListUserTasks(ctx, 0, "", &usertasksv1.ListUserTasksFilters{})
			return items, trace.Wrap(err)
		},
		deleteAll: p.userTasks.DeleteAllUserTasks,
	})
}

func newUserTasks(t *testing.T) *usertasksv1.UserTask {
	t.Helper()

	ut, err := usertasks.NewDiscoverEC2UserTask(&usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    usertasks.TaskTypeDiscoverEC2,
		IssueType:   "ec2-ssm-agent-not-registered",
		State:       "OPEN",
		DiscoverEc2: &usertasksv1.DiscoverEC2{
			AccountId: "123456789012",
			Region:    "us-east-1",
			Instances: map[string]*usertasksv1.DiscoverEC2Instance{
				"i-123": {
					InstanceId:      "i-123",
					DiscoveryConfig: "dc01",
					DiscoveryGroup:  "dg01",
					SyncTime:        timestamppb.Now(),
				},
			},
		},
	})
	require.NoError(t, err)

	return ut
}

// TestDiscoveryConfig tests that CRUD operations on DiscoveryConfig resources are
// replicated from the backend to the cache.
func TestDiscoveryConfig(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*discoveryconfig.DiscoveryConfig]{
		newResource: func(name string) (*discoveryconfig.DiscoveryConfig, error) {
			dc, err := discoveryconfig.NewDiscoveryConfig(
				header.Metadata{Name: "mydc"},
				discoveryconfig.Spec{
					DiscoveryGroup: "group001",
				})
			require.NoError(t, err)
			return dc, nil
		},
		create: func(ctx context.Context, discoveryConfig *discoveryconfig.DiscoveryConfig) error {
			_, err := p.discoveryConfigs.CreateDiscoveryConfig(ctx, discoveryConfig)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*discoveryconfig.DiscoveryConfig, error) {
			results, _, err := p.discoveryConfigs.ListDiscoveryConfigs(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetDiscoveryConfig,
		cacheList: func(ctx context.Context) ([]*discoveryconfig.DiscoveryConfig, error) {
			results, _, err := p.cache.ListDiscoveryConfigs(ctx, 0, "")
			return results, err
		},
		update: func(ctx context.Context, discoveryConfig *discoveryconfig.DiscoveryConfig) error {
			_, err := p.discoveryConfigs.UpdateDiscoveryConfig(ctx, discoveryConfig)
			return trace.Wrap(err)
		},
		deleteAll: p.discoveryConfigs.DeleteAllDiscoveryConfigs,
	})
}

// TestAuditQuery tests that CRUD operations on access list rule resources are
// replicated from the backend to the cache.
func TestAuditQuery(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*secreports.AuditQuery]{
		newResource: func(name string) (*secreports.AuditQuery, error) {
			return newAuditQuery(t, name), nil
		},
		create: func(ctx context.Context, item *secreports.AuditQuery) error {
			err := p.secReports.UpsertSecurityAuditQuery(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.secReports.GetSecurityAuditQueries,
		cacheGet:  p.cache.GetSecurityAuditQuery,
		cacheList: p.cache.GetSecurityAuditQueries,
		update: func(ctx context.Context, item *secreports.AuditQuery) error {
			err := p.secReports.UpsertSecurityAuditQuery(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.secReports.DeleteAllSecurityAuditQueries,
	})
}

// TestSecurityReportState tests that CRUD operations on security report state resources are
// replicated from the backend to the cache.
func TestSecurityReports(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*secreports.Report]{
		newResource: func(name string) (*secreports.Report, error) {
			return newSecurityReport(t, name), nil
		},
		create: func(ctx context.Context, item *secreports.Report) error {
			err := p.secReports.UpsertSecurityReport(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.secReports.GetSecurityReports,
		cacheGet:  p.cache.GetSecurityReport,
		cacheList: p.cache.GetSecurityReports,
		update: func(ctx context.Context, item *secreports.Report) error {
			err := p.secReports.UpsertSecurityReport(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.secReports.DeleteAllSecurityReports,
	})
}

// TestSecurityReportState tests that CRUD operations on security report state resources are
// replicated from the backend to the cache.
func TestSecurityReportState(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*secreports.ReportState]{
		newResource: func(name string) (*secreports.ReportState, error) {
			return newSecurityReportState(t, name), nil
		},
		create: func(ctx context.Context, item *secreports.ReportState) error {
			err := p.secReports.UpsertSecurityReportsState(ctx, item)
			return trace.Wrap(err)
		},
		list:      p.secReports.GetSecurityReportsStates,
		cacheGet:  p.cache.GetSecurityReportState,
		cacheList: p.cache.GetSecurityReportsStates,
		update: func(ctx context.Context, item *secreports.ReportState) error {
			err := p.secReports.UpsertSecurityReportsState(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.secReports.DeleteAllSecurityReportsStates,
	})
}

// TestUserLoginStates tests that CRUD operations on user login state resources are
// replicated from the backend to the cache.
func TestUserLoginStates(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[*userloginstate.UserLoginState]{
		newResource: func(name string) (*userloginstate.UserLoginState, error) {
			return newUserLoginState(t, name), nil
		},
		create: func(ctx context.Context, uls *userloginstate.UserLoginState) error {
			_, err := p.userLoginStates.UpsertUserLoginState(ctx, uls)
			return trace.Wrap(err)
		},
		list:      p.userLoginStates.GetUserLoginStates,
		cacheGet:  p.cache.GetUserLoginState,
		cacheList: p.cache.GetUserLoginStates,
		update: func(ctx context.Context, uls *userloginstate.UserLoginState) error {
			_, err := p.userLoginStates.UpsertUserLoginState(ctx, uls)
			return trace.Wrap(err)
		},
		deleteAll: p.userLoginStates.DeleteAllUserLoginStates,
	})
}

// TestAccessList tests that CRUD operations on access list resources are
// replicated from the backend to the cache.
func TestAccessList(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clock := clockwork.NewFakeClockAt(time.Now())

	testResources(t, p, testFuncs[*accesslist.AccessList]{
		newResource: func(name string) (*accesslist.AccessList, error) {
			return newAccessList(t, name, clock), nil
		},
		create: func(ctx context.Context, item *accesslist.AccessList) error {
			_, err := p.accessLists.UpsertAccessList(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*accesslist.AccessList, error) {
			items, _, err := p.accessLists.ListAccessLists(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		cacheGet: p.cache.GetAccessList,
		cacheList: func(ctx context.Context) ([]*accesslist.AccessList, error) {
			items, _, err := p.cache.ListAccessLists(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		update: func(ctx context.Context, item *accesslist.AccessList) error {
			_, err := p.accessLists.UpsertAccessList(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.accessLists.DeleteAllAccessLists,
	})
}

// TestAccessListMembers tests that CRUD operations on access list member resources are
// replicated from the backend to the cache.
func TestAccessListMembers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clock := clockwork.NewFakeClockAt(time.Now())

	al, err := p.accessLists.UpsertAccessList(context.Background(), newAccessList(t, "access-list", clock))
	require.NoError(t, err)

	testResources(t, p, testFuncs[*accesslist.AccessListMember]{
		newResource: func(name string) (*accesslist.AccessListMember, error) {
			return newAccessListMember(t, al.GetName(), name), nil
		},
		create: func(ctx context.Context, item *accesslist.AccessListMember) error {
			_, err := p.accessLists.UpsertAccessListMember(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*accesslist.AccessListMember, error) {
			items, _, err := p.accessLists.ListAllAccessListMembers(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		cacheGet: func(ctx context.Context, name string) (*accesslist.AccessListMember, error) {
			return p.cache.GetAccessListMember(ctx, al.GetName(), name)
		},
		cacheList: func(ctx context.Context) ([]*accesslist.AccessListMember, error) {
			items, _, err := p.cache.ListAccessListMembers(ctx, al.GetName(), 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		update: func(ctx context.Context, item *accesslist.AccessListMember) error {
			_, err := p.accessLists.UpsertAccessListMember(ctx, item)
			return trace.Wrap(err)
		},
		deleteAll: p.accessLists.DeleteAllAccessListMembers,
	})

	// Verify counting.
	ctx := context.Background()
	for i := 0; i < 40; i++ {
		_, err = p.accessLists.UpsertAccessListMember(ctx, newAccessListMember(t, al.GetName(), strconv.Itoa(i)))
		require.NoError(t, err)
	}

	count, listCount, err := p.accessLists.CountAccessListMembers(ctx, al.GetName())
	require.NoError(t, err)
	require.Equal(t, uint32(40), count)
	require.Equal(t, uint32(0), listCount)

	// Eventually, this should be reflected in the cache.
	require.Eventually(t, func() bool {
		// Make sure the cache has a single resource in it.
		count, listCount, err := p.cache.CountAccessListMembers(ctx, al.GetName())
		assert.NoError(t, err)
		return count == uint32(40) && listCount == uint32(0)
	}, time.Second*2, time.Millisecond*250)
}

// TestAccessListReviews tests that CRUD operations on access list review resources are
// replicated from the backend to the cache.
func TestAccessListReviews(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	clock := clockwork.NewFakeClockAt(time.Now())

	al, _, err := p.accessLists.UpsertAccessListWithMembers(context.Background(), newAccessList(t, "access-list", clock),
		[]*accesslist.AccessListMember{
			newAccessListMember(t, "access-list", "member1"),
			newAccessListMember(t, "access-list", "member2"),
			newAccessListMember(t, "access-list", "member3"),
			newAccessListMember(t, "access-list", "member4"),
			newAccessListMember(t, "access-list", "member5"),
		})
	require.NoError(t, err)

	// Keep track of the reviews, as create can update them. We'll use this
	// to make sure the values are up to date during the test.
	reviews := map[string]*accesslist.Review{}

	testResources(t, p, testFuncs[*accesslist.Review]{
		newResource: func(name string) (*accesslist.Review, error) {
			review := newAccessListReview(t, al.GetName(), name)
			// Store the name in the description.
			review.Metadata.Description = name
			reviews[name] = review
			return review, nil
		},
		create: func(ctx context.Context, item *accesslist.Review) error {
			review, _, err := p.accessLists.CreateAccessListReview(ctx, item)
			// Use the old name from the description.
			oldName := review.Metadata.Description
			reviews[oldName].SetName(review.GetName())
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*accesslist.Review, error) {
			items, _, err := p.accessLists.ListAllAccessListReviews(ctx, 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*accesslist.Review, error) {
			items, _, err := p.cache.ListAccessListReviews(ctx, al.GetName(), 0 /* page size */, "")
			return items, trace.Wrap(err)
		},
		deleteAll: p.accessLists.DeleteAllAccessListReviews,
	})
}

// TestUserNotifications tests that CRUD operations on user notification resources are
// replicated from the backend to the cache.
func TestUserNotifications(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*notificationsv1.Notification]{
		newResource: func(name string) (*notificationsv1.Notification, error) {
			return newUserNotification(t, name), nil
		},
		create: func(ctx context.Context, item *notificationsv1.Notification) error {
			_, err := p.notifications.CreateUserNotification(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*notificationsv1.Notification, error) {
			items, _, err := p.notifications.ListUserNotifications(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*notificationsv1.Notification, error) {
			items, _, err := p.cache.ListUserNotifications(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		deleteAll: p.notifications.DeleteAllUserNotifications,
	})
}

// TestCrownJewel tests that CRUD operations on user notification resources are
// replicated from the backend to the cache.
func TestCrownJewel(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*crownjewelv1.CrownJewel]{
		newResource: func(name string) (*crownjewelv1.CrownJewel, error) {
			return newCrownJewel(t, name), nil
		},
		create: func(ctx context.Context, item *crownjewelv1.CrownJewel) error {
			_, err := p.crownJewels.CreateCrownJewel(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*crownjewelv1.CrownJewel, error) {
			items, _, err := p.crownJewels.ListCrownJewels(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*crownjewelv1.CrownJewel, error) {
			items, _, err := p.crownJewels.ListCrownJewels(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		deleteAll: p.crownJewels.DeleteAllCrownJewels,
	})
}

func TestDatabaseObjects(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*dbobjectv1.DatabaseObject]{
		newResource: func(name string) (*dbobjectv1.DatabaseObject, error) {
			return newDatabaseObject(t, name), nil
		},
		create: func(ctx context.Context, item *dbobjectv1.DatabaseObject) error {
			_, err := p.databaseObjects.CreateDatabaseObject(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*dbobjectv1.DatabaseObject, error) {
			items, _, err := p.databaseObjects.ListDatabaseObjects(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*dbobjectv1.DatabaseObject, error) {
			items, _, err := p.databaseObjects.ListDatabaseObjects(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			token := ""
			var objects []*dbobjectv1.DatabaseObject

			for {
				resp, nextToken, err := p.databaseObjects.ListDatabaseObjects(ctx, 0, token)
				if err != nil {
					return err
				}

				objects = append(objects, resp...)

				if nextToken == "" {
					break
				}
				token = nextToken
			}

			for _, object := range objects {
				err := p.databaseObjects.DeleteDatabaseObject(ctx, object.GetMetadata().GetName())
				if err != nil {
					return err
				}
			}
			return nil
		},
	})
}

// TestAutoUpdateConfig tests that CRUD operations on AutoUpdateConfig resources are
// replicated from the backend to the cache.
func TestAutoUpdateConfig(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*autoupdate.AutoUpdateConfig]{
		newResource: func(name string) (*autoupdate.AutoUpdateConfig, error) {
			return newAutoUpdateConfig(t), nil
		},
		create: func(ctx context.Context, item *autoupdate.AutoUpdateConfig) error {
			_, err := p.autoUpdateService.UpsertAutoUpdateConfig(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*autoupdate.AutoUpdateConfig, error) {
			item, err := p.autoUpdateService.GetAutoUpdateConfig(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdate.AutoUpdateConfig{}, nil
			}
			return []*autoupdate.AutoUpdateConfig{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*autoupdate.AutoUpdateConfig, error) {
			item, err := p.cache.GetAutoUpdateConfig(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdate.AutoUpdateConfig{}, nil
			}
			return []*autoupdate.AutoUpdateConfig{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.autoUpdateService.DeleteAutoUpdateConfig(ctx))
		},
	})
}

// TestAutoUpdateVersion tests that CRUD operations on AutoUpdateVersion resource are
// replicated from the backend to the cache.
func TestAutoUpdateVersion(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*autoupdate.AutoUpdateVersion]{
		newResource: func(name string) (*autoupdate.AutoUpdateVersion, error) {
			return newAutoUpdateVersion(t), nil
		},
		create: func(ctx context.Context, item *autoupdate.AutoUpdateVersion) error {
			_, err := p.autoUpdateService.UpsertAutoUpdateVersion(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*autoupdate.AutoUpdateVersion, error) {
			item, err := p.autoUpdateService.GetAutoUpdateVersion(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdate.AutoUpdateVersion{}, nil
			}
			return []*autoupdate.AutoUpdateVersion{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*autoupdate.AutoUpdateVersion, error) {
			item, err := p.cache.GetAutoUpdateVersion(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdate.AutoUpdateVersion{}, nil
			}
			return []*autoupdate.AutoUpdateVersion{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.autoUpdateService.DeleteAutoUpdateVersion(ctx))
		},
	})
}

// TestAutoUpdateAgentRollout tests that CRUD operations on AutoUpdateAgentRollout resource are
// replicated from the backend to the cache.
func TestAutoUpdateAgentRollout(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*autoupdate.AutoUpdateAgentRollout]{
		newResource: func(name string) (*autoupdate.AutoUpdateAgentRollout, error) {
			return newAutoUpdateAgentRollout(t), nil
		},
		create: func(ctx context.Context, item *autoupdate.AutoUpdateAgentRollout) error {
			_, err := p.autoUpdateService.UpsertAutoUpdateAgentRollout(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*autoupdate.AutoUpdateAgentRollout, error) {
			item, err := p.autoUpdateService.GetAutoUpdateAgentRollout(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdate.AutoUpdateAgentRollout{}, nil
			}
			return []*autoupdate.AutoUpdateAgentRollout{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*autoupdate.AutoUpdateAgentRollout, error) {
			item, err := p.cache.GetAutoUpdateAgentRollout(ctx)
			if trace.IsNotFound(err) {
				return []*autoupdate.AutoUpdateAgentRollout{}, nil
			}
			return []*autoupdate.AutoUpdateAgentRollout{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.autoUpdateService.DeleteAutoUpdateAgentRollout(ctx))
		},
	})
}

// TestGlobalNotifications tests that CRUD operations on global notification resources are
// replicated from the backend to the cache.
func TestGlobalNotifications(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*notificationsv1.GlobalNotification]{
		newResource: func(name string) (*notificationsv1.GlobalNotification, error) {
			return newGlobalNotification(t, name), nil
		},
		create: func(ctx context.Context, item *notificationsv1.GlobalNotification) error {
			_, err := p.notifications.CreateGlobalNotification(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*notificationsv1.GlobalNotification, error) {
			items, _, err := p.notifications.ListGlobalNotifications(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*notificationsv1.GlobalNotification, error) {
			items, _, err := p.cache.ListGlobalNotifications(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		deleteAll: p.notifications.DeleteAllGlobalNotifications,
	})
}

// TestStaticHostUsers tests that CRUD operations on static host user resources are
// replicated from the backend to the cache.
func TestStaticHostUsers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*userprovisioningpb.StaticHostUser]{
		newResource: func(name string) (*userprovisioningpb.StaticHostUser, error) {
			return newStaticHostUser(t, name), nil
		},
		create: func(ctx context.Context, item *userprovisioningpb.StaticHostUser) error {
			_, err := p.staticHostUsers.CreateStaticHostUser(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*userprovisioningpb.StaticHostUser, error) {
			items, _, err := p.staticHostUsers.ListStaticHostUsers(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*userprovisioningpb.StaticHostUser, error) {
			items, _, err := p.cache.ListStaticHostUsers(ctx, 0, "")
			return items, trace.Wrap(err)
		},
		deleteAll: p.cache.staticHostUsersCache.DeleteAllStaticHostUsers,
	})
}

// testResources is a generic tester for resources.
func testResources[T types.Resource](t *testing.T, p *testPack, funcs testFuncs[T]) {
	ctx := context.Background()

	if funcs.changeResource == nil {
		funcs.changeResource = func(t T) {
			if t.Expiry().IsZero() {
				t.SetExpiry(time.Now().Add(30 * time.Minute))
			} else {
				t.SetExpiry(t.Expiry().Add(30 * time.Minute))
			}
		}
	}

	// Create a resource.
	r, err := funcs.newResource("test-sp")
	require.NoError(t, err)
	funcs.changeResource(r)

	err = funcs.create(ctx, r)
	require.NoError(t, err)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
	}

	// Check that the resource is now in the backend.
	out, err := funcs.list(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out, cmpOpts...))

	// Wait until the information has been replicated to the cache.
	require.Eventually(t, func() bool {
		// Make sure the cache has a single resource in it.
		out, err = funcs.cacheList(ctx)
		assert.NoError(t, err)
		return len(cmp.Diff([]T{r}, out, cmpOpts...)) == 0
	}, time.Second*2, time.Millisecond*250)

	// cacheGet is optional as not every resource implements it
	if funcs.cacheGet != nil {
		// Make sure a single cache get works.
		getR, err := funcs.cacheGet(ctx, r.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(r, getR, cmpOpts...))
	}

	// update is optional as not every resource implements it
	if funcs.update != nil {
		// Update the resource and upsert it into the backend again.
		funcs.changeResource(r)
		err = funcs.update(ctx, r)
		require.NoError(t, err)
	}

	// Check that the resource is in the backend and only one exists (so an
	// update occurred).
	out, err = funcs.list(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out, cmpOpts...))

	// Check that information has been replicated to the cache.
	require.Eventually(t, func() bool {
		// Make sure the cache has a single resource in it.
		out, err = funcs.cacheList(ctx)
		assert.NoError(t, err)
		return len(cmp.Diff([]T{r}, out, cmpOpts...)) == 0
	}, time.Second*2, time.Millisecond*250)

	// Remove all service providers from the backend.
	err = funcs.deleteAll(ctx)
	require.NoError(t, err)
	// Check that information has been replicated to the cache.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Check that the cache is now empty.
		out, err = funcs.cacheList(ctx)
		assert.NoError(t, err)
		assert.Empty(t, out)
	}, time.Second*2, time.Millisecond*250)
}

// testResources153 is a generic tester for RFD153-style resources.
func testResources153[T types.Resource153](t *testing.T, p *testPack, funcs testFuncs153[T]) {
	ctx := context.Background()

	// Create a resource.
	r, err := funcs.newResource("test-resource")
	require.NoError(t, err)
	// update is optional as not every resource implements it
	if funcs.update != nil {
		r.GetMetadata().Labels = map[string]string{"label": "value1"}
	}

	err = funcs.create(ctx, r)
	require.NoError(t, err)

	cmpOpts := []cmp.Option{
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	}

	assertCacheContents := func(expected []T) {
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			out, err := funcs.cacheList(ctx)
			assert.NoError(collect, err)

			// If the cache is expected to be empty, then test explicitly for
			// *that* rather than do an equality test. An equality test here
			// would be overly-pedantic about a service returning `nil` rather
			// than an empty slice.
			if len(expected) == 0 {
				assert.Empty(collect, out)
				return
			}

			assert.Empty(collect, cmp.Diff(expected, out, cmpOpts...))
		}, 2*time.Second, 10*time.Millisecond)
	}

	// Check that the resource is now in the backend.
	out, err := funcs.list(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out, cmpOpts...))

	// Wait until the information has been replicated to the cache.
	assertCacheContents([]T{r})

	// cacheGet is optional as not every resource implements it
	if funcs.cacheGet != nil {
		// Make sure a single cache get works.
		getR, err := funcs.cacheGet(ctx, r.GetMetadata().GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(r, getR, cmpOpts...))
	}

	// update is optional as not every resource implements it
	if funcs.update != nil {
		// Update the resource and upsert it into the backend again.
		r.GetMetadata().Labels["label"] = "value2"
		err = funcs.update(ctx, r)
		require.NoError(t, err)
	}

	// Check that the resource is in the backend and only one exists (so an
	// update occurred).
	out, err = funcs.list(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out, cmpOpts...))

	// Check that information has been replicated to the cache.
	assertCacheContents([]T{r})

	if funcs.delete != nil {
		// Add a second resource.
		r2, err := funcs.newResource("test-resource-2")
		require.NoError(t, err)
		require.NoError(t, funcs.create(ctx, r2))
		assertCacheContents([]T{r, r2})
		// Check that only one resource is deleted.
		require.NoError(t, funcs.delete(ctx, r2.GetMetadata().Name))
		assertCacheContents([]T{r})
	}

	// Remove all resources from the backend.
	err = funcs.deleteAll(ctx)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	assertCacheContents([]T{})
}

func TestRelativeExpiry(t *testing.T) {
	t.Parallel()

	const checkInterval = time.Second
	const nodeCount = int64(100)

	// make sure the event buffer is much larger than node count
	// so that we can batch create nodes without waiting on each event
	require.Less(t, int(nodeCount*3), eventBufferSize)

	ctx := context.Background()

	clock := clockwork.NewFakeClockAt(time.Now().Add(time.Hour))
	p := newTestPack(t, func(c Config) Config {
		c.RelativeExpiryCheckInterval = checkInterval
		c.Clock = clock
		return ForAuth(c)
	})
	t.Cleanup(p.Close)

	// add servers that expire at a range of times
	now := clock.Now()
	for i := int64(0); i < nodeCount; i++ {
		exp := now.Add(time.Minute * time.Duration(i))
		server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
		server.SetExpiry(exp)
		_, err := p.presenceS.UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	// wait for nodes to reach cache (we batch insert first for performance reasons)
	for i := int64(0); i < nodeCount; i++ {
		expectEvent(t, p.eventsC, EventProcessed)
	}

	nodes, err := p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, nodes, 100)

	clock.Advance(time.Minute * 25)
	// get rid of events that were emitted before clock advanced
	drainEvents(p.eventsC)
	// wait for next relative expiry check to run
	expectEvent(t, p.eventsC, RelativeExpiry)

	// verify that roughly expected proportion of nodes was removed.
	nodes, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.True(t, len(nodes) < 100 && len(nodes) > 75, "node_count=%d", len(nodes))

	clock.Advance(time.Minute * 25)
	// get rid of events that were emitted before clock advanced
	drainEvents(p.eventsC)
	// wait for next relative expiry check to run
	expectEvent(t, p.eventsC, RelativeExpiry)

	// verify that roughly expected proportion of nodes was removed.
	nodes, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.True(t, len(nodes) < 75 && len(nodes) > 50, "node_count=%d", len(nodes))

	// finally, we check the "sliding window" by verifying that we don't remove all nodes
	// even if we advance well past the latest expiry time.
	clock.Advance(time.Hour * 24)
	// get rid of events that were emitted before clock advanced
	drainEvents(p.eventsC)
	// wait for next relative expiry check to run
	expectEvent(t, p.eventsC, RelativeExpiry)

	// verify that sliding window has preserved most recent nodes
	nodes, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.NotEmpty(t, nodes, "node_count=%d", len(nodes))
}

func TestRelativeExpiryLimit(t *testing.T) {
	t.Parallel()
	const (
		checkInterval = time.Second
		nodeCount     = 100
		expiryLimit   = 10
	)

	// make sure the event buffer is much larger than node count
	// so that we can batch create nodes without waiting on each event
	require.Less(t, int(nodeCount*3), eventBufferSize)

	ctx := context.Background()

	clock := clockwork.NewFakeClockAt(time.Now().Add(time.Hour))
	p := newTestPack(t, func(c Config) Config {
		c.RelativeExpiryCheckInterval = checkInterval
		c.RelativeExpiryLimit = expiryLimit
		c.Clock = clock
		return ForAuth(c)
	})
	t.Cleanup(p.Close)

	// add servers that expire at a range of times
	now := clock.Now()
	for i := 0; i < nodeCount; i++ {
		exp := now.Add(time.Minute * time.Duration(i))
		server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
		server.SetExpiry(exp)
		_, err := p.presenceS.UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	// wait for nodes to reach cache (we batch insert first for performance reasons)
	for i := 0; i < nodeCount; i++ {
		expectEvent(t, p.eventsC, EventProcessed)
	}

	nodes, err := p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, nodes, nodeCount)

	clock.Advance(time.Hour * 24)
	for expired := nodeCount - expiryLimit; expired > expiryLimit; expired -= expiryLimit {
		// get rid of events that were emitted before clock advanced
		drainEvents(p.eventsC)
		// wait for next relative expiry check to run
		expectEvent(t, p.eventsC, RelativeExpiry)

		// verify that the limit is respected.
		nodes, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		require.Len(t, nodes, expired)

		// advance clock to trigger next relative expiry check
		clock.Advance(time.Hour * 24)
	}
}

func TestRelativeExpiryOnlyForAuth(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClockAt(time.Now().Add(time.Hour))
	p := newTestPack(t, func(c Config) Config {
		c.RelativeExpiryCheckInterval = time.Second
		c.Clock = clock
		c.Watches = []types.WatchKind{{Kind: types.KindNode}}
		return c
	})
	t.Cleanup(p.Close)

	p2 := newTestPack(t, func(c Config) Config {
		c.RelativeExpiryCheckInterval = time.Second
		c.Clock = clock
		c.target = "llama"
		c.Watches = []types.WatchKind{
			{Kind: types.KindNode},
			{Kind: types.KindCertAuthority},
		}
		return c
	})
	t.Cleanup(p2.Close)

	for i := 0; i < 2; i++ {
		clock.Advance(time.Hour * 24)
		drainEvents(p.eventsC)
		unexpectedEvent(t, p.eventsC, RelativeExpiry)

		clock.Advance(time.Hour * 24)
		drainEvents(p2.eventsC)
		unexpectedEvent(t, p2.eventsC, RelativeExpiry)
	}
}

func TestCache_Backoff(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	p := newTestPack(t, func(c Config) Config {
		c.MaxRetryPeriod = defaults.MaxWatcherBackoff
		c.Clock = clock
		return ForNode(c)
	})
	t.Cleanup(p.Close)

	// close watchers to trigger a reload event
	watchers := p.eventsS.getWatchers()
	require.Len(t, watchers, 1)
	p.eventsS.closeWatchers()
	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))

	step := p.cache.Config.MaxRetryPeriod / 16.0
	for i := 0; i < 5; i++ {
		// wait for cache to reload
		select {
		case event := <-p.eventsC:
			require.Equal(t, Reloading, event.Type)
			duration, err := time.ParseDuration(event.Event.Resource.GetKind())
			require.NoError(t, err)

			// emulate the logic of exponential backoff multiplier calc
			var mul int64
			if i == 0 {
				mul = 0
			} else {
				mul = 1 << (i - 1)
			}

			stepMin := step * time.Duration(mul) / 2
			stepMax := step * time.Duration(mul+1)

			require.GreaterOrEqual(t, duration, stepMin)
			require.LessOrEqual(t, duration, stepMax)

			// wait for cache to get to retry.After
			clock.BlockUntil(1)

			// add some extra to the duration to ensure the retry occurs
			clock.Advance(p.cache.MaxRetryPeriod)
		case <-time.After(time.Minute):
			t.Fatalf("timeout waiting for event")
		}

		// wait for cache to fail again - backend will still produce a ConnectionProblem error
		select {
		case event := <-p.eventsC:
			require.Equal(t, WatcherFailed, event.Type)
		case <-time.After(30 * time.Second):
			t.Fatalf("timeout waiting for event")
		}
	}
}

// TestSetupConfigFns ensures that all WatchKinds used in setup config functions are present in ForAuth() as well.
func TestSetupConfigFns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	bk, err := memory.New(memory.Config{
		Context: ctx,
		Mirror:  true,
	})
	require.NoError(t, err)
	defer bk.Close()

	clusterConfigCache, err := local.NewClusterConfigurationService(bk)
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	require.NoError(t, err)
	err = clusterConfigCache.UpsertClusterName(clusterName)
	require.NoError(t, err)

	setupFuncs := map[string]SetupConfigFn{
		"ForProxy":          ForProxy,
		"ForRemoteProxy":    ForRemoteProxy,
		"ForNode":           ForNode,
		"ForKubernetes":     ForKubernetes,
		"ForApps":           ForApps,
		"ForDatabases":      ForDatabases,
		"ForWindowsDesktop": ForWindowsDesktop,
		"ForDiscovery":      ForDiscovery,
		"ForOkta":           ForOkta,
	}

	authKindMap := make(map[resourceKind]types.WatchKind)
	for _, wk := range ForAuth(Config{ClusterConfig: clusterConfigCache}).Watches {
		authKindMap[resourceKind{kind: wk.Kind, subkind: wk.SubKind}] = wk
	}

	for name, f := range setupFuncs {
		t.Run(name, func(t *testing.T) {
			for _, wk := range f(Config{ClusterConfig: clusterConfigCache}).Watches {
				authWK, ok := authKindMap[resourceKind{kind: wk.Kind, subkind: wk.SubKind}]
				if !ok || !authWK.Contains(wk) {
					t.Errorf("%s includes WatchKind %s that is missing from ForAuth", name, wk.String())
				}
				if wk.Kind == types.KindCertAuthority {
					require.NotEmpty(t, wk.Filter, "every setup fn except auth should have a CA filter")
				}
			}
		})
	}

	authCAWatchKind, ok := authKindMap[resourceKind{kind: types.KindCertAuthority}]
	require.True(t, ok)
	require.Empty(t, authCAWatchKind.Filter, "auth should not use a CA filter")
}

type proxyEvents struct {
	sync.Mutex
	watchers    []types.Watcher
	events      types.Events
	ignoreKinds map[resourceKind]struct{}
}

func (p *proxyEvents) getWatchers() []types.Watcher {
	p.Lock()
	defer p.Unlock()
	out := make([]types.Watcher, len(p.watchers))
	copy(out, p.watchers)
	return out
}

func (p *proxyEvents) closeWatchers() {
	p.Lock()
	defer p.Unlock()
	for i := range p.watchers {
		p.watchers[i].Close()
	}
	p.watchers = nil
}

func (p *proxyEvents) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	var effectiveKinds []types.WatchKind
	for _, requested := range watch.Kinds {
		if _, ok := p.ignoreKinds[resourceKind{kind: requested.Kind, subkind: requested.SubKind}]; ok {
			continue
		}
		effectiveKinds = append(effectiveKinds, requested)
	}

	if len(effectiveKinds) == 0 {
		return nil, trace.BadParameter("all of the requested kinds were ignored")
	}

	watch.Kinds = effectiveKinds
	w, err := p.events.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.Lock()
	defer p.Unlock()
	p.watchers = append(p.watchers, w)
	return w, nil
}

func newProxyEvents(events types.Events, ignoreKinds []types.WatchKind) *proxyEvents {
	ignoreSet := make(map[resourceKind]struct{}, len(ignoreKinds))
	for _, kind := range ignoreKinds {
		ignoreSet[resourceKind{kind: kind.Kind, subkind: kind.SubKind}] = struct{}{}
	}
	return &proxyEvents{
		events:      events,
		ignoreKinds: ignoreSet,
	}
}

// TestCacheWatchKindExistsInEvents ensures that the watch kinds for each cache component are in sync
// with proto Events delivered via WatchEvents. If a watch kind is added to a cache component, but it
// doesn't exist in the proto Events definition, an error will cause the WatchEvents stream to be closed.
// This causes the cache to reinitialize every time that an unknown message is received and can lead to
// a permanently unhealthy cache.
//
// While this test will ensure that there are no issues for the current release, it does not guarantee
// that this issue won't arise across releases.
func TestCacheWatchKindExistsInEvents(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Now())

	cases := map[string]Config{
		"ForAuth":        ForAuth(Config{}),
		"ForProxy":       ForProxy(Config{}),
		"ForRemoteProxy": ForRemoteProxy(Config{}),
		"ForNode":        ForNode(Config{}),
		"ForKubernetes":  ForKubernetes(Config{}),
		"ForApps":        ForApps(Config{}),
		"ForDatabases":   ForDatabases(Config{}),
		"ForOkta":        ForOkta(Config{}),
	}

	events := map[string]types.Resource{
		types.KindCertAuthority:                     &types.CertAuthorityV2{},
		types.KindClusterName:                       &types.ClusterNameV2{},
		types.KindClusterAuditConfig:                types.DefaultClusterAuditConfig(),
		types.KindClusterNetworkingConfig:           types.DefaultClusterNetworkingConfig(),
		types.KindClusterAuthPreference:             types.DefaultAuthPreference(),
		types.KindSessionRecordingConfig:            types.DefaultSessionRecordingConfig(),
		types.KindUIConfig:                          &types.UIConfigV1{},
		types.KindStaticTokens:                      &types.StaticTokensV2{},
		types.KindToken:                             &types.ProvisionTokenV2{},
		types.KindUser:                              &types.UserV2{},
		types.KindRole:                              &types.RoleV6{Version: types.V4},
		types.KindNamespace:                         &types.Namespace{},
		types.KindNode:                              &types.ServerV2{},
		types.KindProxy:                             &types.ServerV2{},
		types.KindAuthServer:                        &types.ServerV2{},
		types.KindReverseTunnel:                     &types.ReverseTunnelV2{},
		types.KindTunnelConnection:                  &types.TunnelConnectionV2{},
		types.KindAccessRequest:                     &types.AccessRequestV3{},
		types.KindAppServer:                         &types.AppServerV3{},
		types.KindApp:                               &types.AppV3{},
		types.KindWebSession:                        &types.WebSessionV2{SubKind: types.KindWebSession},
		types.KindAppSession:                        &types.WebSessionV2{SubKind: types.KindAppSession},
		types.KindSnowflakeSession:                  &types.WebSessionV2{SubKind: types.KindSnowflakeSession},
		types.KindSAMLIdPSession:                    &types.WebSessionV2{SubKind: types.KindSAMLIdPServiceProvider},
		types.KindWebToken:                          &types.WebTokenV3{},
		types.KindRemoteCluster:                     &types.RemoteClusterV3{},
		types.KindKubeServer:                        &types.KubernetesServerV3{},
		types.KindDatabaseService:                   &types.DatabaseServiceV1{},
		types.KindDatabaseServer:                    &types.DatabaseServerV3{},
		types.KindDatabase:                          &types.DatabaseV3{},
		types.KindNetworkRestrictions:               &types.NetworkRestrictionsV4{},
		types.KindLock:                              &types.LockV2{},
		types.KindWindowsDesktopService:             &types.WindowsDesktopServiceV3{},
		types.KindWindowsDesktop:                    &types.WindowsDesktopV3{},
		types.KindDynamicWindowsDesktop:             &types.DynamicWindowsDesktopV1{},
		types.KindInstaller:                         &types.InstallerV1{},
		types.KindKubernetesCluster:                 &types.KubernetesClusterV3{},
		types.KindSAMLIdPServiceProvider:            &types.SAMLIdPServiceProviderV1{},
		types.KindUserGroup:                         &types.UserGroupV1{},
		types.KindOktaImportRule:                    &types.OktaImportRuleV1{},
		types.KindOktaAssignment:                    &types.OktaAssignmentV1{},
		types.KindIntegration:                       &types.IntegrationV1{},
		types.KindDiscoveryConfig:                   newDiscoveryConfig(t, "discovery-config"),
		types.KindHeadlessAuthentication:            &types.HeadlessAuthentication{},
		types.KindUserLoginState:                    newUserLoginState(t, "user-login-state"),
		types.KindAuditQuery:                        newAuditQuery(t, "audit-query"),
		types.KindSecurityReport:                    newSecurityReport(t, "security-report"),
		types.KindSecurityReportState:               newSecurityReport(t, "security-report-state"),
		types.KindAccessList:                        newAccessList(t, "access-list", clock),
		types.KindAccessListMember:                  newAccessListMember(t, "access-list", "member"),
		types.KindAccessListReview:                  newAccessListReview(t, "access-list", "review"),
		types.KindKubeWaitingContainer:              newKubeWaitingContainer(t),
		types.KindNotification:                      types.Resource153ToLegacy(newUserNotification(t, "test")),
		types.KindGlobalNotification:                types.Resource153ToLegacy(newGlobalNotification(t, "test")),
		types.KindAccessMonitoringRule:              types.Resource153ToLegacy(newAccessMonitoringRule(t)),
		types.KindCrownJewel:                        types.Resource153ToLegacy(newCrownJewel(t, "test")),
		types.KindDatabaseObject:                    types.Resource153ToLegacy(newDatabaseObject(t, "test")),
		types.KindAccessGraphSettings:               types.Resource153ToLegacy(newAccessGraphSettings(t)),
		types.KindSPIFFEFederation:                  types.Resource153ToLegacy(newSPIFFEFederation("test")),
		types.KindStaticHostUser:                    types.Resource153ToLegacy(newStaticHostUser(t, "test")),
		types.KindAutoUpdateConfig:                  types.Resource153ToLegacy(newAutoUpdateConfig(t)),
		types.KindAutoUpdateVersion:                 types.Resource153ToLegacy(newAutoUpdateVersion(t)),
		types.KindAutoUpdateAgentRollout:            types.Resource153ToLegacy(newAutoUpdateAgentRollout(t)),
		types.KindUserTask:                          types.Resource153ToLegacy(newUserTasks(t)),
		types.KindProvisioningPrincipalState:        types.Resource153ToLegacy(newProvisioningPrincipalState("u-alice@example.com")),
		types.KindIdentityCenterAccount:             types.Resource153ToLegacy(newIdentityCenterAccount("some_account")),
		types.KindIdentityCenterAccountAssignment:   types.Resource153ToLegacy(newIdentityCenterAccountAssignment("some_account_assignment")),
		types.KindIdentityCenterPrincipalAssignment: types.Resource153ToLegacy(newIdentityCenterPrincipalAssignment("some_principal_assignment")),
		types.KindPluginStaticCredentials:           &types.PluginStaticCredentialsV1{},
		types.KindGitServer:                         &types.ServerV2{},
		types.KindWorkloadIdentity:                  types.Resource153ToLegacy(newWorkloadIdentity("some_identifier")),
	}

	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			for _, watch := range cfg.Watches {
				resource, ok := events[watch.Kind]
				require.True(t, ok, "missing event for kind %q", watch.Kind)

				protoEvent, err := client.EventToGRPC(types.Event{
					Type:     types.OpPut,
					Resource: resource,
				})
				require.NoError(t, err)

				event, err := client.EventFromGRPC(protoEvent)
				require.NoError(t, err)

				// unwrap the RFD 153 resource if necessary
				switch r := resource.(type) {
				case types.Resource153Unwrapper:
					eventResource, ok := event.Resource.(types.Resource153Unwrapper)
					require.True(t, ok)

					// if the resource is a protobuf message, pass an option so
					// attempting to compare the messages does not result in a panic
					switch r := r.Unwrap().(type) {
					case protobuf.Message:
						require.Empty(t, cmp.Diff(r, eventResource.Unwrap(), protocmp.Transform()))
					default:
						require.Empty(t, cmp.Diff(r, eventResource.Unwrap()))
					}
				default:
					require.Empty(t, cmp.Diff(resource, event.Resource))
				}
			}
		})
	}
}

// TestPartialHealth ensures that when an event source confirms only some resource kinds specified on the watch request,
// Cache operates in partially healthy mode in which it serves reads of the confirmed kinds from the cache and
// lets everything else pass through.
func TestPartialHealth(t *testing.T) {
	ctx := context.Background()

	// setup cache such that role resources wouldn't be recognized by the event source and wouldn't be cached.
	p, err := newPack(t.TempDir(), ForApps, ignoreKinds([]types.WatchKind{{Kind: types.KindRole}}))
	require.NoError(t, err)
	t.Cleanup(p.Close)

	role, err := types.NewRole("editor", types.RoleSpecV6{})
	require.NoError(t, err)
	_, err = p.accessS.UpsertRole(ctx, role)
	require.NoError(t, err)

	user, err := types.NewUser("bob")
	require.NoError(t, err)
	user, err = p.usersS.UpsertUser(ctx, user)
	require.NoError(t, err)
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindUser, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// make sure that the user resource works as normal and gets replicated to cache
	replicatedUsers, err := p.cache.GetUsers(ctx, false)
	require.NoError(t, err)
	require.Len(t, replicatedUsers, 1)

	// now add a label to the user directly in the cache
	meta := user.GetMetadata()
	meta.Labels = map[string]string{"origin": "cache"}
	user.SetMetadata(meta)
	_, err = p.cache.usersCache.UpsertUser(ctx, user)
	require.NoError(t, err)

	// the label on the returned user proves that it came from the cache
	resultUser, err := p.cache.GetUser(ctx, "bob", false)
	require.NoError(t, err)
	require.Equal(t, "cache", resultUser.GetMetadata().Labels["origin"])

	// query cache storage directly to ensure roles haven't been replicated
	rolesStoredInCache, err := p.cache.accessCache.GetRoles(ctx)
	require.NoError(t, err)
	require.Empty(t, rolesStoredInCache)

	// non-empty result here proves that it was not served from cache
	resultRoles, err := p.cache.GetRoles(ctx)
	require.NoError(t, err)
	require.Len(t, resultRoles, 1)

	// ensure that cache cannot be watched for resources that weren't confirmed in regular mode
	testWatch := types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindUser},
			{Kind: types.KindRole},
		},
	}
	_, err = p.cache.NewWatcher(ctx, testWatch)
	require.Error(t, err)

	// same request should work in partial success mode, but WatchStatus on the OpInit event should indicate
	// that only user resources will be watched.
	testWatch.AllowPartialSuccess = true
	w, err := p.cache.NewWatcher(ctx, testWatch)
	require.NoError(t, err)

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
		watchStatus, ok := e.Resource.(types.WatchStatus)
		require.True(t, ok)
		require.Equal(t, []types.WatchKind{{Kind: types.KindUser}}, watchStatus.GetKinds())
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event.")
	}
}

// TestInvalidDatbases given a database that returns an error on validation, the
// cache should not be impacted, and the database must be on it. This scenario
// is most common on Teleport upgrades/downgrades where the database validation
// can have new rules, causing the existing database to fail on validation.
func TestInvalidDatabases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	generateInvalidDB := func(t *testing.T, name string) types.Database {
		db := &types.DatabaseV3{Metadata: types.Metadata{
			Name: name,
		}, Spec: types.DatabaseSpecV3{
			Protocol: "invalid-protocol",
			URI:      "non-empty-uri",
		}}
		// Just ensures the database we're using on this test will trigger a
		// validation failure.
		require.Error(t, services.ValidateDatabase(db))
		return db
	}

	for name, tc := range map[string]struct {
		storeFunc func(*testing.T, *backend.Wrapper, *Cache)
	}{
		"CreateDatabase": {
			storeFunc: func(t *testing.T, b *backend.Wrapper, _ *Cache) {
				db := generateInvalidDB(t, "invalid-db")
				value, err := services.MarshalDatabase(db)
				require.NoError(t, err)
				_, err = b.Create(ctx, backend.Item{
					Key:     backend.NewKey("db", db.GetName()),
					Value:   value,
					Expires: db.Expiry(),
				})
				require.NoError(t, err)
			},
		},
		"UpdateDatabase": {
			storeFunc: func(t *testing.T, b *backend.Wrapper, c *Cache) {
				dbName := "updated-db"
				validDB, err := types.NewDatabaseV3(types.Metadata{
					Name: dbName,
				}, types.DatabaseSpecV3{
					Protocol: defaults.ProtocolPostgres,
					URI:      "postgres://localhost",
				})
				require.NoError(t, err)
				require.NoError(t, services.ValidateDatabase(validDB))

				marshalledDB, err := services.MarshalDatabase(validDB)
				require.NoError(t, err)
				_, err = b.Create(ctx, backend.Item{
					Key:     backend.NewKey("db", validDB.GetName()),
					Value:   marshalledDB,
					Expires: validDB.Expiry(),
				})
				require.NoError(t, err)

				// Wait until the database appear on cache.
				require.EventuallyWithT(t, func(t *assert.CollectT) {
					dbs, err := c.GetDatabases(ctx)
					assert.NoError(t, err)
					assert.Len(t, dbs, 1)
				}, time.Second, 100*time.Millisecond, "expected database to be on cache, but nothing found")

				cacheDB, err := c.GetDatabase(ctx, dbName)
				require.NoError(t, err)

				invalidDB := generateInvalidDB(t, cacheDB.GetName())
				value, err := services.MarshalDatabase(invalidDB)
				require.NoError(t, err)
				_, err = b.Update(ctx, backend.Item{
					Key:     backend.NewKey("db", cacheDB.GetName()),
					Value:   value,
					Expires: invalidDB.Expiry(),
				})
				require.NoError(t, err)
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := newTestPack(t, ForAuth)
			t.Cleanup(p.Close)

			tc.storeFunc(t, p.backend, p.cache)

			// Events processing should not restart due to an invalid database error.
			unexpectedEvent(t, p.eventsC, Reloading)
		})
	}
}

func newDiscoveryConfig(t *testing.T, name string) *discoveryconfig.DiscoveryConfig {
	t.Helper()

	discoveryConfig, err := discoveryconfig.NewDiscoveryConfig(
		header.Metadata{
			Name: name,
		},
		discoveryconfig.Spec{
			DiscoveryGroup: "mygroup",
			AWS:            []types.AWSMatcher{},
			Azure:          []types.AzureMatcher{},
			GCP:            []types.GCPMatcher{},
			Kube:           []types.KubernetesMatcher{},
		},
	)
	require.NoError(t, err)
	discoveryConfig.Status.State = "DISCOVERY_CONFIG_STATE_UNSPECIFIED"
	return discoveryConfig
}

func newAuditQuery(t *testing.T, name string) *secreports.AuditQuery {
	t.Helper()

	item, err := secreports.NewAuditQuery(
		header.Metadata{
			Name: name,
		},
		secreports.AuditQuerySpec{
			Name:        name,
			Title:       "title",
			Description: "desc",
			Query:       "query",
		},
	)
	require.NoError(t, err)
	return item
}

func newSecurityReport(t *testing.T, name string) *secreports.Report {
	t.Helper()
	item, err := secreports.NewReport(
		header.Metadata{
			Name: name,
		},
		secreports.ReportSpec{
			Name:  name,
			Title: "title",
			AuditQueries: []*secreports.AuditQuerySpec{
				{
					Name:        "name",
					Title:       "title",
					Description: "desc",
					Query:       "query",
				},
			},
			Version: "0.0.0",
		},
	)
	require.NoError(t, err)
	return item
}

func newSecurityReportState(t *testing.T, name string) *secreports.ReportState {
	t.Helper()
	item, err := secreports.NewReportState(
		header.Metadata{
			Name: name,
		},
		secreports.ReportStateSpec{
			Status:    "RUNNING",
			UpdatedAt: time.Now().UTC(),
		},
	)
	require.NoError(t, err)
	return item
}

func newUserLoginState(t *testing.T, name string) *userloginstate.UserLoginState {
	t.Helper()

	uls, err := userloginstate.New(
		header.Metadata{
			Name: name,
		},
		userloginstate.Spec{
			Roles:          []string{"role1", "role2"},
			OriginalTraits: trait.Traits{},
			Traits: trait.Traits{
				"key1": []string{"value1"},
				"key2": []string{"value2"},
			},
		},
	)
	require.NoError(t, err)
	return uls
}

func newAccessList(t *testing.T, name string, clock clockwork.Clock) *accesslist.AccessList {
	t.Helper()

	accessList, err := accesslist.NewAccessList(
		header.Metadata{
			Name: name,
		},
		accesslist.Spec{
			Title:       "title",
			Description: "test access list",
			Owners: []accesslist.Owner{
				{
					Name:        "test-user1",
					Description: "test user 1",
				},
				{
					Name:        "test-user2",
					Description: "test user 2",
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: clock.Now(),
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"mrole1", "mrole2"},
				Traits: map[string][]string{
					"mtrait1": {"mvalue1", "mvalue2"},
					"mtrait2": {"mvalue3", "mvalue4"},
				},
			},
			OwnershipRequires: accesslist.Requires{
				Roles: []string{"orole1", "orole2"},
				Traits: map[string][]string{
					"otrait1": {"ovalue1", "ovalue2"},
					"otrait2": {"ovalue3", "ovalue4"},
				},
			},
			Grants: accesslist.Grants{
				Roles: []string{"grole1", "grole2"},
				Traits: map[string][]string{
					"gtrait1": {"gvalue1", "gvalue2"},
					"gtrait2": {"gvalue3", "gvalue4"},
				},
			},
		},
	)
	require.NoError(t, err)

	return accessList
}

func newAccessListMember(t *testing.T, accessList, name string) *accesslist.AccessListMember {
	t.Helper()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: name,
		},
		accesslist.AccessListMemberSpec{
			AccessList: accessList,
			Name:       name,
			Joined:     time.Now(),
			Expires:    time.Now().Add(time.Hour * 24),
			Reason:     "a reason",
			AddedBy:    "dummy",
		},
	)
	require.NoError(t, err)

	return member
}

func newAccessListReview(t *testing.T, accessList, name string) *accesslist.Review {
	t.Helper()

	review, err := accesslist.NewReview(
		header.Metadata{
			Name: name,
		},
		accesslist.ReviewSpec{
			AccessList: accessList,
			Reviewers: []string{
				"user1",
				"user2",
			},
			ReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			Notes:      "Some notes",
			Changes: accesslist.ReviewChanges{
				MembershipRequirementsChanged: &accesslist.Requires{
					Roles: []string{
						"role1",
						"role2",
					},
					Traits: trait.Traits{
						"trait1": []string{
							"value1",
							"value2",
						},
						"trait2": []string{
							"value1",
							"value2",
						},
					},
				},
				RemovedMembers: []string{
					"member1",
					"member2",
				},
				ReviewFrequencyChanged:  accesslist.ThreeMonths,
				ReviewDayOfMonthChanged: accesslist.FifteenthDayOfMonth,
			},
		},
	)
	require.NoError(t, err)

	return review
}

func newKubeWaitingContainer(t *testing.T) types.Resource {
	t.Helper()

	waitingCont, err := kubewaitingcontainer.NewKubeWaitingContainer("container", &kubewaitingcontainerpb.KubernetesWaitingContainerSpec{
		Username:      "user",
		Cluster:       "cluster",
		Namespace:     "namespace",
		PodName:       "pod",
		ContainerName: "container",
		Patch:         []byte("patch"),
		PatchType:     "application/json-patch+json",
	})
	require.NoError(t, err)

	return types.Resource153ToLegacy(waitingCont)
}

func newCrownJewel(t *testing.T, name string) *crownjewelv1.CrownJewel {
	t.Helper()

	crownJewel := &crownjewelv1.CrownJewel{
		Metadata: &headerv1.Metadata{
			Name: name,
		},
	}

	return crownJewel
}

func newDatabaseObject(t *testing.T, name string) *dbobjectv1.DatabaseObject {
	t.Helper()

	r, err := databaseobject.NewDatabaseObject(name, &dbobjectv1.DatabaseObjectSpec{
		Name:                name,
		Protocol:            "postgres",
		DatabaseServiceName: "pg",
		ObjectKind:          "table",
	})
	require.NoError(t, err)
	return r
}

func newAccessGraphSettings(t *testing.T) *clusterconfigpb.AccessGraphSettings {
	t.Helper()

	r, err := clusterconfig.NewAccessGraphSettings(&clusterconfigpb.AccessGraphSettingsSpec{
		SecretsScanConfig: clusterconfigpb.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_ENABLED,
	})
	require.NoError(t, err)
	return r
}

func newUserNotification(t *testing.T, name string) *notificationsv1.Notification {
	t.Helper()

	notification := &notificationsv1.Notification{
		SubKind: "test-subkind",
		Spec: &notificationsv1.NotificationSpec{
			Username: name,
		},
		Metadata: &headerv1.Metadata{
			Labels: map[string]string{types.NotificationTitleLabel: "test-title"},
		},
	}

	return notification
}

func newGlobalNotification(t *testing.T, title string) *notificationsv1.GlobalNotification {
	t.Helper()

	notification := &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_All{
				All: true,
			},
			Notification: &notificationsv1.Notification{
				SubKind: "test-subkind",
				Spec:    &notificationsv1.NotificationSpec{},
				Metadata: &headerv1.Metadata{
					Labels: map[string]string{types.NotificationTitleLabel: title},
				},
			},
		},
	}

	return notification
}

func newAccessMonitoringRule(t *testing.T) *accessmonitoringrulesv1.AccessMonitoringRule {
	t.Helper()
	notification := &accessmonitoringrulesv1.AccessMonitoringRule{
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Notification: &accessmonitoringrulesv1.Notification{},
		},
	}
	return notification
}

func newStaticHostUser(t *testing.T, name string) *userprovisioningpb.StaticHostUser {
	t.Helper()
	return userprovisioning.NewStaticHostUser(name, &userprovisioningpb.StaticHostUserSpec{
		Matchers: []*userprovisioningpb.Matcher{
			{
				NodeLabels: []*labelv1.Label{
					{
						Name:   "foo",
						Values: []string{"bar"},
					},
				},
				Groups: []string{"foo", "bar"},
			},
		},
	})
}

func newAutoUpdateConfig(t *testing.T) *autoupdate.AutoUpdateConfig {
	t.Helper()

	r, err := update.NewAutoUpdateConfig(&autoupdate.AutoUpdateConfigSpec{
		Tools: &autoupdate.AutoUpdateConfigSpecTools{
			Mode: update.ToolsUpdateModeEnabled,
		},
	})
	require.NoError(t, err)
	return r
}

func newAutoUpdateVersion(t *testing.T) *autoupdate.AutoUpdateVersion {
	t.Helper()

	r, err := update.NewAutoUpdateVersion(&autoupdate.AutoUpdateVersionSpec{
		Tools: &autoupdate.AutoUpdateVersionSpecTools{
			TargetVersion: "1.2.3",
		},
	})
	require.NoError(t, err)
	return r
}

func newAutoUpdateAgentRollout(t *testing.T) *autoupdate.AutoUpdateAgentRollout {
	t.Helper()

	r, err := update.NewAutoUpdateAgentRollout(&autoupdate.AutoUpdateAgentRolloutSpec{
		StartVersion:   "1.2.3",
		TargetVersion:  "2.3.4",
		Schedule:       update.AgentsScheduleImmediate,
		AutoupdateMode: update.AgentsUpdateModeEnabled,
		Strategy:       update.AgentsStrategyTimeBased,
	})
	require.NoError(t, err)
	return r
}

func withKeepalive[T any](fn func(context.Context, T) (*types.KeepAlive, error)) func(context.Context, T) error {
	return func(ctx context.Context, resource T) error {
		_, err := fn(ctx, resource)
		return err
	}
}

func modifyNoContext[T any](fn func(T) error) func(context.Context, T) error {
	return func(_ context.Context, resource T) error {
		return fn(resource)
	}
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="IAMShowcase" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`

// TestCAWatcherFilters tests cache CA watchers with filters are not rejected
// by auth, even if a CA filter includes a "new" CA type.
func TestCAWatcherFilters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	allCAsAndNewCAFilter := makeAllKnownCAsFilter()
	// auth will never send such an event, but it won't reject the watch request
	// either since auth cache's confirmedKinds dont have a CA filter.
	allCAsAndNewCAFilter["someBackportedCAType"] = "*"

	tests := []struct {
		desc    string
		filter  types.CertAuthorityFilter
		watcher types.Watcher
	}{
		{
			desc: "empty filter",
		},
		{
			desc:   "all CAs filter",
			filter: makeAllKnownCAsFilter(),
		},
		{
			desc:   "all CAs and a new CA filter",
			filter: allCAsAndNewCAFilter,
		},
	}

	// setup watchers for each test case before we generate events.
	for i := range tests {
		test := &tests[i]
		w, err := p.cache.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{
			{
				Kind:   types.KindCertAuthority,
				Filter: test.filter.IntoMap(),
			},
		}})
		require.NoError(t, err)
		test.watcher = w
		t.Cleanup(func() {
			require.NoError(t, w.Close())
		})
	}

	// generate an OpPut event.
	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ctx, ca))

	const fetchTimeout = time.Second
	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			event := fetchEvent(t, test.watcher, fetchTimeout)
			require.Equal(t, types.OpInit, event.Type)

			event = fetchEvent(t, test.watcher, fetchTimeout)
			require.Equal(t, types.OpPut, event.Type)
			require.Equal(t, types.KindCertAuthority, event.Resource.GetKind())
			gotCA, ok := event.Resource.(*types.CertAuthorityV2)
			require.True(t, ok)
			require.Equal(t, types.UserCA, gotCA.GetType())
		})
	}
}

func fetchEvent(t *testing.T, w types.Watcher, timeout time.Duration) types.Event {
	t.Helper()
	timeoutC := time.After(timeout)
	var ev types.Event
	select {
	case <-timeoutC:
		require.Fail(t, "Timeout waiting for event", w.Error())
	case <-w.Done():
		require.Fail(t, "Watcher exited with error", w.Error())
	case ev = <-w.Events():
	}
	return ev
}
