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
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackingReaderEOF(t *testing.T) {
	reader := bytes.NewReader([]byte{})

	// Wrap the reader in a TrackingReader.
	tr := &trackingReader{r: reader}

	// Make sure it returns an EOF and not a wrapped exception.
	buf := make([]byte, 64)
	_, err := tr.Read(buf)
	require.Equal(t, io.EOF, err)
}

func TestConnWithSrcAddr(t *testing.T) {
	t.Parallel()

	orgConn, _ := net.Pipe()

	t.Run("nil clientSrcAddr", func(t *testing.T) {
		conn := NewConnWithSrcAddr(orgConn, nil)

		require.Equal(t, orgConn.RemoteAddr().String(), conn.RemoteAddr().String())
		require.Equal(t, orgConn.LocalAddr().String(), conn.LocalAddr().String())
		require.Equal(t, orgConn, conn.NetConn())
	})

	t.Run("valid clientSrcAddr", func(t *testing.T) {
		addr := MustParseAddr("11.22.33.44:5566")
		conn := NewConnWithSrcAddr(orgConn, addr)

		require.NotEqual(t, orgConn.RemoteAddr().String(), conn.RemoteAddr().String())
		require.Equal(t, "11.22.33.44:5566", conn.RemoteAddr().String())
		require.Equal(t, orgConn.LocalAddr().String(), conn.LocalAddr().String())
		require.Equal(t, orgConn, conn.NetConn())
	})
}
