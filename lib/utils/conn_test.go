/*
Copyright 2022 Gravitational, Inc.

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
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackingReaderEOF(t *testing.T) {
	reader := bytes.NewReader([]byte{})

	// Wrap the reader in a TrackingReader.
	tr := NewTrackingReader(reader)

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
