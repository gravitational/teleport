/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
