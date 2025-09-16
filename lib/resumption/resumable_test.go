// Teleport
// Copyright (C) 2024  Gravitational, Inc.
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

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/utils/uds"
)

func TestConnNetTest(t *testing.T) {
	testCases := []struct {
		testName  string
		firstConn bool
		syncPipe  bool
	}{
		{
			testName:  "FirstConnSync",
			firstConn: true,
			syncPipe:  true,
		},
		{
			testName:  "NotFirstConnSync",
			firstConn: false,
			syncPipe:  true,
		},
		{
			testName:  "FirstConnSocketpair",
			firstConn: true,
			syncPipe:  false,
		},
		{
			testName:  "NotFirstConnSocketpair",
			firstConn: false,
			syncPipe:  false,
		},
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

				var e errgroup.Group
				r1.mu.Lock()
				e.Go(func() error {
					return runResumeV1Unlocking(r1, p1, tc.firstConn)
				})

				r2.mu.Lock()
				e.Go(func() error {
					return runResumeV1Unlocking(r2, p2, tc.firstConn)
				})

				return r1, r2, func() {
					r1.Close()
					r2.Close()
					e.Wait()
				}, nil
			}

			nettest.TestConn(t, makePipe)
		})
	}
}

func TestConnResume(t *testing.T) {
	testCases := []struct {
		testName string
		syncPipe bool
	}{
		{
			testName: "Sync",
			syncPipe: true,
		},
		{
			testName: "Socketpair",
			syncPipe: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			testConnResume(t, tc.syncPipe)
		})
	}
}

func testConnResume(t *testing.T, syncPipe bool) {
	require := require.New(t)

	r1 := newResumableConn(nil, nil)
	defer r1.Close()

	r2 := newResumableConn(nil, nil)
	defer r2.Close()

	p1, p2 := makePipe(t, syncPipe)

	const isFirstConn = true
	r1.mu.Lock()
	go runResumeV1Unlocking(r1, p1, isFirstConn)
	r2.mu.Lock()
	go runResumeV1Unlocking(r2, p2, isFirstConn)

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

	p1, p2 = makePipe(t, syncPipe)

	const isNotFirstConn = false
	r1.mu.Lock()
	go runResumeV1Unlocking(r1, p1, isNotFirstConn)
	r2.mu.Lock()
	go runResumeV1Unlocking(r2, p2, isNotFirstConn)

	_, err = io.ReadFull(r1, recvB)
	require.NoError(err)
	require.Equal(randB, recvB)
}

func TestConnDesync(t *testing.T) {
	require := require.New(t)

	const isSyncPipe = true
	p1, p2 := makePipe(t, isSyncPipe)

	r1 := newResumableConn(nil, nil)
	defer r1.Close()
	r2 := newResumableConn(nil, nil)
	defer r2.Close()
	r2.receiveBuffer.start = 1
	r2.receiveBuffer.end = 1

	err2C := make(chan error, 1)
	r2.mu.Lock()
	go func() {
		const isNotFirstConn = false
		err2C <- runResumeV1Unlocking(r2, p2, isNotFirstConn)
	}()

	r1.mu.Lock()
	const isNotFirstConn = false
	err1 := runResumeV1Unlocking(r1, p1, isNotFirstConn)
	err2 := <-err2C

	require.Error(err1)
	require.Error(err2)
	require.ErrorAs(err1, new(*trace.BadParameterError))
	require.ErrorContains(err1, "got incompatible resume position")

	require.ErrorIs(err2, net.ErrClosed)
}

func makePipe(t *testing.T, syncPipe bool) (net.Conn, net.Conn) {
	var p1, p2 net.Conn
	if syncPipe {
		p1, p2 = net.Pipe()
	} else {
		var err error
		p1, p2, err = uds.NewSocketpair(uds.SocketTypeStream)
		require.NoError(t, err)
	}
	t.Cleanup(func() { _ = p1.Close() })
	t.Cleanup(func() { _ = p2.Close() })
	return p1, p2
}
