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

package auditqueue

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

var sealPrefix = []byte("sealed:")

type prefixSealer struct{}

func (prefixSealer) Seal(_ context.Context, plaintext []byte) ([]byte, bool, error) {
	return slices.Concat(sealPrefix, plaintext), true, nil
}

func (prefixSealer) Close() error { return nil }

type passthroughSealer struct{}

func (passthroughSealer) Seal(_ context.Context, plaintext []byte) ([]byte, bool, error) {
	return plaintext, false, nil
}

func (passthroughSealer) Close() error { return nil }

type errorSealer struct{}

func (errorSealer) Seal(_ context.Context, _ []byte) ([]byte, bool, error) {
	return nil, false, trace.Errorf("keys unavailable")
}

func (errorSealer) Close() error { return nil }

func newSealedTestQueue(t *testing.T, sealer Sealer) *sqliteQueue {
	t.Helper()
	q, err := newSQLiteQueue(Config{
		Path:   filepath.Join(t.TempDir(), queueDir),
		Sealer: sealer,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, q.Close()) })
	return q
}

func TestSeal_SealedRowsAreStoredSealed(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	q := newSealedTestQueue(t, prefixSealer{})

	require.NoError(t, q.Enqueue(newTestEvent(1)))
	require.NoError(t, q.Enqueue(newTestEvent(2)))

	var (
		payload    []byte
		format     int
		eventCount int
		enqueuedAt int64
	)
	require.NoError(t, q.db.QueryRow(
		"SELECT payload, format, event_count, enqueued_at FROM audit_queue ORDER BY id ASC LIMIT 1").
		Scan(&payload, &format, &eventCount, &enqueuedAt))
	require.Equal(t, FormatAgeV1, format)
	require.Equal(t, 1, eventCount)
	require.NotZero(t, enqueuedAt)
	require.True(t, bytes.HasPrefix(payload, sealPrefix), "stored payload should be the sealed bytes")

	stats, err := q.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), stats.PendingCount)
	require.Zero(t, stats.DeadLetterCount)

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 2)
	for _, item := range items {
		require.True(t, item.Sealed())
		require.Equal(t, FormatAgeV1, item.Format)
		require.Equal(t, 1, item.EventCount)
		require.Empty(t, item.Events)
		require.True(t, bytes.HasPrefix(item.Payload, sealPrefix),
			"fetched payload should be the sealed bytes")
	}
	require.Equal(t, 2, countRows(t, q, auditQueueTable),
		"sealed rows stay queued until acked, they must not be quarantined")
	require.Zero(t, countRows(t, q, corruptEventsTable))

	require.NoError(t, q.ack(items))
	require.Zero(t, countRows(t, q, auditQueueTable))
}

func TestSeal_PassthroughStoresPlaintextBatch(t *testing.T) {
	t.Parallel()
	q := newSealedTestQueue(t, passthroughSealer{})

	require.NoError(t, q.Enqueue(newTestEvent(7)))

	var format int
	require.NoError(t, q.db.QueryRow("SELECT format FROM audit_queue").Scan(&format))
	require.Equal(t, FormatPlaintext, format)

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Events, 1)
	require.Equal(t, int64(7), items[0].Events[0].GetIndex())
}

type decodableSealer struct {
	payload []byte
}

func (s decodableSealer) Seal(_ context.Context, _ []byte) ([]byte, bool, error) {
	return s.payload, true, nil
}

func (decodableSealer) Close() error { return nil }

func TestSeal_SealedRowsAreNeverDecoded(t *testing.T) {
	t.Parallel()
	q := newSealedTestQueue(t, decodableSealer{payload: marshalTestBatch(t, 99)})

	require.NoError(t, q.Enqueue(newTestEvent(1)))

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].Sealed())
	require.Empty(t, items[0].Events,
		"a sealed row must never be decoded, even if its ciphertext decodes as a batch")
	require.Equal(t, marshalTestBatch(t, 99), items[0].Payload)
	require.Equal(t, 1, countRows(t, q, auditQueueTable),
		"the sealed row must stay queued, not be acked away or quarantined")
	require.Zero(t, countRows(t, q, corruptEventsTable))
}

func TestSeal_UnknownFormatIsQuarantined(t *testing.T) {
	t.Parallel()
	q := newSqliteTestQueue(t)
	quietLogs(t)

	_, err := q.db.Exec(
		"INSERT INTO audit_queue (payload, format, event_count, enqueued_at) VALUES (?, ?, ?, ?)",
		marshalTestBatch(t, 1), 99, 1, 1)
	require.NoError(t, err)

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, countRows(t, q, auditQueueTable))
	require.Equal(t, 1, countRows(t, q, corruptEventsTable))

	var quarantinedError string
	require.NoError(t, q.db.QueryRow("SELECT error FROM corrupt_events").Scan(&quarantinedError))
	require.Contains(t, quarantinedError, "payload format 99 cannot be decoded")
}

