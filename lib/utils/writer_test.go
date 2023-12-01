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

func TestCaptureNBytesWriter(t *testing.T) {
	t.Parallel()

	data := []byte("abcdef")
	w := NewCaptureNBytesWriter(10)

	// Write 6 bytes. Captured 6 bytes in total.
	n, err := w.Write(data)
	require.Equal(t, 6, n)
	require.NoError(t, err)
	require.Equal(t, "abcdef", string(w.Bytes()))

	// Write 6 bytes. Captured 10 bytes in total.
	n, err = w.Write(data)
	require.Equal(t, 6, n)
	require.NoError(t, err)
	require.Equal(t, "abcdefabcd", string(w.Bytes()))

	// Write 6 bytes. Captured 10 bytes in total.
	n, err = w.Write(data)
	require.Equal(t, 6, n)
	require.NoError(t, err)
	require.Equal(t, "abcdefabcd", string(w.Bytes()))
}
