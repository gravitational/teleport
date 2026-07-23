/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package events

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/auditqueue"
)

type recordingEmitter struct {
	mu           sync.Mutex
	singleEvents []string
	batchCalls   [][]string
	failIDs      map[string]bool
	batchErr     error
}

func (r *recordingEmitter) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failIDs[event.GetID()] {
		return trace.Errorf("emit failed for %q", event.GetID())
	}
	r.singleEvents = append(r.singleEvents, event.GetID())
	return nil
}

func (r *recordingEmitter) EmitAuditEvents(_ context.Context, events []apievents.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, len(events))
	for i, event := range events {
		ids[i] = event.GetID()
	}
	r.batchCalls = append(r.batchCalls, ids)
	return r.batchErr
}

// unaryEmitter only implements Emitter (not BatchEmitter), recording each event.
type unaryEmitter struct {
	mu     sync.Mutex
	events []string
}

func (u *unaryEmitter) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.events = append(u.events, event.GetID())
	return nil
}

func testBatch(ids ...string) auditqueue.Item {
	events := make([]apievents.AuditEvent, len(ids))
	for i, id := range ids {
		event := &apievents.UserLogin{}
		event.SetID(id)
		events[i] = event
	}
	return auditqueue.Item{Events: events}
}

func itemIDs(items []auditqueue.Item) []string {
	var ids []string
	for _, item := range items {
		for _, event := range item.Events {
			ids = append(ids, event.GetID())
		}
	}
	return ids
}

func TestAsyncEmitterDeliverBatchFastPath(t *testing.T) {
	inner := &recordingEmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	items := []auditqueue.Item{testBatch("a", "b", "c")}
	delivered := a.deliver(context.Background(), items)

	require.Equal(t, []string{"a", "b", "c"}, itemIDs(delivered), "all events should be delivered")
	require.Equal(t, [][]string{{"a", "b", "c"}}, inner.batchCalls, "the batch should be emitted as a single request")
	require.Empty(t, inner.singleEvents, "no per-event emits should happen on the fast path")
}

func TestAsyncEmitterDeliverBatchIsAtomic(t *testing.T) {
	inner := &recordingEmitter{
		batchErr: trace.Errorf("batch unavailable"),
		failIDs:  map[string]bool{"b": true},
	}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	items := []auditqueue.Item{testBatch("a", "b", "c")}
	delivered := a.deliver(t.Context(), items)

	require.Empty(t, delivered, "a batch with any failing event must not be acked")
	require.Len(t, inner.batchCalls, 2, "the aggregate request and the per-batch request should each be attempted before per-event fallback")
	require.Equal(t, []string{"a"}, inner.singleEvents, "fallback should stop at the first failing event")
}

func TestAsyncEmitterDeliverFallbackIndependentBatches(t *testing.T) {
	inner := &recordingEmitter{
		batchErr: trace.Errorf("batch unavailable"),
		failIDs:  map[string]bool{"b": true},
	}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	items := []auditqueue.Item{testBatch("a"), testBatch("b"), testBatch("c")}
	delivered := a.deliver(t.Context(), items)

	require.Equal(t, []string{"a", "c"}, itemIDs(delivered), "only batches whose events all delivered should be acked")
	require.Len(t, inner.batchCalls, 4, "one aggregate request, then each batch attempted as its own request")
	require.Equal(t, []string{"a", "c"}, inner.singleEvents, "fallback should emit each event individually")
}

func TestAsyncEmitterDeliverAggregatesRows(t *testing.T) {
	inner := &recordingEmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	items := []auditqueue.Item{testBatch("a"), testBatch("b"), testBatch("c")}
	delivered := a.deliver(t.Context(), items)

	require.Equal(t, []string{"a", "b", "c"}, itemIDs(delivered), "all batches should be acked")
	require.Equal(t, [][]string{{"a", "b", "c"}}, inner.batchCalls, "all batches should be delivered in a single request")
	require.Empty(t, inner.singleEvents, "no per-event emits should happen on the fast path")
}

func TestAsyncEmitterDeliverUnaryInner(t *testing.T) {
	inner := &unaryEmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	items := []auditqueue.Item{testBatch("a"), testBatch("b")}
	delivered := a.deliver(context.Background(), items)

	require.Equal(t, []string{"a", "b"}, itemIDs(delivered))
	require.Equal(t, []string{"a", "b"}, inner.events)
}

func TestMultiEmitterEmitAuditEvents(t *testing.T) {
	batchChild := &recordingEmitter{}
	unaryChild := &unaryEmitter{}
	multi := NewMultiEmitter(batchChild, unaryChild)

	var events []apievents.AuditEvent
	for _, id := range []string{"a", "b"} {
		event := &apievents.UserLogin{}
		event.SetID(id)
		events = append(events, event)
	}

	require.NoError(t, multi.EmitAuditEvents(context.Background(), events))
	require.Equal(t, [][]string{{"a", "b"}}, batchChild.batchCalls, "batch-capable child should get one batch call")
	require.Equal(t, []string{"a", "b"}, unaryChild.events, "unary child should get per-event calls")
}

func TestCheckingEmitterEmitAuditEvents(t *testing.T) {
	inner := &recordingEmitter{}
	emitter, err := NewCheckingEmitter(CheckingEmitterConfig{
		Inner:       inner,
		ClusterName: "test-cluster",
	})
	require.NoError(t, err)

	var batch []apievents.AuditEvent
	for _, id := range []string{"a", "b", "c"} {
		batch = append(batch, &apievents.UserLogin{
			Metadata: apievents.Metadata{
				ID:   id,
				Type: UserLoginEvent,
				Code: UserLocalLoginCode,
			},
		})
	}

	require.NoError(t, emitter.EmitAuditEvents(context.Background(), batch))
	require.Equal(t, [][]string{{"a", "b", "c"}}, inner.batchCalls, "events should reach the inner emitter as a single batch")
	require.Empty(t, inner.singleEvents, "no per-event emits should reach a batch-capable inner")
}

func TestEmitAuditEventsHelper(t *testing.T) {
	events := []apievents.AuditEvent{&apievents.UserLogin{}, &apievents.UserLogin{}}
	for i, event := range events {
		event.SetID(strconv.Itoa(i))
	}

	t.Run("batch-capable", func(t *testing.T) {
		inner := &recordingEmitter{}
		require.NoError(t, EmitAuditEvents(context.Background(), inner, events))
		require.Len(t, inner.batchCalls, 1)
		require.Empty(t, inner.singleEvents)
	})

	t.Run("unary", func(t *testing.T) {
		inner := &unaryEmitter{}
		require.NoError(t, EmitAuditEvents(context.Background(), inner, events))
		require.Equal(t, []string{"0", "1"}, inner.events)
	})
}
