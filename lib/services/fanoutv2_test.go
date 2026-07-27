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

package services

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	scopedaccessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/v1"
	"github.com/gravitational/teleport/api/types"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"
)

// TestFanoutV2Init verifies that Init event is sent exactly once.
func TestFanoutV2Init(t *testing.T) {
	ctx := t.Context()

	f := NewFanoutV2(FanoutV2Config{})

	w, err := f.NewWatcher(ctx, types.Watch{
		Name:  "test",
		Kinds: []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
	})
	require.NoError(t, err)

	f.SetInit([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}})

	select {
	case e := <-w.Events():
		require.Equal(t, types.OpInit, e.Type)
		require.Equal(t, types.NewWatchStatus([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}}), e.Resource)
	case <-time.After(time.Second * 10):
		t.Fatalf("Expected init event")
	}

	select {
	case e := <-w.Events():
		t.Fatalf("Unexpected second event: %+v", e)
	case <-time.After(time.Millisecond * 200):
	}
}

func TestFanoutV2StreamFiltering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	f := NewFanoutV2(FanoutV2Config{})

	stream := f.NewStream(ctx, types.Watch{
		Name:  "test",
		Kinds: []types.WatchKind{{Kind: "spam"}},
	})

	f.SetInit([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}})

	require.True(t, stream.Next())
	require.Equal(t, types.OpInit, stream.Item().Type)
	require.Equal(t, types.NewWatchStatus([]types.WatchKind{{Kind: "spam"}}), stream.Item().Resource)

	put := func(kind string) {
		f.Emit(types.Event{Type: types.OpPut, Resource: &types.ResourceHeader{Kind: kind}})
	}

	put("spam")
	put("eggs")
	put("spam")

	require.True(t, stream.Next())
	require.Equal(t, "spam", stream.Item().Resource.GetKind())

	require.True(t, stream.Next())
	require.Equal(t, "spam", stream.Item().Resource.GetKind())

	require.NoError(t, stream.Done())
}

// TestFanoutV2ScopeFiltering verifies that a watch kind's scope filter selects which scoped events a
// stream receives, that it applies to delete events (which carry scope) as well as puts, and that a
// stream with no scope filter receives every event for the kind.
func TestFanoutV2ScopeFiltering(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	scopedRole := func(scope string) types.Resource {
		return types.Resource153ToLegacy(scopedaccessv1.ScopedRole_builder{
			Kind:     scopedaccess.KindScopedRole,
			Metadata: headerv1.Metadata_builder{Name: "role"}.Build(),
			Scope:    scope,
		}.Build())
	}

	scopeOf := func(r types.Resource) string {
		scoped, ok := r.(interface{ GetScope() string })
		require.True(t, ok, "resource %T does not expose a scope", r)
		return scoped.GetScope()
	}

	f := NewFanoutV2(FanoutV2Config{})

	// a stream that watches only the exact scope /foo.
	exact := f.NewStream(ctx, types.Watch{
		Name: "exact",
		Kinds: []types.WatchKind{{
			Kind:        scopedaccess.KindScopedRole,
			ScopeFilter: types.ScopeFilterFromProto(scopesv1.Filter_builder{Scope: "/foo", Mode: scopesv1.Mode_MODE_EXACT}.Build()),
		}},
	})

	// a stream with no scope filter, which should receive every event for the kind.
	all := f.NewStream(ctx, types.Watch{
		Name:  "all",
		Kinds: []types.WatchKind{{Kind: scopedaccess.KindScopedRole}},
	})

	f.SetInit([]types.WatchKind{{Kind: scopedaccess.KindScopedRole}})

	require.True(t, exact.Next())
	require.Equal(t, types.OpInit, exact.Item().Type)
	require.True(t, all.Next())
	require.Equal(t, types.OpInit, all.Item().Type)

	events := []struct {
		op    types.OpType
		scope string
	}{
		{types.OpPut, "/foo"},
		{types.OpPut, "/bar"},
		{types.OpPut, "/foo/sub"},
		{types.OpDelete, "/foo"},
		{types.OpDelete, "/bar"},
	}
	for _, e := range events {
		f.Emit(types.Event{Type: e.op, Resource: scopedRole(e.scope)})
	}

	// the exact-scope stream sees only the two /foo events (put and delete), in order.
	for _, want := range []struct {
		op    types.OpType
		scope string
	}{
		{types.OpPut, "/foo"},
		{types.OpDelete, "/foo"},
	} {
		require.True(t, exact.Next())
		require.Equal(t, want.op, exact.Item().Type)
		require.Equal(t, want.scope, scopeOf(exact.Item().Resource))
	}
	require.NoError(t, exact.Done())

	// the unfiltered stream sees every event, in order.
	for _, want := range events {
		require.True(t, all.Next())
		require.Equal(t, want.op, all.Item().Type)
		require.Equal(t, want.scope, scopeOf(all.Item().Resource))
	}
	require.NoError(t, all.Done())
}

func TestFanoutV2StreamOrdering(t *testing.T) {
	const streams = 100
	const events = 400
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	f := NewFanoutV2(FanoutV2Config{})

	f.SetInit([]types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}})

	put := func(kind string) {
		f.Emit(types.Event{Type: types.OpPut, Resource: &types.ResourceHeader{Kind: kind}})
	}

	results := make(chan []string, streams)

	var inputs []string
	for range events {
		kind := "spam"
		if rand.N(2) == 0 {
			kind = "eggs"
		}
		inputs = append(inputs, kind)
	}

	for range streams {
		stream := f.NewStream(ctx, types.Watch{
			Name:  "test",
			Kinds: []types.WatchKind{{Kind: "spam"}, {Kind: "eggs"}},
		})

		require.True(t, stream.Next())
		require.Equal(t, types.OpInit, stream.Item().Type)

		go func() {
			defer stream.Done()

			var kinds []string
			for stream.Next() {
				kinds = append(kinds, stream.Item().Resource.GetKind())
				if len(kinds) == events {
					break
				}
			}

			results <- kinds
		}()
	}

	for _, k := range inputs {
		put(k)
	}

	for range 100 {
		require.Equal(t, inputs, <-results)
	}
}
