// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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

package resumption

import (
	"crypto/rand"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestResumableConnPipe(t *testing.T) {
	testCases := []struct {
		testName  string
		firstConn bool
		syncPipe  bool
	}{
		{"FirstConnSync", true, true},
		{"NotFirstConnSync", false, true},
		{"FirstConnSocketpair", true, false},
		{"NotFirstConnSocketpair", false, false},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			makePipe := func() (c1 net.Conn, c2 net.Conn, stop func(), err error) {
				r1 := newResumableConn(nil, nil)
				r2 := newResumableConn(nil, nil)

				var p1, p2 net.Conn
				if tc.syncPipe {
					p1, p2 = net.Pipe()
				} else {
					var err error
					p1, p2, err = uds.NewSocketpair(uds.SocketTypeStream)
					if err != nil {
						return nil, nil, nil, err
					}
				}

				go RunResumeV1(r1, p1, tc.firstConn)
				go RunResumeV1(r2, p2, tc.firstConn)

				return r1, r2, func() {
					r1.Close()
					r2.Close()
					p1.Close()
					p2.Close()
				}, nil
			}

			nettest.TestConn(t, makePipe)
		})
	}
}

func TestResumableConn(t *testing.T) {
	testCases := []struct {
		testName string
		syncPipe bool
	}{
		{"Sync", true},
		{"Socketpair", false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			testResumableConn(t, tc.syncPipe)
		})
	}
}

func testResumableConn(t *testing.T, syncPipe bool) {
	require := require.New(t)

	r1 := newResumableConn(nil, nil)
	defer r1.Close()

	r2 := newResumableConn(nil, nil)
	defer r2.Close()

	var p1, p2 net.Conn
	if syncPipe {
		p1, p2 = net.Pipe()
	} else {
		var err error
		p1, p2, err = uds.NewSocketpair(uds.SocketTypeStream)
		require.NoError(err)
	}
	defer p1.Close()
	defer p2.Close()
	go RunResumeV1(r1, p1, true)
	go RunResumeV1(r2, p2, true)

	randB := make([]byte, 100)
	_, err := rand.Read(randB)
	require.NoError(err)

	_, err = r1.Write(randB)
	require.NoError(err)

	recvB := make([]byte, 100)
	_, err = io.ReadFull(r2, recvB)
	require.NoError(err)
	require.Equal(randB, recvB)

	_ = p1.Close()
	_ = p2.Close()

	_, err = r2.Write(randB)
	require.NoError(err)

	if syncPipe {
		p1, p2 = net.Pipe()
	} else {
		var err error
		p1, p2, err = uds.NewSocketpair(uds.SocketTypeStream)
		require.NoError(err)
	}
	defer p1.Close()
	defer p2.Close()
	go RunResumeV1(r1, p1, false)
	go RunResumeV1(r2, p2, false)

	_, err = io.ReadFull(r1, recvB)
	require.NoError(err)
	require.Equal(randB, recvB)
}
