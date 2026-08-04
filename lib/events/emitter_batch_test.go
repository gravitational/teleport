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
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	recordingencryptionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
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

func sealedItem() auditqueue.Item {
	return auditqueue.Item{
		Payload:    []byte("ciphertext"),
		Format:     auditqueue.FormatAgeV1,
		EventCount: 1,
	}
}

type fakeSealedSubmitter struct {
	mu       sync.Mutex
	requests []*recordingencryptionv1pb.SubmitAuditQueueBatchRequest
	err      error
}

func (f *fakeSealedSubmitter) SubmitAuditQueueBatch(_ context.Context, req *recordingencryptionv1pb.SubmitAuditQueueBatchRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.requests = append(f.requests, req)
	return nil
}

func (f *fakeSealedSubmitter) submitted() []*recordingencryptionv1pb.SubmitAuditQueueBatchRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests)
}

func TestAsyncEmitterDeliverSubmitsSealed(t *testing.T) {
	inner := &recordingEmitter{}
	submitter := &fakeSealedSubmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner, SealedSubmitter: submitter})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	sealed := sealedItem()
	items := []auditqueue.Item{testBatch("a"), sealed, testBatch("b")}
	delivered := a.deliver(t.Context(), items)

	require.Len(t, delivered, 3, "plaintext and sealed items must all be acked")
	require.Equal(t, []string{"a", "b"}, itemIDs(delivered))
	require.Equal(t, [][]string{{"a", "b"}}, inner.batchCalls, "the inner emitter must only see plaintext events")

	submitted := submitter.submitted()
	require.Len(t, submitted, 1)
	batches := submitted[0].GetBatches()
	require.Len(t, batches, 1)
	require.Equal(t, sealed.Payload, batches[0].GetPayload())
	require.Equal(t, recordingencryptionv1pb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1, batches[0].GetFormat())
	require.Equal(t, int64(sealed.EventCount), batches[0].GetEventCount())
}

func TestAsyncEmitterDeliverSealedSubmitFailure(t *testing.T) {
	inner := &recordingEmitter{}
	submitter := &fakeSealedSubmitter{err: trace.Errorf("auth unavailable")}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner, SealedSubmitter: submitter})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	items := []auditqueue.Item{testBatch("a"), sealedItem()}
	delivered := a.deliver(t.Context(), items)

	require.Equal(t, []string{"a"}, itemIDs(delivered), "plaintext delivery must not be affected by sealed submit failures")
	for _, item := range delivered {
		require.False(t, item.Sealed(), "a failed sealed item must be nacked")
	}
	require.Empty(t, submitter.submitted())
}

func TestAsyncEmitterSubmitSealedChunkMixedFormats(t *testing.T) {
	inner := &recordingEmitter{}
	submitter := &fakeSealedSubmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner, SealedSubmitter: submitter})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	good := auditqueue.Item{Payload: []byte("good-ciphertext"), Format: auditqueue.FormatAgeV1, EventCount: 1}
	unknown := auditqueue.Item{Payload: []byte("unknown-format-ciphertext"), Format: 7, EventCount: 1}

	delivered := a.deliver(t.Context(), []auditqueue.Item{unknown, good})

	require.Len(t, delivered, 1)
	require.Equal(t, good.Payload, delivered[0].Payload)
	require.Equal(t, auditqueue.FormatAgeV1, delivered[0].Format)

	submitted := submitter.submitted()
	require.Len(t, submitted, 1)
	batches := submitted[0].GetBatches()
	require.Len(t, batches, 1)
	require.Equal(t, good.Payload, batches[0].GetPayload())
}

func TestAsyncEmitterDeliverSealedUnknownFormat(t *testing.T) {
	inner := &recordingEmitter{}
	submitter := &fakeSealedSubmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner, SealedSubmitter: submitter})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	unknown := auditqueue.Item{Payload: []byte("ciphertext"), Format: 7, EventCount: 1}
	delivered := a.deliver(t.Context(), []auditqueue.Item{unknown})

	require.Empty(t, delivered, "an unknown format must be nacked, never submitted")
	require.Empty(t, submitter.submitted())
}

