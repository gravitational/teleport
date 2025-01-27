// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package srv

import (
	"context"
	"io"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type mockEvents struct {
	events chan types.Event
	done   chan struct{}
}

func newMockEvents() *mockEvents {
	return &mockEvents{
		events: make(chan types.Event),
		done:   make(chan struct{}),
	}
}

func (m *mockEvents) NewWatcher(_ context.Context, _ types.Watch) (types.Watcher, error) {
	return &mockWatcher{
		events: m.events,
		done:   m.done,
	}, nil
}

func (m *mockEvents) Close() error {
	close(m.events)
	close(m.done)
	return nil
}

type mockWatcher struct {
	events <-chan types.Event
	done   <-chan struct{}
}

func (m *mockWatcher) Events() <-chan types.Event {
	return m.events
}

func (m *mockWatcher) Done() <-chan struct{} {
	return m.done
}

func (m *mockWatcher) Close() error {
	return nil
}

func (m *mockWatcher) Error() error {
	return nil
}

type mockStaticHostUsers struct {
	services.StaticHostUser
	hostUsers []*userprovisioningv2.StaticHostUser
}

func (m *mockStaticHostUsers) ListStaticHostUsers(_ context.Context, _ int, _ string) ([]*userprovisioningv2.StaticHostUser, string, error) {
	return m.hostUsers, "", nil
}

type mockInfoGetter struct {
	labels map[string]string
}

func (m mockInfoGetter) GetInfo() types.Server {
	s, _ := types.NewServer("test", types.KindNode, types.ServerSpecV2{})
	s.SetStaticLabels(m.labels)
	return s
}

type mockHostUsers struct {
	HostUsers
	upsertedUsers map[string]services.HostUsersInfo
}

func (m *mockHostUsers) UpsertUser(name string, ui services.HostUsersInfo) (io.Closer, error) {
	if m.upsertedUsers == nil {
		m.upsertedUsers = make(map[string]services.HostUsersInfo)
	}
	m.upsertedUsers[name] = ui
	return nil, nil
}

type mockHostSudoers struct {
	HostSudoers
	sudoers map[string][]string
}

func (m *mockHostSudoers) WriteSudoers(name string, sudoers []string) error {
	if m.sudoers == nil {
		m.sudoers = make(map[string][]string)
	}
	m.sudoers[name] = sudoers
	return nil
}

type eventSender func(ctx context.Context, events *mockEvents, clock *clockwork.FakeClock) error

func TestStaticHostUserHandler(t *testing.T) {
	t.Parallel()

	sendEvents := func(eventList []types.Event) eventSender {
		return func(ctx context.Context, events *mockEvents, clock *clockwork.FakeClock) error {
			for _, event := range eventList {
				select {
				case events.events <- event:
				case <-ctx.Done():
					break
				}
			}
			events.Close()
			<-ctx.Done()
			return nil
		}
	}

	makeStaticHostUser := func(name string, labels map[string]string, groups []string) *userprovisioningv2.StaticHostUser {
		nodeLabels := make([]*labelv1.Label, 0, len(labels))
		for k, v := range labels {
			nodeLabels = append(nodeLabels, &labelv1.Label{
				Name:   k,
				Values: []string{v},
			})
		}
		return userprovisioning.NewStaticHostUser(name, &userprovisioningv2.StaticHostUserSpec{
			Matchers: []*userprovisioningv2.Matcher{
				{
					NodeLabels:   nodeLabels,
					Groups:       groups,
					Uid:          1234,
					Gid:          5678,
					Sudoers:      []string{"abcd1234"},
					DefaultShell: "/bin/bash",
				},
			},
		})
	}

	tests := []struct {
		name          string
		existingUsers []*userprovisioningv2.StaticHostUser
		sendEvents    eventSender
		wantUsers     map[string]services.HostUsersInfo
		wantSudoers   map[string][]string
	}{
		{
			name: "ok users",
			existingUsers: []*userprovisioningv2.StaticHostUser{
				makeStaticHostUser("test-1", map[string]string{"foo": "bar"}, []string{"foo", "bar"}),
			},
			sendEvents: sendEvents([]types.Event{
				{
					Type: types.OpInit,
				},
				{
					Type: types.OpPut,
					Resource: types.Resource153ToLegacy(
						makeStaticHostUser("test-2", map[string]string{"foo": "bar"}, []string{"baz", "quux"}),
					),
				},
			}),
			wantUsers: map[string]services.HostUsersInfo{
				"test-1": {
					Groups: []string{"foo", "bar"},
					Mode:   services.HostUserModeStatic,
					UID:    "1234",
					GID:    "5678",
					Shell:  "/bin/bash",
				},
				"test-2": {
					Groups: []string{"baz", "quux"},
					Mode:   services.HostUserModeStatic,
					UID:    "1234",
					GID:    "5678",
					Shell:  "/bin/bash",
				},
			},
			wantSudoers: map[string][]string{
				"test-1": {"abcd1234"},
				"test-2": {"abcd1234"},
			},
		},
		{
			name: "ignore non-matching user",
			existingUsers: []*userprovisioningv2.StaticHostUser{
				makeStaticHostUser("ignore-me", map[string]string{"baz": "quux"}, []string{"foo", "bar"}),
			},
			sendEvents: sendEvents([]types.Event{
				{
					Type: types.OpInit,
				},
				{
					Type: types.OpPut,
					Resource: types.Resource153ToLegacy(
						makeStaticHostUser("ignore-me-too", map[string]string{"abc": "xyz"}, []string{"foo", "bar"}),
					),
				},
			}),
		},
		{
			name: "ignore multiple matches",
			existingUsers: []*userprovisioningv2.StaticHostUser{
				userprovisioning.NewStaticHostUser("test", &userprovisioningv2.StaticHostUserSpec{
					Matchers: []*userprovisioningv2.Matcher{
						{
							NodeLabels: []*labelv1.Label{
								{
									Name:   "foo",
									Values: []string{"bar"},
								},
							},
							Groups: []string{"foo", "bar"},
						},
						{
							NodeLabelsExpression: "labels.foo == 'bar'",
							Groups:               []string{"baz", "quux"},
						},
					},
				}),
			},
		},
		{
			name: "update user",
			existingUsers: []*userprovisioningv2.StaticHostUser{
				makeStaticHostUser("test", map[string]string{"foo": "bar"}, []string{"foo"}),
			},
			sendEvents: sendEvents([]types.Event{
				{
					Type: types.OpInit,
				},
				{
					Type: types.OpPut,
					Resource: types.Resource153ToLegacy(
						makeStaticHostUser("test", map[string]string{"foo": "bar"}, []string{"bar"}),
					),
				},
				// Delete events should be ignored.
				{
					Type: types.OpDelete,
					Resource: &types.ResourceHeader{
						Kind:     types.KindStaticHostUser,
						Version:  types.V2,
						Metadata: types.Metadata{Name: "test"},
					},
				},
			}),
			wantUsers: map[string]services.HostUsersInfo{
				"test": {
					Groups: []string{"bar"},
					Mode:   services.HostUserModeStatic,
					UID:    "1234",
					GID:    "5678",
					Shell:  "/bin/bash",
				},
			},
			wantSudoers: map[string][]string{
				"test": {"abcd1234"},
			},
		},
		{
			name: "restart on watcher init failure",
			sendEvents: func(ctx context.Context, events *mockEvents, clock *clockwork.FakeClock) error {
				// Wait until the handler is waiting for an init event.
				clock.BlockUntil(1)
				// Send a wrong event type first, which will cause the handler to fail and restart.
				select {
				case events.events <- types.Event{
					Type: types.OpPut,
					Resource: types.Resource153ToLegacy(
						makeStaticHostUser("test", map[string]string{"foo": "bar"}, []string{"foo"}),
					),
				}:
				case <-ctx.Done():
					return nil
				}

				// Even though the watcher timeout won't fire since the event
				// was received first, we still need to advance the clock for
				// it so we can guarantee that there are no waiters afterwards.
				clock.Advance(staticHostUserWatcherTimeout)
				// Advance past the retryer.
				clock.BlockUntil(1)
				clock.Advance(defaults.MaxWatcherBackoff)

				// Emit events as normal.
				return sendEvents([]types.Event{
					{
						Type: types.OpInit,
					},
					{
						Type: types.OpPut,
						Resource: types.Resource153ToLegacy(
							makeStaticHostUser("test", map[string]string{"foo": "bar"}, []string{"bar"}),
						),
					},
				})(ctx, events, clock)
			},
			wantUsers: map[string]services.HostUsersInfo{
				"test": {
					Groups: []string{"bar"},
					Mode:   services.HostUserModeStatic,
					UID:    "1234",
					GID:    "5678",
					Shell:  "/bin/bash",
				},
			},
			wantSudoers: map[string][]string{
				"test": {"abcd1234"},
			},
		},
		{
			name: "restart on watcher timeout failure",
			sendEvents: func(ctx context.Context, events *mockEvents, clock *clockwork.FakeClock) error {
				// Force a timeout on waiting for the init event.
				clock.BlockUntil(1)
				clock.Advance(staticHostUserWatcherTimeout)
				// Advance past the retryer.
				clock.BlockUntil(1)
				clock.Advance(defaults.MaxWatcherBackoff)
				// Once the handler re-watches, send events as usual.
				return sendEvents([]types.Event{
					{
						Type: types.OpInit,
					},
					{
						Type: types.OpPut,
						Resource: types.Resource153ToLegacy(
							makeStaticHostUser("test", map[string]string{"foo": "bar"}, []string{"bar"}),
						),
					},
				})(ctx, events, clock)
			},
			wantUsers: map[string]services.HostUsersInfo{
				"test": {
					Groups: []string{"bar"},
					Mode:   services.HostUserModeStatic,
					UID:    "1234",
					GID:    "5678",
					Shell:  "/bin/bash",
				},
			},
			wantSudoers: map[string][]string{
				"test": {"abcd1234"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Send just an init event to the watcher by default.
			if tc.sendEvents == nil {
				tc.sendEvents = sendEvents([]types.Event{{
					Type: types.OpInit,
				}})
			}

			events := newMockEvents()
			shu := &mockStaticHostUsers{hostUsers: tc.existingUsers}
			users := &mockHostUsers{}
			sudoers := &mockHostSudoers{}
			clock := clockwork.NewFakeClock()
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			utils.RunTestBackgroundTask(ctx, t, &utils.TestBackgroundTask{
				Name: "event sender",
				Task: func(ctx context.Context) error {
					return trace.Wrap(tc.sendEvents(ctx, events, clock))
				},
			})

			handler, err := NewStaticHostUserHandler(StaticHostUserHandlerConfig{
				Events:         events,
				StaticHostUser: shu,
				Server: mockInfoGetter{
					labels: map[string]string{"foo": "bar"},
				},
				Users:   users,
				Sudoers: sudoers,
				clock:   clock,
			})
			require.NoError(t, err)

			assert.NoError(t, handler.Run(ctx))
			assert.Equal(t, tc.wantUsers, users.upsertedUsers)
			assert.Equal(t, tc.wantSudoers, sudoers.sudoers)
		})
	}
}
