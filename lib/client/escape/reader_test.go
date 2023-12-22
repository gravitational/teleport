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

package escape

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type readerTestCase struct {
	inChunks [][]byte
	inErr    error

	wantReadErr       error
	wantDisconnectErr error
	wantOut           string
	wantHelp          string
}

func runCase(t *testing.T, tc readerTestCase) {
	in := &mockReader{chunks: tc.inChunks, finalErr: tc.inErr}
	helpOut := new(bytes.Buffer)
	out := new(bytes.Buffer)
	var disconnectErr error

	r := NewReader(in, helpOut, func(err error) {
		disconnectErr = err
	})

	_, err := io.Copy(out, r)
	require.Equal(t, tc.wantReadErr, err)
	require.Equal(t, tc.wantDisconnectErr, disconnectErr)
	require.Equal(t, tc.wantOut, out.String())
	require.Equal(t, tc.wantHelp, helpOut.String())
}

func TestNormalReads(t *testing.T) {
	t.Log("normal read")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("hello world")},
		wantOut:  "hello world",
	})

	t.Log("incomplete sequence")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~world")},
		wantOut:  "hello\r~world",
	})

	t.Log("escaped tilde character")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~~world")},
		wantOut:  "hello\r~world",
	})

	t.Log("other character between newline and tilde")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("hello\rw~orld")},
		wantOut:  "hello\rw~orld",
	})

	t.Log("other character between newline and disconnect sequence")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("hello\rw~.orld")},
		wantOut:  "hello\rw~.orld",
	})
}

func TestReadError(t *testing.T) {
	customErr := errors.New("oh no")

	runCase(t, readerTestCase{
		inChunks:          [][]byte{[]byte("hello world")},
		inErr:             customErr,
		wantOut:           "hello world",
		wantReadErr:       customErr,
		wantDisconnectErr: customErr,
	})
}

func TestEscapeHelp(t *testing.T) {
	t.Log("single help sequence between reads")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~?world")},
		wantOut:  "hello\rworld",
		wantHelp: helpText,
	})

	t.Log("single help sequence before any data")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("~?hello world")},
		wantOut:  "hello world",
		wantHelp: helpText,
	})

	t.Log("repeated help sequences")
	runCase(t, readerTestCase{
		inChunks: [][]byte{[]byte("hello\r~?world\n~?")},
		wantOut:  "hello\rworld\n",
		wantHelp: helpText + helpText,
	})

	t.Log("help sequence split across reads")
	runCase(t, readerTestCase{
		inChunks: [][]byte{
			[]byte("hello\r"),
			[]byte("~"),
			[]byte("?"),
			[]byte("world"),
		},
		wantOut:  "hello\rworld",
		wantHelp: helpText,
	})
}

func TestEscapeDisconnect(t *testing.T) {
	t.Log("single disconnect sequence between reads")
	runCase(t, readerTestCase{
		inChunks: [][]byte{
			[]byte("hello"),
			[]byte("\r~."),
			[]byte("world"),
		},
		wantOut:           "hello",
		wantReadErr:       ErrDisconnect,
		wantDisconnectErr: ErrDisconnect,
	})

	t.Log("disconnect sequence before any data")
	runCase(t, readerTestCase{
		inChunks:          [][]byte{[]byte("~.hello world")},
		wantReadErr:       ErrDisconnect,
		wantDisconnectErr: ErrDisconnect,
	})

	t.Log("disconnect sequence split across reads")
	runCase(t, readerTestCase{
		inChunks: [][]byte{
			[]byte("hello\r"),
			[]byte("~"),
			[]byte("."),
			[]byte("world"),
		},
		wantOut:           "hello\r",
		wantReadErr:       ErrDisconnect,
		wantDisconnectErr: ErrDisconnect,
	})
}

func TestBufferOverflow(t *testing.T) {
	in := &mockReader{chunks: [][]byte{make([]byte, 100)}}
	helpOut := new(bytes.Buffer)
	out := new(bytes.Buffer)
	var disconnectErr error

	r := newUnstartedReader(in, helpOut, func(err error) {
		disconnectErr = err
	})
	r.bufferLimit = 10
	go r.runReads()

	_, err := io.Copy(out, r)
	require.Equal(t, err, ErrTooMuchBufferedData)
	require.Equal(t, disconnectErr, ErrTooMuchBufferedData)
}

type mockReader struct {
	chunks   [][]byte
	finalErr error
}

func (r *mockReader) Read(buf []byte) (int, error) {
	if len(r.chunks) == 0 {
		if r.finalErr != nil {
			return 0, r.finalErr
		}
		return 0, io.EOF
	}

	chunk := r.chunks[0]
	r.chunks = r.chunks[1:]
	copy(buf, chunk)
	return len(chunk), nil
}
