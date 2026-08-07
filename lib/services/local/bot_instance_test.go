/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/services"
)

// newBotInstance creates (but does not insert) a bot instance that is ready for
// insertion into the backend. If any modifier functions are provided, they will
// be executed on the instance before it is returned.
func newBotInstance(botName string, fns ...func(*machineidv1.BotInstance)) *machineidv1.BotInstance {
	id := uuid.New()

	bi := machineidv1.BotInstance_builder{
		Kind:    types.KindBotInstance,
		Version: types.V1,
		Spec: machineidv1.BotInstanceSpec_builder{
			BotName:    botName,
			InstanceId: id.String(),
		}.Build(),
		Status: &machineidv1.BotInstanceStatus{},
	}.Build()

	for _, fn := range fns {
		fn(bi)
	}

	return bi
}

// withBotInstanceInvalidMetadata modifies a BotInstance such that it should
// raise an error during an insert attempt.
func withBotInstanceInvalidMetadata() func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		bi.SetMetadata(headerv1.Metadata_builder{
			Name: "invalid",
		}.Build())
	}
}

// withBotInstanceExpiry sets the .Metadata.Expires field of a bot instance to
// the given timestamp.
func withBotInstanceExpiry(expiry time.Time) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if !bi.HasMetadata() {
			bi.SetMetadata(&headerv1.Metadata{})
		}

		bi.GetMetadata().SetExpires(timestamppb.New(expiry))
	}
}

// withBotInstanceScope sets the scope of a bot instance to the given value.
func withBotInstanceScope(scope string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		bi.SetScope(scope)
	}
}

// withBotInstanceId sets the .Spec.InstanceId field of a bot instance to
// the given value.
func withBotInstanceId(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if !bi.HasSpec() {
			bi.SetSpec(&machineidv1.BotInstanceSpec{})
		}

		bi.GetSpec().SetInstanceId(value)
	}
}

// withBotInstanceHeartbeatJoinMethod sets the .Status.InitialHeartbeat.JoinMethod
// field of a bot instance to the given value.
func withBotInstanceHeartbeatJoinMethod(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if !bi.HasStatus() {
			bi.SetStatus(&machineidv1.BotInstanceStatus{})
		}

		if !bi.GetStatus().HasInitialHeartbeat() {
			bi.GetStatus().SetInitialHeartbeat(&machineidv1.BotInstanceStatusHeartbeat{})
		}

		bi.GetStatus().GetInitialHeartbeat().SetJoinMethod(value)
	}
}

// withBotInstanceHeartbeatVersion sets the .Status.InitialHeartbeat.Version
// field of a bot instance to the given value.
func withBotInstanceHeartbeatVersion(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if !bi.HasStatus() {
			bi.SetStatus(&machineidv1.BotInstanceStatus{})
		}

		if !bi.GetStatus().HasInitialHeartbeat() {
			bi.GetStatus().SetInitialHeartbeat(&machineidv1.BotInstanceStatusHeartbeat{})
		}

		bi.GetStatus().GetInitialHeartbeat().SetVersion(value)
	}
}

// withBotInstanceHeartbeatHostname sets the .Status.InitialHeartbeat.Hostname
// field of a bot instance to the given value.
func withBotInstanceHeartbeatHostname(value string) func(*machineidv1.BotInstance) {
	return func(bi *machineidv1.BotInstance) {
		if !bi.HasStatus() {
			bi.SetStatus(&machineidv1.BotInstanceStatus{})
		}

		if !bi.GetStatus().HasInitialHeartbeat() {
			bi.GetStatus().SetInitialHeartbeat(&machineidv1.BotInstanceStatusHeartbeat{})
		}

		bi.GetStatus().GetInitialHeartbeat().SetHostname(value)
	}
}

