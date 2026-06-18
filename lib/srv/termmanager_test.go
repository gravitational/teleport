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

package srv

import (
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCTRLCPassthrough(t *testing.T) {
	t.Parallel()

	m := NewTermManager()
	m.On()
	r, w := io.Pipe()
	m.AddReader("foo", r)
	go w.Write([]byte("\x03"))
	buf := make([]byte, 1)
	_, err := m.Read(buf)
	require.NoError(t, err)
	require.Equal(t, []byte("\x03"), buf)
}

func TestCTRLCCapture(t *testing.T) {
	t.Parallel()

	m := NewTermManager()
	r, w := io.Pipe()
	m.AddReader("foo", r)
	w.Write([]byte("\x03"))

	select {
	case <-m.TerminateNotifier():
	case <-time.After(time.Second * 10):
		t.Fatal("terminateNotifier should've seen an event")
	}
}

func TestHistoryKept(t *testing.T) {
	t.Parallel()

	m := NewTermManager()
	m.On()

	data := make([]byte, 10000)
	n, err := rand.Read(data)
	require.NoError(t, err)
	require.Len(t, data, n)

	n, err = m.Write(data[:len(data)/2])
	require.NoError(t, err)
	require.Equal(t, len(data)/2, n)

	n, err = m.Write(data[len(data)/2:])
	require.NoError(t, err)
	require.Equal(t, len(data)/2, n)

	kept := data[len(data)-maxHistoryBytes:]
	require.Equal(t, m.GetRecentHistory(), kept)
}

func TestBufferedKept(t *testing.T) {
	t.Parallel()

	m := NewTermManager()

	data := make([]byte, 20000)
	n, err := rand.Read(data)
	require.NoError(t, err)
	require.Len(t, data, n)

	n, err = m.Write(data)
	require.NoError(t, err)
	require.Len(t, data, n)

	kept := data[len(data)-maxPausedHistoryBytes:]
	require.Equal(t, m.buffer, kept)
}

func TestNoReadWhenOff(t *testing.T) {
	t.Parallel()

	m := NewTermManager()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	r, w := io.Pipe()
	data := make([]byte, 90)
	n, err := rand.Read(data)
	require.NoError(t, err)

	discarded := make(chan struct{})
	m.onDiscard = func() { close(discarded) }

	// add our data source to the multiplexer
	m.AddReader("reader", r)

	// write some data into our pipe, making it available to the multiplexer
	_, err = w.Write(data[:n])
	require.NoError(t, err)

	// wait for the multiplexer to read the data, it should be discarded which will close the discard channel
	// and continue execution
	select {
	case <-discarded:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// enable the multiplexer, data should now flow through instead of being dropped
	m.On()

	// write 3 more bytes, these should be buffered until a reader picks them up
	_, err = w.Write([]byte{1, 2, 3})
	require.NoError(t, err)

	// attempt to read the buffered data, this should succeed since the multiplexer should now allow dataflow
	n, err = m.Read(data)
	require.NoError(t, err)
	require.Equal(t, []byte{1, 2, 3}, data[:n])
}

// wrappedTermManagerReader wraps a TermManager and alerts
// that a Read is attempted via the reading chanel
type wrappedTermManagerReader struct {
	m       *TermManager
	reading chan struct{}
}

func (r *wrappedTermManagerReader) Read(p []byte) (int, error) {
	r.reading <- struct{}{}
	return r.m.Read(p)
}

func TestTermManagerRead(t *testing.T) {
	t.Parallel()

	data := []byte{1, 2, 3}
	const timeout = 10 * time.Second

	cases := []struct {
		name          string
		reader        func() io.Reader
		readAssertion func(n int, err error, d []byte)
	}{
		{
			name: "data flow on and data to read",
			reader: func() io.Reader {
				m := NewTermManager()
				m.On()
				m.remaining = data

				return m
			},
			readAssertion: func(n int, err error, d []byte) {
				assert.NoError(t, err)
				assert.Len(t, data, n)
				assert.Equal(t, data, d)
			},
		},
		{
			name: "data flow on and data to read but closed",
			reader: func() io.Reader {
				m := NewTermManager()
				m.On()
				m.remaining = data
				m.Close()

				return m
			},
			readAssertion: func(n int, err error, d []byte) {
				assert.ErrorIs(t, err, io.EOF)
				assert.Zero(t, n)
				assert.Empty(t, d)
			},
		},
		{
			name: "closed while waiting",
			reader: func() io.Reader {
				m := NewTermManager()
				m.Off()

				r := &wrappedTermManagerReader{
					m:       m,
					reading: make(chan struct{}, 1),
				}

				go func() {
					// wait until read occurs
					select {
					case <-r.reading:
					case <-time.After(timeout):
						m.Close()
						return
					}

					// transition between on and off a few times
					for i := range 5 {
						if i%2 == 0 {
							m.Off()
						} else {
							m.On()
						}
					}

					// close the reader and return
					m.Close()
				}()

				return r
			},
			readAssertion: func(n int, err error, d []byte) {
				assert.ErrorIs(t, err, io.EOF)
				assert.Zero(t, n)
				assert.Empty(t, d)
			},
		},
		{
			name: "data flow changes to on while waiting with existing data to read",
			reader: func() io.Reader {
				m := NewTermManager()
				m.Off()
				m.remaining = data

				r := &wrappedTermManagerReader{
					m:       m,
					reading: make(chan struct{}, 1),
				}

				go func() {
					// wait until read occurs
					select {
					case <-r.reading:
					case <-time.After(timeout):
						m.Close()
						return
					}

					// transition between on and off a few times
					for i := 0; i <= 5; i++ {
						if i < 5 {
							m.Off()
						} else {
							m.On()
						}
					}
				}()

				return r
			},
			readAssertion: func(n int, err error, d []byte) {
				assert.NoError(t, err)
				assert.Len(t, data, n)
				assert.Equal(t, data, d)
			},
		},
		{
			name: "data flow is on but waits for data to read",
			reader: func() io.Reader {
				m := NewTermManager()
				m.On()

				r := &wrappedTermManagerReader{
					m:       m,
					reading: make(chan struct{}, 1),
				}

				go func() {
					// wait until read occurs
					select {
					case <-r.reading:
					case <-time.After(timeout):
						m.Close()
						return
					}

					// transition between on and off a few times
					for i := range 5 {
						if i%2 == 0 {
							m.Off()
						} else {
							m.On()
						}
					}

					// explicitly set on
					m.On()

					// send empty data first
					m.incoming <- []byte{}

					// send actual data that should be read
					m.incoming <- data
				}()

				return r
			},
			readAssertion: func(n int, err error, d []byte) {
				assert.NoError(t, err)
				assert.Len(t, data, n)
				assert.Equal(t, data, d)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.reader()
			d := make([]byte, 100)
			n, err := r.Read(d)
			tt.readAssertion(n, err, d[:n])
		})
	}

}

// fakeGate is a test commandGate that records the commands it is asked to
// approve and answers deterministically.
type fakeGate struct {
	deny   bool
	denyFn func(string) bool
	seen   []string
}

func (f *fakeGate) approve(cmd string) bool {
	f.seen = append(f.seen, cmd)
	if f.denyFn != nil {
		return !f.denyFn(cmd)
	}
	return !f.deny
}

// readAllWithin reads from the manager's shell-facing Read until either no more
// data arrives within a short quiescent window or the overall deadline passes.
// It is used to drain the gated output of the input path so tests can assert on
// the exact bytes the shell would receive.
func readAllWithin(t *testing.T, tm *TermManager, dur time.Duration) []byte {
	t.Helper()

	var out []byte
	deadline := time.Now().Add(dur)
	type readResult struct {
		n   int
		err error
	}

	for time.Now().Before(deadline) {
		buf := make([]byte, 1024)
		resCh := make(chan readResult, 1)
		go func() {
			n, err := tm.Read(buf)
			resCh <- readResult{n, err}
		}()

		select {
		case res := <-resCh:
			if res.n > 0 {
				out = append(out, buf[:res.n]...)
			}
			if res.err != nil {
				return out
			}
		case <-time.After(200 * time.Millisecond):
			// No more data arriving; consider the stream drained. The pending
			// Read goroutine will unblock when the manager is closed.
			return out
		}
	}
	return out
}

func TestTermManagerCommandGateDisabledByDefault(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	require.False(t, tm.commandGateEnabled())
}

func TestTermManagerEnableCommandGate(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	tm.SetCommandGate(&fakeGate{})
	require.True(t, tm.commandGateEnabled())
}

func TestTermManagerGateApprovesForwardsNewline(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	tm.On()
	gate := &fakeGate{}
	tm.SetCommandGate(gate)
	defer tm.Close()

	r, w := io.Pipe()
	tm.AddReader("reader", r)
	go w.Write([]byte("ls\r"))

	out := readAllWithin(t, tm, 5*time.Second)
	require.Equal(t, []byte("ls\r"), out)
	require.Equal(t, []string{"ls"}, gate.seen)
}

func TestTermManagerGateDeniesDropsNewline(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	tm.On()
	gate := &fakeGate{deny: true}
	tm.SetCommandGate(gate)
	defer tm.Close()

	r, w := io.Pipe()
	tm.AddReader("reader", r)
	go w.Write([]byte("rm -rf /\r"))

	out := readAllWithin(t, tm, 5*time.Second)
	require.NotContains(t, out, byte(byteCR))
	require.Contains(t, out, byte(byteCtrlU))
	require.Equal(t, []string{"rm -rf /"}, gate.seen)
}

func TestTermManagerGateMultiLinePaste(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	tm.On()
	gate := &fakeGate{denyFn: func(cmd string) bool { return cmd == "rm -rf /" }}
	tm.SetCommandGate(gate)
	defer tm.Close()

	r, w := io.Pipe()
	tm.AddReader("reader", r)
	go w.Write([]byte("ls\nrm -rf /\nwhoami\n"))

	out := readAllWithin(t, tm, 5*time.Second)

	crCount := 0
	ctrlUCount := 0
	for _, b := range out {
		switch b {
		case byteCR:
			crCount++
		case byteCtrlU:
			ctrlUCount++
		}
	}
	require.Equal(t, 2, crCount, "expected one CR each for ls and whoami")
	require.Equal(t, 1, ctrlUCount, "expected one Ctrl-U for the denied rm -rf /")
	require.Equal(t, []string{"ls", "rm -rf /", "whoami"}, gate.seen)
}

func TestTermManagerGateCRLF(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	tm.On()
	gate := &fakeGate{}
	tm.SetCommandGate(gate)
	defer tm.Close()

	r, w := io.Pipe()
	tm.AddReader("reader", r)
	go w.Write([]byte("ls\r\n"))

	out := readAllWithin(t, tm, 5*time.Second)

	crCount := 0
	for _, b := range out {
		if b == byteCR {
			crCount++
		}
	}
	require.Equal(t, 1, crCount, "CRLF should yield exactly one CR, not two")
	require.Equal(t, []string{"ls"}, gate.seen)
}

func TestTermManagerGateCRLFSplitAcrossReads(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	tm.On()
	gate := &fakeGate{}
	tm.SetCommandGate(gate)
	defer tm.Close()

	r, w := io.Pipe()
	tm.AddReader("reader", r)
	// Split the CRLF across two PTY reads: \r ends the first read, \n begins
	// the second. The \n must be collapsed as the second half of the CRLF, not
	// treated as a fresh blank-line terminator.
	go func() {
		w.Write([]byte("ls\r"))
		w.Write([]byte("\n"))
	}()

	out := readAllWithin(t, tm, 5*time.Second)

	crCount := 0
	for _, b := range out {
		if b == byteCR {
			crCount++
		}
	}
	require.Equal(t, 1, crCount, "CRLF split across reads should yield exactly one CR, not two")
	require.Equal(t, []string{"ls"}, gate.seen)
}

func TestTermManagerGateCRLFSplitMultiple(t *testing.T) {
	t.Parallel()

	tm := NewTermManager()
	tm.On()
	gate := &fakeGate{}
	tm.SetCommandGate(gate)
	defer tm.Close()

	r, w := io.Pipe()
	tm.AddReader("reader", r)
	// Each CRLF is split across read boundaries: "a\r" | "\nb\r" | "\n".
	go func() {
		w.Write([]byte("a\r"))
		w.Write([]byte("\nb\r"))
		w.Write([]byte("\n"))
	}()

	out := readAllWithin(t, tm, 5*time.Second)

	crCount := 0
	for _, b := range out {
		if b == byteCR {
			crCount++
		}
	}
	require.Equal(t, 2, crCount, "two CRLFs split across reads should yield exactly two CR, not extras")
	require.Equal(t, []string{"a", "b"}, gate.seen)
}
