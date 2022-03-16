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

	"github.com/gravitational/teleport"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"

	"github.com/gravitational/trace"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

type CacheSuite struct{}

var _ = check.Suite(&CacheSuite{})

// bootstrap check
func TestState(t *testing.T) { check.TestingT(t) }

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

	usersS         services.UsersService
	accessS        services.Access
	dynamicAccessS services.DynamicAccessCore
	presenceS      services.Presence
	appSessionS    services.AppSession
	webSessionS    types.WebSessionInterface
	webTokenS      types.WebTokenInterface
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

func (s *CacheSuite) newPackForAuth(c *check.C) *testPack {
	return s.newPack(c, ForAuth)
}

func (s *CacheSuite) newPackForProxy(c *check.C) *testPack {
	return s.newPack(c, ForProxy)
}

func (s *CacheSuite) newPackForNode(c *check.C) *testPack {
	return s.newPack(c, ForNode)
}

func (s *CacheSuite) newPack(c *check.C, setupConfig SetupConfigFn) *testPack {
	pack, err := newPack(c.MkDir(), setupConfig)
	c.Assert(err, check.IsNil)
	return pack
}

func (s *CacheSuite) newPackWithoutCache(c *check.C, setupConfig SetupConfigFn) *testPack {
	pack, err := newPackWithoutCache(c.MkDir(), setupConfig)
	c.Assert(err, check.IsNil)
	return pack
}

func newTestPack(t *testing.T, setupConfig SetupConfigFn) *testPack {
	pack, err := newPack(t.TempDir(), setupConfig)
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
func newPackWithoutCache(dir string, setupConfig SetupConfigFn, opts ...packOption) (*testPack, error) {
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
			Context: context.TODO(),
			Mirror:  true,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.eventsC = make(chan Event, 100)

	p.trustS = local.NewCAService(p.backend)
	p.clusterConfigS = local.NewClusterConfigurationService(p.backend)
	p.provisionerS = local.NewProvisioningService(p.backend)
	p.eventsS = &proxyEvents{events: local.NewEventsService(p.backend)}
	p.presenceS = local.NewPresenceService(p.backend)
	p.usersS = local.NewIdentityService(p.backend)
	p.accessS = local.NewAccessService(p.backend)
	p.dynamicAccessS = local.NewDynamicAccessService(p.backend)
	p.appSessionS = local.NewIdentityService(p.backend)
	p.webSessionS = local.NewIdentityService(p.backend).WebSessions()
	p.webTokenS = local.NewIdentityService(p.backend).WebTokens()

	return p, nil
}

// newPack returns a new test pack or fails the test on error
func newPack(dir string, setupConfig func(c Config) Config, opts ...packOption) (*testPack, error) {
	ctx := context.Background()
	p, err := newPackWithoutCache(dir, setupConfig, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.cache, err = New(setupConfig(Config{
		Context:        ctx,
		Backend:        p.cacheBackend,
		Events:         p.eventsS,
		ClusterConfig:  p.clusterConfigS,
		Provisioner:    p.provisionerS,
		Trust:          p.trustS,
		Users:          p.usersS,
		Access:         p.accessS,
		DynamicAccess:  p.dynamicAccessS,
		Presence:       p.presenceS,
		AppSession:     p.appSessionS,
		WebSession:     p.webSessionS,
		WebToken:       p.webTokenS,
		MaxRetryPeriod: 200 * time.Millisecond,
		EventsC:        p.eventsC,
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		return nil, trace.ConnectionProblem(nil, "wait for the watcher to start")
	}
	return p, nil
}

// TestCA tests certificate authorities
func (s *CacheSuite) TestCA(c *check.C) {
	p := s.newPackForAuth(c)
	defer p.Close()

	ca := suite.NewTestCA(services.UserCA, "example.com")
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetCertAuthority(ca.GetID(), true)
	c.Assert(err, check.IsNil)
	ca.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, ca, out)

	err = p.trustS.DeleteCertAuthority(ca.GetID())
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetCertAuthority(ca.GetID(), false)
	fixtures.ExpectNotFound(c, err)
}

// TestWatchers tests watchers connected to the cache,
// verifies that all watchers of the cache will be closed
// if the underlying watcher to the target backend is closed
func (s *CacheSuite) TestWatchers(c *check.C) {
	p := s.newPackForAuth(c)
	defer p.Close()

	w, err := p.cache.NewWatcher(context.TODO(), services.Watch{Kinds: []services.WatchKind{
		{
			Kind: services.KindCertAuthority,
		},
		{
			Kind: services.KindAccessRequest,
			Filter: map[string]string{
				"user": "alice",
			},
		},
	}})
	c.Assert(err, check.IsNil)
	defer w.Close()

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, backend.OpInit)
	case <-time.After(100 * time.Millisecond):
		c.Fatalf("Timeout waiting for event.")
	}

	ca := suite.NewTestCA(services.UserCA, "example.com")
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, backend.OpPut)
		c.Assert(e.Resource.GetKind(), check.Equals, services.KindCertAuthority)
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// create an access request that matches the supplied filter
	req, err := services.NewAccessRequest("alice", "dictator")
	c.Assert(err, check.IsNil)

	c.Assert(p.dynamicAccessS.CreateAccessRequest(context.TODO(), req), check.IsNil)

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, backend.OpPut)
		c.Assert(e.Resource.GetKind(), check.Equals, services.KindAccessRequest)
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	c.Assert(p.dynamicAccessS.DeleteAccessRequest(context.TODO(), req.GetName()), check.IsNil)

	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, backend.OpDelete)
		c.Assert(e.Resource.GetKind(), check.Equals, services.KindAccessRequest)
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// create an access request that does not match the supplied filter
	req2, err := services.NewAccessRequest("bob", "dictator")
	c.Assert(err, check.IsNil)

	// create and then delete the non-matching request.
	c.Assert(p.dynamicAccessS.CreateAccessRequest(context.TODO(), req2), check.IsNil)
	c.Assert(p.dynamicAccessS.DeleteAccessRequest(context.TODO(), req2.GetName()), check.IsNil)

	// because our filter did not match the request, the create event should never
	// have been created, meaning that the next event on the pipe is the delete
	// event (which cannot be filtered out because username is not visible inside
	// a delete event).
	select {
	case e := <-w.Events():
		c.Assert(e.Type, check.Equals, backend.OpDelete)
		c.Assert(e.Resource.GetKind(), check.Equals, services.KindAccessRequest)
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for event.")
	}

	// event has arrived, now close the watchers
	p.backend.CloseWatchers()

	// make sure watcher has been closed
	select {
	case <-w.Done():
	case <-time.After(time.Second):
		c.Fatalf("Timeout waiting for close event.")
	}
}

