//go:build bpf && !386

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package bpf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDrainQueue(t *testing.T) {
	t.Parallel()

	var q drainQueue

	b, ok := q.enqueue()
	require.True(t, ok)
	require.NotNil(t, b)

	// takePending returns the registered barriers and clears them.
	pending, keepGoing := q.takePending()
	require.True(t, keepGoing)
	require.Len(t, pending, 1)
	require.Equal(t, b, pending[0])

	pending, keepGoing = q.takePending()
	require.True(t, keepGoing)
	require.Empty(t, pending)

	// Once closed, nothing is registered and the reader is told to stop.
	q.close()

	_, ok = q.enqueue()
	require.False(t, ok)

	_, keepGoing = q.takePending()
	require.False(t, keepGoing)
}

// TestDrainPipelines checks that drainPipelines blocks until every barrier it
// registered is signaled, as sendEvents and the emit goroutine do at runtime.
func TestDrainPipelines(t *testing.T) {
	t.Parallel()

	var q1, q2 drainQueue

	flushed := make(chan struct{}, 1)
	flush := func() error {
		// Stand in for the reader and emitter: take the pending barriers and
		// close them. drainPipelines hangs (test timeout) if it doesn't wait.
		go func() {
			for _, q := range []*drainQueue{&q1, &q2} {
				barriers, _ := q.takePending()
				for _, b := range barriers {
					close(b)
				}
			}
		}()
		flushed <- struct{}{}
		return nil
	}

	drainPipelines("test", flush, &q1, &q2)

	require.Len(t, flushed, 1, "flush should have been called exactly once")
}

// TestDrainPipelinesSkipsClosed checks that draining a closed pipeline is a
// no-op: nothing is registered and the ring buffer is not flushed.
func TestDrainPipelinesSkipsClosed(t *testing.T) {
	t.Parallel()

	var q drainQueue
	q.close()

	flushCalled := false
	drainPipelines("test", func() error {
		flushCalled = true
		return nil
	}, &q)

	require.False(t, flushCalled, "flush must not run when there is nothing to drain")
}
