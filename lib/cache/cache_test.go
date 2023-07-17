/*
Copyright 2018-2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"
)

const eventBufferSize = 1024

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
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
	samlIDPServiceProviders services.SAMLIdPServiceProviders
	userGroups              services.UserGroups
	okta                    services.Okta
	integrations            services.Integrations
}

// testFuncs are functions to support testing an object in a cache.
type testFuncs[T types.Resource] struct {
	newResource func(string) (T, error)
	create      func(context.Context, T) error
	list        func(context.Context) ([]T, error)
	cacheGet    func(context.Context, string) (T, error)
	cacheList   func(context.Context) ([]T, error)
	update      func(context.Context, T) error
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
		log.Warningf("Failed to close %v", err)
	}
}

func newPackForAuth(t *testing.T) *testPack {
	return newTestPack(t, ForAuth)
}

func newPackForProxy(t *testing.T) *testPack {
	return newTestPack(t, ForProxy)
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

	p.trustS = local.NewCAService(p.backend)
	p.clusterConfigS = clusterConfig
	p.provisionerS = local.NewProvisioningService(p.backend)
	p.eventsS = newProxyEvents(local.NewEventsService(p.backend), cfg.ignoreKinds)
	p.presenceS = local.NewPresenceService(p.backend)
	p.usersS = local.NewIdentityService(p.backend)
	p.accessS = local.NewAccessService(p.backend)
	p.dynamicAccessS = local.NewDynamicAccessService(p.backend)
	p.appSessionS = local.NewIdentityService(p.backend)
	p.webSessionS = local.NewIdentityService(p.backend).WebSessions()
	p.snowflakeSessionS = local.NewIdentityService(p.backend)
	p.samlIdPSessionsS = local.NewIdentityService(p.backend)
	p.webTokenS = local.NewIdentityService(p.backend).WebTokens()
	p.restrictions = local.NewRestrictionsService(p.backend)
	p.apps = local.NewAppService(p.backend)
	p.kubernetes = local.NewKubernetesService(p.backend)
	p.databases = local.NewDatabasesService(p.backend)
	p.databaseServices = local.NewDatabaseServicesService(p.backend)
	p.windowsDesktops = local.NewWindowsDesktopService(p.backend)
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

	igSvc, err := local.NewIntegrationsService(p.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.integrations = igSvc

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
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
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
	ca.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(ca, out))

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

	require.NoError(t, p.dynamicAccessS.CreateAccessRequest(ctx, req))

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
	require.NoError(t, p.dynamicAccessS.CreateAccessRequest(ctx, req2))
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
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
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
			SAMLIdPServiceProviders: p.samlIDPServiceProviders,
			UserGroups:              p.userGroups,
			Okta:                    p.okta,
			Integrations:            p.integrations,
			MaxRetryPeriod:          200 * time.Millisecond,
			EventsC:                 p.eventsC,
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
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 p.eventsC,
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
	benchGetNodes(b, backend.DefaultRangeLimit)
}

func benchGetNodes(b *testing.B, nodeCount int) {
	p, err := newPack(b.TempDir(), ForAuth, memoryBackend(true))
	require.NoError(b, err)
	defer p.Close()

	ctx := context.Background()

	for i := 0; i < nodeCount; i++ {
		server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
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
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 p.eventsC,
		neverOK:                 true, // ensure reads are never healthy
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
	require.Eventually(t, func() bool {
		resp, err := p.cache.ListResources(ctx, proto.ListResourcesRequest{
			Namespace:    apidefaults.Namespace,
			ResourceType: types.KindNode,
			StartKey:     listResourcesStartKey,
			Limit:        int32(pageSize),
			SortBy:       sortBy,
		})
		require.NoError(t, err)
		resources = append(resources, resp.Resources...)
		listResourcesStartKey = resp.NextKey
		return len(resources) == nodeCount
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
		SAMLIdPServiceProviders: p.samlIDPServiceProviders,
		UserGroups:              p.userGroups,
		Okta:                    p.okta,
		Integrations:            p.integrations,
		MaxRetryPeriod:          200 * time.Millisecond,
		EventsC:                 p.eventsC,
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
		ca.SetResourceID(0)
		ca.SetExpiry(time.Time{})
		types.RemoveCASecrets(ca)
		return ca
	}
	_ = normalizeCA

	out, err := p.cache.GetCertAuthority(ctx, ca.GetID(), false)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(normalizeCA(ca), normalizeCA(out)))

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
	require.Equal(t, out.GetResourceID(), out2.GetResourceID())
	require.Empty(t, cmp.Diff(normalizeCA(ca), normalizeCA(out)))

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
	require.Empty(t, cmp.Diff(normalizeCA(ca), normalizeCA(out)))
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
	ca2.SetResourceID(out.GetResourceID())
	types.RemoveCASecrets(ca2)
	require.Empty(t, cmp.Diff(ca2, out))
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
	staticTokens.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(staticTokens, out))

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
	token.SetResourceID(tout.GetResourceID())
	require.Empty(t, cmp.Diff(token, tout))

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
	err = p.clusterConfigS.SetAuthPreference(ctx, authPref)
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

	authPref.SetResourceID(outAuthPref.GetResourceID())
	require.Empty(t, cmp.Diff(outAuthPref, authPref))
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
	err = p.clusterConfigS.SetClusterNetworkingConfig(ctx, netConfig)
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

	netConfig.SetResourceID(outNetConfig.GetResourceID())
	require.Empty(t, cmp.Diff(outNetConfig, netConfig))
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
	err = p.clusterConfigS.SetSessionRecordingConfig(ctx, recConfig)
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

	recConfig.SetResourceID(outRecConfig.GetResourceID())
	require.Empty(t, cmp.Diff(outRecConfig, recConfig))
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

	auditConfig.SetResourceID(outAuditConfig.GetResourceID())
	require.Empty(t, cmp.Diff(outAuditConfig, auditConfig))
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

	clusterName.SetResourceID(outName.GetResourceID())
	require.Empty(t, cmp.Diff(outName, clusterName))
}

// TestNamespaces tests caching of namespaces
func TestNamespaces(t *testing.T) {
	t.Parallel()

	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	v, err := types.NewNamespace("universe")
	require.NoError(t, err)
	ns := &v
	err = p.presenceS.UpsertNamespace(*ns)
	require.NoError(t, err)

	ns, err = p.presenceS.GetNamespace(ns.GetName())
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetNamespace(ns.GetName())
	require.NoError(t, err)
	ns.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(ns, out))

	// update namespace metadata
	ns.Metadata.Labels = map[string]string{"a": "b"}
	require.NoError(t, err)
	err = p.presenceS.UpsertNamespace(*ns)
	require.NoError(t, err)

	ns, err = p.presenceS.GetNamespace(ns.GetName())
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNamespace(ns.GetName())
	require.NoError(t, err)
	ns.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(ns, out))

	err = p.presenceS.DeleteNamespace(ns.GetName())
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetNamespace(ns.GetName())
	require.True(t, trace.IsNotFound(err))
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
		create: modifyNoContext(p.usersS.UpsertUser),
		list: func(ctx context.Context) ([]types.User, error) {
			return p.usersS.GetUsers(false)
		},
		cacheList: func(ctx context.Context) ([]types.User, error) {
			return p.cache.GetUsers(false)
		},
		update: modifyNoContext(p.usersS.UpsertUser),
		deleteAll: func(_ context.Context) error {
			return p.usersS.DeleteAllUsers()
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
		create:    p.accessS.UpsertRole,
		list:      p.accessS.GetRoles,
		cacheGet:  p.cache.GetRole,
		cacheList: p.cache.GetRoles,
		update:    p.accessS.UpsertRole,
		deleteAll: func(_ context.Context) error {
			return p.accessS.DeleteAllRoles()
		},
	})
}

// TestReverseTunnels tests reverse tunnels caching
func TestReverseTunnels(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.ReverseTunnel]{
		newResource: func(name string) (types.ReverseTunnel, error) {
			return types.NewReverseTunnel(name, []string{"example.com:2023"})
		},
		create: modifyNoContext(p.presenceS.UpsertReverseTunnel),
		list: func(ctx context.Context) ([]types.ReverseTunnel, error) {
			return p.presenceS.GetReverseTunnels(ctx)
		},
		cacheList: func(ctx context.Context) ([]types.ReverseTunnel, error) {
			return p.cache.GetReverseTunnels(ctx)
		},
		update: modifyNoContext(p.presenceS.UpsertReverseTunnel),
		deleteAll: func(ctx context.Context) error {
			return p.presenceS.DeleteAllReverseTunnels()
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
		create: modifyNoContext(p.presenceS.UpsertTunnelConnection),
		list: func(ctx context.Context) ([]types.TunnelConnection, error) {
			return p.presenceS.GetTunnelConnections(clusterName)
		},
		cacheList: func(ctx context.Context) ([]types.TunnelConnection, error) {
			return p.cache.GetTunnelConnections(clusterName)
		},
		update: modifyNoContext(p.presenceS.UpsertTunnelConnection),
		deleteAll: func(ctx context.Context) error {
			return p.presenceS.DeleteAllTunnelConnections()
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
		create: modifyNoContext(p.presenceS.CreateRemoteCluster),
		list: func(ctx context.Context) ([]types.RemoteCluster, error) {
			return p.presenceS.GetRemoteClusters()
		},
		cacheGet: func(ctx context.Context, name string) (types.RemoteCluster, error) {
			return p.cache.GetRemoteCluster(name)
		},
		cacheList: func(_ context.Context) ([]types.RemoteCluster, error) {
			return p.cache.GetRemoteClusters()
		},
		update: p.presenceS.UpdateRemoteCluster,
		deleteAll: func(_ context.Context) error {
			return p.presenceS.DeleteAllRemoteClusters()
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
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
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

// testResources is a generic tester for resources.
func testResources[T types.Resource](t *testing.T, p *testPack, funcs testFuncs[T]) {
	ctx := context.Background()

	// Create a resource.
	r, err := funcs.newResource("test-sp")
	require.NoError(t, err)
	r.SetExpiry(time.Now().Add(30 * time.Minute))

	err = funcs.create(ctx, r)
	require.NoError(t, err)

	// Check that the resource is now in the backend.
	out, err := funcs.list(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Wait until the information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single resource in it.
	out, err = funcs.cacheList(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// cacheGet is optional as not every resource implements it
	if funcs.cacheGet != nil {
		// Make sure a single cache get works.
		getR, err := funcs.cacheGet(ctx, r.GetName())
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(r, getR,
			cmpopts.IgnoreFields(types.Metadata{}, "ID")))
	}

	// Update the resource and upsert it into the backend again.
	r.SetExpiry(r.Expiry().Add(30 * time.Minute))
	err = funcs.update(ctx, r)
	require.NoError(t, err)

	// Check that the resource is in the backend and only one exists (so an
	// update occurred).
	out, err = funcs.list(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single resource in it.
	out, err = funcs.cacheList(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]T{r}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Remove all service providers from the backend.
	err = funcs.deleteAll(ctx)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = funcs.cacheList(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

func TestRelativeExpiry(t *testing.T) {
	t.Parallel()

	const checkInterval = time.Second
	const nodeCount = int64(100)

	// make sure the event buffer is much larger than node count
	// so that we can batch create nodes without waiting on each event
	require.True(t, int(nodeCount*3) < eventBufferSize)

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
	require.True(t, len(nodes) > 0, "node_count=%d", len(nodes))
}

func TestRelativeExpiryLimit(t *testing.T) {
	const (
		checkInterval = time.Second
		nodeCount     = 100
		expiryLimit   = 10
	)

	// make sure the event buffer is much larger than node count
	// so that we can batch create nodes without waiting on each event
	require.True(t, int(nodeCount*3) < eventBufferSize)

	ctx := context.Background()

	clock := clockwork.NewFakeClockAt(time.Now().Add(time.Hour))
	p := newTestPack(t, func(c Config) Config {
		c.RelativeExpiryCheckInterval = checkInterval
		c.RelativeExpiryLimit = expiryLimit
		c.Clock = clock
		return ForProxy(c)
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
	for expired := nodeCount - expiryLimit; expired > 0; expired -= expiryLimit {
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

func TestRelativeExpiryOnlyForNodeWatches(t *testing.T) {
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
		c.Watches = []types.WatchKind{
			{Kind: types.KindNamespace},
			{Kind: types.KindNamespace},
			{Kind: types.KindCertAuthority},
		}
		return c
	})
	t.Cleanup(p2.Close)

	for i := 0; i < 2; i++ {
		clock.Advance(time.Hour * 24)
		drainEvents(p.eventsC)
		expectEvent(t, p.eventsC, RelativeExpiry)

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
	setupFuncs := map[string]SetupConfigFn{
		"ForProxy":          ForProxy,
		"ForRemoteProxy":    ForRemoteProxy,
		"ForOldRemoteProxy": ForOldRemoteProxy,
		"ForNode":           ForNode,
		"ForKubernetes":     ForKubernetes,
		"ForApps":           ForApps,
		"ForDatabases":      ForDatabases,
		"ForWindowsDesktop": ForWindowsDesktop,
		"ForDiscovery":      ForDiscovery,
		"ForOkta":           ForOkta,
	}

	authKindMap := make(map[resourceKind]types.WatchKind)
	for _, wk := range ForAuth(Config{}).Watches {
		authKindMap[resourceKind{kind: wk.Kind, subkind: wk.SubKind}] = wk
	}

	for name, f := range setupFuncs {
		t.Run(name, func(t *testing.T) {
			for _, wk := range f(Config{}).Watches {
				authWK, ok := authKindMap[resourceKind{kind: wk.Kind, subkind: wk.SubKind}]
				if !ok || !authWK.Contains(wk) {
					t.Errorf("%s includes WatchKind %s that is missing from ForAuth", name, wk.String())
				}
			}
		})
	}
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

	cases := map[string]Config{
		"ForAuth":           ForAuth(Config{}),
		"ForProxy":          ForProxy(Config{}),
		"ForRemoteProxy":    ForRemoteProxy(Config{}),
		"ForOldRemoteProxy": ForOldRemoteProxy(Config{}),
		"ForNode":           ForNode(Config{}),
		"ForKubernetes":     ForKubernetes(Config{}),
		"ForApps":           ForApps(Config{}),
		"ForDatabases":      ForDatabases(Config{}),
		"ForOkta":           ForOkta(Config{}),
	}

	events := map[string]types.Resource{
		types.KindCertAuthority:           &types.CertAuthorityV2{},
		types.KindClusterName:             &types.ClusterNameV2{},
		types.KindClusterAuditConfig:      types.DefaultClusterAuditConfig(),
		types.KindClusterNetworkingConfig: types.DefaultClusterNetworkingConfig(),
		types.KindClusterAuthPreference:   types.DefaultAuthPreference(),
		types.KindSessionRecordingConfig:  types.DefaultSessionRecordingConfig(),
		types.KindUIConfig:                &types.UIConfigV1{},
		types.KindStaticTokens:            &types.StaticTokensV2{},
		types.KindToken:                   &types.ProvisionTokenV2{},
		types.KindUser:                    &types.UserV2{},
		types.KindRole:                    &types.RoleV6{Version: types.V4},
		types.KindNamespace:               &types.Namespace{},
		types.KindNode:                    &types.ServerV2{},
		types.KindProxy:                   &types.ServerV2{},
		types.KindAuthServer:              &types.ServerV2{},
		types.KindReverseTunnel:           &types.ReverseTunnelV2{},
		types.KindTunnelConnection:        &types.TunnelConnectionV2{},
		types.KindAccessRequest:           &types.AccessRequestV3{},
		types.KindAppServer:               &types.AppServerV3{},
		types.KindApp:                     &types.AppV3{},
		types.KindWebSession:              &types.WebSessionV2{SubKind: types.KindWebSession},
		types.KindAppSession:              &types.WebSessionV2{SubKind: types.KindAppSession},
		types.KindSnowflakeSession:        &types.WebSessionV2{SubKind: types.KindSnowflakeSession},
		types.KindSAMLIdPSession:          &types.WebSessionV2{SubKind: types.KindSAMLIdPServiceProvider},
		types.KindWebToken:                &types.WebTokenV3{},
		types.KindRemoteCluster:           &types.RemoteClusterV3{},
		types.KindKubeServer:              &types.KubernetesServerV3{},
		types.KindDatabaseService:         &types.DatabaseServiceV1{},
		types.KindDatabaseServer:          &types.DatabaseServerV3{},
		types.KindDatabase:                &types.DatabaseV3{},
		types.KindNetworkRestrictions:     &types.NetworkRestrictionsV4{},
		types.KindLock:                    &types.LockV2{},
		types.KindWindowsDesktopService:   &types.WindowsDesktopServiceV3{},
		types.KindWindowsDesktop:          &types.WindowsDesktopV3{},
		types.KindInstaller:               &types.InstallerV1{},
		types.KindKubernetesCluster:       &types.KubernetesClusterV3{},
		types.KindSAMLIdPServiceProvider:  &types.SAMLIdPServiceProviderV1{},
		types.KindUserGroup:               &types.UserGroupV1{},
		types.KindOktaImportRule:          &types.OktaImportRuleV1{},
		types.KindOktaAssignment:          &types.OktaAssignmentV1{},
		types.KindIntegration:             &types.IntegrationV1{},
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

				require.Empty(t, cmp.Diff(resource, event.Resource))
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
	require.NoError(t, p.accessS.UpsertRole(ctx, role))

	user, err := types.NewUser("bob")
	require.NoError(t, err)
	require.NoError(t, p.usersS.UpsertUser(user))
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
		require.Equal(t, types.KindUser, event.Event.Resource.GetKind())
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// make sure that the user resource works as normal and gets replicated to cache
	replicatedUsers, err := p.cache.GetUsers(false)
	require.NoError(t, err)
	require.Len(t, replicatedUsers, 1)

	// now add a label to the user directly in the cache
	meta := user.GetMetadata()
	meta.Labels = map[string]string{"origin": "cache"}
	user.SetMetadata(meta)
	require.NoError(t, p.cache.usersCache.UpsertUser(user))

	// the label on the returned user proves that it came from the cache
	resultUser, err := p.cache.GetUser("bob", false)
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

	generateInvalidDB := func(t *testing.T, resourceID int64, name string) types.Database {
		db := &types.DatabaseV3{Metadata: types.Metadata{
			ID:   resourceID,
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
				db := generateInvalidDB(t, 0, "invalid-db")
				value, err := services.MarshalDatabase(db)
				require.NoError(t, err)
				_, err = b.Create(ctx, backend.Item{
					Key:     backend.Key("db", db.GetName()),
					Value:   value,
					Expires: db.Expiry(),
					ID:      db.GetResourceID(),
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
					Key:     backend.Key("db", validDB.GetName()),
					Value:   marshalledDB,
					Expires: validDB.Expiry(),
					ID:      validDB.GetResourceID(),
				})
				require.NoError(t, err)

				// Wait until the database appear on cache.
				require.Eventually(t, func() bool {
					if dbs, err := c.GetDatabases(ctx); err == nil {
						return len(dbs) == 1
					}

					return false
				}, time.Second, 100*time.Millisecond, "expected database to be on cache, but nothing found")

				cacheDB, err := c.GetDatabase(ctx, dbName)
				require.NoError(t, err)

				invalidDB := generateInvalidDB(t, cacheDB.GetResourceID(), cacheDB.GetName())
				value, err := services.MarshalDatabase(invalidDB)
				require.NoError(t, err)
				_, err = b.Update(ctx, backend.Item{
					Key:     backend.Key("db", cacheDB.GetName()),
					Value:   value,
					Expires: invalidDB.Expiry(),
					ID:      cacheDB.GetResourceID(),
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
