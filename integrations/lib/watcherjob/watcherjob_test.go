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

package watcherjob

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestSequential checks that events with the different resource names are being processed in parallel.
func TestConcurrent(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	config := Config{MaxConcurrency: 4}
	countdown := NewCountdown(config.MaxConcurrency)
	process := NewMockEventsProcess(ctx, t, config, func(ctx context.Context, event types.Event) error {
		defer countdown.Decrement()
		time.Sleep(time.Second)
		return trace.Wrap(ctx.Err())
	})

	timeBefore := time.Now()
	for i := 0; i < config.MaxConcurrency; i++ {
		resource, err := types.NewAccessRequest(fmt.Sprintf("REQ-%v", i+1), "foo", "admin")
		require.NoError(t, err)
		process.Events.Fire(types.Event{Type: types.OpPut, Resource: resource})
	}
	require.NoError(t, countdown.Wait(ctx))

	timeAfter := time.Now()
	assert.InDelta(t, time.Second, timeAfter.Sub(timeBefore), float64(500*time.Millisecond))
}

// TestSequential checks that events with the same resource name are being processed one by one (no races).
func TestSequential(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	config := Config{MaxConcurrency: 4}
	countdown := NewCountdown(config.MaxConcurrency)
	process := NewMockEventsProcess(ctx, t, config, func(ctx context.Context, event types.Event) error {
		defer countdown.Decrement()
		time.Sleep(time.Second)
		return trace.Wrap(ctx.Err())
	})

	timeBefore := time.Now()
	for i := 0; i < config.MaxConcurrency; i++ {
		resource, err := types.NewAccessRequest("REQ-SAME", "foo", "admin")
		require.NoError(t, err)
		process.Events.Fire(types.Event{Type: types.OpPut, Resource: resource})
	}
	require.NoError(t, countdown.Wait(ctx))

	timeAfter := time.Now()
	assert.InDelta(t, 4*time.Second, timeAfter.Sub(timeBefore), float64(750*time.Millisecond))
}

// TestConcurrencyLimit checks the case when the queue is full and there're incoming requests
func TestConcurrencyLimit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	config := Config{MaxConcurrency: 4}
	countdown := NewCountdown(config.MaxConcurrency * 2)
	process := NewMockEventsProcess(ctx, t, config, func(ctx context.Context, event types.Event) error {
		defer countdown.Decrement()
		time.Sleep(time.Second)
		return trace.Wrap(ctx.Err())
	})

	timeBefore := time.Now()
	for i := 0; i < config.MaxConcurrency; i++ {
		resource, err := types.NewAccessRequest(fmt.Sprintf("REQ-%v", i+1), "foo", "admin")
		require.NoError(t, err)

		for j := 0; j < 2; j++ {
			process.Events.Fire(types.Event{Type: types.OpPut, Resource: resource})
		}
	}
	require.NoError(t, countdown.Wait(ctx))

	timeAfter := time.Now()
	assert.InDelta(t, 4*time.Second, timeAfter.Sub(timeBefore), float64(750*time.Millisecond))
}

// TestNewJobWithConfirmedWatchKinds checks that the watch kinds are passed back after init.
func TestNewJobWithConfirmedWatchKinds(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	watchKinds := []types.WatchKind{
		{Kind: types.KindAccessRequest},
	}
	config := Config{
		MaxConcurrency: 4,
		Watch: types.Watch{
			Kinds: watchKinds,
		},
	}
	countdown := NewCountdown(config.MaxConcurrency)
	var acceptedWatchKinds []string
	onWatchInit := func(ws types.WatchStatus) {
		for _, watchKind := range ws.GetKinds() {
			acceptedWatchKinds = append(acceptedWatchKinds, watchKind.Kind)
		}
	}

	process := NewMockEventsProcessWithConfirmedWatchJobs(ctx, t, config,
		func(ctx context.Context, event types.Event) error {
			defer countdown.Decrement()
			time.Sleep(time.Second)
			return trace.Wrap(ctx.Err())
		}, onWatchInit)

	_, err := process.WaitReady(ctx)
	require.NoError(t, err)

	if !slices.ContainsFunc(acceptedWatchKinds, func(kind string) bool {
		return kind == types.KindAccessRequest
	}) {
		t.Error("access request watch kind not returned after init: %V", acceptedWatchKinds)
	}

	timeBefore := time.Now()
	for i := 0; i < config.MaxConcurrency; i++ {
		resource, err := types.NewAccessRequest("REQ-SAME", "foo", "admin")
		require.NoError(t, err)
		process.Events.Fire(types.Event{Type: types.OpPut, Resource: resource})
	}
	require.NoError(t, countdown.Wait(ctx))

	timeAfter := time.Now()
	assert.InDelta(t, 4*time.Second, timeAfter.Sub(timeBefore), float64(1000*time.Millisecond))
}
