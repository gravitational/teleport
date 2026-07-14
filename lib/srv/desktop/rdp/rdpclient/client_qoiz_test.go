//go:build desktop_access_rdp && desktop_encoder

/*
 * *
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package rdpclient

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeQOIZ(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		frames, err := EncodeQOIZ(nil, 0, 0, 0, 0)
		require.NoError(t, err)
		require.Empty(t, frames)
	})

	t.Run("size mismatch", func(t *testing.T) {
		_, err := EncodeQOIZ([]byte{0xFF}, 0, 0, 2, 5)
		require.Error(t, err)
	})

	t.Run("single pixel", func(t *testing.T) {
		// this test aim is to verify we get the same data as in tests on Rust side:
		// encode_qoiz_single in encoder.rs
		frames, err := EncodeQOIZ([]byte{0xFF, 0xFF, 0xFF, 0xFF}, 0, 0, 1, 1)
		require.NoError(t, err)
		require.Len(t, frames, 1)
		require.Equal(t, []byte{0, 59, 4, 54, 0, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 11, 1, 0, 1, 0, 32, 0, 0,
			0, 40, 181, 47, 253, 0, 88, 185, 0, 0, 113, 111, 105, 102, 0, 0, 0, 1, 0, 0, 0, 1,
			3, 0, 85, 0, 0, 0, 0, 0, 0, 0, 1}, frames[0].Pdu)
	})

	t.Run("random", func(t *testing.T) {
		// test with random data to verify we get correct number of frames from Rust
		data := make([]byte, 4*500*500)
		for i := 0; i < 4*500*500; i += 4 {
			data[i] = byte(rand.IntN(256))
			data[i+1] = byte(rand.IntN(256))
			data[i+2] = byte(rand.IntN(256))
			data[i+3] = 0xFF
		}

		frames, err := EncodeQOIZ(data, 0, 0, 500, 500)
		require.NoError(t, err)
		require.Len(t, frames, 27)
	})
}
