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
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

// TestHeartbeatKeepAlive tests keep alive cycle used for nodes and apps.
func TestHeartbeatKeepAlive(t *testing.T) {
	var tests = []struct {
		name       string
		mode       HeartbeatMode
		makeServer func() types.Resource
	}{
		{
			name: "keep alive node",
			mode: HeartbeatModeNode,
			makeServer: func() types.Resource {
				return &types.ServerV2{
					Kind:    services.KindNode,
					Version: services.V2,
					Metadata: types.Metadata{
						Namespace: defaults.Namespace,
						Name:      "1",
					},
					Spec: types.ServerSpecV2{
						Addr:     "127.0.0.1:1234",
						Hostname: "2",
					},
				}
			},
		},
		{
			name: "keep alive app server",
			mode: HeartbeatModeApp,
			makeServer: func() types.Resource {
				return &types.ServerV2{
					Kind:    services.KindAppServer,
					Version: services.V2,
					Metadata: types.Metadata{
						Namespace: defaults.Namespace,
						Name:      "1",
					},
					Spec: types.ServerSpecV2{
						Addr:     "127.0.0.1:1234",
						Hostname: "2",
					},
				}
			},
		},
		{
			name: "keep alive database server",
			mode: HeartbeatModeDB,
			makeServer: func() types.Resource {
				return &types.DatabaseServerV3{
					Kind:    types.KindDatabaseServer,
					Version: types.V3,
					Metadata: types.Metadata{
						Namespace: defaults.Namespace,
						Name:      "1",
					},
					Spec: types.DatabaseServerSpecV3{
						Protocol: defaults.ProtocolPostgres,
						URI:      "127.0.0.1:1234",
						Hostname: "2",
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()
			clock := clockwork.NewFakeClock()
			announcer := newFakeAnnouncer(ctx)

			server := tt.makeServer()

			hb, err := NewHeartbeat(HeartbeatConfig{
				Context:         ctx,
				Mode:            tt.mode,
				Component:       "test",
				Announcer:       announcer,
				CheckPeriod:     time.Second,
				AnnouncePeriod:  60 * time.Second,
				KeepAlivePeriod: 10 * time.Second,
				ServerTTL:       600 * time.Second,
				Clock:           clock,
				GetServerInfo: func() (types.Resource, error) {
					server.SetExpiry(clock.Now().UTC().Add(defaults.ServerAnnounceTTL))
					return server, nil
				},
			})
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateInit, hb.state)

			// on the first run, heartbeat will move to announce state,
			// will call announce right away
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateAnnounce, hb.state)

			err = hb.announce()
			require.NoError(t, err)
			require.Equal(t, 1, announcer.upsertCalls[hb.Mode])
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.KeepAlivePeriod), hb.nextKeepAlive)

			// next call will not move to announce, because time is not up yet
			err = hb.fetchAndAnnounce()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)

			// advance time, and heartbeat will move to keep alive
			clock.Advance(hb.KeepAlivePeriod + time.Second)
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateKeepAlive, hb.state)

			err = hb.announce()
			require.NoError(t, err)
			require.Len(t, announcer.keepAlivesC, 1)
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.KeepAlivePeriod), hb.nextKeepAlive)

			// update server info, system should switch to announce state
			server = tt.makeServer()
			server.SetName("2")

			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateAnnounce, hb.state)
			err = hb.announce()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.KeepAlivePeriod), hb.nextKeepAlive)

			// in case of any error while sending keep alive, system should fail
			// and go back to init state
			announcer.keepAlivesC = make(chan types.KeepAlive)
			announcer.err = trace.ConnectionProblem(nil, "ooops")
			announcer.Close()
			clock.Advance(hb.KeepAlivePeriod + time.Second)
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateKeepAlive, hb.state)
			err = hb.announce()
			require.Error(t, err)
			require.IsType(t, announcer.err, err)
			require.Equal(t, HeartbeatStateInit, hb.state)
			require.Equal(t, 2, announcer.upsertCalls[hb.Mode])

			// on the next run, system will try to reannounce
			announcer.err = nil
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateAnnounce, hb.state)
			err = hb.announce()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)
			require.Equal(t, 3, announcer.upsertCalls[hb.Mode])
		})
	}
}

