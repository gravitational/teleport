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

package utils

import (
	"sync"

	"github.com/gravitational/trace"
)

// CircularBuffer implements an in-memory circular buffer of predefined size
type CircularBuffer struct {
	sync.Mutex
	buf   []float64
	start int
	end   int
	size  int
}

// NewCircularBuffer returns a new instance of a circular buffer that will hold
// size elements before it rotates
func NewCircularBuffer(size int) (*CircularBuffer, error) {
	if size <= 0 {
		return nil, trace.BadParameter("circular buffer size should be > 0")
	}
	buf := &CircularBuffer{
		buf:   make([]float64, size),
		start: -1,
		end:   -1,
		size:  0,
	}
	return buf, nil
}

// Data returns the most recent n elements in the correct order
func (t *CircularBuffer) Data(n int) []float64 {
	t.Lock()
	defer t.Unlock()

	if n <= 0 || t.size == 0 {
		return nil
	}

	// skip first N items so that the most recent are always provided
	start := t.start
	if n < t.size {
		start = (t.start + (t.size - n)) % len(t.buf)
	}

	if start <= t.end {
		return t.buf[start : t.end+1]
	}

	return append(t.buf[start:], t.buf[:t.end+1]...)
}

// Add pushes a new item onto the buffer
func (t *CircularBuffer) Add(d float64) {
	t.Lock()
	defer t.Unlock()

	if t.size == 0 {
		t.start = 0
		t.end = 0
		t.size = 1
	} else if t.size < len(t.buf) {
		t.end++
		t.size++
	} else {
		t.end = t.start
		t.start = (t.start + 1) % len(t.buf)
	}

	t.buf[t.end] = d
}