func TestSeal_SealedLifecycleRetryPromoteSweep(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	quietLogs(t)
	q := newSealedTestQueue(t, prefixSealer{})
	q.maxAttempts = 2

	require.NoError(t, q.Enqueue(newTestEvent(1)))

	var storedPayload []byte
	require.NoError(t, q.db.QueryRow("SELECT payload FROM audit_queue").Scan(&storedPayload))

	for range q.maxAttempts {
		items, err := q.fetch(10)
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.True(t, items[0].Sealed())
		_, err = q.processFailedDeliveries(ctx, items)
		require.NoError(t, err)
	}

	require.Zero(t, countRows(t, q, auditQueueTable))
	require.Equal(t, 1, countRows(t, q, auditDeadLetterTable),
		"a sealed batch that exhausts its attempts must be promoted, not quarantined")

	var (
		payload    []byte
		format     int
		eventCount int
	)
	require.NoError(t, q.db.QueryRow(
		"SELECT payload, format, event_count FROM audit_dead_letter").
		Scan(&payload, &format, &eventCount))
	require.Equal(t, storedPayload, payload, "promotion must preserve the sealed payload byte-for-byte")
	require.Equal(t, FormatAgeV1, format)
	require.Equal(t, 1, eventCount)

	items, err := q.fetchDeadLetterRange(0, 1<<62, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].Sealed())
	require.Equal(t, storedPayload, items[0].Payload)
	require.Equal(t, 1, items[0].EventCount)

	require.NoError(t, q.ackDeadLetter(items))
	require.Zero(t, countRows(t, q, auditDeadLetterTable))
}

func TestSeal_SealErrorFailsEnqueueAndStoresNothing(t *testing.T) {
	t.Parallel()
	q := newSealedTestQueue(t, errorSealer{})

	err := q.Enqueue(newTestEvent(1))
	require.ErrorContains(t, err, "keys unavailable")
	require.Zero(t, countRows(t, q, auditQueueTable), "no row may be written when sealing fails")
}

func TestSeal_MultiEventBatchRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	q := newSqliteTestQueue(t)

	const n = 5
	batch := make([]writeRequest, 0, n)
	for i := int64(0); i < n; i++ {
		oneOf, err := apievents.ToOneOf(newTestEvent(i))
		require.NoError(t, err)
		batch = append(batch, writeRequest{oneOf: oneOf, resp: make(chan error, 1)})
	}
	require.NoError(t, q.commitBatch(batch))

	require.Equal(t, 1, countRows(t, q, auditQueueTable), "one batch must be one row")

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Events, n)
	for i, event := range items[0].Events {
		require.Equal(t, int64(i), event.GetIndex(), "batch must preserve event order")
	}

	q.maxAttempts = 1
	promoted, err := q.processFailedDeliveries(ctx, items)
	require.NoError(t, err)
	require.Equal(t, n, promoted, "promotion is counted in events")

	var format, eventCount int
	var enqueuedAt int64
	require.NoError(t, q.db.QueryRow(
		"SELECT format, event_count, enqueued_at FROM audit_dead_letter").
		Scan(&format, &eventCount, &enqueuedAt))
	require.Equal(t, FormatPlaintext, format)
	require.Equal(t, n, eventCount)
	require.NotZero(t, enqueuedAt)
}

type sealTestWatcher struct {
	events chan types.Event
	done   chan struct{}
	once   sync.Once
}

func (w *sealTestWatcher) Events() <-chan types.Event { return w.events }
func (w *sealTestWatcher) Done() <-chan struct{}      { return w.done }
func (w *sealTestWatcher) Error() error               { return nil }
func (w *sealTestWatcher) Close() error {
	w.once.Do(func() { close(w.done) })
	return nil
}

type sealTestSRCWatcher struct {
	src types.SessionRecordingConfig
}

func (s *sealTestSRCWatcher) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return s.src, nil
}

func (s *sealTestSRCWatcher) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	w := &sealTestWatcher{
		events: make(chan types.Event, 1),
		done:   make(chan struct{}),
	}
	w.events <- types.Event{Type: types.OpInit}
	return w, nil
}

type sealTestUnwrapper struct {
	key *rsa.PrivateKey
}

func (u *sealTestUnwrapper) UnwrapKey(ctx context.Context, in recordingencryption.UnwrapInput) ([]byte, error) {
	fileKey, err := u.key.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	return fileKey, trace.Wrap(err)
}

func TestSeal_SQLiteRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

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
	src.SetEncryptionKeys(slices.Values([]*types.AgeEncryptionKey{{PublicKey: pubDER}}))

	sealer, err := recordingencryption.NewAuditQueueSealer(ctx, &sealTestSRCWatcher{src: src})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sealer.Close()) })
	q := newSealedTestQueue(t, sealer)

	const n = 3
	events := make([]apievents.AuditEvent, 0, n)
	for i := int64(0); i < n; i++ {
		event := newTestEvent(i)
		events = append(events, event)
		require.NoError(t, q.Enqueue(event))
	}

	opener, err := recordingencryption.NewAuditQueueOpener(&sealTestUnwrapper{key: key})
	require.NoError(t, err)

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.NotEmpty(t, items)

	var decrypted []apievents.AuditEvent
	var fetchedEventCount int
	for _, item := range items {
		require.True(t, item.Sealed())
		require.Equal(t, FormatAgeV1, item.Format)
		require.Empty(t, item.Events)
		fetchedEventCount += item.EventCount

		batchEvents, err := opener.DecryptBatch(ctx, item.Payload)
		require.NoError(t, err)
		decrypted = append(decrypted, batchEvents...)
	}
	require.Equal(t, events, decrypted)
	require.Equal(t, len(events), fetchedEventCount)
}
