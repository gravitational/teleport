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

package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/auditqueue"
)

// fakePrimary mocks an audit backend and can be toggled to simulate failures.
type fakePrimary struct {
	mu   sync.Mutex
	fail bool
	got  []apievents.AuditEvent
}

func (f *fakePrimary) setFail(fail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = fail
}

func (f *fakePrimary) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.got)
}

func (f *fakePrimary) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return trace.ConnectionProblem(nil, "backend unavailable")
	}
	f.got = append(f.got, event)
	return nil
}

func newFallbackTestEmitter(t *testing.T, primary apievents.Emitter, enableQueue bool) *events.FallbackEmitter {
	t.Helper()
	e, err := events.NewFallbackEmitter(events.FallbackEmitterConfig{
		Primary:            primary,
		DataDir:            t.TempDir(),
		EnableAuditQueue:   enableQueue,
		AuditQueueBackends: []auditqueue.Kind{auditqueue.KindSQLiteMemory},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, e.Close()) })
	return e
}

func testEvent() apievents.AuditEvent {
	return &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserLocalLoginCode,
		},
	}
}

func TestFallbackEmitter_PrimarySucceeds(t *testing.T) {
	t.Parallel()
	primary := &fakePrimary{}
	e := newFallbackTestEmitter(t, primary, true)

	require.NoError(t, e.EmitAuditEvent(t.Context(), testEvent()))
	require.Equal(t, 1, primary.count())
}

func TestFallbackEmitter_QueuesAndRecovers(t *testing.T) {
	t.Parallel()
	primary := &fakePrimary{}
	primary.setFail(true)
	e := newFallbackTestEmitter(t, primary, true)

	// Backend is down. The event is queued, not lost, and emit reports success.
	require.NoError(t, e.EmitAuditEvent(t.Context(), testEvent()))
	require.Equal(t, 0, primary.count())

	// Backend recovers. The background consumer re-delivers the queued event.
	primary.setFail(false)
	require.Eventually(t, func() bool {
		return primary.count() == 1
	}, 5*time.Second, 50*time.Millisecond, "queued event should be re-delivered after recovery")
}

func TestFallbackEmitter_ForwardedNotQueued(t *testing.T) {
	t.Parallel()
	primary := &fakePrimary{}
	primary.setFail(true)
	e := newFallbackTestEmitter(t, primary, true)

	ctx := events.WithForwardedEmit(t.Context())
	require.Error(t, e.EmitAuditEvent(ctx, testEvent()), "forwarded event delivery error must propagate")

	// Even after the backend recovers, the forwarded event must never appear,
	// because it was never queued.
	primary.setFail(false)
	require.Never(t, func() bool {
		return primary.count() > 0
	}, time.Second, 50*time.Millisecond, "forwarded event must not be queued/re-delivered")
}

func TestFallbackEmitter_QueueDisabled(t *testing.T) {
	t.Parallel()
	primary := &fakePrimary{}
	primary.setFail(true)
	e := newFallbackTestEmitter(t, primary, false)

	require.Error(t, e.EmitAuditEvent(t.Context(), testEvent()))

	primary.setFail(false)
	require.NoError(t, e.EmitAuditEvent(t.Context(), testEvent()))
	require.Equal(t, 1, primary.count())
}

type authBoundary struct {
	fallback apievents.Emitter
}

func (a authBoundary) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return a.fallback.EmitAuditEvent(events.WithForwardedEmit(ctx), event)
}

func newQueuedAsyncEmitter(t *testing.T, inner apievents.Emitter) *events.AsyncEmitter {
	t.Helper()
	a, err := events.NewAsyncEmitter(events.AsyncEmitterConfig{
		Inner:            inner,
		DataDir:          t.TempDir(),
		EnableAuditQueue: true,
		// A short dead-letter sweep keeps recovery fast. If an event exhausts
		// its retries during the outage, the sweeper re-delivers it promptly
		// once the backend is healthy again.
		AuditQueueCfg: auditqueue.Config{
			MaxAttempts:             3,
			DeadLetterSweepInterval: 50 * time.Millisecond,
		},
		AuditQueueBackends: []auditqueue.Kind{auditqueue.KindSQLiteMemory},
	})
	require.NoError(t, err)
	return a
}

func TestForwardedEvent_RetriedByAgent(t *testing.T) {
	t.Parallel()
	backend := &fakePrimary{}
	backend.setFail(true)

	authFallback := newFallbackTestEmitter(t, backend, true)
	agent := newQueuedAsyncEmitter(t, authBoundary{fallback: authFallback})
	t.Cleanup(func() { require.NoError(t, agent.Close()) })

	require.NoError(t, agent.EmitAuditEvent(t.Context(), testEvent()))

	// Backend is down. The event is not delivered, but it is not lost. The
	// agent retries from its own queue.
	require.Never(t, func() bool {
		return backend.count() > 0
	}, 500*time.Millisecond, 50*time.Millisecond, "event must not be delivered while the backend is down")

	// Backend recovers. The agent's retry loop delivers the event end-to-end.
	backend.setFail(false)
	require.Eventually(t, func() bool {
		return backend.count() >= 1
	}, 5*time.Second, 50*time.Millisecond, "agent should re-deliver the forwarded event after recovery")
}

func TestForwardedEvent_NotAdoptedByAuth(t *testing.T) {
	t.Parallel()
	backend := &fakePrimary{}
	backend.setFail(true)

	authFallback := newFallbackTestEmitter(t, backend, true)
	agent := newQueuedAsyncEmitter(t, authBoundary{fallback: authFallback})

	require.NoError(t, agent.EmitAuditEvent(t.Context(), testEvent()))
	require.Never(t, func() bool {
		return backend.count() > 0
	}, 500*time.Millisecond, 50*time.Millisecond, "event must not be delivered while the backend is down")

	// Stop the only queue that owns the forwarded event while the backend is
	// still down.
	require.NoError(t, agent.Close())

	// The auth server never queued the forwarded event, so recovery delivers
	// nothing. It was the agent's responsibility and its queue is gone.
	backend.setFail(false)
	require.Never(t, func() bool {
		return backend.count() > 0
	}, time.Second, 50*time.Millisecond, "auth server must not have adopted the forwarded event")
}
