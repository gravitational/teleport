/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
