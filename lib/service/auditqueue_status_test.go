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

package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/auditqueue"
)

type fakeEmitter struct {
	stats auditqueue.Stats
	err   error
}

func (f *fakeEmitter) Stats(context.Context) (auditqueue.Stats, error) {
	return f.stats, f.err
}

func (f *fakeEmitter) Shutdown(context.Context) error {
	return nil
}

func (f *fakeEmitter) Close() error {
	return nil
}

func TestAuditQueueStatusAggregation(t *testing.T) {
	ctx := t.Context()
	process := &TeleportProcess{logger: slog.Default()}

	// No emitters registered -> nil status (audit queue disabled / not present).
	require.Nil(t, process.AuditQueueStatus(ctx))

	now := time.Now()
	a := &fakeEmitter{stats: auditqueue.Stats{
		PendingCount:      3,
		DeadLetterCount:   1,
		CorruptCount:      4,
		OldestPendingTime: now.Add(-10 * time.Minute),
	}}
	b := &fakeEmitter{stats: auditqueue.Stats{
		PendingCount:         5,
		DeadLetterCount:      2,
		CorruptCount:         6,
		OldestPendingTime:    now.Add(-20 * time.Minute),
		OldestDeadLetterTime: now.Add(-time.Hour),
	}}
	process.registerEmitter(a)
	process.registerEmitter(b)

	// Depth is summed across all registered emitters; the oldest ages take
	// the largest non-zero value, ignoring emitters with empty queues.
	status := process.AuditQueueStatus(ctx)
	require.NotNil(t, status)
	require.Equal(t, int64(8), status.PendingCount)
	require.Equal(t, int64(3), status.DeadLetterCount)
	require.Equal(t, int64(10), status.CorruptCount)
	require.InDelta(t, 20*60, status.OldestPendingAgeSeconds, 5)
	require.InDelta(t, 60*60, status.OldestDeadLetterAgeSeconds, 5)

	// An erroring getter is skipped, not fatal.
	c := &fakeEmitter{err: errors.New("queue closed")}
	process.registerEmitter(c)
	status = process.AuditQueueStatus(ctx)
	require.Equal(t, int64(8), status.PendingCount)
	require.Equal(t, int64(3), status.DeadLetterCount)
	require.Equal(t, int64(10), status.CorruptCount)

	// Unregistering drops the emitter from the sum.
	process.unregisterEmitter(b)
	status = process.AuditQueueStatus(ctx)
	require.Equal(t, int64(3), status.PendingCount)
	require.Equal(t, int64(1), status.DeadLetterCount)
	require.Equal(t, int64(4), status.CorruptCount)
}
