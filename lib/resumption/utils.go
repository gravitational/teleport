// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/multiplexer"
)

type peeker interface {
	// Peek has the semantics of [*bufio.Reader.Peek].
	Peek(int) ([]byte, error)
}

// peekPrelude checks that the next bytes that will be read from the peeker
// match the given prelude. Doesn't advance the read pointer but if the prelude
// is found, reading the same amount of bytes from the underlying connection
// should never fail.
func peekPrelude(peeker peeker, prelude string) (bool, error) {
	for i, c := range []byte(prelude) {
		buf, err := peeker.Peek(i + 1)
		if err != nil {
			return false, trace.Wrap(err)
		}
		if buf[i] != c {
			return false, nil
		}
	}
	return true, nil
}

// peekLine peeks up to maxSize bytes looking for a newline ('\n'), returning
// the peeked line (or the first maxSize bytes, if no newline is found).
func peekLine(peeker peeker, maxSize int) (line []byte, err error) {
	for i := 0; i < maxSize; i++ {
		line, err = peeker.Peek(i + 1)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if line[i] == '\n' {
			break
		}
	}
	return line, nil
}

func ensureMultiplexerConn(nc net.Conn) *multiplexer.Conn {
	if conn, ok := nc.(*multiplexer.Conn); ok {
		return conn
	}
	return multiplexer.NewConn(nc)
}