func waitForRestart(c *check.C, eventsC <-chan Event) {
	waitForEvent(c, eventsC, WatcherStarted, Reloading, WatcherFailed)
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

func waitForEvent(c *check.C, eventsC <-chan Event, expectedEvent string, skipEvents ...string) {
	timeC := time.After(5 * time.Second)
	for {
		// wait for watcher to restart
		select {
		case event := <-eventsC:
			if utils.SliceContainsStr(skipEvents, event.Type) {
				continue
			}
			c.Assert(event.Type, check.Equals, expectedEvent)
			return
		case <-timeC:
			c.Fatalf("Timeout waiting for expected event: %s", expectedEvent)
		}
	}
}

// TestCompletenessInit verifies that flaky backends don't cause
// the cache to return partial results during init.
func (s *CacheSuite) TestCompletenessInit(c *check.C) {
	const caCount = 100
	const inits = 20
	p := s.newPackWithoutCache(c, ForAuth)
	defer p.Close()

	// put lots of CAs in the backend
	for i := 0; i < caCount; i++ {
		ca := suite.NewTestCA(services.UserCA, fmt.Sprintf("%d.example.com", i))
		c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)
	}

	for i := 0; i < inits; i++ {
		var err error

		p.cacheBackend, err = memory.New(
			memory.Config{
				Context: context.TODO(),
				Mirror:  true,
			})
		c.Assert(err, check.IsNil)

		// simulate bad connection to auth server
		p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
		p.eventsS.closeWatchers()

		p.cache, err = New(ForAuth(Config{
			Context:        context.TODO(),
			Backend:        p.cacheBackend,
			Events:         p.eventsS,
			ClusterConfig:  p.clusterConfigS,
			Provisioner:    p.provisionerS,
			Trust:          p.trustS,
			Users:          p.usersS,
			Access:         p.accessS,
			DynamicAccess:  p.dynamicAccessS,
			Presence:       p.presenceS,
			AppSession:     p.appSessionS,
			WebSession:     p.webSessionS,
			WebToken:       p.webTokenS,
			MaxRetryPeriod: 200 * time.Millisecond,
			EventsC:        p.eventsC,
		}))
		c.Assert(err, check.IsNil)

		p.backend.SetReadError(nil)

		cas, err := p.cache.GetCertAuthorities(services.UserCA, false)
		// we don't actually care whether the cache ever fully constructed
		// the CA list.  for the purposes of this test, we just care that it
		// doesn't return the CA list *unless* it was successfully constructed.
		if err == nil {
			c.Assert(len(cas), check.Equals, caCount)
		} else {
			fixtures.ExpectConnectionProblem(c, err)
		}

		c.Assert(p.cache.Close(), check.IsNil)
		p.cache = nil
		c.Assert(p.cacheBackend.Close(), check.IsNil)
		p.cacheBackend = nil
	}
}