// TestHeartbeatAnnounce tests announce cycles used for proxies and auth servers
func TestHeartbeatAnnounce(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode HeartbeatMode
		kind string
	}{
		{mode: HeartbeatModeProxy, kind: services.KindProxy},
		{mode: HeartbeatModeAuth, kind: services.KindAuthServer},
		{mode: HeartbeatModeKube, kind: services.KindKubeService},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()
			clock := clockwork.NewFakeClock()

			announcer := newFakeAnnouncer(ctx)
			hb, err := NewHeartbeat(HeartbeatConfig{
				Context:         ctx,
				Mode:            tt.mode,
				Component:       "test",
				Announcer:       announcer,
				CheckPeriod:     time.Second,
				AnnouncePeriod:  60 * time.Second,
				KeepAlivePeriod: 10 * time.Second,
				ServerTTL:       600 * time.Second,
				Clock:           clock,
				GetServerInfo: func() (types.Resource, error) {
					srv := &types.ServerV2{
						Kind:    tt.kind,
						Version: services.V2,
						Metadata: types.Metadata{
							Namespace: defaults.Namespace,
							Name:      "1",
						},
						Spec: types.ServerSpecV2{
							Addr:     "127.0.0.1:1234",
							Hostname: "2",
						},
					}
					srv.SetExpiry(clock.Now().UTC().Add(defaults.ServerAnnounceTTL))
					return srv, nil
				},
			})
			require.NoError(t, err)
			require.Equal(t, hb.state, HeartbeatStateInit)

			// on the first run, heartbeat will move to announce state,
			// will call announce right away
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, hb.state, HeartbeatStateAnnounce)

			err = hb.announce()
			require.NoError(t, err)
			require.Equal(t, announcer.upsertCalls[hb.Mode], 1)
			require.Equal(t, hb.state, HeartbeatStateAnnounceWait)
			require.Equal(t, hb.nextAnnounce, clock.Now().UTC().Add(hb.AnnouncePeriod))

			// next call will not move to announce, because time is not up yet
			err = hb.fetchAndAnnounce()
			require.NoError(t, err)
			require.Equal(t, hb.state, HeartbeatStateAnnounceWait)

			// advance time, and heartbeat will move to announce
			clock.Advance(hb.AnnouncePeriod * time.Second)
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, hb.state, HeartbeatStateAnnounce)
			err = hb.announce()
			require.NoError(t, err)
			require.Equal(t, announcer.upsertCalls[hb.Mode], 2)
			require.Equal(t, hb.state, HeartbeatStateAnnounceWait)
			require.Equal(t, hb.nextAnnounce, clock.Now().UTC().Add(hb.AnnouncePeriod))

			// in case of error, system will move to announce wait state,
			// with next attempt scheduled on the next keep alive period
			announcer.err = trace.ConnectionProblem(nil, "boom")
			clock.Advance(hb.AnnouncePeriod + time.Second)
			err = hb.fetchAndAnnounce()
			require.Error(t, err)
			require.True(t, trace.IsConnectionProblem(err))
			require.Equal(t, announcer.upsertCalls[hb.Mode], 3)
			require.Equal(t, hb.state, HeartbeatStateAnnounceWait)
			require.Equal(t, hb.nextAnnounce, clock.Now().UTC().Add(hb.KeepAlivePeriod))

			// once announce is successful, next announce is set on schedule
			announcer.err = nil
			clock.Advance(hb.KeepAlivePeriod + time.Second)
			err = hb.fetchAndAnnounce()
			require.NoError(t, err)
			require.Equal(t, announcer.upsertCalls[hb.Mode], 4)
			require.Equal(t, hb.state, HeartbeatStateAnnounceWait)
			require.Equal(t, hb.nextAnnounce, clock.Now().UTC().Add(hb.AnnouncePeriod))
		})
	}
}

func newFakeAnnouncer(ctx context.Context) *fakeAnnouncer {
	ctx, cancel := context.WithCancel(ctx)
	return &fakeAnnouncer{
		upsertCalls: make(map[HeartbeatMode]int),
		ctx:         ctx,
		cancel:      cancel,
		keepAlivesC: make(chan types.KeepAlive, 100),
	}
}

type fakeAnnouncer struct {
	err         error
	upsertCalls map[HeartbeatMode]int
	closeCalls  int
	ctx         context.Context
	cancel      context.CancelFunc
	keepAlivesC chan<- types.KeepAlive
}

func (f *fakeAnnouncer) UpsertAppServer(ctx context.Context, s types.Server) (*types.KeepAlive, error) {
	f.upsertCalls[HeartbeatModeApp]++
	if f.err != nil {
		return nil, f.err
	}
	return &types.KeepAlive{}, nil
}

func (f *fakeAnnouncer) UpsertDatabaseServer(ctx context.Context, s types.DatabaseServer) (*types.KeepAlive, error) {
	f.upsertCalls[HeartbeatModeDB]++
	if f.err != nil {
		return nil, f.err
	}
	return &types.KeepAlive{}, nil
}

func (f *fakeAnnouncer) UpsertNode(ctx context.Context, s types.Server) (*types.KeepAlive, error) {
	f.upsertCalls[HeartbeatModeNode]++
	if f.err != nil {
		return nil, f.err
	}
	return &types.KeepAlive{}, nil
}

func (f *fakeAnnouncer) UpsertProxy(s types.Server) error {
	f.upsertCalls[HeartbeatModeProxy]++
	return f.err
}

func (f *fakeAnnouncer) UpsertAuthServer(s types.Server) error {
	f.upsertCalls[HeartbeatModeAuth]++
	return f.err
}

func (f *fakeAnnouncer) UpsertKubeService(ctx context.Context, s types.Server) error {
	f.upsertCalls[HeartbeatModeKube]++
	return f.err
}

func (f *fakeAnnouncer) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	return f, f.err
}

// KeepAlives allows to receive keep alives
func (f *fakeAnnouncer) KeepAlives() chan<- types.KeepAlive {
	return f.keepAlivesC
}

// Done returns the channel signalling the closure
func (f *fakeAnnouncer) Done() <-chan struct{} {
	return f.ctx.Done()
}

// Close closes the watcher and releases
// all associated resources
func (f *fakeAnnouncer) Close() error {
	f.closeCalls++
	f.cancel()
	return nil
}

// Error returns error associated with keep aliver if any
func (f *fakeAnnouncer) Error() error {
	return f.err
}
