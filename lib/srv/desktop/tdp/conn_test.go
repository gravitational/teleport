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

package tdp

import (
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTDPConnTracksLocalRemoteAddrs verifies that a TDP connection
// uses the underlying local/remote addrs when available.
func TestTDPConnTracksLocalRemoteAddrs(t *testing.T) {
	local := &net.IPAddr{IP: net.ParseIP("192.168.1.2")}
	remote := &net.IPAddr{IP: net.ParseIP("192.168.1.3")}

	for _, test := range []struct {
		desc   string
		conn   io.ReadWriteCloser
		local  net.Addr
		remote net.Addr
	}{
		{
			desc: "implements srv.TrackingConn",
			conn: fakeTrackingConn{
				local:  local,
				remote: remote,
			},
			local:  local,
			remote: remote,
		},
		{
			desc:   "does not implement srv.TrackingConn",
			conn:   &fakeConn{Buffer: &bytes.Buffer{}},
			local:  nil,
			remote: nil,
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			tc := NewConn(test.conn)
			l := tc.LocalAddr()
			r := tc.RemoteAddr()
			require.Equal(t, test.local, l)
			require.Equal(t, test.remote, r)
		})
	}
}

type fakeConn struct {
	*bytes.Buffer
}

func (t *fakeConn) Close() error { return nil }

type fakeTrackingConn struct {
	*fakeConn
	local  net.Addr
	remote net.Addr
}

func (f fakeTrackingConn) LocalAddr() net.Addr {
	return f.local
}

func (f fakeTrackingConn) RemoteAddr() net.Addr {
	return f.remote
}
