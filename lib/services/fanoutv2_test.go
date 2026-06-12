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

	"github.com/gravitational/teleport/api/types"
)

// TestFanoutV2Init verifies that Init event is sent exactly once.
func TestFanoutV2Init(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	for i := 0; i < events; i++ {
		kind := "spam"
		if rand.N(2) == 0 {
			kind = "eggs"
		}
		inputs = append(inputs, kind)
	}

	for i := 0; i < streams; i++ {
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

	for i := 0; i < 100; i++ {
		require.Equal(t, inputs, <-results)
	}
}
