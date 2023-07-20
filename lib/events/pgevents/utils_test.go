// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
