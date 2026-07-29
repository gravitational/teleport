// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package role_bench

import (
	"context"
	"fmt"
	"sync"
	"testing"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	googleproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cache"
	"github.com/gravitational/teleport/lib/services"
)

type mockAccess struct {
	services.Access

	listRoles []types.Role
}

func (m *mockAccess) GetRoles(context.Context) ([]types.Role, error) {
	if m.listRoles == nil {
		panic("GetRoles called twice")
	}
	l := m.listRoles
	m.listRoles = nil
	return l, nil
}

func BenchmarkCachedRoles(b *testing.B) {
	var wg sync.WaitGroup
	defer wg.Wait()
	eventsC := make(chan cache.Event, 1024)
	eventsDone := make(chan struct{})
	defer close(eventsDone)
	wg.Go(func() {
		for {
			var e cache.Event
			select {
			case <-eventsDone:
				return
			case e = <-eventsC:
			}

			if e.Type == cache.Reloading {
				panic("a cache failed")
			}
		}
	})

	for _, roleCount := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("role_count=%d", roleCount), func(b *testing.B) {
			ctx := b.Context()

			access := &mockAccess{
				listRoles: []types.Role{},
			}
			for i := range roleCount {
				r, err := types.NewRole(fmt.Sprintf("role-bench-%06d", i), types.RoleSpecV6{
					Options: types.RoleOptions{
						DesktopClipboard: &types.BoolOption{Value: true},
					},
					Allow: types.RoleConditions{
						NodeLabels: types.Labels{
							"foo":  []string{"bar", "baz"},
							"foo2": []string{"qux"},
							"foo3": nil,
						},
					},
					Deny: types.RoleConditions{},
				})
				require.NoError(b, err)
				access.listRoles = append(access.listRoles, r)
			}

			c, err := cache.New(cache.Config{
				Context: ctx,
				Watches: []types.WatchKind{
					{
						Kind: types.KindRole,
					},
				},
				Events:  noopEvents{},
				Access:  access,
				EventsC: eventsC,
			})
			require.NoError(b, err)
			defer c.Close()

			<-c.FirstInit()

			b.Run("op=getroles", func(b *testing.B) {
				ctx := b.Context()

				for b.Loop() {
					_, err := c.GetRoles(ctx)
					require.NoError(b, err)
				}
			})

			b.Run("op=listroles", func(b *testing.B) {
				var listRolesForGRPC func(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
				if cg, _ := any(c).(interface {
					ListRolesForGRPC(ctx context.Context, req *proto.ListRolesRequest) (*proto.ListRolesResponse, error)
				}); cg != nil {
					listRolesForGRPC = cg.ListRolesForGRPC
				} else {
					listRolesForGRPC = c.ListRoles
				}

				marshalers := []struct {
					name string
					fn   func(t testing.TB, resp *proto.ListRolesResponse)
				}{
					{"none", func(testing.TB, *proto.ListRolesResponse) {}},
					{"grpcgo", func(t testing.TB, resp *proto.ListRolesResponse) {
						// default grpc-go proto codec

						respv2 := protoadapt.MessageV2Of(resp)
						_ = googleproto.Size(respv2)

						_, err := googleproto.MarshalOptions{UseCachedSize: true}.Marshal(respv2)
						require.NoError(t, err)
					}},
					{"gogo", func(t testing.TB, resp *proto.ListRolesResponse) {
						// a hypothetical proto codec that uses gogoproto

						_, err := gogoproto.Marshal(resp)
						require.NoError(t, err)
					}},
				}

				for _, m := range marshalers {
					b.Run("marshal="+m.name, func(b *testing.B) {
						ctx := b.Context()

						for b.Loop() {
							req := new(proto.ListRolesRequest)
							for {
								resp, err := listRolesForGRPC(ctx, req)
								require.NoError(b, err)

								m.fn(b, resp)

								if resp.GetNextKey() == "" {
									break
								}

								req = &proto.ListRolesRequest{
									StartKey: resp.GetNextKey(),
								}
							}
						}
					})
				}
			})
		})
	}
}

type noopEvents struct{}

// NewWatcher implements [types.Events].
func (m noopEvents) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	events := make(chan types.Event, 1)
	events <- types.Event{
		Type:     types.OpInit,
		Resource: nil,
	}
	done := make(chan struct{})
	return &mockWatcher{events: events, done: done}, nil
}

type mockWatcher struct {
	events chan (types.Event)
	done   chan struct{}
	once   sync.Once
}

// Close implements [types.Watcher].
func (m *mockWatcher) Close() error {
	m.once.Do(func() {
		close(m.done)
	})
	return nil
}

// Done implements [types.Watcher].
func (m *mockWatcher) Done() <-chan struct{} {
	return m.done
}

// Error implements [types.Watcher].
func (m *mockWatcher) Error() error {
	return nil
}

// Events implements [types.Watcher].
func (m *mockWatcher) Events() <-chan types.Event {
	return m.events
}