// TestCompletenessReset verifies that flaky backends don't cause
// the cache to return partial results during reset.
func (s *CacheSuite) TestCompletenessReset(c *check.C) {
	const caCount = 100
	const resets = 20
	p := s.newPackWithoutCache(c, ForAuth)
	defer p.Close()

	// put lots of CAs in the backend
	for i := 0; i < caCount; i++ {
		ca := suite.NewTestCA(services.UserCA, fmt.Sprintf("%d.example.com", i))
		c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)
	}

	var err error
	p.cache, err = New(ForAuth(Config{
		Context:        context.TODO(),
		Backend:        p.cacheBackend,
		Events:         p.eventsS,
		ClusterConfig:  p.clusterConfigS,
		Provisioner:    p.provisionerS,
		Trust:          p.trustS,
		Users:          p.usersS,
		Access:         p.accessS,
		DynamicAccess:  p.dynamicAccessS,
		Presence:       p.presenceS,
		AppSession:     p.appSessionS,
		WebSession:     p.webSessionS,
		WebToken:       p.webTokenS,
		MaxRetryPeriod: 200 * time.Millisecond,
		EventsC:        p.eventsC,
	}))
	c.Assert(err, check.IsNil)

	// verify that CAs are immediately available
	cas, err := p.cache.GetCertAuthorities(services.UserCA, false)
	c.Assert(err, check.IsNil)
	c.Assert(len(cas), check.Equals, caCount)

	for i := 0; i < resets; i++ {
		// simulate bad connection to auth server
		p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
		p.eventsS.closeWatchers()
		p.backend.SetReadError(nil)

		// load CAs while connection is bad
		cas, err := p.cache.GetCertAuthorities(services.UserCA, false)
		// we don't actually care whether the cache ever fully constructed
		// the CA list.  for the purposes of this test, we just care that it
		// doesn't return the CA list *unless* it was successfully constructed.
		if err == nil {
			c.Assert(len(cas), check.Equals, caCount)
		} else {
			fixtures.ExpectConnectionProblem(c, err)
		}
	}
}

// TestTombstones verifies that healthy caches leave tombstones
// on closure, giving new caches the ability to start from a known
// good state if the origin state is unavailable.
func (s *CacheSuite) TestTombstones(c *check.C) {
	const caCount = 10
	p := s.newPackWithoutCache(c, ForAuth)
	defer p.Close()

	// put lots of CAs in the backend
	for i := 0; i < caCount; i++ {
		ca := suite.NewTestCA(services.UserCA, fmt.Sprintf("%d.example.com", i))
		c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)
	}

	var err error
	p.cache, err = New(ForAuth(Config{
		Context:        context.TODO(),
		Backend:        p.cacheBackend,
		Events:         p.eventsS,
		ClusterConfig:  p.clusterConfigS,
		Provisioner:    p.provisionerS,
		Trust:          p.trustS,
		Users:          p.usersS,
		Access:         p.accessS,
		DynamicAccess:  p.dynamicAccessS,
		Presence:       p.presenceS,
		AppSession:     p.appSessionS,
		WebSession:     p.webSessionS,
		WebToken:       p.webTokenS,
		MaxRetryPeriod: 200 * time.Millisecond,
		EventsC:        p.eventsC,
	}))
	c.Assert(err, check.IsNil)

	// verify that CAs are immediately available
	cas, err := p.cache.GetCertAuthorities(services.UserCA, false)
	c.Assert(err, check.IsNil)
	c.Assert(len(cas), check.Equals, caCount)

	c.Assert(p.cache.Close(), check.IsNil)
	// wait for TombstoneWritten, ignoring all other event types
	waitForEvent(c, p.eventsC, TombstoneWritten, WatcherStarted, EventProcessed, WatcherFailed)
	// simulate bad connection to auth server
	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
	p.eventsS.closeWatchers()

	p.cache, err = New(ForAuth(Config{
		Context:        context.TODO(),
		Backend:        p.cacheBackend,
		Events:         p.eventsS,
		ClusterConfig:  p.clusterConfigS,
		Provisioner:    p.provisionerS,
		Trust:          p.trustS,
		Users:          p.usersS,
		Access:         p.accessS,
		DynamicAccess:  p.dynamicAccessS,
		Presence:       p.presenceS,
		AppSession:     p.appSessionS,
		WebSession:     p.webSessionS,
		WebToken:       p.webTokenS,
		MaxRetryPeriod: 200 * time.Millisecond,
		EventsC:        p.eventsC,
	}))
	c.Assert(err, check.IsNil)

	// verify that CAs are immediately available despite the fact
	// that the origin state was never available.
	cas, err = p.cache.GetCertAuthorities(services.UserCA, false)
	c.Assert(err, check.IsNil)
	c.Assert(len(cas), check.Equals, caCount)
}