// listInstances fetches all instances from the BotInstanceService matching the botName filter
func listInstances(t *testing.T, ctx context.Context, service *BotInstanceService, options *services.ListBotInstancesRequestOptions) []*machineidv1.BotInstance {
	t.Helper()

	var resources []*machineidv1.BotInstance
	var bis []*machineidv1.BotInstance
	var nextKey string
	var err error

	for {
		bis, nextKey, err = service.ListBotInstances(ctx, 0, nextKey, options)
		require.NoError(t, err)

		resources = append(resources, bis...)

		if nextKey == "" {
			break
		}
	}

	return resources
}

// TestBotInstanceCreateMetadata ensures bot instance metadata is constructed
// correctly when a new bot instance is inserted into the backend.
func TestBotInstanceCreateMetadata(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()

	tests := []struct {
		name        string
		instance    *machineidv1.BotInstance
		assertError require.ErrorAssertionFunc
		assertValue require.ValueAssertionFunc
	}{
		{
			name:        "non-nil metadata",
			instance:    newBotInstance("foo", withBotInstanceInvalidMetadata()),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i any, _ ...any) {
				bi, ok := i.(*machineidv1.BotInstance)
				require.True(t, ok)

				// .Metadata.Name should be overwritten with the correct value
				require.Equal(t, bi.GetSpec().GetInstanceId(), bi.GetMetadata().GetName())
				require.Nil(t, bi.GetMetadata().GetExpires())
			},
		},
		{
			name:        "valid without expiry",
			instance:    newBotInstance("foo"),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i any, _ ...any) {
				bi, ok := i.(*machineidv1.BotInstance)
				require.True(t, ok)

				require.Equal(t, bi.GetSpec().GetInstanceId(), bi.GetMetadata().GetName())
				require.Nil(t, bi.GetMetadata().GetExpires())
			},
		},
		{
			name:        "valid with expiry",
			instance:    newBotInstance("foo", withBotInstanceExpiry(clock.Now().Add(time.Hour))),
			assertError: require.NoError,
			assertValue: func(t require.TestingT, i any, _ ...any) {
				bi, ok := i.(*machineidv1.BotInstance)
				require.True(t, ok)

				require.Equal(t, bi.GetSpec().GetInstanceId(), bi.GetMetadata().GetName())
				require.Equal(t, clock.Now().Add(time.Hour).UTC(), bi.GetMetadata().GetExpires().AsTime())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()

			mem, err := memory.New(memory.Config{
				Context: ctx,
				Clock:   clock,
			})
			require.NoError(t, err)

			service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
			require.NoError(t, err)

			value, err := service.CreateBotInstance(ctx, tc.instance)
			tc.assertError(t, err)
			tc.assertValue(t, value)
		})
	}
}

// TestBotInstanceInvalidGetters ensures proper behavior for an invalid
// GetBotInstance call.
func TestBotInstanceInvalidGetters(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	_, err = service.CreateBotInstance(ctx, newBotInstance("example"))
	require.NoError(t, err)

	_, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "", BotName: "example", InstanceId: "invalid"}.Build())
	require.True(t, trace.IsNotFound(err))
}

