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
	"path/filepath"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
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
	require.Equal(t, formatAgeV1, format)
	require.Equal(t, 1, eventCount)
	require.NotZero(t, enqueuedAt)
	require.True(t, bytes.HasPrefix(payload, sealPrefix), "stored payload should be the sealed bytes")

	stats, err := q.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), stats.PendingCount)
	require.Zero(t, stats.DeadLetterCount)

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, countRows(t, q, auditQueueTable))
	require.Equal(t, 2, countRows(t, q, corruptEventsTable))

	var (
		quarantinedFormat int
		quarantinedError  string
	)
	require.NoError(t, q.db.QueryRow(
		"SELECT format, error FROM corrupt_events ORDER BY id ASC LIMIT 1").
		Scan(&quarantinedFormat, &quarantinedError))
	require.Equal(t, formatAgeV1, quarantinedFormat)
	require.Contains(t, quarantinedError, "payload format 1 cannot be decoded",
		"sealed rows should be quarantined by format, not by decode failure")

	q.recoverCorruptEvents()
	require.Zero(t, countRows(t, q, auditQueueTable))
	require.Equal(t, 2, countRows(t, q, corruptEventsTable))
}

func TestSeal_PassthroughStoresPlaintextBatch(t *testing.T) {
	t.Parallel()
	q := newSealedTestQueue(t, passthroughSealer{})

	require.NoError(t, q.Enqueue(newTestEvent(7)))

	var format int
	require.NoError(t, q.db.QueryRow("SELECT format FROM audit_queue").Scan(&format))
	require.Equal(t, formatPlaintext, format)

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

func TestSeal_QuarantineDispatchesOnFormat(t *testing.T) {
	t.Parallel()
	q := newSealedTestQueue(t, decodableSealer{payload: marshalTestBatch(t, 99)})

	require.NoError(t, q.Enqueue(newTestEvent(1)))

	items, err := q.fetch(10)
	require.NoError(t, err)
	require.Empty(t, items,
		"a sealed row must never be returned as deliverable, even if its ciphertext decodes as a batch")
	require.Zero(t, countRows(t, q, auditQueueTable))
	require.Equal(t, 1, countRows(t, q, corruptEventsTable),
		"the sealed row must be quarantined, not silently acked as an empty batch")
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
	require.Equal(t, formatPlaintext, format)
	require.Equal(t, n, eventCount)
	require.NotZero(t, enqueuedAt)
}
