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
)

func TestResumableConn(t *testing.T) {
	nettest.TestConn(t, func() (c1 net.Conn, c2 net.Conn, stop func(), err error) {
		r1 := &ResumableConn{}
		r1.allowRoaming = true
		r1.cond.L = &r1.mu

		r2 := &ResumableConn{}
		r2.allowRoaming = true
		r2.cond.L = &r2.mu

		p1, p2 := net.Pipe()

		go HandleResumeV1(r1, p1, false)
		go HandleResumeV1(r2, p2, false)

		return r1, r2, func() {
			r1.Close()
			r2.Close()
		}, nil
	})
}