// TestInitStrategy verifies that cache uses expected init strategy
// of serving backend state when init is taking too long.
func (s *CacheSuite) TestInitStrategy(c *check.C) {
	for i := 0; i < utils.GetIterations(); i++ {
		s.initStrategy(c)
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
			server := suite.NewServer(types.KindNode, uuid.New(), "127.0.0.1:2022", apidefaults.Namespace)
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
			server := suite.NewServer(types.KindNode, uuid.New(), "127.0.0.1:2022", apidefaults.Namespace)
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

// TestListNodesTTLVariant verifies that the custom ListNodes impl that we fallback to when
// using ttl-based caching works as expected.
func TestListNodesTTLVariant(t *testing.T) {
	const nodeCount = 100
	const pageSize = 10
	var err error

	ctx := context.Background()

	p, err := newPackWithoutCache(t.TempDir(), ForAuth)
	require.NoError(t, err)
	defer p.Close()

	p.cache, err = New(ForAuth(Config{
		Context:        ctx,
		Backend:        p.cacheBackend,
		Events:         p.eventsS,
		ClusterConfig:  p.clusterConfigS,
		Provisioner:    p.provisionerS,
		Trust:          p.trustS,
		Users:          p.usersS,
		Access:         p.accessS,
		DynamicAccess:  p.dynamicAccessS,
		Presence:       p.presenceS,
		AppSession:     p.appSessionS,
		WebSession:     p.webSessionS,
		WebToken:       p.webTokenS,
		MaxRetryPeriod: 200 * time.Millisecond,
		EventsC:        p.eventsC,
		neverOK:        true, // ensure reads are never healthy
	}))
	require.NoError(t, err)

	for i := 0; i < nodeCount; i++ {
		server := suite.NewServer(types.KindNode, uuid.New(), "127.0.0.1:2022", apidefaults.Namespace)
		_, err := p.presenceS.UpsertNode(ctx, server)
		require.NoError(t, err)
	}

	time.Sleep(time.Second * 2)

	allNodes, err := p.cache.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, allNodes, nodeCount)

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
}

func (s *CacheSuite) initStrategy(c *check.C) {
	p := s.newPackWithoutCache(c, ForAuth)
	defer p.Close()

	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is out"))
	var err error
	p.cache, err = New(ForAuth(Config{
		Context:        context.TODO(),
		Backend:        p.cacheBackend,
		Events:         p.eventsS,
		ClusterConfig:  p.clusterConfigS,
		Provisioner:    p.provisionerS,
		Trust:          p.trustS,
		Users:          p.usersS,
		Access:         p.accessS,
		DynamicAccess:  p.dynamicAccessS,
		Presence:       p.presenceS,
		AppSession:     p.appSessionS,
		WebSession:     p.webSessionS,
		WebToken:       p.webTokenS,
		MaxRetryPeriod: 200 * time.Millisecond,
		EventsC:        p.eventsC,
	}))
	c.Assert(err, check.IsNil)

	_, err = p.cache.GetCertAuthorities(services.UserCA, false)
	fixtures.ExpectConnectionProblem(c, err)

	ca := suite.NewTestCA(services.UserCA, "example.com")
	// NOTE 1: this could produce event processed
	// below, based on whether watcher restarts to get the event
	// or not, which is normal, but has to be accounted below
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)
	p.backend.SetReadError(nil)

	// wait for watcher to restart
	waitForRestart(c, p.eventsC)

	normalizeCA := func(ca types.CertAuthority) types.CertAuthority {
		ca = ca.Clone()
		ca.SetResourceID(0)
		ca.SetExpiry(time.Time{})
		types.RemoveCASecrets(ca)
		return ca
	}

	out, err := p.cache.GetCertAuthority(ca.GetID(), false)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, normalizeCA(ca), normalizeCA(out))

	// fail again, make sure last recent data is still served
	// on errors
	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
	p.eventsS.closeWatchers()
	// wait for the watcher to fail
	// there could be optional event processed event,
	// see NOTE 1 above
	waitForEvent(c, p.eventsC, WatcherFailed, EventProcessed, Reloading)

	// backend is out, but old value is available
	out2, err := p.cache.GetCertAuthority(ca.GetID(), false)
	c.Assert(err, check.IsNil)
	c.Assert(out.GetResourceID(), check.Equals, out2.GetResourceID())
	fixtures.DeepCompare(c, normalizeCA(ca), normalizeCA(out))

	// add modification and expect the resource to recover
	ca.SetRoleMap(services.RoleMap{services.RoleMapping{Remote: "test", Local: []string{"local-test"}}})
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)

	// now, recover the backend and make sure the
	// service is back and the new value has propagated
	p.backend.SetReadError(nil)

	// wait for watcher to restart successfully; ignoring any failed
	// attempts which occurred before backend became healthy again.
	waitForEvent(c, p.eventsC, WatcherStarted, WatcherFailed, Reloading)

	// new value is available now
	out, err = p.cache.GetCertAuthority(ca.GetID(), false)
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, normalizeCA(ca), normalizeCA(out))
}

