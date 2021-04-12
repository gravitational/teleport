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

// newPackWithoutCache returns a new test pack without creating cache
func newPackWithoutCache(dir string, ssetupConfig SetupConfigFn) (*testPack, error) {
	p := &testPack{
		dataDir: dir,
	}
	bk, err := lite.NewWithConfig(context.TODO(), lite.Config{
		Path:             p.dataDir,
		PollStreamPeriod: 200 * time.Millisecond,
	})
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
func newPack(dir string, setupConfig func(c Config) Config) (*testPack, error) {
	p, err := newPackWithoutCache(dir, setupConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.cache, err = New(setupConfig(Config{
		Context:       context.TODO(),
		Backend:       p.cacheBackend,
		Events:        p.eventsS,
		ClusterConfig: p.clusterConfigS,
		Provisioner:   p.provisionerS,
		Trust:         p.trustS,
		Users:         p.usersS,
		Access:        p.accessS,
		DynamicAccess: p.dynamicAccessS,
		Presence:      p.presenceS,
		AppSession:    p.appSessionS,
		WebSession:    p.webSessionS,
		WebToken:      p.webTokenS,
		RetryPeriod:   200 * time.Millisecond,
		EventsC:       p.eventsC,
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

// TestOnlyRecentInit makes sure init fails
// with "only recent" cache strategy
func (s *CacheSuite) TestOnlyRecentInit(c *check.C) {
	p := s.newPackWithoutCache(c, ForAuth)
	defer p.Close()

	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is out"))
	_, err := New(ForAuth(Config{
		Context:       context.TODO(),
		Backend:       p.cacheBackend,
		Events:        p.eventsS,
		ClusterConfig: p.clusterConfigS,
		Provisioner:   p.provisionerS,
		Trust:         p.trustS,
		Users:         p.usersS,
		Access:        p.accessS,
		DynamicAccess: p.dynamicAccessS,
		Presence:      p.presenceS,
		AppSession:    p.appSessionS,
		WebSession:    p.webSessionS,
		WebToken:      p.webTokenS,
		RetryPeriod:   200 * time.Millisecond,
		EventsC:       p.eventsC,
	}))
	fixtures.ExpectConnectionProblem(c, err)
}

// TestOnlyRecentDisconnect tests that cache
// with "only recent" cache strategy will not serve
// stale data during disconnects
func (s *CacheSuite) TestOnlyRecentDisconnect(c *check.C) {
	for i := 0; i < utils.GetIterations(); i++ {
		s.onlyRecentDisconnect(c)
	}
}

func (s *CacheSuite) onlyRecentDisconnect(c *check.C) {
	p := s.newPackForAuth(c)
	defer p.Close()

	ca := suite.NewTestCA(services.UserCA, "example.com")
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	// event has arrived, now close the watchers and the backend
	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
	p.eventsS.closeWatchers()

	// wait for the watcher to fail
	waitForEvent(c, p.eventsC, WatcherFailed)

	// backend is out, so no service is available
	_, err := p.cache.GetCertAuthority(ca.GetID(), false)
	fixtures.ExpectConnectionProblem(c, err)

	// add modification and expect the resource to recover
	ca.SetRoleMap(services.RoleMap{services.RoleMapping{Remote: "test", Local: []string{"local-test"}}})
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)

	// now, recover the backend and make sure the
	// service is back
	p.backend.SetReadError(nil)

	// wait for watcher to restart
	waitForRestart(c, p.eventsC)

	// new value is available now
	out, err := p.cache.GetCertAuthority(ca.GetID(), false)
	c.Assert(err, check.IsNil)
	ca.SetResourceID(out.GetResourceID())
	services.RemoveCASecrets(ca)
	fixtures.DeepCompare(c, ca, out)
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
	waitForEvent(c, eventsC, WatcherStarted, WatcherFailed)
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
			Context:       context.TODO(),
			Backend:       p.cacheBackend,
			Events:        p.eventsS,
			ClusterConfig: p.clusterConfigS,
			Provisioner:   p.provisionerS,
			Trust:         p.trustS,
			Users:         p.usersS,
			Access:        p.accessS,
			DynamicAccess: p.dynamicAccessS,
			Presence:      p.presenceS,
			AppSession:    p.appSessionS,
			WebSession:    p.webSessionS,
			WebToken:      p.webTokenS,
			RetryPeriod:   200 * time.Millisecond,
			EventsC:       p.eventsC,
			PreferRecent: PreferRecent{
				Enabled: true,
			},
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
		Context:       context.TODO(),
		Backend:       p.cacheBackend,
		Events:        p.eventsS,
		ClusterConfig: p.clusterConfigS,
		Provisioner:   p.provisionerS,
		Trust:         p.trustS,
		Users:         p.usersS,
		Access:        p.accessS,
		DynamicAccess: p.dynamicAccessS,
		Presence:      p.presenceS,
		AppSession:    p.appSessionS,
		WebSession:    p.webSessionS,
		WebToken:      p.webTokenS,
		RetryPeriod:   200 * time.Millisecond,
		EventsC:       p.eventsC,
		PreferRecent: PreferRecent{
			Enabled: true,
		},
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
		Context:       context.TODO(),
		Backend:       p.cacheBackend,
		Events:        p.eventsS,
		ClusterConfig: p.clusterConfigS,
		Provisioner:   p.provisionerS,
		Trust:         p.trustS,
		Users:         p.usersS,
		Access:        p.accessS,
		DynamicAccess: p.dynamicAccessS,
		Presence:      p.presenceS,
		AppSession:    p.appSessionS,
		WebSession:    p.webSessionS,
		WebToken:      p.webTokenS,
		RetryPeriod:   200 * time.Millisecond,
		EventsC:       p.eventsC,
		PreferRecent: PreferRecent{
			Enabled: true,
		},
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
		Context:       context.TODO(),
		Backend:       p.cacheBackend,
		Events:        p.eventsS,
		ClusterConfig: p.clusterConfigS,
		Provisioner:   p.provisionerS,
		Trust:         p.trustS,
		Users:         p.usersS,
		Access:        p.accessS,
		DynamicAccess: p.dynamicAccessS,
		Presence:      p.presenceS,
		AppSession:    p.appSessionS,
		WebSession:    p.webSessionS,
		WebToken:      p.webTokenS,
		RetryPeriod:   200 * time.Millisecond,
		EventsC:       p.eventsC,
		PreferRecent: PreferRecent{
			Enabled: true,
		},
	}))
	c.Assert(err, check.IsNil)

	// verify that CAs are immediately available despite the fact
	// that the origin state was never available.
	cas, err = p.cache.GetCertAuthorities(services.UserCA, false)
	c.Assert(err, check.IsNil)
	c.Assert(len(cas), check.Equals, caCount)
}

// TestPreferRecent makes sure init proceeds
// with "prefer recent" cache strategy
// even if the backend is unavailable
// then recovers against failures and serves data during failures
func (s *CacheSuite) TestPreferRecent(c *check.C) {
	for i := 0; i < utils.GetIterations(); i++ {
		s.preferRecent(c)
	}
}

func (s *CacheSuite) preferRecent(c *check.C) {
	p := s.newPackWithoutCache(c, ForAuth)
	defer p.Close()

	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is out"))
	var err error
	p.cache, err = New(ForAuth(Config{
		Context:       context.TODO(),
		Backend:       p.cacheBackend,
		Events:        p.eventsS,
		ClusterConfig: p.clusterConfigS,
		Provisioner:   p.provisionerS,
		Trust:         p.trustS,
		Users:         p.usersS,
		Access:        p.accessS,
		DynamicAccess: p.dynamicAccessS,
		Presence:      p.presenceS,
		AppSession:    p.appSessionS,
		WebSession:    p.webSessionS,
		WebToken:      p.webTokenS,
		RetryPeriod:   200 * time.Millisecond,
		EventsC:       p.eventsC,
		PreferRecent: PreferRecent{
			Enabled: true,
		},
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

	out, err := p.cache.GetCertAuthority(ca.GetID(), false)
	c.Assert(err, check.IsNil)
	ca.SetResourceID(out.GetResourceID())
	ca.SetExpiry(out.Expiry())
	services.RemoveCASecrets(ca)
	fixtures.DeepCompare(c, ca, out)

	// fail again, make sure last recent data is still served
	// on errors
	p.backend.SetReadError(trace.ConnectionProblem(nil, "backend is unavailable"))
	p.eventsS.closeWatchers()
	// wait for the watcher to fail
	// there could be optional event processed event,
	// see NOTE 1 above
	waitForEvent(c, p.eventsC, WatcherFailed, EventProcessed)

	// backend is out, but old value is available
	out, err = p.cache.GetCertAuthority(ca.GetID(), false)
	log.Debugf("Resource ID after fail: %v vs the one ca has %v", out.GetResourceID(), ca.GetResourceID())
	ca.SetExpiry(out.Expiry())
	c.Assert(err, check.IsNil)
	fixtures.DeepCompare(c, ca, out)

	// add modification and expect the resource to recover
	ca.SetRoleMap(services.RoleMap{services.RoleMapping{Remote: "test", Local: []string{"local-test"}}})
	c.Assert(p.trustS.UpsertCertAuthority(ca), check.IsNil)

	// now, recover the backend and make sure the
	// service is back and the new value has propagated
	p.backend.SetReadError(nil)

	// wait for watcher to restart successfully; ignoring any failed
	// attempts which occurred before backend became healthy again.
	waitForEvent(c, p.eventsC, WatcherStarted, WatcherFailed)

	// new value is available now
	out, err = p.cache.GetCertAuthority(ca.GetID(), false)
	c.Assert(err, check.IsNil)
	ca.SetExpiry(out.Expiry())
	ca.SetResourceID(out.GetResourceID())
	fixtures.DeepCompare(c, ca, out)
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

	// wait for watcher to restart
	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, WatcherStarted)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

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

	role, err := services.NewRole("role1", services.RoleSpecV3{
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
	p := s.newPackForProxy(c)
	defer p.Close()

	server := suite.NewServer(services.KindNode, "srv1", "127.0.0.1:2022", defaults.Namespace)
	_, err := p.presenceS.UpsertNode(server)
	c.Assert(err, check.IsNil)

	out, err := p.presenceS.GetNodes(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv := out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	fixtures.DeepCompare(c, srv, out[0])

	// update srv parameters
	srv.SetExpiry(time.Now().Add(30 * time.Minute).UTC())
	srv.SetAddr("127.0.0.2:2033")

	lease, err := p.presenceS.UpsertNode(srv)
	c.Assert(err, check.IsNil)

	out, err = p.presenceS.GetNodes(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)
	srv = out[0]

	select {
	case event := <-p.eventsC:
		c.Assert(event.Type, check.Equals, EventProcessed)
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(defaults.Namespace)
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

	out, err = p.cache.GetNodes(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(out, check.HasLen, 1)

	srv.SetResourceID(out[0].GetResourceID())
	srv.SetExpiry(lease.Expires)
	fixtures.DeepCompare(c, srv, out[0])

	err = p.presenceS.DeleteAllNodes(defaults.Namespace)
	c.Assert(err, check.IsNil)

	select {
	case <-p.eventsC:
	case <-time.After(time.Second):
		c.Fatalf("timeout waiting for event")
	}

	out, err = p.cache.GetNodes(defaults.Namespace)
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
