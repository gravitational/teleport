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

package srv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLineBufferFeedLines(t *testing.T) {
	type ev struct {
		in   []byte
		want []string
	}
	tests := []struct {
		name string
		evs  []ev
	}{
		{"single line CR", []ev{{[]byte("ls -la\r"), []string{"ls -la"}}}},
		{"single line LF", []ev{{[]byte("whoami\n"), []string{"whoami"}}}},
		{"crlf single", []ev{{[]byte("ls\r\n"), []string{"ls"}}}},
		{"crlf multiple", []ev{{[]byte("a\r\nb\r\nc\r\n"), []string{"a", "b", "c"}}}},
		{"blank lines between crlf", []ev{{[]byte("rm -rf /\r\n\r\nls\r\n"), []string{"rm -rf /", "ls"}}}},
		{"backspace edits", []ev{{[]byte("lss\x7f -l\r"), []string{"ls -l"}}}},
		{"ctrl-u clears", []ev{{[]byte("rm -rf /\x15ls\r"), []string{"ls"}}}},
		{"ctrl-c aborts line", []ev{
			{[]byte("dangerous\x03"), nil},
			{[]byte("ls\r"), []string{"ls"}},
		}},
		{"blank line", []ev{{[]byte("\r"), nil}}},
		{"multiple lines one write", []ev{{[]byte("ls\nrm -rf /\nwhoami\n"), []string{"ls", "rm -rf /", "whoami"}}}},
		{"trailing partial retained", []ev{
			{[]byte("ls\necho "), []string{"ls"}},
			{[]byte("hi\n"), []string{"echo hi"}},
		}},
		{"blank lines skipped", []ev{{[]byte("\n\nls\n"), []string{"ls"}}}},
		{"no completed line", []ev{{[]byte("partial"), nil}}},
		{"control chars within", []ev{{[]byte("rm\x15ls\n"), []string{"ls"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := &lineBuffer{}
			for _, e := range tt.evs {
				got := lb.feedLines(e.in)
				require.Len(t, got, len(e.want))
				require.Equal(t, e.want, got)
			}
		})
	}
}
