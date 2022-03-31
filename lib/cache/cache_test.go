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

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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

	usersS          services.UsersService
	accessS         services.Access
	dynamicAccessS  services.DynamicAccessCore
	presenceS       services.Presence
	appSessionS     services.AppSession
	restrictions    services.Restrictions
	apps            services.Apps
	databases       services.Databases
	webSessionS     types.WebSessionInterface
	webTokenS       types.WebTokenInterface
	windowsDesktops services.WindowsDesktops
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

func newPackForNode(t *testing.T) *testPack {
	return newTestPack(t, ForNode)
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
}

type packOption func(cfg *packCfg)

func memoryBackend(bool) packOption {
	return func(cfg *packCfg) {
		cfg.memoryBackend = true
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
	p.eventsS = &proxyEvents{events: local.NewEventsService(p.backend)}
	p.presenceS = local.NewPresenceService(p.backend)
	p.usersS = local.NewIdentityService(p.backend)
	p.accessS = local.NewAccessService(p.backend)
	p.dynamicAccessS = local.NewDynamicAccessService(p.backend)
	p.appSessionS = local.NewIdentityService(p.backend)
	p.webSessionS = local.NewIdentityService(p.backend).WebSessions()
	p.webTokenS = local.NewIdentityService(p.backend).WebTokens()
	p.restrictions = local.NewRestrictionsService(p.backend)
	p.apps = local.NewAppService(p.backend)
	p.databases = local.NewDatabasesService(p.backend)
	p.windowsDesktops = local.NewWindowsDesktopService(p.backend)

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
		Context:         ctx,
		Backend:         p.cacheBackend,
		Events:          p.eventsS,
		ClusterConfig:   p.clusterConfigS,
		Provisioner:     p.provisionerS,
		Trust:           p.trustS,
		Users:           p.usersS,
		Access:          p.accessS,
		DynamicAccess:   p.dynamicAccessS,
		Presence:        p.presenceS,
		AppSession:      p.appSessionS,
		WebSession:      p.webSessionS,
		WebToken:        p.webTokenS,
		Restrictions:    p.restrictions,
		Apps:            p.apps,
		Databases:       p.databases,
		WindowsDesktops: p.windowsDesktops,
		MaxRetryPeriod:  200 * time.Millisecond,
		EventsC:         p.eventsC,
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	select {
	case event := <-p.eventsC:
		if event.Type != WatcherStarted {
			return nil, trace.CompareFailed("%q != %q", event.Type, WatcherStarted)
		}
	case <-time.After(time.Second):
		return nil, trace.ConnectionProblem(nil, "wait for the watcher to start")
	}
	return p, nil
}

// TestCA tests certificate authorities
func TestCA(t *testing.T) {
	p := newPackForAuth(t)
	t.Cleanup(p.Close)
	ctx := context.Background()

	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ca))

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetCertAuthority(ctx, ca.GetID(), true)
	require.NoError(t, err)
	ca.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(ca, out))

	err = p.trustS.DeleteCertAuthority(ca.GetID())
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
	require.NoError(t, p.trustS.UpsertCertAuthority(ca))

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
	require.NoError(t, p.trustS.UpsertCertAuthority(filteredCa))
	require.NoError(t, p.trustS.DeleteCertAuthority(filteredCa.GetID()))

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
		Events:          p.cache,
		Trust:           p.cache.trustCache,
		ClusterConfig:   p.cache.clusterConfigCache,
		Provisioner:     p.cache.provisionerCache,
		Users:           p.cache.usersCache,
		Access:          p.cache.accessCache,
		DynamicAccess:   p.cache.dynamicAccessCache,
		Presence:        p.cache.presenceCache,
		Restrictions:    p.cache.restrictionsCache,
		Apps:            p.cache.appsCache,
		Databases:       p.cache.databasesCache,
		AppSession:      p.cache.appSessionCache,
		WebSession:      p.cache.webSessionCache,
		WebToken:        p.cache.webTokenCache,
		WindowsDesktops: p.cache.windowsDesktopsCache,
		Backend:         nodeCacheBackend,
	}))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, nodeCache.Close()) })

	cacheWatcher, err := nodeCache.NewWatcher(ctx, types.Watch{Kinds: []types.WatchKind{{Kind: types.KindCertAuthority}}})
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
	require.NoError(t, p.trustS.UpsertCertAuthority(localCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(localCA.GetID()))

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
	require.NoError(t, p.trustS.UpsertCertAuthority(nonlocalCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(nonlocalCA.GetID()))

	ev = fetchEvent()
	require.Equal(t, types.OpDelete, ev.Type)
	require.Equal(t, types.KindCertAuthority, ev.Resource.GetKind())
	require.Equal(t, "example.net", ev.Resource.GetName())

	// whereas we expect to see the Put and Delete for a trusted *user* CA
	trustedUserCA := suite.NewTestCA(types.UserCA, "example.net")
	require.NoError(t, p.trustS.UpsertCertAuthority(trustedUserCA))
	require.NoError(t, p.trustS.DeleteCertAuthority(trustedUserCA.GetID()))

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

func expectNextEvent(t *testing.T, eventsC <-chan Event, expectedEvent string, skipEvents ...string) {
	timeC := time.After(5 * time.Second)
	for {
		// wait for watcher to restart
		select {
		case event := <-eventsC:
			if apiutils.SliceContainsStr(skipEvents, event.Type) {
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
	ctx := context.Background()
	const caCount = 100
	const inits = 20
	p := newTestPackWithoutCache(t)
	t.Cleanup(p.Close)

	// put lots of CAs in the backend
	for i := 0; i < caCount; i++ {
		ca := suite.NewTestCA(types.UserCA, fmt.Sprintf("%d.example.com", i))
		require.NoError(t, p.trustS.UpsertCertAuthority(ca))
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
			Context:         ctx,
			Backend:         p.cacheBackend,
			Events:          p.eventsS,
			ClusterConfig:   p.clusterConfigS,
			Provisioner:     p.provisionerS,
			Trust:           p.trustS,
			Users:           p.usersS,
			Access:          p.accessS,
			DynamicAccess:   p.dynamicAccessS,
			Presence:        p.presenceS,
			AppSession:      p.appSessionS,
			WebSession:      p.webSessionS,
			WebToken:        p.webTokenS,
			Restrictions:    p.restrictions,
			Apps:            p.apps,
			Databases:       p.databases,
			WindowsDesktops: p.windowsDesktops,
			MaxRetryPeriod:  200 * time.Millisecond,
			EventsC:         p.eventsC,
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
	ctx := context.Background()
	const caCount = 100
	const resets = 20
	p := newTestPackWithoutCache(t)
	t.Cleanup(p.Close)

	// put lots of CAs in the backend
	for i := 0; i < caCount; i++ {
		ca := suite.NewTestCA(types.UserCA, fmt.Sprintf("%d.example.com", i))
		require.NoError(t, p.trustS.UpsertCertAuthority(ca))
	}

	var err error
	p.cache, err = New(ForAuth(Config{
		Context:         ctx,
		Backend:         p.cacheBackend,
		Events:          p.eventsS,
		ClusterConfig:   p.clusterConfigS,
		Provisioner:     p.provisionerS,
		Trust:           p.trustS,
		Users:           p.usersS,
		Access:          p.accessS,
		DynamicAccess:   p.dynamicAccessS,
		Presence:        p.presenceS,
		AppSession:      p.appSessionS,
		WebSession:      p.webSessionS,
		WebToken:        p.webTokenS,
		Restrictions:    p.restrictions,
		Apps:            p.apps,
		Databases:       p.databases,
		WindowsDesktops: p.windowsDesktops,
		MaxRetryPeriod:  200 * time.Millisecond,
		EventsC:         p.eventsC,
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
		func() {
			server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
			_, err := p.presenceS.UpsertNode(ctx, server)
			require.NoError(b, err)
			timeout := time.NewTimer(time.Millisecond * 200)
			defer timeout.Stop()
			select {
			case event := <-p.eventsC:
				require.Equal(b, EventProcessed, event.Type)
			case <-timeout.C:
				b.Fatalf("timeout waiting for event, iteration=%d", i)
			}
		}()
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
cpu: Intel(R) Core(TM) i9-10885H CPU @ 2.40GHz
BenchmarkListMaxNodes-16    	       1	1136071399 ns/op
*/
func BenchmarkListMaxNodes(b *testing.B) {
	benchListNodes(b, backend.DefaultRangeLimit, apidefaults.DefaultChunkSize)
}

func benchListNodes(b *testing.B, nodeCount int, pageSize int) {
	p, err := newPack(b.TempDir(), ForAuth, memoryBackend(true))
	require.NoError(b, err)
	defer p.Close()

	ctx := context.Background()

	for i := 0; i < nodeCount; i++ {
		func() {
			server := suite.NewServer(types.KindNode, uuid.New().String(), "127.0.0.1:2022", apidefaults.Namespace)
			_, err := p.presenceS.UpsertNode(ctx, server)
			require.NoError(b, err)
			timeout := time.NewTimer(time.Millisecond * 200)
			defer timeout.Stop()
			select {
			case event := <-p.eventsC:
				require.Equal(b, EventProcessed, event.Type)
			case <-timeout.C:
				b.Fatalf("timeout waiting for event, iteration=%d", i)
			}
		}()
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var nodes []types.Server
		req := proto.ListNodesRequest{
			Namespace: apidefaults.Namespace,
			Limit:     int32(pageSize),
		}
		for {
			page, nextKey, err := p.cache.ListNodes(ctx, req)
			require.NoError(b, err)
			nodes = append(nodes, page...)
			require.True(b, len(page) == pageSize || nextKey == "")
			if nextKey == "" {
				break
			}
			req.StartKey = nextKey
		}
		require.Len(b, nodes, nodeCount)
	}
}

// TestListResources_NodesTTLVariant verifies that the custom ListNodes impl that we fallback to when
// using ttl-based caching works as expected.
func TestListResources_NodesTTLVariant(t *testing.T) {
	const nodeCount = 100
	const pageSize = 10
	var err error

	ctx := context.Background()

	p, err := newPackWithoutCache(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(p.Close)

	p.cache, err = New(ForAuth(Config{
		Context:         ctx,
		Backend:         p.cacheBackend,
		Events:          p.eventsS,
		ClusterConfig:   p.clusterConfigS,
		Provisioner:     p.provisionerS,
		Trust:           p.trustS,
		Users:           p.usersS,
		Access:          p.accessS,
		DynamicAccess:   p.dynamicAccessS,
		Presence:        p.presenceS,
		AppSession:      p.appSessionS,
		WebSession:      p.webSessionS,
		WebToken:        p.webTokenS,
		Restrictions:    p.restrictions,
		Apps:            p.apps,
		Databases:       p.databases,
		WindowsDesktops: p.windowsDesktops,
		MaxRetryPeriod:  200 * time.Millisecond,
		EventsC:         p.eventsC,
		neverOK:         true, // ensure reads are never healthy
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

	// DELETE IN 10.0.0 this block with ListNodes is replaced
	// by the following block with ListResources test.
	var nodes []types.Server
	var startKey string
	for {
		page, nextKey, err := p.cache.ListNodes(ctx, proto.ListNodesRequest{
			Namespace: apidefaults.Namespace,
			Limit:     int32(pageSize),
			StartKey:  startKey,
		})
		require.NoError(t, err)

		if nextKey != "" {
			require.Len(t, page, pageSize)
		}

		nodes = append(nodes, page...)

		startKey = nextKey

		if startKey == "" {
			break
		}
	}
	require.Len(t, nodes, nodeCount)

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
		Context:         ctx,
		Backend:         p.cacheBackend,
		Events:          p.eventsS,
		ClusterConfig:   p.clusterConfigS,
		Provisioner:     p.provisionerS,
		Trust:           p.trustS,
		Users:           p.usersS,
		Access:          p.accessS,
		DynamicAccess:   p.dynamicAccessS,
		Presence:        p.presenceS,
		AppSession:      p.appSessionS,
		WebSession:      p.webSessionS,
		WebToken:        p.webTokenS,
		Restrictions:    p.restrictions,
		Apps:            p.apps,
		Databases:       p.databases,
		WindowsDesktops: p.windowsDesktops,
		MaxRetryPeriod:  200 * time.Millisecond,
		EventsC:         p.eventsC,
	}))
	require.NoError(t, err)

	_, err = p.cache.GetCertAuthorities(ctx, types.UserCA, false)
	require.True(t, trace.IsConnectionProblem(err))

	ca := suite.NewTestCA(types.UserCA, "example.com")
	// NOTE 1: this could produce event processed
	// below, based on whether watcher restarts to get the event
	// or not, which is normal, but has to be accounted below
	require.NoError(t, p.trustS.UpsertCertAuthority(ca))
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
	require.NoError(t, p.trustS.UpsertCertAuthority(ca))

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
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	ca := suite.NewTestCA(types.UserCA, "example.com")
	require.NoError(t, p.trustS.UpsertCertAuthority(ca))

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
	require.NoError(t, p.trustS.UpsertCertAuthority(ca2))

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
	ctx := context.Background()
	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	user, err := types.NewUser("bob")
	require.NoError(t, err)
	err = p.usersS.UpsertUser(user)
	require.NoError(t, err)

	user, err = p.usersS.GetUser(user.GetName(), false)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetUser(user.GetName(), false)
	require.NoError(t, err)
	user.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(user, out))

	// update user's roles
	user.SetRoles([]string{"access"})
	require.NoError(t, err)
	err = p.usersS.UpsertUser(user)
	require.NoError(t, err)

	user, err = p.usersS.GetUser(user.GetName(), false)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetUser(user.GetName(), false)
	require.NoError(t, err)
	user.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(user, out))

	err = p.usersS.DeleteUser(ctx, user.GetName())
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetUser(user.GetName(), false)
	require.True(t, trace.IsNotFound(err))
}

// TestRoles tests caching of roles
func TestRoles(t *testing.T) {
	ctx := context.Background()
	p := newPackForNode(t)
	t.Cleanup(p.Close)

	role, err := types.NewRoleV3("role1", types.RoleSpecV5{
		Options: types.RoleOptions{
			MaxSessionTTL: types.Duration(time.Hour),
		},
		Allow: types.RoleConditions{
			Logins:     []string{"root", "bob"},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
		Deny: types.RoleConditions{},
	})
	require.NoError(t, err)
	err = p.accessS.UpsertRole(ctx, role)
	require.NoError(t, err)

	role, err = p.accessS.GetRole(ctx, role.GetName())
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetRole(ctx, role.GetName())
	require.NoError(t, err)
	role.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(role, out))

	// update role
	role.SetLogins(types.Allow, []string{"admin"})
	require.NoError(t, err)
	err = p.accessS.UpsertRole(ctx, role)
	require.NoError(t, err)

	role, err = p.accessS.GetRole(ctx, role.GetName())
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRole(ctx, role.GetName())
	require.NoError(t, err)
	role.SetResourceID(out.GetResourceID())
	require.Empty(t, cmp.Diff(role, out))

	err = p.accessS.DeleteRole(ctx, role.GetName())
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetRole(ctx, role.GetName())
	require.True(t, trace.IsNotFound(err))
}

// TestReverseTunnels tests reverse tunnels caching
func TestReverseTunnels(t *testing.T) {
	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	tunnel, err := types.NewReverseTunnel("example.com", []string{"example.com:2023"})
	require.NoError(t, err)
	require.NoError(t, p.presenceS.UpsertReverseTunnel(tunnel))

	tunnel, err = p.presenceS.GetReverseTunnel(tunnel.GetName())
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetReverseTunnels()
	require.NoError(t, err)
	require.Len(t, out, 1)

	tunnel.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(tunnel, out[0]))

	// update tunnel's parameters
	tunnel.SetClusterName("new.example.com")
	require.NoError(t, err)
	err = p.presenceS.UpsertReverseTunnel(tunnel)
	require.NoError(t, err)

	out, err = p.presenceS.GetReverseTunnels()
	require.NoError(t, err)
	require.Len(t, out, 1)
	tunnel = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetReverseTunnels()
	require.NoError(t, err)
	require.Len(t, out, 1)

	tunnel.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(tunnel, out[0]))

	err = p.presenceS.DeleteAllReverseTunnels()
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetReverseTunnels()
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestTunnelConnections tests tunnel connections caching
func TestTunnelConnections(t *testing.T) {
	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	clusterName := "example.com"
	hb := time.Now().UTC()
	conn, err := types.NewTunnelConnection("conn1", types.TunnelConnectionSpecV2{
		ClusterName:   clusterName,
		ProxyName:     "p1",
		LastHeartbeat: hb,
	})
	require.NoError(t, err)
	require.NoError(t, p.presenceS.UpsertTunnelConnection(conn))

	out, err := p.presenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Len(t, out, 1)
	conn = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Len(t, out, 1)

	conn.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(conn, out[0]))

	// update conn's parameters
	hb = hb.Add(time.Second)
	conn.SetLastHeartbeat(hb)

	err = p.presenceS.UpsertTunnelConnection(conn)
	require.NoError(t, err)

	out, err = p.presenceS.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Len(t, out, 1)
	conn = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Len(t, out, 1)

	conn.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(conn, out[0]))

	err = p.presenceS.DeleteTunnelConnections(clusterName)
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetTunnelConnections(clusterName)
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestNodes tests nodes cache
func TestNodes(t *testing.T) {
	ctx := context.Background()

	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	server := suite.NewServer(types.KindNode, "srv1", "127.0.0.1:2022", apidefaults.Namespace)
	_, err := p.presenceS.UpsertNode(ctx, server)
	require.NoError(t, err)

	out, err := p.presenceS.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv := out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)

	srv.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))

	// update srv parameters
	srv.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	srv.SetAddr("127.0.0.2:2033")

	lease, err := p.presenceS.UpsertNode(ctx, srv)
	require.NoError(t, err)

	out, err = p.presenceS.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)

	srv.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))

	// update keep alive on the node and make sure
	// it propagates
	lease.Expires = time.Now().UTC().Add(time.Hour)
	err = p.presenceS.KeepAliveNode(ctx, *lease)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)

	srv.SetResourceID(out[0].GetResourceID())
	srv.SetExpiry(lease.Expires)
	require.Empty(t, cmp.Diff(srv, out[0]))

	err = p.presenceS.DeleteAllNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestProxies tests proxies cache
func TestProxies(t *testing.T) {
	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	server := suite.NewServer(types.KindProxy, "srv1", "127.0.0.1:2022", apidefaults.Namespace)
	err := p.presenceS.UpsertProxy(server)
	require.NoError(t, err)

	out, err := p.presenceS.GetProxies()
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv := out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetProxies()
	require.NoError(t, err)
	require.Len(t, out, 1)

	srv.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))

	// update srv parameters
	srv.SetAddr("127.0.0.2:2033")

	err = p.presenceS.UpsertProxy(srv)
	require.NoError(t, err)

	out, err = p.presenceS.GetProxies()
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetProxies()
	require.NoError(t, err)
	require.Len(t, out, 1)

	srv.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))

	err = p.presenceS.DeleteAllProxies()
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetProxies()
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestAuthServers tests auth servers cache
func TestAuthServers(t *testing.T) {
	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	server := suite.NewServer(types.KindAuthServer, "srv1", "127.0.0.1:2022", apidefaults.Namespace)
	err := p.presenceS.UpsertAuthServer(server)
	require.NoError(t, err)

	out, err := p.presenceS.GetAuthServers()
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv := out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetAuthServers()
	require.NoError(t, err)
	require.Len(t, out, 1)

	srv.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))

	// update srv parameters
	srv.SetAddr("127.0.0.2:2033")

	err = p.presenceS.UpsertAuthServer(srv)
	require.NoError(t, err)

	out, err = p.presenceS.GetAuthServers()
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetAuthServers()
	require.NoError(t, err)
	require.Len(t, out, 1)

	srv.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))

	err = p.presenceS.DeleteAllAuthServers()
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetAuthServers()
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestRemoteClusters tests remote clusters caching
func TestRemoteClusters(t *testing.T) {
	ctx := context.Background()
	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	clusterName := "example.com"
	rc, err := types.NewRemoteCluster(clusterName)
	require.NoError(t, err)
	require.NoError(t, p.presenceS.CreateRemoteCluster(rc))

	out, err := p.presenceS.GetRemoteClusters()
	require.NoError(t, err)
	require.Len(t, out, 1)
	rc = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRemoteClusters()
	require.NoError(t, err)
	require.Len(t, out, 1)

	rc.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(rc, out[0]))

	// update conn's parameters
	meta := rc.GetMetadata()
	meta.Labels = map[string]string{"env": "prod"}
	rc.SetMetadata(meta)

	err = p.presenceS.UpdateRemoteCluster(ctx, rc)
	require.NoError(t, err)

	out, err = p.presenceS.GetRemoteClusters()
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Empty(t, cmp.Diff(meta.Labels, out[0].GetMetadata().Labels))
	rc = out[0]

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRemoteClusters()
	require.NoError(t, err)
	require.Len(t, out, 1)

	rc.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(rc, out[0]))

	err = p.presenceS.DeleteAllRemoteClusters()
	require.NoError(t, err)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRemoteClusters()
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestAppServers tests that CRUD operations are replicated from the backend to
// the cache.
func TestAppServers(t *testing.T) {
	p := newPackForProxy(t)
	t.Cleanup(p.Close)

	// Upsert application into backend.
	server := suite.NewAppServer("foo", "http://127.0.0.1:8080", "foo.example.com")
	_, err := p.presenceS.UpsertAppServer(context.Background(), server)
	require.NoError(t, err)

	// Check that the application is now in the backend.
	out, err := p.presenceS.GetAppServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv := out[0]

	// Wait until the information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	// Make sure the cache has a single application in it.
	out, err = p.cache.GetAppServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)

	// Check that the value in the cache, value in the backend, and original
	// services.App all exactly match.
	srv.SetResourceID(out[0].GetResourceID())
	server.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))
	require.Empty(t, cmp.Diff(server, out[0]))

	// Update the application and upsert it into the backend again.
	srv.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	_, err = p.presenceS.UpsertAppServer(context.Background(), srv)
	require.NoError(t, err)

	// Check that the application is in the backend and only one exists (so an
	// update occurred).
	out, err = p.presenceS.GetAppServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)
	srv = out[0]

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	// Make sure the cache has a single application in it.
	out, err = p.cache.GetAppServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, out, 1)

	// Check that the value in the cache, value in the backend, and original
	// services.App all exactly match.
	srv.SetResourceID(out[0].GetResourceID())
	require.Empty(t, cmp.Diff(srv, out[0]))

	// Remove all applications from the backend.
	err = p.presenceS.DeleteAllAppServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = p.cache.GetAppServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, out)
}

