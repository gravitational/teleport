/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mysql

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func Test_isDialError(t *testing.T) {
	tests := []struct {
		desc string
		err  error
		want bool
	}{
		{
			desc: "non dial error",
			err:  errors.New("llama stampede!"),
		},
		{
			desc: "dial error",
			err:  newDialError(errors.New("failed to dial x.x.x.x")),
			want: true,
		},
		{
			desc: "nil error",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			require.Equal(t, test.want, isDialError(trace.Wrap(test.err)))
		})
	}
}

func Test_recorderConn(t *testing.T) {
	mustWrite := func(t *testing.T, w io.Writer, data string) {
		t.Helper()
		n, err := io.WriteString(w, data)
		require.NoError(t, err)
		require.Equal(t, len(data), n)
	}

	mustRead := func(t *testing.T, r io.Reader, want string) {
		t.Helper()
		buf := make([]byte, len(want))
		n, err := io.ReadFull(r, buf)
		require.NoError(t, err)
		require.Equal(t, want, string(buf[:n]))
	}

	conn := &fakeConn{}
	recorder := newRecorderConn(conn)

	t.Run("rewind and replay reads", func(t *testing.T) {
		mustWrite(t, conn, "hello")
		mustRead(t, recorder, "hello")

		mustWrite(t, conn, "world")
		mustRead(t, recorder, "world")

		recorder.rewind()
		mustRead(t, recorder, "helloworld")
		recorder.rewind()
		mustRead(t, recorder, "helloworld")
		recorder.rewind()
		mustRead(t, recorder, "helloworld")

		mustWrite(t, conn, "!")
		recorder.rewind()
		mustRead(t, recorder, "helloworld!")
		rawConn := recorder.rewind()
		mustRead(t, rawConn, "helloworld!")
		mustRead(t, rawConn, "")
		mustRead(t, recorder, "")
	})

	t.Run("limits memory usage", func(t *testing.T) {
		recorder.reset()
		bufferLimit := recorder.remainingBufferSize()
		require.Positive(t, bufferLimit)
		xs := strings.Repeat("x", bufferLimit)
		mustWrite(t, conn, xs)
		mustWrite(t, conn, "y")

		got, err := io.ReadAll(recorder)
		require.NoError(t, err, "exceeding the buffer limit should not produce an error")
		require.Equal(t, xs+"y", string(got))

		for range 3 {
			t.Run("rewind and replay up to buffer limit", func(t *testing.T) {
				recorder.rewind()
				remaining := recorder.remainingBufferSize()
				require.Equal(t, bufferLimit, remaining, "after rewinding we should be able to record the full limit again")
				got, err := io.ReadAll(recorder)
				require.NoError(t, err, "exceeding the buffer limit should not produce an error")
				require.Equal(t, xs, string(got), "the last byte exceeded the recording limit and should not have been recorded")
				mustWrite(t, conn, "z")
				mustRead(t, recorder, "z")
			})
		}
	})
}

type fakeConn struct {
	net.Conn
	buf bytes.Buffer
}

func (c *fakeConn) Read(b []byte) (n int, err error) {
	return c.buf.Read(b)
}

func (c *fakeConn) Write(b []byte) (n int, err error) {
	return c.buf.Write(b)
}
