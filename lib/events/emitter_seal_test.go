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

package events

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	recordingencryptionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events/auditqueue"
)

type failingSealer struct {
	closed atomic.Bool
}

func (s *failingSealer) Seal(_ context.Context, _ []byte) ([]byte, bool, error) {
	return nil, false, trace.Errorf("sealer has no keys")
}

func (s *failingSealer) Close() error {
	s.closed.Store(true)
	return nil
}

type roundTripWatcher struct {
	events chan types.Event
	done   chan struct{}
	once   sync.Once
}

func (w *roundTripWatcher) Events() <-chan types.Event { return w.events }
func (w *roundTripWatcher) Done() <-chan struct{}      { return w.done }
func (w *roundTripWatcher) Error() error               { return nil }
func (w *roundTripWatcher) Close() error {
	w.once.Do(func() { close(w.done) })
	return nil
}

type roundTripSRCWatcher struct {
	src types.SessionRecordingConfig
}

func (s *roundTripSRCWatcher) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return s.src, nil
}

func (s *roundTripSRCWatcher) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	w := &roundTripWatcher{
		events: make(chan types.Event, 1),
		done:   make(chan struct{}),
	}
	w.events <- types.Event{Type: types.OpInit}
	return w, nil
}

type roundTripUnwrapper struct {
	key *rsa.PrivateKey
}

func (u roundTripUnwrapper) UnwrapKey(ctx context.Context, in recordingencryption.UnwrapInput) ([]byte, error) {
	fileKey, err := u.key.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	return fileKey, trace.Wrap(err)
}

type openerSubmitter struct {
	opener *recordingencryption.AuditQueueOpener
	mu     sync.Mutex
	events []apievents.AuditEvent
}

func (s *openerSubmitter) SubmitAuditQueueBatch(ctx context.Context, req *recordingencryptionv1pb.SubmitAuditQueueBatchRequest) error {
	for _, sealed := range req.GetBatches() {
		if sealed.GetFormat() != recordingencryptionv1pb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1 {
			return trace.BadParameter("unexpected format %v", sealed.GetFormat())
		}
		batch, err := s.opener.DecryptBatch(ctx, sealed.GetPayload())
		if err != nil {
			return trace.Wrap(err)
		}
		s.mu.Lock()
		s.events = append(s.events, batch...)
		s.mu.Unlock()
	}
	return nil
}

func (s *openerSubmitter) received() []apievents.AuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]apievents.AuditEvent(nil), s.events...)
}

func TestAsyncEmitterSealedDeliveryRoundTrip(t *testing.T) {
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA4096)
	require.NoError(t, err)
	key, ok := signer.(*rsa.PrivateKey)
	require.True(t, ok)
	pubDER, err := x509.MarshalPKIXPublicKey(key.Public())
	require.NoError(t, err)

	src := &types.SessionRecordingConfigV2{
		Spec: types.SessionRecordingConfigSpecV2{
			Encryption: &types.SessionRecordingEncryptionConfig{Enabled: true},
		},
	}
	src.SetEncryptionKeys(func(yield func(*types.AgeEncryptionKey) bool) {
		yield(&types.AgeEncryptionKey{PublicKey: pubDER})
	})

	sealer, err := recordingencryption.NewAuditQueueSealer(t.Context(), &roundTripSRCWatcher{src: src})
	require.NoError(t, err)

	opener, err := recordingencryption.NewAuditQueueOpener(roundTripUnwrapper{key: key})
	require.NoError(t, err)
	submitter := &openerSubmitter{opener: opener}

	inner := &unaryEmitter{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{
		Inner:            inner,
		DataDir:          t.TempDir(),
		EnableAuditQueue: true,
		Sealer:           sealer,
		SealedSubmitter:  submitter,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	const n = 5
	sent := make([]apievents.AuditEvent, 0, n)
	for i := range n {
		event := &apievents.UserLogin{
			Metadata: apievents.Metadata{
				Index: int64(i),
				Type:  "user.login",
				ID:    "event-" + string(rune('a'+i)),
				Code:  "T1000I",
			},
		}
		sent = append(sent, event)
		require.NoError(t, a.EmitAuditEvent(t.Context(), event))
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Len(t, submitter.received(), n)
	}, 10*time.Second, 10*time.Millisecond)

	require.ElementsMatch(t, sent, submitter.received(), "events must survive seal, SQLite, fetch, submit, and decrypt unchanged; sealed batches are submitted concurrently so cross-batch order is not guaranteed")
	inner.mu.Lock()
	defer inner.mu.Unlock()
	require.Empty(t, inner.events, "ciphertext must never enter the parsed-event emit path")
}

type flappingPrimary struct {
	failing atomic.Bool
	mu      sync.Mutex
	events  []apievents.AuditEvent
}

