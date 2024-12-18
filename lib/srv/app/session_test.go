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
	"github.com/gravitational/teleport/lib/utils"
)

func newSessionChunk(timeout time.Duration) *sessionChunk {
	return &sessionChunk{
		id:           uuid.New().String(),
		closeC:       make(chan struct{}),
		inflightCond: sync.NewCond(&sync.Mutex{}),
		closeTimeout: timeout,
		log:          utils.NewSlogLoggerForTests(),
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