// TestBotInstanceCRUD tests backend CRUD functionality for the bot instance
// service.
func TestBotInstanceCRUD(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	bi := newBotInstance("example")
	patched, err := service.CreateBotInstance(ctx, bi)
	require.NoError(t, err)

	// metadata should be generated from the bot spec
	require.Equal(t, bi.GetSpec().GetInstanceId(), patched.GetMetadata().GetName())

	// we should be able to retrieve a matching instance
	bi2, err := service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "", BotName: bi.GetSpec().GetBotName(), InstanceId: bi.GetSpec().GetInstanceId()}.Build())
	require.NoError(t, err)
	require.EqualExportedValues(t, patched, bi2)
	require.Equal(t, bi.GetMetadata().GetName(), bi2.GetMetadata().GetName())

	resources := listInstances(t, ctx, service, &services.ListBotInstancesRequestOptions{
		FilterBotName: "example",
	})

	require.Len(t, resources, 1, "must list only 1 bot instance")
	require.EqualExportedValues(t, patched, resources[0])

	// append a heartbeat to a stored instance
	heartbeat := machineidv1.BotInstanceStatusHeartbeat_builder{
		Hostname: "foo",
	}.Build()

	patched, err = service.PatchBotInstance(ctx, services.PatchBotInstanceOpts{
		Bot:        scopes.QualifiedName{Name: bi.GetSpec().GetBotName()},
		InstanceID: bi.GetSpec().GetInstanceId(),
		UpdateFn: func(bi *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
			bi.GetStatus().SetLatestHeartbeats(append([]*machineidv1.BotInstanceStatusHeartbeat{heartbeat}, bi.GetStatus().GetLatestHeartbeats()...))
			return bi, nil
		},
	})
	require.NoError(t, err)

	require.Len(t, patched.GetStatus().GetLatestHeartbeats(), 1)
	require.EqualExportedValues(t, heartbeat, patched.GetStatus().GetLatestHeartbeats()[0])

	// delete the stored instance
	require.NoError(t, service.DeleteBotInstance(ctx, machineidv1.DeleteBotInstanceRequest_builder{BotScope: "", BotName: bi.GetSpec().GetBotName(), InstanceId: bi.GetSpec().GetInstanceId()}.Build()))

	// subsequent delete attempts should fail
	require.Error(t, service.DeleteBotInstance(ctx, machineidv1.DeleteBotInstanceRequest_builder{BotScope: "", BotName: bi.GetSpec().GetBotName(), InstanceId: bi.GetSpec().GetInstanceId()}.Build()))
}

// listInstanceSpec declaratively describes a bot instance for
// TestBotInstanceList cases.
type listInstanceSpec struct {
	scope, botName, id            string
	hostname, version, joinMethod string
}

func (s listInstanceSpec) build() *machineidv1.BotInstance {
	fns := []func(*machineidv1.BotInstance){withBotInstanceId(s.id)}
	if s.scope != "" {
		fns = append(fns, withBotInstanceScope(s.scope))
	}
	if s.hostname != "" {
		fns = append(fns, withBotInstanceHeartbeatHostname(s.hostname))
	}
	if s.version != "" {
		fns = append(fns, withBotInstanceHeartbeatVersion(s.version))
	}
	if s.joinMethod != "" {
		fns = append(fns, withBotInstanceHeartbeatJoinMethod(s.joinMethod))
	}
	return newBotInstance(s.botName, fns...)
}

