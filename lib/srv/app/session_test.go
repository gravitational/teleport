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

package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func newSessionChunk(timeout time.Duration) *sessionChunk {
	return &sessionChunk{
		id:           uuid.New().String(),
		closeC:       make(chan struct{}),
		inflightCond: sync.NewCond(&sync.Mutex{}),
		closeTimeout: timeout,
		log:          logtest.NewLogger(),
		streamCloser: events.NewDiscardRecorder(),
	}
}

func TestSessionChunkCloseNormal(t *testing.T) {
	const timeout = 1 * time.Second

	sess := newSessionChunk(timeout)

	err := sess.acquire()
	require.NoError(t, err)

	go sess.close(context.Background())
	sess.release()

	select {
	case <-time.After(timeout / 2):
		require.FailNow(t, "session chunk did not close, despite all requests ending")
	case <-sess.closeC:
	}
}

func TestSessionChunkCloseTimeout(t *testing.T) {
	const timeout = 1 * time.Second

	start := time.Now()
	sess := newSessionChunk(timeout)

	err := sess.acquire() // this is never released
	require.NoError(t, err)

	go sess.close(context.Background())

	select {
	case <-time.After(timeout * 2):
		require.FailNow(t, "session chunk did not close, despite timeout elapsing")
	case <-sess.closeC:
	}

	elapsed := time.Since(start)
	require.Greater(t, elapsed, timeout, "expected to wait at least %s before closing, waited only %s", timeout, elapsed)
}

func TestSessionChunkCannotAcquireAfterClosing(t *testing.T) {
	const timeout = 1 * time.Second
	sess := newSessionChunk(timeout)
	sess.close(context.Background())
	require.Error(t, sess.acquire())
}

// TestMaxActiveSessionChunksDefault verifies that CheckAndSetDefaults
// sets MaxActiveSessionChunks to DefaultMaxActiveSessionChunks when
// the caller leaves it at zero. Without this default the semaphore
// would have zero capacity and reject every session chunk.
func TestMaxActiveSessionChunksDefault(t *testing.T) {
	cfg := &ConnectionsHandlerConfig{MaxActiveSessionChunks: 0}
	// CheckAndSetDefaults will fail on other missing fields, but
	// MaxActiveSessionChunks is set before any error return.
	_ = cfg.CheckAndSetDefaults()
	require.Equal(t, DefaultMaxActiveSessionChunks, cfg.MaxActiveSessionChunks)
}

// TestSessionChunkSemaphore verifies that the session chunk semaphore
// limits the number of concurrently open recording streams per agent.
// Each chunk holds an open stream whose memory footprint is dominated
// by the ProtoStreamer upload buffer (~7 MiB). Without a bound, an
// agent handling many concurrent app sessions can accumulate hundreds
// of open streams, exhausting memory.
//
// The chunkSem is a buffered channel shared across all chunks on one
// agent. A slot is acquired in newSessionChunk before the recording
// stream is opened (returning LimitExceeded when full) and released
// in close after the stream shuts down.
func TestSessionChunkSemaphore(t *testing.T) {
	t.Run("close releases semaphore slot", func(t *testing.T) {
		sem := make(chan struct{}, 2)
		sess := newSessionChunk(time.Second)
		sess.chunkSem = sem

		// Simulate what newSessionChunk does: acquire a slot.
		sem <- struct{}{}
		require.Len(t, sem, 1, "slot should be occupied after creation")

		sess.close(context.Background())
		require.Empty(t, sem, "close should release the semaphore slot")
	})

	t.Run("close releases semaphore even with inflight requests", func(t *testing.T) {
		// Simulate: a request acquired the chunk (inflight=1), then
		// close() force-closed it (setting inflight to -1) because
		// the closeTimeout elapsed. close() must still drain the
		// semaphore slot so the agent does not leak slots.
		sem := make(chan struct{}, 2)
		sess := newSessionChunk(100 * time.Millisecond)
		sess.chunkSem = sem

		// Simulate what newSessionChunk does.
		sem <- struct{}{}

		require.NoError(t, sess.acquire())
		require.Len(t, sem, 1, "semaphore should still have one slot from creation")

		// Force-close: the chunk's closeTimeout is 100ms and we
		// never release, so close() will force inflight to -1.
		sess.close(context.Background())
		require.Empty(t, sem, "close must drain the semaphore even when force-closing with in-flight requests, otherwise the slot leaks")
	})
}
