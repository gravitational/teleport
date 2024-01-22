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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncMap(t *testing.T) {
	t.Parallel()

	var m SyncMap[string, UID]

	// Nothing stored yet.
	v, ok := m.Load("key1")
	require.Nil(t, v)
	require.False(t, ok)

	// Store some values.
	value1 := NewRealUID()
	value2 := NewFakeUID()
	m.Store("key1", value1)
	m.Store("key2", value2)

	// Check stored values.
	v, ok = m.Load("key1")
	require.Equal(t, value1, v)
	require.True(t, ok)

	v, ok = m.Load("key2")
	require.Equal(t, value2, v)
	require.True(t, ok)

	// Delete one.
	m.Delete("key1")
	v, ok = m.Load("key1")
	require.Nil(t, v)
	require.False(t, ok)

	v, ok = m.Load("key2")
	require.Equal(t, value2, v)
	require.True(t, ok)
}