// TestBotInstanceList exercises ListBotInstances across filtering and sorting.
func TestBotInstanceList(t *testing.T) {
	t.Parallel()

	botInstanceIDs := func(instances []*machineidv1.BotInstance) []string {
		out := make([]string, 0, len(instances))
		for _, b := range instances {
			out = append(out, b.GetSpec().GetInstanceId())
		}
		return out
	}

	// Two unscoped bots and a same-named scoped bot, to prove bot filters are
	// scope-strict.
	botFilterInstances := []listInstanceSpec{
		{botName: "db", id: "db-a"},
		{botName: "web", id: "web-a"},
		{botName: "web", id: "web-b"},
		{scope: "/prod", botName: "web", id: "web-c"},
	}

	// Instance IDs name their scope, so expectations read as selected scopes.
	scopeFilterInstances := []listInstanceSpec{
		{botName: "u", id: "u"},
		{scope: "/foo", botName: "f", id: "foo"},
		{scope: "/foo/sub", botName: "fs", id: "foo-sub"},
		{scope: "/bar", botName: "b", id: "bar"},
	}

	filterFnInstances := []listInstanceSpec{
		{botName: "bot", id: "i-0"},
		{botName: "bot", id: "i-1"},
		{scope: "/s", botName: "bot", id: "i-2"},
		{scope: "/s", botName: "bot", id: "i-3"},
	}
	evenFilter := func(b *machineidv1.BotInstance) bool {
		id := b.GetSpec().GetInstanceId()
		return (id[len(id)-1]-'0')%2 == 0
	}

	tests := []struct {
		name      string
		instances []listInstanceSpec
		opts      *services.ListBotInstancesRequestOptions
		want      []string
		wantErr   string
	}{
		{
			name:      "no filter spans both ranges, unscoped instances first",
			instances: botFilterInstances,
			want:      []string{"db-a", "web-a", "web-b", "web-c"},
		},
		{
			name:      "bot name filter matches only the unscoped bot of that name",
			instances: botFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{FilterBotName: "web"},
			want:      []string{"web-a", "web-b"},
		},
		{
			name:      "bot name and scope filter matches the scoped bot",
			instances: botFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{FilterBotName: "web", FilterBotScope: "/prod"},
			want:      []string{"web-c"},
		},
		{
			// A bot scope only qualifies a bot name; standalone scope
			// selection is ScopeFilter.
			name:      "bot scope without a bot name filter is rejected",
			instances: botFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{FilterBotScope: "/prod"},
			wantErr:   "bot scope filter requires a bot name filter",
		},
		{
			name:      "scope filter mode ALL matches every scope",
			instances: scopeFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build()},
			want:      []string{"u", "bar", "foo", "foo-sub"},
		},
		{
			// Unscoped is orthogonal to every scoped value, not the root.
			name:      "scope filter mode UNSCOPED matches only unscoped instances",
			instances: scopeFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build()},
			want:      []string{"u"},
		},
		{
			name:      "scope filter mode EXACT matches one scope",
			instances: scopeFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT, Scope: "/foo"}.Build()},
			want:      []string{"foo"},
		},
		{
			name:      "scope filter mode DESCENDANTS includes the scope and below",
			instances: scopeFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_DESCENDANTS, Scope: "/foo"}.Build()},
			want:      []string{"foo", "foo-sub"},
		},
		{
			name:      "scope filter mode ANCESTORS includes the scope and above",
			instances: scopeFilterInstances,
			opts:      &services.ListBotInstancesRequestOptions{ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ANCESTORS, Scope: "/foo/sub"}.Build()},
			want:      []string{"foo", "foo-sub"},
		},
		{
			name:      "malformed scope filter is rejected",
			instances: scopeFilterInstances,
			// EXACT requires a scope.
			opts:    &services.ListBotInstancesRequestOptions{ScopeFilter: scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_EXACT}.Build()},
			wantErr: "requires a non-empty scope",
		},
		{
			name:      "scope filter with a bot filter is rejected",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				FilterBotName:  "f",
				FilterBotScope: "/foo",
				ScopeFilter:    scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_UNSCOPED}.Build(),
			},
			wantErr: "scope filter cannot be combined with a bot name filter",
		},
		{
			// Rejected even though it would be harmless as a predicate, so the
			// rule is one a caller can state without knowing the modes.
			name:      "scope filter mode ALL with a bot filter is rejected",
			instances: scopeFilterInstances,
			opts: &services.ListBotInstancesRequestOptions{
				FilterBotName:  "f",
				FilterBotScope: "/foo",
				ScopeFilter:    scopesv1.Filter_builder{Mode: scopesv1.Mode_MODE_ALL}.Build(),
			},
			wantErr: "scope filter cannot be combined with a bot name filter",
		},
		{
			name: "search matches bot name",
			instances: []listInstanceSpec{
				{botName: "this-is-nicks-test-bot", id: "match"},
				{botName: "bot-not-matched", id: "decoy"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "nick"},
			want: []string{"match"},
		},
		{
			name: "search matches instance id",
			instances: []listInstanceSpec{
				{botName: "test-bot", id: "cb2c3523-01f6-4258-966b-ace9f38f9862"},
				{botName: "bot-not-matched", id: "decoy"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "CB2C352"},
			want: []string{"cb2c3523-01f6-4258-966b-ace9f38f9862"},
		},
		{
			name: "search matches join method",
			instances: []listInstanceSpec{
				{botName: "test-bot", id: "match", joinMethod: "kubernetes"},
				{botName: "bot-not-matched", id: "decoy"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "uber"},
			want: []string{"match"},
		},
		{
			name: "search matches version",
			instances: []listInstanceSpec{
				{botName: "test-bot", id: "match", version: "1.0.0-dev-a2g3hd"},
				{botName: "bot-not-matched", id: "decoy"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "1.0.0"},
			want: []string{"match"},
		},
		{
			name: "search matches version with v prefix",
			instances: []listInstanceSpec{
				{botName: "test-bot", id: "match", version: "1.0.0-dev-a2g3hd"},
				{botName: "bot-not-matched", id: "decoy"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "v1.0.0"},
			want: []string{"match"},
		},
		{
			name: "search matches hostname",
			instances: []listInstanceSpec{
				{botName: "test-bot", id: "match", hostname: "svr-eu-tel-123-a"},
				{botName: "bot-not-matched", id: "decoy"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "tel-123"},
			want: []string{"match"},
		},
		{
			name: "search matches across scopes",
			instances: []listInstanceSpec{
				{botName: "runner", id: "r1", hostname: "host-match"},
				{scope: "/foo", botName: "runner", id: "r2", hostname: "host-match"},
				{botName: "runner", id: "r3", hostname: "host-other"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterSearchTerm: "host-match"},
			want: []string{"r1", "r2"},
		},
		{
			name: "predicate query matches",
			instances: []listInstanceSpec{
				{botName: "test-bot", id: "match", hostname: "svr-eu-tel-123-a"},
				{botName: "bot-not-matched", id: "decoy"},
			},
			opts: &services.ListBotInstancesRequestOptions{FilterQuery: `status.latest_heartbeat.hostname == "svr-eu-tel-123-a"`},
			want: []string{"match"},
		},
		{
			name:    "unsupported sort field is rejected",
			opts:    &services.ListBotInstancesRequestOptions{SortField: "test_field"},
			wantErr: `unsupported sort, only bot_name field is supported, but got "test_field"`,
		},
		{
			name:    "descending sort is rejected",
			opts:    &services.ListBotInstancesRequestOptions{SortField: "bot_name", SortDesc: true},
			wantErr: "unsupported sort, only ascending order is supported",
		},
		{
			name:      "filter fn spans both ranges",
			instances: filterFnInstances,
			opts:      &services.ListBotInstancesRequestOptions{FilterFn: evenFilter},
			want:      []string{"i-0", "i-2"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			clock := clockwork.NewFakeClock()

			mem, err := memory.New(memory.Config{
				Context: ctx,
				Clock:   clock,
			})
			require.NoError(t, err)

			service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
			require.NoError(t, err)

			for _, s := range tc.instances {
				_, err := service.CreateBotInstance(ctx, s.build())
				require.NoError(t, err)
			}

			got, _, err := service.ListBotInstances(ctx, 0, "", tc.opts)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, botInstanceIDs(got))

			// Page-size-1 pagination must agree with the single-page listing;
			// cursors that don't line up across the unscoped/scoped ranges
			// would skip entries.
			var paged []*machineidv1.BotInstance
			var token string
			for {
				page, next, err := service.ListBotInstances(ctx, 1, token, tc.opts)
				require.NoError(t, err)
				paged = append(paged, page...)
				if next == "" {
					break
				}
				token = next
			}
			require.Equal(t, tc.want, botInstanceIDs(paged))
		})
	}
}

// TestBotInstanceScopedCoexistence verifies that instances of same-named bots
// in different scopes (and unscoped) are stored disjointly and are only
// addressable under their own bot's scope.
func TestBotInstanceScopedCoexistence(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service, err := NewBotInstanceService(backend.NewSanitizer(mem), clock)
	require.NoError(t, err)

	// Three bots named "x": unscoped, in /foo, and in /bar.
	unscoped := newBotInstance("x")
	foo := newBotInstance("x", withBotInstanceScope("/foo"))
	bar := newBotInstance("x", withBotInstanceScope("/bar"))
	for _, bi := range []*machineidv1.BotInstance{unscoped, foo, bar} {
		_, err := service.CreateBotInstance(ctx, bi)
		require.NoError(t, err)
	}

	// The scoped instance is stored in the scope-namespaced key range.
	encodedScope, err := scopes.EncodeForKey("/foo")
	require.NoError(t, err)
	_, err = mem.Get(ctx, backend.NewKey(scopedPrefix, botInstancePrefix, encodedScope, "x", foo.GetSpec().GetInstanceId()))
	require.NoError(t, err)

	// Each instance resolves only under its own bot's scope.
	got, err := service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "", BotName: "x", InstanceId: unscoped.GetSpec().GetInstanceId()}.Build())
	require.NoError(t, err)
	require.Empty(t, got.GetScope())
	got, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "/foo", BotName: "x", InstanceId: foo.GetSpec().GetInstanceId()}.Build())
	require.NoError(t, err)
	require.Equal(t, "/foo", got.GetScope())

	// The wrong (or missing) scope does not resolve.
	_, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "", BotName: "x", InstanceId: foo.GetSpec().GetInstanceId()}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	_, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "/bar", BotName: "x", InstanceId: foo.GetSpec().GetInstanceId()}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	_, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "/foo", BotName: "x", InstanceId: unscoped.GetSpec().GetInstanceId()}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)

	// Patching routes by scope, and the scope itself cannot be patched.
	patched, err := service.PatchBotInstance(ctx, services.PatchBotInstanceOpts{
		Bot:        scopes.QualifiedName{Scope: "/foo", Name: "x"},
		InstanceID: foo.GetSpec().GetInstanceId(),
		UpdateFn: func(bi *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
			bi.GetStatus().SetLatestHeartbeats([]*machineidv1.BotInstanceStatusHeartbeat{
				machineidv1.BotInstanceStatusHeartbeat_builder{Hostname: "scoped-host"}.Build(),
			})
			return bi, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "/foo", patched.GetScope())

	_, err = service.PatchBotInstance(ctx, services.PatchBotInstanceOpts{
		Bot:        scopes.QualifiedName{Scope: "/foo", Name: "x"},
		InstanceID: foo.GetSpec().GetInstanceId(),
		UpdateFn: func(bi *machineidv1.BotInstance) (*machineidv1.BotInstance, error) {
			bi.SetScope("/other")
			return bi, nil
		},
	})
	require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	require.ErrorContains(t, err, "scope: cannot be patched")

	// Deleting under the wrong scope does not delete; the right scope deletes
	// only that bot's instance.
	err = service.DeleteBotInstance(ctx, machineidv1.DeleteBotInstanceRequest_builder{BotScope: "/bar", BotName: "x", InstanceId: foo.GetSpec().GetInstanceId()}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	require.NoError(t, service.DeleteBotInstance(ctx, machineidv1.DeleteBotInstanceRequest_builder{BotScope: "/foo", BotName: "x", InstanceId: foo.GetSpec().GetInstanceId()}.Build()))
	_, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "/foo", BotName: "x", InstanceId: foo.GetSpec().GetInstanceId()}.Build())
	require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	_, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "", BotName: "x", InstanceId: unscoped.GetSpec().GetInstanceId()}.Build())
	require.NoError(t, err)
	_, err = service.GetBotInstance(ctx, machineidv1.GetBotInstanceRequest_builder{BotScope: "/bar", BotName: "x", InstanceId: bar.GetSpec().GetInstanceId()}.Build())
	require.NoError(t, err)

	// DeleteAllBotInstances empties both the unscoped and scoped ranges.
	require.NoError(t, service.DeleteAllBotInstances(ctx))
	remaining := listInstances(t, ctx, service, nil)
	require.Empty(t, remaining)
}
