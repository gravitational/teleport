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

package srv

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestHeartbeatKeepAlive tests keep alive cycle used for nodes and apps.
func TestHeartbeatKeepAlive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mode       HeartbeatMode
		makeServer func() types.Resource
	}{
		{
			name: "keep alive node",
			mode: HeartbeatModeNode,
			makeServer: func() types.Resource {
				return &types.ServerV2{
					Kind:    types.KindNode,
					Version: types.V2,
					Metadata: types.Metadata{
						Namespace: apidefaults.Namespace,
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
				return &types.AppServerV3{
					Kind:    types.KindAppServer,
					Version: types.V3,
					Metadata: types.Metadata{
						Namespace: apidefaults.Namespace,
						Name:      "1",
					},
					Spec: types.AppServerSpecV3{
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
						Namespace: apidefaults.Namespace,
						Name:      "db-1",
					},
					Spec: types.DatabaseServerSpecV3{
						Database: mustCreateDatabase(t, "db-1", defaults.ProtocolPostgres, "127.0.0.1:1234"),
						Hostname: "2",
					},
				}
			},
		},
		{
			name: "keep alive database service",
			mode: HeartbeatModeDatabaseService,
			makeServer: func() types.Resource {
				return &types.DatabaseServiceV1{
					ResourceHeader: types.ResourceHeader{
						Kind:    types.KindDatabaseService,
						Version: types.V1,
						Metadata: types.Metadata{
							Name:      "1",
							Namespace: apidefaults.Namespace,
						},
					},
					Spec: types.DatabaseServiceSpecV1{
						ResourceMatchers: []*types.DatabaseResourceMatcher{
							{Labels: &types.Labels{"env": []string{"prod", "qa"}}},
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
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
					server.SetExpiry(clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
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

			doneSomething, err := hb.announce()
			require.NoError(t, err)
			require.True(t, doneSomething)
			require.Equal(t, 1, announcer.upsertCalls[hb.Mode])
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.KeepAlivePeriod), hb.nextKeepAlive)

			// next call will not move to announce, because time is not up yet
			doneSomething, err = hb.fetchAndAnnounce()
			require.NoError(t, err)
			require.False(t, doneSomething)
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)

			// advance time, and heartbeat will move to keep alive
			clock.Advance(hb.KeepAlivePeriod + time.Second)
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateKeepAlive, hb.state)

			doneSomething, err = hb.announce()
			require.NoError(t, err)
			require.True(t, doneSomething)
			require.Len(t, announcer.keepAlivesC, 1)
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.KeepAlivePeriod), hb.nextKeepAlive)

			// update server info, system should switch to announce state
			server = tt.makeServer()
			server.SetName("2")

			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateAnnounce, hb.state)
			doneSomething, err = hb.announce()
			require.NoError(t, err)
			require.True(t, doneSomething)
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
			_, err = hb.announce()
			require.Error(t, err)
			require.IsType(t, announcer.err, err)
			require.Equal(t, HeartbeatStateInit, hb.state)
			require.Equal(t, 2, announcer.upsertCalls[hb.Mode])

			// on the next run, system will try to reannounce
			announcer.err = nil
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateAnnounce, hb.state)
			doneSomething, err = hb.announce()
			require.NoError(t, err)
			require.True(t, doneSomething)
			require.Equal(t, HeartbeatStateKeepAliveWait, hb.state)
			require.Equal(t, 3, announcer.upsertCalls[hb.Mode])
		})
	}
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