// TestRecovery tests error recovery scenario
func (s *CacheSuite) TestRecovery(c *check.C) {
	p := s.newPackForAuth(c)
	defer p.Close()

	ca := suite.NewTestCA(services.UserCA, "example.com")
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	// event has arrived, now close the watchers
	watchers := p.eventsS.getWatchers()
	c.Assert(watchers, check.HasLen, 1)
	p.eventsS.closeWatchers()

	waitForRestart(c, p.eventsC)

	// add modification and expect the resource to recover
	ca2 := suite.NewTestCA(services.UserCA, "example2.com")
	c.Assert(p.trustS.UpsertCertAuthority(ca2), check.IsNil)

	// wait for watcher to receive an event
	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetCertAuthority(ca2.GetID(), false)
	c.Assert(err, check.IsNil)
	ca2.SetResourceID(out.GetResourceID())
	services.RemoveCASecrets(ca2)
	fixtures.DeepCompare(c, ca2, out)
}

// TestTokens tests static and dynamic tokens
func (s *CacheSuite) TestTokens(c *check.C) {
	ctx := context.Background()
	p := s.newPackForAuth(c)
	defer p.Close()

	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionTokenV1{
			{
				Token:   "static1",
				Roles:   teleport.Roles{teleport.RoleAuth, teleport.RoleNode},
				Expires: time.Now().UTC().Add(time.Hour),
			},
		},
	})
	c.Assert(err, check.IsNil)

	err = p.clusterConfigS.SetStaticTokens(staticTokens)
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetStaticTokens()
	c.Assert(err, check.IsNil)
	staticTokens.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, staticTokens, out)

	expires := time.Now().Add(10 * time.Hour).Truncate(time.Second).UTC()
	token, err := services.NewProvisionToken("token", teleport.Roles{teleport.RoleAuth, teleport.RoleNode}, expires)
	c.Assert(err, check.IsNil)

	err = p.provisionerS.UpsertToken(ctx, token)
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	tout, err := p.cache.GetToken(ctx, token.GetName())
	c.Assert(err, check.IsNil)
	token.SetResourceID(tout.GetResourceID())
	fixtures.DeepCompare(c, token, tout)

	err = p.provisionerS.DeleteToken(ctx, token.GetName())
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetToken(ctx, token.GetName())
	fixtures.ExpectNotFound(c, err)
}

