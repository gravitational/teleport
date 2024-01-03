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
	"net"
	"testing"

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
				r1 := &ResumableConn{}
				r1.allowRoaming = !tc.firstConn
				r1.cond.L = &r1.mu

				r2 := &ResumableConn{}
				r2.allowRoaming = !tc.firstConn
				r2.cond.L = &r2.mu

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

				go HandleResumeV1(r1, p1, tc.firstConn)
				go HandleResumeV1(r2, p2, tc.firstConn)

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
