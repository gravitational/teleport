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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSQLiteInMemoryQueue_IsInMemory(t *testing.T) {
	t.Parallel()

	q, err := New(KindSQLiteInMemory, Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, q.Close()) })

	mem, ok := q.(*sqliteInMemoryQueue)
	require.True(t, ok)

	rows, err := mem.inner.db.Query("PRAGMA database_list")
	require.NoError(t, err)
	defer rows.Close()

	var foundMain bool
	for rows.Next() {
		var seq int
		var name, file string
		require.NoError(t, rows.Scan(&seq, &name, &file))
		if name == "main" {
			foundMain = true
			require.Empty(t, file,
				"main database file path should be empty for in-memory DB, got %q", file)
		}
	}
	require.NoError(t, rows.Err())
	require.True(t, foundMain, "main database not found in PRAGMA database_list")
}

func TestSQLiteInMemoryQueue_InstancesAreIsolated(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	a, err := New(KindSQLiteInMemory, Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	b, err := New(KindSQLiteInMemory, Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, b.Close()) })

	require.NoError(t, a.Enqueue(ctx, newTestEvent(1)))

	memA := a.(*sqliteInMemoryQueue)
	memB := b.(*sqliteInMemoryQueue)
	require.NotEqual(t, memA.id, memB.id, "each instance should have a unique DB id")

	// Wait for the event to land in instance A's DB.
	require.Eventually(t, func() bool {
		var n int
		if err := memA.inner.db.QueryRow("SELECT COUNT(*) FROM audit_queue").Scan(&n); err != nil {
			return false
		}
		return n == 1
	}, testDefaultTimeout, 10*time.Millisecond, "instance a never received the enqueued event")

	// b should see zero events because it owns a different DB.
	var n int
	require.NoError(t, memB.inner.db.QueryRow("SELECT COUNT(*) FROM audit_queue").Scan(&n))
	require.Equal(t, 0, n, "instance b should not see events enqueued into instance a")
}

func TestSQLiteInMemoryQueue_MaxBytesEnforced(t *testing.T) {
	t.Parallel()

	const maxBytes = 1 * 1024 * 1024 // 1 MiB
	q, err := New(KindSQLiteInMemory, Config{MaxBytes: maxBytes})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, q.Close()) })

	mem, ok := q.(*sqliteInMemoryQueue)
	require.True(t, ok)

	var pages int64
	require.NoError(t, mem.inner.db.QueryRow("PRAGMA max_page_count").Scan(&pages))
	require.Equal(t, bytesToPages(maxBytes), pages,
		"max_page_count should be derived from cfg.MaxBytes")
}