// TestClusterConfig tests cluster configuration
func (s *CacheSuite) TestClusterConfig(c *check.C) {
	p := s.newPackForAuth(c)
	defer p.Close()

	// update cluster config to record at the proxy
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: services.RecordAtProxy,
		Audit: services.AuditConfig{
			AuditEventsURI: []string{"dynamodb://audit_table_name", "file:///home/log"},
		},
	})
	c.Assert(err, check.IsNil)
	err = p.clusterConfigS.SetClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	clusterConfig, err = p.clusterConfigS.GetClusterConfig()
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetClusterConfig()
	c.Assert(err, check.IsNil)
	clusterConfig.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, clusterConfig, out)

	// update cluster name resource metadata
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	c.Assert(err, check.IsNil)
	err = p.clusterConfigS.SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	clusterName, err = p.clusterConfigS.GetClusterName()
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	outName, err := p.cache.GetClusterName()
	c.Assert(err, check.IsNil)

	clusterName.SetResourceID(outName.GetResourceID())
	fixtures.DeepCompare(c, outName, clusterName)
}

// TestNamespaces tests caching of namespaces
func (s *CacheSuite) TestNamespaces(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	v := services.NewNamespace("universe")
	ns := &v
	err := p.presenceS.UpsertNamespace(*ns)
	c.Assert(err, check.IsNil)

	ns, err = p.presenceS.GetNamespace(ns.GetName())
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetNamespace(ns.GetName())
	c.Assert(err, check.IsNil)
	ns.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, ns, out)

	// update namespace metadata
	ns.Metadata.Labels = map[string]string{"a": "b"}
	c.Assert(err, check.IsNil)
	err = p.presenceS.UpsertNamespace(*ns)
	c.Assert(err, check.IsNil)

	ns, err = p.presenceS.GetNamespace(ns.GetName())
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNamespace(ns.GetName())
	c.Assert(err, check.IsNil)
	ns.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, ns, out)

	err = p.presenceS.DeleteNamespace(ns.GetName())
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetNamespace(ns.GetName())
	fixtures.ExpectNotFound(c, err)
}

// TestUsers tests caching of users
func (s *CacheSuite) TestUsers(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	user, err := services.NewUser("bob")
	c.Assert(err, check.IsNil)
	err = p.usersS.UpsertUser(user)
	c.Assert(err, check.IsNil)

	user, err = p.usersS.GetUser(user.GetName(), false)
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetUser(user.GetName(), false)
	c.Assert(err, check.IsNil)
	user.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, user, out)

	// update user's roles
	user.SetRoles([]string{"admin"})
	c.Assert(err, check.IsNil)
	err = p.usersS.UpsertUser(user)
	c.Assert(err, check.IsNil)

	user, err = p.usersS.GetUser(user.GetName(), false)
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetUser(user.GetName(), false)
	c.Assert(err, check.IsNil)
	user.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, user, out)

	err = p.usersS.DeleteUser(context.TODO(), user.GetName())
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetUser(user.GetName(), false)
	fixtures.ExpectNotFound(c, err)
}

// TestRoles tests caching of roles
func (s *CacheSuite) TestRoles(c *check.C) {
	ctx := context.Background()
	p := s.newPackForNode(c)
	defer p.Close()

	role, err := services.NewRole("role1", types.RoleSpecV4{
		Options: services.RoleOptions{
			MaxSessionTTL: services.Duration(time.Hour),
		},
		Allow: services.RoleConditions{
			Logins:     []string{"root", "bob"},
			NodeLabels: services.Labels{services.Wildcard: []string{services.Wildcard}},
		},
		Deny: services.RoleConditions{},
	})
	c.Assert(err, check.IsNil)
	err = p.accessS.UpsertRole(ctx, role)
	c.Assert(err, check.IsNil)

	role, err = p.accessS.GetRole(ctx, role.GetName())
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetRole(ctx, role.GetName())
	c.Assert(err, check.IsNil)
	role.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, role, out)

	// update role
	role.SetLogins(services.Allow, []string{"admin"})
	c.Assert(err, check.IsNil)
	err = p.accessS.UpsertRole(ctx, role)
	c.Assert(err, check.IsNil)

	role, err = p.accessS.GetRole(ctx, role.GetName())
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRole(ctx, role.GetName())
	c.Assert(err, check.IsNil)
	role.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, role, out)

	err = p.accessS.DeleteRole(ctx, role.GetName())
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetRole(ctx, role.GetName())
	fixtures.ExpectNotFound(c, err)
}