func (p *flappingPrimary) EmitAuditEvent(_ context.Context, event apievents.AuditEvent) error {
	if p.failing.Load() {
		return trace.Errorf("audit backend down")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
	return nil
}

func (p *flappingPrimary) received() []apievents.AuditEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]apievents.AuditEvent(nil), p.events...)
}

func TestFallbackEmitterSealedRoundTrip(t *testing.T) {
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA4096)
	require.NoError(t, err)
	key, ok := signer.(*rsa.PrivateKey)
	require.True(t, ok)
	pubDER, err := x509.MarshalPKIXPublicKey(key.Public())
	require.NoError(t, err)

	src := &types.SessionRecordingConfigV2{
		Spec: types.SessionRecordingConfigSpecV2{
			Encryption: &types.SessionRecordingEncryptionConfig{Enabled: true},
		},
	}
	src.SetEncryptionKeys(func(yield func(*types.AgeEncryptionKey) bool) {
		yield(&types.AgeEncryptionKey{PublicKey: pubDER})
	})

	sealer, err := recordingencryption.NewLazyAuditQueueSealer(t.Context(), &roundTripSRCWatcher{src: src})
	require.NoError(t, err)
	opener, err := recordingencryption.NewAuditQueueOpener(roundTripUnwrapper{key: key})
	require.NoError(t, err)

	primary := &flappingPrimary{}
	primary.failing.Store(true)

	f, err := NewFallbackEmitter(FallbackEmitterConfig{
		Primary:          primary,
		DataDir:          t.TempDir(),
		EnableAuditQueue: true,
		Sealer:           sealer,
		Opener:           opener,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })

	const n = 3
	sent := make([]apievents.AuditEvent, 0, n)
	for i := range n {
		event := &apievents.UserLogin{
			Metadata: apievents.Metadata{
				Index: int64(i),
				Type:  "user.login",
				ID:    "fallback-" + string(rune('a'+i)),
				Code:  "T1000I",
			},
		}
		sent = append(sent, event)
		require.NoError(t, f.EmitAuditEvent(t.Context(), event),
			"a primary failure must divert the event to the fallback queue, not fail the emit")
	}

	stats, err := f.Stats(t.Context())
	require.NoError(t, err)
	require.Equal(t, int64(n), stats.PendingCount+stats.DeadLetterCount,
		"all events must be durably queued while the primary is down")
	require.Empty(t, primary.received())

	primary.failing.Store(false)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Len(t, primary.received(), n)
	}, 10*time.Second, 10*time.Millisecond)

	require.Equal(t, sent, primary.received(),
		"events must survive seal, SQLite, decrypt-on-deliver, and re-emit unchanged and in order")
}

func TestAsyncEmitterThreadsSealerIntoQueue(t *testing.T) {
	sealer := &failingSealer{}
	a, err := NewAsyncEmitter(AsyncEmitterConfig{
		Inner:            &unaryEmitter{},
		DataDir:          t.TempDir(),
		EnableAuditQueue: true,
		Sealer:           sealer,
	})
	require.NoError(t, err)

	event := &apievents.UserLogin{}
	event.SetID("a")
	err = a.EmitAuditEvent(t.Context(), event)
	require.ErrorContains(t, err, "sealer has no keys")

	require.NoError(t, a.Close())
	require.True(t, sealer.closed.Load(), "emitter must close the sealer it owns")
}

type barrierSubmitter struct {
	mu       sync.Mutex
	inflight int
	peak     int
	need     int
	release  chan struct{}
}

func (s *barrierSubmitter) SubmitAuditQueueBatch(ctx context.Context, _ *recordingencryptionv1pb.SubmitAuditQueueBatchRequest) error {
	s.mu.Lock()
	s.inflight++
	if s.inflight > s.peak {
		s.peak = s.inflight
	}
	if s.inflight == s.need {
		close(s.release)
	}
	s.mu.Unlock()

	select {
	case <-s.release:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.mu.Lock()
	s.inflight--
	s.mu.Unlock()
	return nil
}

func TestAsyncEmitterSealedDeliveryConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	const need = sealedSubmitConcurrency
	require.Greater(t, need, 1, "this test cannot observe concurrency with a submit limit of 1")
	sub := &barrierSubmitter{need: need, release: make(chan struct{})}
	a := &AsyncEmitter{cfg: AsyncEmitterConfig{SealedSubmitter: sub}}

	items := make([]auditqueue.Item, need*sealedRowsPerRequest)
	for i := range items {
		items[i] = auditqueue.Item{
			Payload:    []byte("sealed"),
			Format:     auditqueue.FormatAgeV1,
			EventCount: 1,
		}
	}

	delivered := a.deliverSealed(ctx, items)
	require.Len(t, delivered, len(items),
		"all sealed items must be delivered; a serial implementation deadlocks on the barrier and times out")
	require.GreaterOrEqual(t, sub.peak, need)
}
