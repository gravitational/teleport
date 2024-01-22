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

// Package escape implements client-side escape character logic.
// This logic mimics OpenSSH: https://man.openbsd.org/ssh#ESCAPE_CHARACTERS.
package escape

import (
	"errors"
	"io"
	"sync"
)

const (
	readerBufferLimit = 10 * 1024 * 1024 // 10MB

	// Note: on a raw terminal, "\r\n" is needed to move a cursor to the start
	// of next line.
	helpText = "\r\ntsh escape characters:\r\n  ~? - display a list of escape characters\r\n  ~. - disconnect\r\n"
)

var (
	// ErrDisconnect is returned when the user has entered a disconnect
	// sequence, requesting connection to be interrupted.
	ErrDisconnect = errors.New("disconnect escape sequence detected")
	// ErrTooMuchBufferedData is returned when the Reader's internal buffer has
	// filled over 10MB. Either the consumer of Reader can't keep up with the
	// data or it's entirely stuck and not consuming the data.
	ErrTooMuchBufferedData = errors.New("internal buffer has grown too big")
)

// Reader is an io.Reader wrapper that catches OpenSSH-like escape sequences in
// the input stream. See NewReader for more info.
//
// Reader is safe for concurrent use.
type Reader struct {
	inner        io.Reader
	out          io.Writer
	onDisconnect func(error)
	bufferLimit  int

	// cond protects buf and err and also announces to blocked readers that
	// more data is available.
	cond sync.Cond
	buf  []byte
	err  error
}

// NewReader creates a new Reader to catch escape sequences from 'in'.
//
// Two sequences are supported:
//   - "~?": prints help text to 'out' listing supported sequences
//   - "~.": disconnect stops any future reads from in; after this sequence,
//     callers can still read any unread data up to this sequence from Reader but
//     all future Read calls will return ErrDisconnect; onDisconnect will also be
//     called with ErrDisconnect immediately
//
// NewReader starts consuming 'in' immediately in the background. This allows
// Reader to detect sequences without the caller actively calling Read (such as
// when it's stuck writing out the received data).
//
// Unread data is accumulated in an internal buffer. If this buffer grows to a
// limit (currently 10MB), Reader will stop permanently. onDisconnect will get
// called with ErrTooMuchBufferedData. Read can still be called to consume the
// internal buffer but all future reads after that will return
// ErrTooMuchBufferedData.
//
// If the internal buffer is empty, calls to Read will block until some data is
// available or an error occurs.
func NewReader(in io.Reader, out io.Writer, onDisconnect func(error)) *Reader {
	r := newUnstartedReader(in, out, onDisconnect)
	go r.runReads()
	return r
}

// newUnstartedReader allows unit tests to mutate Reader before runReads
// starts.
func newUnstartedReader(in io.Reader, out io.Writer, onDisconnect func(error)) *Reader {
	return &Reader{
		inner:        in,
		out:          out,
		onDisconnect: onDisconnect,
		bufferLimit:  readerBufferLimit,
		cond:         sync.Cond{L: &sync.Mutex{}},
		// note: no need to pre-allocate buf, it will allocate and grow as
		// needed in runReads via append.
	}
}

func (r *Reader) runReads() {
	readBuf := make([]byte, 1024)
	// writeBuf is a copy of data in readBuf after filtering out any escape
	// sequences.
	writeBuf := make([]byte, 0, 1024)
	// newLine is set iff the previous character was a newline.
	// escape is set iff the two previous characters were a newline and '~'.
	//
	// Note: at most one of these is ever set. When escape is true, then
	// newLine is false.
	newLine, escape := true, false
	for {
		n, err := r.inner.Read(readBuf)
		if err != nil {
			r.setErr(err)
			return
		}

		// Reset the output buffer from previous state.
		writeBuf = writeBuf[:0]
	inner:
		for _, b := range readBuf[:n] {
			// Note: this switch only filters and updates newLine and escape.
			// b is written to writeBuf afterwards.
			switch b {
			case '\r', '\n':
				if escape {
					// An incomplete escape sequence, send out a '~' that was
					// previously suppressed.
					writeBuf = append(writeBuf, '~')
				}
				newLine, escape = true, false
			case '~':
				if newLine {
					// Start escape sequence, don't write the '~' just yet.
					newLine, escape = false, true
					continue inner
				} else if escape {
					newLine, escape = false, false
				}
			case '?':
				if escape {
					// Complete help sequence.
					r.printHelp()
					newLine, escape = false, false
					continue inner
				}
				newLine = false
			case '.':
				if escape {
					// Complete disconnect sequence.
					r.setErr(ErrDisconnect)
					return
				}
				newLine = false
			default:
				if escape {
					// An incomplete escape sequence, send out a '~' that was
					// previously suppressed.
					writeBuf = append(writeBuf, '~')
				}
				newLine, escape = false, false
			}
			// Write the character out as-is, it wasn't filtered out above.
			writeBuf = append(writeBuf, b)
		}

		// Add new data to internal buffer.
		r.cond.L.Lock()
		if len(r.buf)+len(writeBuf) > r.bufferLimit {
			// Unlock because setErr will want to lock too.
			r.cond.L.Unlock()
			r.setErr(ErrTooMuchBufferedData)
			return
		}
		r.buf = append(r.buf, writeBuf...)
		// Notify blocked Read calls about new data.
		r.cond.Broadcast()
		r.cond.L.Unlock()
	}
}

// Read fills buf with available data. If no data is available, Read will
// block.
func (r *Reader) Read(buf []byte) (int, error) {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()
	// Block until some data was read in runReads.
	for len(r.buf) == 0 && r.err == nil {
		r.cond.Wait()
	}

	// Have some data to return.
	n := len(r.buf)
	if n > len(buf) {
		n = len(buf)
	}
	// Write n available bytes to buf and trim them from r.buf.
	copy(buf, r.buf[:n])
	r.buf = r.buf[n:]

	return n, r.err
}

func (r *Reader) setErr(err error) {
	r.cond.L.Lock()
	r.err = err
	r.cond.Broadcast()
	// Skip EOF, it's a normal clean exit.
	if err != io.EOF {
		r.onDisconnect(err)
	}
	r.cond.L.Unlock()
}

func (r *Reader) printHelp() {
	r.out.Write([]byte(helpText))
}