// TestReverseTunnels tests reverse tunnels caching
func (s *CacheSuite) TestReverseTunnels(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	tunnel := services.NewReverseTunnel("example.com", []string{"example.com:2023"})
	c.Assert(p.presenceS.UpsertReverseTunnel(tunnel), check.IsNil)

	var err error
	tunnel, err = p.presenceS.GetReverseTunnel(tunnel.GetName())
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	tunnel.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, tunnel, out[0])

	// update tunnel's parameters
	tunnel.SetClusterName("new.example.com")
	c.Assert(err, check.IsNil)
	err = p.presenceS.UpsertReverseTunnel(tunnel)
	c.Assert(err, check.IsNil)

	out, err = p.presenceS.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	tunnel = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	tunnel.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, tunnel, out[0])

	err = p.presenceS.DeleteAllReverseTunnels()
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetReverseTunnels()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// TestTunnelConnections tests tunnel connections caching
func (s *CacheSuite) TestTunnelConnections(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	clusterName := "example.com"
	hb := time.Now().UTC()
	conn, err := services.NewTunnelConnection("conn1", services.TunnelConnectionSpecV2{
		ClusterName:   clusterName,
		ProxyName:     "p1",
		LastHeartbeat: hb,
	})
	c.Assert(err, check.IsNil)
	c.Assert(p.presenceS.UpsertTunnelConnection(conn), check.IsNil)

	out, err := p.presenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	conn = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, conn, out[0])

	// update conn's parameters
	hb = hb.Add(time.Second)
	conn.SetLastHeartbeat(hb)

	err = p.presenceS.UpsertTunnelConnection(conn)
	c.Assert(err, check.IsNil)

	out, err = p.presenceS.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	conn = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	conn.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, conn, out[0])

	err = p.presenceS.DeleteTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetTunnelConnections(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// TestNodes tests nodes cache
func (s *CacheSuite) TestNodes(c *check.C) {
	ctx := context.Background()

	p := s.newPackForProxy(c)
	defer p.Close()

	server := suite.NewServer(services.KindNode, "srv1", "127.0.0.1:2022", defaults.Namespace)
	_, err := p.presenceS.UpsertNode(ctx, server)
	c.Assert(err, check.IsNil)

	out, err := p.presenceS.GetNodes(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv := out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	// update srv parameters
	srv.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	srv.SetAddr("127.0.0.2:2033")

	lease, err := p.presenceS.UpsertNode(ctx, srv)
	c.Assert(err, check.IsNil)

	out, err = p.presenceS.GetNodes(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	// update keep alive on the node and make sure
	// it propagates
	lease.Expires = time.Now().UTC().Add(time.Hour)
	err = p.presenceS.KeepAliveNode(context.TODO(), *lease)
	c.Assert(err, check.IsNil)

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	srv.SetExpiry(lease.Expires)
	fixtures.DeepCompare(c, srv, out[0])

	err = p.presenceS.DeleteAllNodes(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(ctx, defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// TestProxies tests proxies cache
func (s *CacheSuite) TestProxies(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	server := suite.NewServer(services.KindProxy, "srv1", "127.0.0.1:2022", defaults.Namespace)
	err := p.presenceS.UpsertProxy(server)
	c.Assert(err, check.IsNil)

	out, err := p.presenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv := out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	// update srv parameters
	srv.SetAddr("127.0.0.2:2033")

	err = p.presenceS.UpsertProxy(srv)
	c.Assert(err, check.IsNil)

	out, err = p.presenceS.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	err = p.presenceS.DeleteAllProxies()
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetProxies()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// TestAuthServers tests auth servers cache
func (s *CacheSuite) TestAuthServers(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	server := suite.NewServer(services.KindAuthServer, "srv1", "127.0.0.1:2022", defaults.Namespace)
	err := p.presenceS.UpsertAuthServer(server)
	c.Assert(err, check.IsNil)

	out, err := p.presenceS.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv := out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	// update srv parameters
	srv.SetAddr("127.0.0.2:2033")

	err = p.presenceS.UpsertAuthServer(srv)
	c.Assert(err, check.IsNil)

	out, err = p.presenceS.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	err = p.presenceS.DeleteAllAuthServers()
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetAuthServers()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// TestRemoteClusters tests remote clusters caching
func (s *CacheSuite) TestRemoteClusters(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	clusterName := "example.com"
	rc, err := services.NewRemoteCluster(clusterName)
	c.Assert(err, check.IsNil)
	c.Assert(p.presenceS.CreateRemoteCluster(rc), check.IsNil)

	out, err := p.presenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	rc = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	rc.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, rc, out[0])

	// update conn's parameters
	meta := rc.GetMetadata()
	meta.Labels = map[string]string{"env": "prod"}
	rc.SetMetadata(meta)

	ctx := context.TODO()
	err = p.presenceS.UpdateRemoteCluster(ctx, rc)
	c.Assert(err, check.IsNil)

	out, err = p.presenceS.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	fixtures.DeepCompare(c, meta.Labels, out[0].GetMetadata().Labels)
	rc = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	rc.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, rc, out[0])

	err = p.presenceS.DeleteAllRemoteClusters()
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// TestAppServers tests that CRUD operations are replicated from the backend to
// the cache.
func (s *CacheSuite) TestAppServers(c *check.C) {
	p := s.newPackForProxy(c)
	defer p.Close()

	// Upsert application into backend.
	server := suite.NewAppServer("foo", "http://127.0.0.1:8080", "foo.example.com")
	_, err := p.presenceS.UpsertAppServer(context.Background(), server)
	c.Assert(err, check.IsNil)

	// Check that the application is now in the backend.
	out, err := p.presenceS.GetAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv := out[0]

	// Wait until the information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	// Make sure the cache has a single application in it.
	out, err = p.cache.GetAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	// Check that the value in the cache, value in the backend, and original
	// services.App all exactly match.
	srv.SetResourceID(out[0].GetResourceID())
	server.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])
	fixtures.DeepCompare(c, server, out[0])

	// Update the application and upsert it into the backend again.
	srv.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	_, err = p.presenceS.UpsertAppServer(context.Background(), srv)
	c.Assert(err, check.IsNil)

	// Check that the application is in the backend and only one exists (so an
	// update occurred).
	out, err = p.presenceS.GetAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv = out[0]

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	// Make sure the cache has a single application in it.
	out, err = p.cache.GetAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	// Check that the value in the cache, value in the backend, and original
	// services.App all exactly match.
	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	// Remove all applications from the backend.
	err = p.presenceS.DeleteAllAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = p.cache.GetAppServers(context.Background(), defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 0)
}

// TestDatabaseServers tests that CRUD operations on database servers are
// replicated from the backend to the cache.
func TestDatabaseServers(t *testing.T) {
	p, err := newPack(t.TempDir(), ForProxy)
	require.NoError(t, err)
	defer p.Close()

	ctx := context.Background()

	// Upsert database server into backend.
	server := types.NewDatabaseServerV3("foo", nil,
		types.DatabaseServerSpecV3{
			Protocol: defaults.ProtocolPostgres,
			URI:      "localhost:5432",
			Hostname: "localhost",
			HostID:   uuid.New(),
		})
	_, err = p.presenceS.UpsertDatabaseServer(ctx, server)
	require.NoError(t, err)

	// Check that the database server is now in the backend.
	out, err := p.presenceS.GetDatabaseServers(context.Background(), defaults.Namespace)
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
	out, err = p.cache.GetDatabaseServers(context.Background(), defaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Update the server and upsert it into the backend again.
	server.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	_, err = p.presenceS.UpsertDatabaseServer(context.Background(), server)
	require.NoError(t, err)

	// Check that the server is in the backend and only one exists (so an
	// update occurred).
	out, err = p.presenceS.GetDatabaseServers(context.Background(), defaults.Namespace)
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
	out, err = p.cache.GetDatabaseServers(context.Background(), defaults.Namespace)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff([]types.DatabaseServer{server}, out,
		cmpopts.IgnoreFields(types.Metadata{}, "ID")))

	// Remove all database servers from the backend.
	err = p.presenceS.DeleteAllDatabaseServers(context.Background(), defaults.Namespace)
	require.NoError(t, err)

	// Check that information has been replicated to the cache.
	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Check that the cache is now empty.
	out, err = p.cache.GetDatabaseServers(context.Background(), defaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 0, len(out))
}

func TestRelativeExpiry(t *testing.T) {
	const checkInterval = time.Second
	const nodeCount = int64(100)

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
		server := suite.NewServer(types.KindNode, uuid.New(), "127.0.0.1:2022", apidefaults.Namespace)
		server.SetExpiry(exp)
		_, err := p.presenceS.UpsertNode(ctx, server)
		require.NoError(t, err)
		// Check that information has been replicated to the cache.
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
	watchers []services.Watcher
	events   services.Events
}

func (p *proxyEvents) getWatchers() []services.Watcher {
	p.Lock()
	defer p.Unlock()
	out := make([]services.Watcher, len(p.watchers))
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

func (p *proxyEvents) NewWatcher(ctx context.Context, watch services.Watch) (services.Watcher, error) {
	w, err := p.events.NewWatcher(ctx, watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.Lock()
	defer p.Unlock()
	p.watchers = append(p.watchers, w)
	return w, nil
}
