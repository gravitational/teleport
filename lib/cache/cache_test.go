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
	healthcheckconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/healthcheckconfig/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	kubewaitingcontainerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
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
	scopedrole "github.com/gravitational/teleport/lib/scopes/roles"
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

	eventsS                 *proxyEvents
	trustS                  *local.CA
	provisionerS            *local.ProvisioningService
	clusterConfigS          *local.ClusterConfigurationService
	usersS                  *local.IdentityService
	accessS                 *local.AccessService
	dynamicAccessS          *local.DynamicAccessService
	presenceS               *local.PresenceService
	appSessionS             *local.IdentityService
	snowflakeSessionS       *local.IdentityService
	restrictions            *local.RestrictionsService
	apps                    *local.AppService
	kubernetes              *local.KubernetesService
	databases               *local.DatabaseService
	databaseServices        *local.DatabaseServicesService
	webSessionS             types.WebSessionInterface
	webTokenS               types.WebTokenInterface
	windowsDesktops         *local.WindowsDesktopService
	dynamicWindowsDesktops  *local.DynamicWindowsDesktopService
	samlIDPServiceProviders *local.SAMLIdPServiceProviderService
	userGroups              *local.UserGroupService
	okta                    *local.OktaService
	integrations            *local.IntegrationsService
	userTasks               *local.UserTasksService
	discoveryConfigs        *local.DiscoveryConfigService
	userLoginStates         *local.UserLoginStateService
	secReports              *local.SecReportsService
	accessLists             *local.AccessListService
	kubeWaitingContainers   *local.KubeWaitingContainerService
	notifications           *local.NotificationsService
	accessMonitoringRules   *local.AccessMonitoringRulesService
	crownJewels             *local.CrownJewelsService
	databaseObjects         *local.DatabaseObjectService
	spiffeFederations       *local.SPIFFEFederationService
	staticHostUsers         *local.StaticHostUserService
	autoUpdateService       *local.AutoUpdateService
	provisioningStates      *local.ProvisioningStateService
	identityCenter          *local.IdentityCenterService
	pluginStaticCredentials *local.PluginStaticCredentialsService
	gitServers              *local.GitServerService
	workloadIdentity        *local.WorkloadIdentityService
	healthCheckConfig       *local.HealthCheckConfigService
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

	p.healthCheckConfig, err = local.NewHealthCheckConfigService(p.backend)
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
		HealthCheckConfig:       p.healthCheckConfig,
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
			HealthCheckConfig:       p.healthCheckConfig,
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
		HealthCheckConfig:       p.healthCheckConfig,
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

	for _, limit := range []int32{100, 1_000, 10_000, 100_000} {
		for _, totalCount := range []bool{true, false} {
			b.Run(fmt.Sprintf("limit=%d,needTotal=%t", limit, totalCount), func(b *testing.B) {
				for b.Loop() {
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
		HealthCheckConfig:       p.healthCheckConfig,
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
		HealthCheckConfig:       p.healthCheckConfig,
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
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Make sure the cache has a single resource in it.
		out, err = funcs.cacheList(ctx)
		assert.NoError(t, err)
		assert.Empty(t, cmp.Diff([]T{r}, out, cmpOpts...))
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
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Make sure the cache has a single resource in it.
		out, err = funcs.cacheList(ctx)
		assert.NoError(t, err)
		assert.Empty(t, cmp.Diff([]T{r}, out, cmpOpts...))
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
		cmpopts.EquateEmpty(),
	}

	assertCacheContents := func(expected []T) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			out, err := funcs.cacheList(ctx)
			assert.NoError(t, err)

			// If the cache is expected to be empty, then test explicitly for
			// *that* rather than do an equality test. An equality test here
			// would be overly-pedantic about a service returning `nil` rather
			// than an empty slice.
			if len(expected) == 0 {
				assert.Empty(t, out)
				return
			}

			assert.Empty(t, cmp.Diff(expected, out, cmpOpts...))
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
		types.KindAutoUpdateAgentReport:             types.Resource153ToLegacy(newAutoUpdateAgentReport(t, "test")),
		types.KindUserTask:                          types.Resource153ToLegacy(newUserTasks(t)),
		types.KindProvisioningPrincipalState:        types.Resource153ToLegacy(newProvisioningPrincipalState("u-alice@example.com")),
		types.KindIdentityCenterAccount:             types.Resource153ToLegacy(newIdentityCenterAccount("some_account")),
		types.KindIdentityCenterAccountAssignment:   types.Resource153ToLegacy(newIdentityCenterAccountAssignment("some_account_assignment")),
		types.KindIdentityCenterPrincipalAssignment: types.Resource153ToLegacy(newIdentityCenterPrincipalAssignment("some_principal_assignment")),
		types.KindPluginStaticCredentials:           &types.PluginStaticCredentialsV1{},
		types.KindGitServer:                         &types.ServerV2{},
		types.KindWorkloadIdentity:                  types.Resource153ToLegacy(newWorkloadIdentity("some_identifier")),
		types.KindHealthCheckConfig:                 types.Resource153ToLegacy(newHealthCheckConfig(t, "some-name")),
		scopedrole.KindScopedRole:                   types.Resource153ToLegacy(&accessv1.ScopedRole{}),
		scopedrole.KindScopedRoleAssignment:         types.Resource153ToLegacy(&accessv1.ScopedRoleAssignment{}),
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
				switch uw := event.Resource.(type) {
				case types.Resource153UnwrapperT[*workloadidentityv1.WorkloadIdentity]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*workloadidentityv1.WorkloadIdentity]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*identitycenterv1.PrincipalAssignment]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*identitycenterv1.PrincipalAssignment]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*identitycenterv1.AccountAssignment]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*identitycenterv1.AccountAssignment]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*identitycenterv1.Account]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*identitycenterv1.Account]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*provisioningv1.PrincipalState]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*provisioningv1.PrincipalState]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*usertasksv1.UserTask]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*usertasksv1.UserTask]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*autoupdate.AutoUpdateAgentReport]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*autoupdate.AutoUpdateAgentReport]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*autoupdate.AutoUpdateAgentRollout]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*autoupdate.AutoUpdateAgentRollout]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*autoupdate.AutoUpdateVersion]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*autoupdate.AutoUpdateVersion]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*autoupdate.AutoUpdateConfig]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*autoupdate.AutoUpdateConfig]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*userprovisioningpb.StaticHostUser]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*userprovisioningpb.StaticHostUser]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*machineidv1.SPIFFEFederation]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*machineidv1.SPIFFEFederation]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*clusterconfigpb.AccessGraphSettings]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*clusterconfigpb.AccessGraphSettings]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*dbobjectv1.DatabaseObject]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*dbobjectv1.DatabaseObject]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*crownjewelv1.CrownJewel]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*crownjewelv1.CrownJewel]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*accessmonitoringrulesv1.AccessMonitoringRule]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*accessmonitoringrulesv1.AccessMonitoringRule]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*notificationsv1.GlobalNotification]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*notificationsv1.GlobalNotification]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*notificationsv1.Notification]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*notificationsv1.Notification]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*kubewaitingcontainerpb.KubernetesWaitingContainer]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*kubewaitingcontainerpb.KubernetesWaitingContainer]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*healthcheckconfigv1.HealthCheckConfig]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*healthcheckconfigv1.HealthCheckConfig]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*accessv1.ScopedRole]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*accessv1.ScopedRole]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
				case types.Resource153UnwrapperT[*accessv1.ScopedRoleAssignment]:
					require.Empty(t, cmp.Diff(resource.(types.Resource153UnwrapperT[*accessv1.ScopedRoleAssignment]).UnwrapT(), uw.UnwrapT(), protocmp.Transform()))
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
	err = p.cache.collections.users.onPut(user)
	require.NoError(t, err)

	// the label on the returned user proves that it came from the cache
	resultUser, err := p.cache.GetUser(ctx, "bob", false)
	require.NoError(t, err)
	require.Equal(t, "cache", resultUser.GetMetadata().Labels["origin"])

	// query cache storage directly to ensure roles haven't been replicated
	require.Empty(t, p.cache.collections.roles.store.len())

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
		Kind:     types.KindAccessMonitoringRule,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Notification: &accessmonitoringrulesv1.Notification{
				Name: "test",
			},
			Subjects:  []string{"llama", "shark"},
			Condition: "test",
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

func newAutoUpdateAgentReport(t *testing.T, name string) *autoupdate.AutoUpdateAgentReport {
	t.Helper()

	r, err := update.NewAutoUpdateAgentReport(&autoupdate.AutoUpdateAgentReportSpec{
		Timestamp: timestamppb.Now(),
		Groups: map[string]*autoupdate.AutoUpdateAgentReportSpecGroup{
			"foo": {
				Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
					"1.2.3": {Count: 1},
					"1.2.4": {Count: 2},
				},
			},
			"bar": {
				Versions: map[string]*autoupdate.AutoUpdateAgentReportSpecGroupVersion{
					"2.3.4": {Count: 3},
					"2.3.5": {Count: 4},
				},
			},
		},
	}, name)
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
