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

package pgevents

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPaginationKeyRoundtrip(t *testing.T) {
	t.Parallel()

	for i := 0; i < 1000; i++ {
		var b [32]byte
		_, err := rand.Read(b[:])
		require.NoError(t, err)

		startKey := base64.URLEncoding.EncodeToString(b[:])

		key, err := fromStartKey(startKey)
		require.NoError(t, err)

		require.Equal(t, startKey, toNextKey(key.time, key.index, key.id))
	}

	for i := 0; i < 1000; i++ {
		var b [8]byte
		_, err := rand.Read(b[:])
		require.NoError(t, err)
		n := binary.LittleEndian.Uint64(b[:])

		nextTime := time.UnixMicro(int64(n)).UTC()
		nextIndex := int64(n >> 1)
		nextID := uuid.New()

		key, err := fromStartKey(toNextKey(nextTime, nextIndex, nextID))
		require.NoError(t, err)
		require.Equal(t, nextTime, key.time)
		require.Equal(t, nextIndex, key.index)
		require.Equal(t, nextID, key.id)
		require.True(t, key.hasIndex)
	}
}

func TestPaginationKeyRoundtripLegacy(t *testing.T) {
	t.Parallel()

	var b [24]byte
	_, err := rand.Read(b[:])
	require.NoError(t, err)

	startKey := base64.URLEncoding.EncodeToString(b[:])

	key, err := fromStartKey(startKey)
	require.NoError(t, err)
	require.False(t, key.hasIndex)
	require.Equal(t, time.UnixMicro(int64(binary.LittleEndian.Uint64(b[0:8]))).UTC(), key.time)
}
