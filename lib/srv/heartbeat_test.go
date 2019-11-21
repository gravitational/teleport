/*
Copyright 2018 Gravitational, Inc.

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

package srv

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

// HeartbeatSuite also implements ssh.ConnMetadata
type HeartbeatSuite struct {
}

var _ = check.Suite(&HeartbeatSuite{})
var _ = fmt.Printf

func (s *HeartbeatSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

// TestHeartbeatAnnounce tests announce cycles used for proxies and auth servers
func (s *HeartbeatSuite) TestHeartbeatAnnounce(c *check.C) {
	s.heartbeatAnnounce(c, HeartbeatModeProxy, services.KindProxy)
	s.heartbeatAnnounce(c, HeartbeatModeAuth, services.KindAuthServer)
}

// TestHeartbeatKeepAlive tests keep alive cycle used for nodes
func (s *HeartbeatSuite) TestHeartbeatKeepAlive(c *check.C) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	clock := clockwork.NewFakeClock()
	announcer := newFakeAnnouncer(ctx)

	srv := &services.ServerV2{
		Kind:    services.KindNode,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      "1",
		},
		Spec: services.ServerSpecV2{
			Addr:     "127.0.0.1:1234",
			Hostname: "2",
		},
	}
	hb, err := NewHeartbeat(HeartbeatConfig{
		Context:         ctx,
		Mode:            HeartbeatModeNode,
		Component:       "test",
		Announcer:       announcer,
		CheckPeriod:     time.Second,
		AnnouncePeriod:  60 * time.Second,
		KeepAlivePeriod: 10 * time.Second,
		ServerTTL:       600 * time.Second,
		Clock:           clock,
		GetServerInfo: func() (services.Server, error) {
			srv.SetTTL(clock, defaults.ServerAnnounceTTL)
			return srv, nil
		},
	})
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateInit)

	// on the first run, heartbeat will move to announce state,
	// will call announce right away
	err = hb.fetch()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounce)

	err = hb.announce()
	c.Assert(err, check.IsNil)
	c.Assert(announcer.upsertCalls[hb.Mode], check.Equals, 1)
	c.Assert(hb.state, check.Equals, HeartbeatStateKeepAliveWait)
	c.Assert(hb.nextKeepAlive, check.Equals, clock.Now().UTC().Add(hb.KeepAlivePeriod))

	// next call will not move to announce, because time is not up yet
	err = hb.fetchAndAnnounce()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateKeepAliveWait)

	// advance time, and heartbeat will move to keep alive
	clock.Advance(hb.KeepAlivePeriod + time.Second)
	err = hb.fetch()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateKeepAlive)
	err = hb.announce()
	c.Assert(err, check.IsNil)
	c.Assert(announcer.keepAlivesC, check.HasLen, 1)
	c.Assert(hb.state, check.Equals, HeartbeatStateKeepAliveWait)
	c.Assert(hb.nextKeepAlive, check.Equals, clock.Now().UTC().Add(hb.KeepAlivePeriod))

	// update server info, system should switch to announce state
	srv = &services.ServerV2{
		Kind:    services.KindNode,
		Version: services.V2,
		Metadata: services.Metadata{
			Namespace: defaults.Namespace,
			Name:      "1",
			Labels:    map[string]string{"a": "b"},
		},
		Spec: services.ServerSpecV2{
			Addr:     "127.0.0.1:1234",
			Hostname: "2",
		},
	}
	err = hb.fetch()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounce)
	err = hb.announce()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateKeepAliveWait)
	c.Assert(hb.nextKeepAlive, check.Equals, clock.Now().UTC().Add(hb.KeepAlivePeriod))

	// in case of any error while sending keep alive, system should fail
	// and go back to init state
	announcer.keepAlivesC = make(chan services.KeepAlive)
	announcer.err = trace.ConnectionProblem(nil, "ooops")
	announcer.Close()
	clock.Advance(hb.KeepAlivePeriod + time.Second)
	err = hb.fetch()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateKeepAlive)
	err = hb.announce()
	fixtures.ExpectConnectionProblem(c, err)
	c.Assert(hb.state, check.Equals, HeartbeatStateInit)
	c.Assert(announcer.upsertCalls[hb.Mode], check.Equals, 2)

	// on the next run, system will try to reannounce
	announcer.err = nil
	err = hb.fetch()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounce)
	err = hb.announce()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateKeepAliveWait)
	c.Assert(announcer.upsertCalls[hb.Mode], check.Equals, 3)
}

func (s *HeartbeatSuite) heartbeatAnnounce(c *check.C, mode HeartbeatMode, kind string) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	clock := clockwork.NewFakeClock()

	announcer := newFakeAnnouncer(ctx)
	hb, err := NewHeartbeat(HeartbeatConfig{
		Context:         ctx,
		Mode:            mode,
		Component:       "test",
		Announcer:       announcer,
		CheckPeriod:     time.Second,
		AnnouncePeriod:  60 * time.Second,
		KeepAlivePeriod: 10 * time.Second,
		ServerTTL:       600 * time.Second,
		Clock:           clock,
		GetServerInfo: func() (services.Server, error) {
			srv := &services.ServerV2{
				Kind:    kind,
				Version: services.V2,
				Metadata: services.Metadata{
					Namespace: defaults.Namespace,
					Name:      "1",
				},
				Spec: services.ServerSpecV2{
					Addr:     "127.0.0.1:1234",
					Hostname: "2",
				},
			}
			srv.SetTTL(clock, defaults.ServerAnnounceTTL)
			return srv, nil
		},
	})
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateInit)

	// on the first run, heartbeat will move to announce state,
	// will call announce right away
	err = hb.fetch()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounce)

	err = hb.announce()
	c.Assert(err, check.IsNil)
	c.Assert(announcer.upsertCalls[hb.Mode], check.Equals, 1)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounceWait)
	c.Assert(hb.nextAnnounce, check.Equals, clock.Now().UTC().Add(hb.AnnouncePeriod))

	// next call will not move to announce, because time is not up yet
	err = hb.fetchAndAnnounce()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounceWait)

	// advance time, and heartbeat will move to announce
	clock.Advance(hb.AnnouncePeriod * time.Second)
	err = hb.fetch()
	c.Assert(err, check.IsNil)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounce)
	err = hb.announce()
	c.Assert(err, check.IsNil)
	c.Assert(announcer.upsertCalls[hb.Mode], check.Equals, 2)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounceWait)
	c.Assert(hb.nextAnnounce, check.Equals, clock.Now().UTC().Add(hb.AnnouncePeriod))

	// in case of error, system will move to announce wait state,
	// with next attempt scheduled on the next keep alive period
	announcer.err = trace.ConnectionProblem(nil, "boom")
	clock.Advance(hb.AnnouncePeriod + time.Second)
	err = hb.fetchAndAnnounce()
	c.Assert(announcer.upsertCalls[hb.Mode], check.Equals, 3)
	fixtures.ExpectConnectionProblem(c, err)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounceWait)
	c.Assert(hb.nextAnnounce, check.Equals, clock.Now().UTC().Add(hb.KeepAlivePeriod))

	// once announce is successfull, next announce is set on schedule
	announcer.err = nil
	clock.Advance(hb.KeepAlivePeriod + time.Second)
	err = hb.fetchAndAnnounce()
	c.Assert(err, check.IsNil)
	c.Assert(announcer.upsertCalls[hb.Mode], check.Equals, 4)
	c.Assert(hb.state, check.Equals, HeartbeatStateAnnounceWait)
	c.Assert(hb.nextAnnounce, check.Equals, clock.Now().UTC().Add(hb.AnnouncePeriod))
}

func newFakeAnnouncer(ctx context.Context) *fakeAnnouncer {
	ctx, cancel := context.WithCancel(ctx)
	return &fakeAnnouncer{
		upsertCalls: make(map[HeartbeatMode]int),
		ctx:         ctx,
		cancel:      cancel,
		keepAlivesC: make(chan services.KeepAlive, 100),
	}
}

type fakeAnnouncer struct {
	err         error
	srv         services.Server
	upsertCalls map[HeartbeatMode]int
	closeCalls  int
	ctx         context.Context
	cancel      context.CancelFunc
	keepAlivesC chan<- services.KeepAlive
}

func (f *fakeAnnouncer) UpsertNode(s services.Server) (*services.KeepAlive, error) {
	f.upsertCalls[HeartbeatModeNode] += 1
	if f.err != nil {
		return nil, f.err
	}
	return &services.KeepAlive{}, nil
}

func (f *fakeAnnouncer) UpsertProxy(s services.Server) error {
	f.upsertCalls[HeartbeatModeProxy] += 1
	return f.err
}

func (f *fakeAnnouncer) UpsertAuthServer(s services.Server) error {
	f.upsertCalls[HeartbeatModeAuth] += 1
	return f.err
}

func (f *fakeAnnouncer) NewKeepAliver(ctx context.Context) (services.KeepAliver, error) {
	return f, f.err
}

// KeepAlives allows to receive keep alives
func (f *fakeAnnouncer) KeepAlives() chan<- services.KeepAlive {
	return f.keepAlivesC
}

// Done returns the channel signalling the closure
func (f *fakeAnnouncer) Done() <-chan struct{} {
	return f.ctx.Done()
}

// Close closes the watcher and releases
// all associated resources
func (f *fakeAnnouncer) Close() error {
	f.closeCalls += 1
	f.cancel()
	return nil
}

// Error returns error associated with keep aliver if any
func (f *fakeAnnouncer) Error() error {
	return f.err
}
