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
		var b [24]byte
		_, err := rand.Read(b[:])
		require.NoError(t, err)

		startKey := base64.URLEncoding.EncodeToString(b[:])

		eventTime, eventID, err := fromStartKey(startKey)
		require.NoError(t, err)

		require.Equal(t, startKey, toNextKey(eventTime, eventID))
	}

	for i := 0; i < 1000; i++ {
		var b [8]byte
		_, err := rand.Read(b[:])
		require.NoError(t, err)
		n := binary.LittleEndian.Uint64(b[:])

		nextTime := time.UnixMicro(int64(n)).UTC()
		nextID := uuid.New()

		startTime, startID, err := fromStartKey(toNextKey(nextTime, nextID))
		require.NoError(t, err)
		require.Equal(t, nextTime, startTime)
		require.Equal(t, nextID, startID)
	}
}