// TestHeartbeatAnnounce tests announce cycles used for proxies and auth servers
func TestHeartbeatAnnounce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode HeartbeatMode
		kind string
	}{
		{mode: HeartbeatModeProxy, kind: types.KindProxy},
		{mode: HeartbeatModeAuth, kind: types.KindAuthServer},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			ctx := t.Context()
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
						Version: types.V2,
						Metadata: types.Metadata{
							Namespace: apidefaults.Namespace,
							Name:      "1",
						},
						Spec: types.ServerSpecV2{
							Addr:     "127.0.0.1:1234",
							Hostname: "2",
						},
					}
					srv.SetExpiry(clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
					return srv, nil
				},
			})
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateInit, hb.state)

			// on the first run, heartbeat will move to announce state,
			// will call announce right away
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateAnnounce, hb.state)

			doneSomething, err := hb.announce()
			require.NoError(t, err)
			require.True(t, doneSomething)
			require.Equal(t, 1, announcer.upsertCalls[hb.Mode])
			require.Equal(t, HeartbeatStateAnnounceWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.AnnouncePeriod), hb.nextAnnounce)

			// next call will not move to announce, because time is not up yet
			doneSomething, err = hb.fetchAndAnnounce()
			require.NoError(t, err)
			require.False(t, doneSomething)
			require.Equal(t, HeartbeatStateAnnounceWait, hb.state)

			// advance time, and heartbeat will move to announce
			clock.Advance(hb.AnnouncePeriod + time.Second)
			err = hb.fetch()
			require.NoError(t, err)
			require.Equal(t, HeartbeatStateAnnounce, hb.state)
			doneSomething, err = hb.announce()
			require.NoError(t, err)
			require.True(t, doneSomething)
			require.Equal(t, 2, announcer.upsertCalls[hb.Mode])
			require.Equal(t, HeartbeatStateAnnounceWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.AnnouncePeriod), hb.nextAnnounce)

			// in case of error, system will move to announce wait state,
			// with next attempt scheduled on the next keep alive period
			announcer.err = trace.ConnectionProblem(nil, "boom")
			clock.Advance(hb.AnnouncePeriod + time.Second)
			_, err = hb.fetchAndAnnounce()
			require.Error(t, err)
			require.True(t, trace.IsConnectionProblem(err))
			require.Equal(t, 3, announcer.upsertCalls[hb.Mode])
			require.Equal(t, HeartbeatStateAnnounceWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.KeepAlivePeriod), hb.nextAnnounce)

			// once announce is successful, next announce is set on schedule
			announcer.err = nil
			clock.Advance(hb.KeepAlivePeriod + time.Second)
			doneSomething, err = hb.fetchAndAnnounce()
			require.NoError(t, err)
			require.True(t, doneSomething)
			require.Equal(t, 4, announcer.upsertCalls[hb.Mode])
			require.Equal(t, HeartbeatStateAnnounceWait, hb.state)
			require.Equal(t, clock.Now().UTC().Add(hb.AnnouncePeriod), hb.nextAnnounce)
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

func (f *fakeAnnouncer) UpsertApplicationServer(ctx context.Context, s types.AppServer) (*types.KeepAlive, error) {
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

func (f *fakeAnnouncer) UpsertProxy(ctx context.Context, s types.Server) error {
	f.upsertCalls[HeartbeatModeProxy]++
	return f.err
}

func (f *fakeAnnouncer) UpsertAuthServer(ctx context.Context, s types.Server) error {
	f.upsertCalls[HeartbeatModeAuth]++
	return f.err
}

func (f *fakeAnnouncer) UpsertKubernetesServer(ctx context.Context, s types.KubeServer) (*types.KeepAlive, error) {
	f.upsertCalls[HeartbeatModeKube]++
	if f.err != nil {
		return nil, f.err
	}
	return &types.KeepAlive{}, f.err
}

func (f *fakeAnnouncer) UpsertWindowsDesktopService(ctx context.Context, s types.WindowsDesktopService) (*types.KeepAlive, error) {
	f.upsertCalls[HeartbeatModeWindowsDesktopService]++
	if f.err != nil {
		return nil, f.err
	}
	return &types.KeepAlive{}, nil
}

func (f *fakeAnnouncer) UpsertWindowsDesktop(ctx context.Context, s types.WindowsDesktop) error {
	f.upsertCalls[HeartbeatModeWindowsDesktop]++
	return f.err
}

func (f *fakeAnnouncer) UpsertDatabaseService(ctx context.Context, s types.DatabaseService) (*types.KeepAlive, error) {
	f.upsertCalls[HeartbeatModeDatabaseService]++
	if f.err != nil {
		return nil, f.err
	}
	return &types.KeepAlive{}, nil
}

func (f *fakeAnnouncer) NewKeepAliver(ctx context.Context) (types.KeepAliver, error) {
	return f, f.err
}

// KeepAlives allows to receive keep alives
func (f *fakeAnnouncer) KeepAlives() chan<- types.KeepAlive {
	return f.keepAlivesC
}

// Done returns the channel signaling the closure
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
