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

package utils

import (
	"context"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchReadChannelWaitsForFirstMessage(t *testing.T) {
	// ctx is a context that will cancel at the end of the test. Any spawned
	// goroutines must exit when this context expires
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// GIVEN a buffered channel
	ch := make(chan int, 10)
	t.Cleanup(func() { close(ch) })

	// GIVEN a consumer process that reads a single batch of messages from the
	// channel
	var msgCount atomic.Int32
	go func() {
		for _, ok := range BatchReadChannel(ctx, ch, math.MaxInt32) {
			if !ok {
				return
			}
			msgCount.Add(1)
		}
	}()

	// WHEN I write a message to the channel after an arbitrary delay
	go func() {
		select {
		case <-ctx.Done():
			return

		case <-time.After(500 * time.Millisecond):
			ch <- 1
		}
	}()

	// EXPECT that the message was received in the single batch that was read
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			assert.Equal(c, int32(1), msgCount.Load())
		},
		1*time.Second,
		10*time.Millisecond)
}

func TestReadBatchHonorsSizeLimit(t *testing.T) {
	const (
		msgCount  = 100
		batchSize = 11
	)

	// GIVEN a large buffered channel full of messages
	ch := make(chan int, msgCount)
	t.Cleanup(func() { close(ch) })
	for i := range msgCount {
		ch <- i
	}

	// WHEN I attempt to read a batch of messages, where the maximum batch size
	// is less than the number of pending items in the channel
	count := 0
	for _, ok := range BatchReadChannel(context.Background(), ch, batchSize) {
		require.True(t, ok)
		count++
	}

	require.Equal(t, batchSize, count)
}

func TestBatchReadChannelDetectsClose(t *testing.T) {
	const channelCapacity = 5

	type producer struct {
		ctx     context.Context
		cancel  context.CancelFunc
		ch      chan int
		closeCh bool
	}

	testCases := []struct {
		name     string
		msgCount int

		// closer is a function that performs some action that should cause the
		// message reader to stop, wither by closing the channel, cancelling the
		// context or by some other as-yet-undefined mechanism
		closer func(*producer)

		// tolerance defines how many messages we are allowed to miss before the
		// test fails. In tests where we cancel the context to end the test we
		// may miss the last few messages if the cancellation is detected before
		// the final messages are read, so we allow those tests to miss some
		// messages.
		tolerance int
	}{
		{
			name: "empty channel",
			closer: func(p *producer) {
				p.closeCh = false
				close(p.ch)
			},
		},
		{
			name:     "non-empty channel",
			msgCount: 101,
			closer: func(p *producer) {
				p.closeCh = false
				close(p.ch)
			},
		},
		{
			name: "context with empty channel",
			closer: func(p *producer) {
				p.cancel()
			},
		},
		{
			name:     "context with non-empty channel",
			msgCount: 101,
			closer: func(p *producer) {
				p.cancel()
			},
			tolerance: channelCapacity,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// ctx is a context that will cancel at the end of the test. Any spawned
			// goroutines must exit when this context expires
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)

			// GIVEN an asynchronous producer process that writes messages into
			// a channel and then does something to indicate that there will be
			// no more messages...
			p := producer{
				ch:      make(chan int, channelCapacity),
				closeCh: true,
			}
			p.ctx, p.cancel = context.WithCancel(context.Background())
			t.Cleanup(func() {
				p.cancel()
				if p.closeCh {
					close(p.ch)
				}
			})
			go func() {
				for i := range test.msgCount {
					select {
					case p.ch <- i:
						continue
					case <-ctx.Done():
						return
					}
				}
				test.closer(&p)
			}()

			// WHEN I run an asynchronous consumer process that logs all the
			// messages until it receives a close signal via a non-OK read
			var msgCount atomic.Int32
			var closeDetected atomic.Bool
			go func() {
				for ctx.Err() == nil {
					for _, ok := range BatchReadChannel(p.ctx, p.ch, math.MaxInt32) {
						// A non-OK value indicates that the channel was closed or the
						// context expired
						if !ok {
							closeDetected.Store(true)
							return
						}
						msgCount.Add(1)
					}
				}
			}()

			// EXPECT that all messages were read from the channel and that the explicit
			// close signal was detected.
			require.EventuallyWithT(t,
				func(c *assert.CollectT) {
					require.True(c, closeDetected.Load())

					// in cases where we cancel the context, we may miss the last
					// few messages if the cancellation is detected before the
					// final messages are read, hence the tolerance
					require.GreaterOrEqual(t, msgCount.Load(), int32(test.msgCount-test.tolerance))
				},
				5*time.Second,
				10*time.Millisecond)
		})
	}
}