// TestApplicationServers tests that CRUD operations on app servers are
// replicated from the backend to the cache.
func TestApplicationServers(t *testing.T) {
	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	ctx := context.Background()

	// Upsert app server into backend.
	app, err := types.NewAppV3(types.Metadata{Name: "app"}, types.AppSpecV3{URI: "localhost"})
	require.NoError(t, err)
	server, err := types.NewAppServerV3FromApp(app, "host", uuid.New().String())
	require.NoError(t, err)

	_, err = p.presenceS.UpsertApplicationServer(ctx, server)
	require.NoError(t, err)

	// Check that the app server is now in the backend.
	out, err := p.presenceS.GetApplicationServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Wait until the information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single app server in it.
	out, err = p.cache.GetApplicationServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Update the server and upsert it into the backend again.
	server.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	_, err = p.presenceS.UpsertApplicationServer(context.Background(), server)
	require.NoError(t, err)

	// Check that the server is in the backend and only one exists (so an
	// update occurred).
	out, err = p.presenceS.GetApplicationServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single server in it.
	out, err = p.cache.GetApplicationServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.AppServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Remove all servers from the backend.
	err = p.presenceS.DeleteAllApplicationServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = p.cache.GetApplicationServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

// TestApps tests that CRUD operations on application resources are
// replicated from the backend to the cache.
func TestApps(t *testing.T) {
	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	ctx := context.Background()

	// Create an app.
	app, err := types.NewAppV3(types.Metadata{
		Name: "foo",
	}, types.AppSpecV3{
		URI: "localhost",
	})
	require.NoError(t, err)

	err = p.apps.CreateApp(ctx, app)
	require.NoError(t, err)

	// Check that the app is now in the backend.
	out, err := p.apps.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Wait until the information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single app in it.
	out, err = p.apps.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Update the app and upsert it into the backend again.
	app.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	err = p.apps.UpdateApp(ctx, app)
	require.NoError(t, err)

	// Check that the app is in the backend and only one exists (so an
	// update occurred).
	out, err = p.apps.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single app in it.
	out, err = p.cache.GetApps(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Application{app}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Remove all apps from the backend.
	err = p.apps.DeleteAllApps(ctx)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = p.apps.GetApps(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

// TestDatabaseServers tests that CRUD operations on database servers are
// replicated from the backend to the cache.
func TestDatabaseServers(t *testing.T) {
	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	ctx := context.Background()

	// Upsert database server into backend.
	server, err := types.NewDatabaseServerV3(types.Metadata{
		Name: "foo",
	}, types.DatabaseServerSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		Hostname: "localhost",
		HostID:   uuid.New().String(),
	})
	require.NoError(t, err)

	_, err = p.presenceS.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)

	// Check that the database server is now in the backend.
	out, err := p.presenceS.GetDatabaseServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Wait until the information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single database server in it.
	out, err = p.cache.GetDatabaseServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Update the server and upsert it into the backend again.
	server.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	_, err = p.presenceS.UpsertDatabaseServer(context.Background(), server)
	require.NoError(t, err)

	// Check that the server is in the backend and only one exists (so an
	// update occurred).
	out, err = p.presenceS.GetDatabaseServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single database server in it.
	out, err = p.cache.GetDatabaseServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Remove all database servers from the backend.
	err = p.presenceS.DeleteAllDatabaseServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = p.cache.GetDatabaseServers(context.Background(), apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

// TestDatabases tests that CRUD operations on database resources are
// replicated from the backend to the cache.
func TestDatabases(t *testing.T) {
	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	ctx := context.Background()

	// Create a database resource.
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "foo",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	err = p.databases.CreateDatabase(ctx, database)
	require.NoError(t, err)

	// Check that the database is now in the backend.
	out, err := p.databases.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{database}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Wait until the information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single database in it.
	out, err = p.databases.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{database}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Update the database and upsert it into the backend again.
	database.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	err = p.databases.UpdateDatabase(ctx, database)
	require.NoError(t, err)

	// Check that the database is in the backend and only one exists (so an
	// update occurred).
	out, err = p.databases.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{database}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Make sure the cache has a single database in it.
	out, err = p.cache.GetDatabases(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.Database{database}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Remove all database from the backend.
	err = p.databases.DeleteAllDatabases(ctx)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = p.databases.GetDatabases(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

func TestRelativeExpiry(t *testing.T) {
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

func TestCache_Backoff(t *testing.T) {
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

	step := p.cache.Config.MaxRetryPeriod / 5.0
	for i := 0; i < 5; i++ {
		// wait for cache to reload
		select {
		case event := <-p.eventsC:
			require.Equal(t, Reloading, event.Type)
			duration, err := time.ParseDuration(event.Event.Resource.GetKind())
			require.NoError(t, err)

			stepMin := step * time.Duration(i) / 2
			stepMax := step * time.Duration(i+1)

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

type proxyEvents struct {
	sync.Mutex
	watchers []types.Watcher
	events   types.Events
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
	w, err := p.events.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.Lock()
	defer p.Unlock()
	p.watchers = append(p.watchers, w)
	return w, nil
}
