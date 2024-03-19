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

	"github.com/stretchr/testify/assert"
)

func TestSyncMap(t *testing.T) {
	t.Parallel()

	var m SyncMap[string, UID]

	// Nothing stored yet.
	v, ok := m.Load("key1")
	assert.Nil(t, v)
	assert.False(t, ok)

	// Store some values.
	value1 := NewRealUID()
	value2 := NewFakeUID()
	m.Store("key1", value1)
	m.Store("key2", value2)

	// Check stored values.
	v, ok = m.Load("key1")
	assert.Equal(t, value1, v)
	assert.True(t, ok)

	v, ok = m.Load("key2")
	assert.Equal(t, value2, v)
	assert.True(t, ok)

	// Iterate stored values with Range.
	var keys []string
	var values []UID
	m.Range(func(key string, value UID) bool {
		keys = append(keys, key)
		values = append(values, value)
		return true
	})
	assert.ElementsMatch(t, []string{"key1", "key2"}, keys)
	assert.ElementsMatch(t, []UID{value1, value2}, values)

	// Delete one.
	m.Delete("key1")
	v, ok = m.Load("key1")
	assert.Nil(t, v)
	assert.False(t, ok)

	v, ok = m.Load("key2")
	assert.Equal(t, value2, v)
	assert.True(t, ok)

	// Load and delete.
	v, ok = m.LoadAndDelete("key2")
	assert.Equal(t, value2, v)
	assert.True(t, ok)
	v, ok = m.LoadAndDelete("key2")
	assert.Nil(t, v)
	assert.False(t, ok)
}