func TestAsyncEmitterDeliverSkipsSealedItems(t *testing.T) {
	inner := &recordingEmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	items := []auditqueue.Item{testBatch("a"), sealedItem(), testBatch("b")}
	delivered := a.deliver(t.Context(), items)

	require.Equal(t, []string{"a", "b"}, itemIDs(delivered), "plaintext batches should still be delivered")
	for _, item := range delivered {
		require.False(t, item.Sealed(), "sealed items must never be reported as delivered")
	}
	require.Equal(t, [][]string{{"a", "b"}}, inner.batchCalls, "sealed items must not reach the inner emitter or poison the batch path")
	require.Empty(t, inner.singleEvents)
}

func TestAsyncEmitterDeliverAllSealed(t *testing.T) {
	inner := &recordingEmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{Inner: inner})
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	delivered := a.deliver(t.Context(), []auditqueue.Item{sealedItem(), sealedItem()})

	require.Empty(t, delivered, "sealed items must be nacked so the queue retries them")
	require.Empty(t, inner.batchCalls, "the inner emitter must never see sealed payloads")
	require.Empty(t, inner.singleEvents)
}

type fakeOpener struct {
	events []apievents.AuditEvent
	err    error
}

func (f fakeOpener) DecryptBatch(_ context.Context, _ []byte) ([]apievents.AuditEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.events, nil
}

func TestFallbackEmitterDeliverOpensSealed(t *testing.T) {
	inner := &recordingEmitter{}
	opened := testBatch("x", "y")
	f := &FallbackEmitter{cfg: FallbackEmitterConfig{Primary: inner, Opener: fakeOpener{events: opened.Events}}, deliveryTimeout: auditqueue.DefaultDeliveryTimeout}

	items := []auditqueue.Item{testBatch("a"), sealedItem()}
	delivered := f.deliver(t.Context(), items)

	require.Len(t, delivered, 2, "plaintext and decrypted sealed items must all be acked")
	require.Equal(t, []string{"a", "x", "y"}, inner.singleEvents,
		"the primary must receive the plaintext events and the decrypted sealed events")
}

func TestFallbackEmitterDeliverOpenerFailure(t *testing.T) {
	inner := &recordingEmitter{}
	f := &FallbackEmitter{cfg: FallbackEmitterConfig{Primary: inner, Opener: fakeOpener{err: trace.Errorf("no decryption key")}}, deliveryTimeout: auditqueue.DefaultDeliveryTimeout}

	items := []auditqueue.Item{testBatch("a"), sealedItem()}
	delivered := f.deliver(t.Context(), items)

	require.Equal(t, []string{"a"}, itemIDs(delivered), "plaintext delivery must not be affected by decrypt failures")
	for _, item := range delivered {
		require.False(t, item.Sealed(), "an undecryptable sealed item must be nacked")
	}
	require.Equal(t, []string{"a"}, inner.singleEvents)
}

func TestFallbackEmitterDeliverSkipsSealedItems(t *testing.T) {
	inner := &recordingEmitter{}
	f := &FallbackEmitter{cfg: FallbackEmitterConfig{Primary: inner}, deliveryTimeout: auditqueue.DefaultDeliveryTimeout}

	items := []auditqueue.Item{testBatch("a"), sealedItem(), testBatch("b")}
	delivered := f.deliver(t.Context(), items)

	require.Equal(t, []string{"a", "b"}, itemIDs(delivered), "plaintext batches should still be delivered")
	for _, item := range delivered {
		require.False(t, item.Sealed(), "sealed items must never be reported as delivered")
	}
	require.Equal(t, []string{"a", "b"}, inner.singleEvents, "the primary emitter must only see plaintext events")
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
