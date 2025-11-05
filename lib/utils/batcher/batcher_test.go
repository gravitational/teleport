// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package batcher_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/batcher"
)

func TestCollectBatch_TimeWindow(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	collector := batcher.New[string](
		batcher.WithWindow(100*time.Millisecond),
		batcher.WithClock(fakeClock),
		batcher.WithThreshold(10),
	)

	events := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var batch []string
	var err error

	done := make(chan bool)
	go func() {
		batch, err = collector.CollectBatch(ctx, events)
		done <- true
	}()

	events <- "event1"
	fakeClock.BlockUntil(1)

	events <- "event2"

	fakeClock.Advance(100 * time.Millisecond)

	<-done
	require.NoError(t, err)
	assert.Equal(t, []string{"event1", "event2"}, batch)
}

func TestCollectBatch_Threshold(t *testing.T) {
	collector := batcher.New[int](batcher.WithThreshold(3))

	events := make(chan int, 10)
	for i := 1; i <= 7; i++ {
		events <- i
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batch, err := collector.CollectBatch(ctx, events)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, batch)

	batch, err = collector.CollectBatch(ctx, events)
	require.NoError(t, err)
	assert.Equal(t, []int{4, 5, 6}, batch)
}

func TestCollectBatch_ChannelClosed(t *testing.T) {
	collector := batcher.New[string]()

	events := make(chan string, 2)
	events <- "event1"
	events <- "event2"
	close(events)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	batch, err := collector.CollectBatch(ctx, events)

	require.ErrorIs(t, err, io.EOF)
	assert.Equal(t, []string{"event1", "event2"}, batch)
}

func TestCollectBatch_ContextCanceled(t *testing.T) {
	collector := batcher.New[string](batcher.WithWindow(1 * time.Hour))

	events := make(chan string, 2)
	events <- "event1"

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := collector.CollectBatch(ctx, events)
	assert.Equal(t, context.Canceled, err)
}

func TestCollectBatch_EmptyChannel(t *testing.T) {
	fakeClock := clockwork.NewFakeClock()
	collector := batcher.New[string](
		batcher.WithWindow(100*time.Millisecond),
		batcher.WithClock(fakeClock),
	)

	events := make(chan string)
	close(events)

	ctx := context.Background()
	batch, err := collector.CollectBatch(ctx, events)

	require.ErrorIs(t, err, io.EOF)
	assert.Empty(t, batch)
}
