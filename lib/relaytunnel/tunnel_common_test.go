// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package relaytunnel

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/stretchr/testify/require"
)

func TestYamuxStreamConnDeadline(t *testing.T) {
	t.Parallel()

	conn1, conn2 := net.Pipe()

	cfg := yamux.DefaultConfig()
	cfg.LogOutput = t.Output()

	sess1, err := yamux.Client(conn1, cfg)
	require.NoError(t, err)
	defer sess1.Close()

	sess2, err := yamux.Server(conn2, cfg)
	require.NoError(t, err)
	defer sess2.Close()

	stream, err := sess1.OpenStream()
	require.NoError(t, err)
	defer stream.Close()

	streamConn := &yamuxStreamConn{Stream: stream}
	require.NoError(t, streamConn.SetDeadline(time.Unix(1, 0)))

	n, err := streamConn.Read(make([]byte, 1))
	require.Zero(t, n)

	//nolint:errorlint // because of bad practices around net.Conn in the
	// ecosystem, we want the exact error value to be returned rather than a
	// wrapper; require.Equal checks for deep equality, require.Same relies on
	// the implementation detail of os.ErrDeadlineExceeded being a pointer, so
	// we spell out the actual equality instead
	if err != os.ErrDeadlineExceeded {
		require.FailNow(t, "expected os.ErrDeadlineExceeded, got %v", err)
	}
}
