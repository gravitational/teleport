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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
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

func TestStaticHostUserHandler(t *testing.T) {
	t.Parallel()

	sendEvents := func(ctx context.Context, events *mockEvents, eventList []types.Event) {
		for _, event := range eventList {
			select {
			case events.events <- event:
			case <-ctx.Done():
				break
			}
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
		name             string
		existingUsers    []*userprovisioningv2.StaticHostUser
		events           []types.Event
		onEventsFinished func(ctx context.Context, clock *clockwork.FakeClock)
		assert           assert.ErrorAssertionFunc
		wantUsers        map[string]services.HostUsersInfo
		wantSudoers      map[string][]string
	}{
		{
			name: "ok users",
			existingUsers: []*userprovisioningv2.StaticHostUser{
				makeStaticHostUser("test-1", map[string]string{"foo": "bar"}, []string{"foo", "bar"}),
			},
			events: []types.Event{
				{
					Type: types.OpInit,
				},
				{
					Type: types.OpPut,
					Resource: types.Resource153ToLegacy(
						makeStaticHostUser("test-2", map[string]string{"foo": "bar"}, []string{"baz", "quux"}),
					),
				},
			},
			assert: assert.NoError,
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
			events: []types.Event{
				{
					Type: types.OpInit,
				},
				{
					Type: types.OpPut,
					Resource: types.Resource153ToLegacy(
						makeStaticHostUser("ignore-me-too", map[string]string{"abc": "xyz"}, []string{"foo", "bar"}),
					),
				},
			},
			assert: assert.NoError,
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
			events: []types.Event{
				{
					Type: types.OpInit,
				},
			},
			assert: assert.NoError,
		},
		{
			name: "update user",
			existingUsers: []*userprovisioningv2.StaticHostUser{
				makeStaticHostUser("test", map[string]string{"foo": "bar"}, []string{"foo"}),
			},
			events: []types.Event{
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
			},
			assert: assert.NoError,
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
			name: "error on watcher init failure",
			events: []types.Event{
				{
					Type: types.OpPut,
					Resource: types.Resource153ToLegacy(
						makeStaticHostUser("test", map[string]string{"foo": "bar"}, []string{"foo"}),
					),
				},
			},
			assert: assert.Error,
		},
		{
			name: "error on watcher timeout failure",
			onEventsFinished: func(ctx context.Context, clock *clockwork.FakeClock) {
				clock.BlockUntilContext(ctx, 1)
				clock.Advance(staticHostUserWatcherTimeout)
				// Wait to close the watcher until the test is done.
				<-ctx.Done()
			},
			assert: assert.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
					sendEvents(ctx, events, tc.events)
					if tc.onEventsFinished != nil {
						tc.onEventsFinished(ctx, clock)
					}
					events.Close()
					<-ctx.Done()
					return nil
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

			tc.assert(t, handler.run(ctx))
			if tc.wantUsers != nil {
				assert.Equal(t, tc.wantUsers, users.upsertedUsers)
			} else {
				assert.Empty(t, users.upsertedUsers)
			}
			if tc.wantSudoers != nil {
				assert.Equal(t, tc.wantSudoers, sudoers.sudoers)
			} else {
				assert.Empty(t, sudoers.sudoers)
			}
		})
	}
}
