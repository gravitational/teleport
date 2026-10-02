// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package sse

import "bytes"

// utf8BOM is the byte order mark some producers prepend to a stream.
var utf8BOM = []byte("\xef\xbb\xbf")

// fieldPrefixes are the line prefixes that can open a stream. A stream need
// not begin with "data:".
var fieldPrefixes = [][]byte{
	[]byte("data:"),
	[]byte("event:"),
	[]byte("id:"),
	[]byte("retry:"),
}

// TrimBOM returns b without a leading UTF-8 byte order mark. [ReadEvents]
// does not skip one, so callers reading a body that may carry it should trim
// it first.
func TrimBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, utf8BOM)
}

// LooksLikeStream reports whether b, the start of a body, looks like an SSE
// stream rather than some other payload such as JSON. It accepts a leading
// byte order mark, blank lines and comment lines before the first field line.
// A body that holds only comments is not judged a stream, because nothing in
// it tells a stream apart from anything else.
func LooksLikeStream(b []byte) bool {
	b = TrimBOM(b)
	for {
		b = bytes.TrimLeft(b, " \t\r\n")
		if len(b) == 0 {
			return false
		}
		// A comment line carries no data of its own, so judge the first
		// field line that follows it.
		if b[0] == ':' {
			i := bytes.IndexAny(b, "\r\n")
			if i < 0 {
				return false
			}
			b = b[i+1:]
			continue
		}
		for _, prefix := range fieldPrefixes {
			if bytes.HasPrefix(b, prefix) {
				return true
			}
		}
		return false
	}
}
