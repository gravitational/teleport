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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/auditqueue"
)

type fakeStatsEmitter struct {
	stats auditqueue.Stats
	err   error
}

func (f *fakeStatsEmitter) Stats(context.Context) (auditqueue.Stats, error) {
	return f.stats, f.err
}

func TestAuditQueueStatusAggregation(t *testing.T) {
	ctx := context.Background()
	process := &TeleportProcess{logger: slog.Default()}

	// No emitters registered -> nil status (audit queue disabled / not present).
	require.Nil(t, process.AuditQueueStatus(ctx))

	a := &fakeStatsEmitter{stats: auditqueue.Stats{PendingCount: 3, DeadLetterCount: 1}}
	b := &fakeStatsEmitter{stats: auditqueue.Stats{PendingCount: 5, DeadLetterCount: 2}}
	process.registerAuditQueueStats(a)
	process.registerAuditQueueStats(b)

	// Depth is summed across all registered emitters.
	status := process.AuditQueueStatus(ctx)
	require.NotNil(t, status)
	require.Equal(t, int64(8), status.PendingCount)
	require.Equal(t, int64(3), status.DeadLetterCount)

	// An erroring getter is skipped, not fatal.
	c := &fakeStatsEmitter{err: errors.New("queue closed")}
	process.registerAuditQueueStats(c)
	status = process.AuditQueueStatus(ctx)
	require.Equal(t, int64(8), status.PendingCount)
	require.Equal(t, int64(3), status.DeadLetterCount)

	// Unregistering drops the emitter from the sum.
	process.unregisterAuditQueueStats(b)
	status = process.AuditQueueStatus(ctx)
	require.Equal(t, int64(3), status.PendingCount)
	require.Equal(t, int64(1), status.DeadLetterCount)
}
