// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
