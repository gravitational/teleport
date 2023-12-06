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

package integration

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"time"
)

// Terminal emulates stdin+stdout for integration testing
type Terminal struct {
	typed   chan byte
	close   chan struct{}
	mu      *sync.Mutex
	written *bytes.Buffer
}

func NewTerminal(capacity int) *Terminal {
	return &Terminal{
		typed:   make(chan byte, capacity),
		mu:      &sync.Mutex{},
		written: bytes.NewBuffer(nil),
		close:   make(chan struct{}),
	}
}

func (t *Terminal) Type(data string) {
	for _, b := range []byte(data) {
		t.typed <- b
	}
}

// Output returns a number of first 'limit' bytes printed into this fake terminal
func (t *Terminal) Output(limit int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	buff := t.written.Bytes()
	if len(buff) > limit {
		buff = buff[:limit]
	}
	// clean up white space for easier comparison:
	return strings.TrimSpace(string(buff))
}

// AllOutput returns the entire recorded output from the fake terminal
func (t *Terminal) AllOutput() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimSpace(t.written.String())
}

func (t *Terminal) Write(data []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.written.Write(data)
}

func (t *Terminal) Read(p []byte) (n int, err error) {
	for n = 0; n < len(p); n++ {
		select {
		case p[n] = <-t.typed:
		case <-t.close:
			return n, io.EOF
		}

		if p[n] == '\r' {
			break
		}
		if p[n] == '\a' { // 'alert' used for debugging, means 'pause for 1 second'
			select {
			case <-time.After(time.Second):
				n--
			case <-t.close:
				return n, io.EOF
			}
		}

		select {
		case <-time.After(time.Millisecond * 10):
		case <-t.close:
			return n, io.EOF
		}

	}
	return n, nil
}

func (t *Terminal) Close() error {
	select {
	case <-t.close:
	default:
		close(t.close)
	}

	return nil
}
