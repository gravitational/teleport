/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package memory

import (
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestMemory(t *testing.T) {
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		cfg, err := test.ApplyOptions(options)

		if err != nil {
			return nil, nil, err
		}

		if cfg.ConcurrentBackend != nil {
			if _, ok := cfg.ConcurrentBackend.(*Memory); !ok {
				return nil, nil, trace.BadParameter("target is not a Memory backend")
			}
			return cfg.ConcurrentBackend, nil, nil
		}

		clock := clockwork.NewFakeClock()
		mem, err := New(Config{
			Clock:  clock,
			Mirror: cfg.MirrorMode,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return mem, clock, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

func TestMemoryItemOwnership(t *testing.T) {
	mem, err := New(Config{})
	require.NoError(t, err)

	key := backend.NewKey("testing")
	value := []byte{0, 1, 2, 3}
	lease, err := mem.Put(t.Context(), backend.Item{
		Key:   key,
		Value: value,
	})
	require.NoError(t, err)

	// Mutating the caller-owned write buffer must not mutate the stored item.
	copy(value, []byte{9, 8, 7, 6})

	got, err := mem.Get(t.Context(), key)
	require.NoError(t, err)
	require.Equal(t, lease.Revision, got.Revision)
	require.Equal(t, []byte{0, 1, 2, 3}, got.Value)

	// Mutating the item returned by Get must not mutate the stored item.
	got.Revision = "tampered"
	got.Value[0] = 9

	got, err = mem.Get(t.Context(), key)
	require.NoError(t, err)
	require.Equal(t, lease.Revision, got.Revision)
	require.Equal(t, []byte{0, 1, 2, 3}, got.Value)

	var itemsFound bool
	for item, err := range mem.Items(t.Context(), backend.ItemsParams{
		StartKey: key,
		EndKey:   backend.NewKey("zzzz"),
		Limit:    1,
	}) {
		itemsFound = true
		require.NoError(t, err)
		require.Equal(t, lease.Revision, item.Revision)
		require.Equal(t, []byte{0, 1, 2, 3}, item.Value)

		// Mutating the item returned by Items must not mutate the stored item.
		item.Revision = "tampered"
		item.Value[0] = 8
	}
	require.True(t, itemsFound)

	got, err = mem.Get(t.Context(), key)
	require.NoError(t, err)
	require.Equal(t, lease.Revision, got.Revision)
	require.Equal(t, []byte{0, 1, 2, 3}, got.Value)
}
