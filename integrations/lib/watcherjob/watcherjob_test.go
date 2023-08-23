// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package watcherjob

import (
	"context"
	"fmt"
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
