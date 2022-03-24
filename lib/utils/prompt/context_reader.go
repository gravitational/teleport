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

package prompt

import (
	"context"
	"errors"
	"io"
	"sync"
)

// ErrReaderClosed is returned from ContextReader.Read after it was closed.
var ErrReaderClosed = errors.New("ContextReader has been closed")

// ContextReader is a wrapper around io.Reader where each individual
// ReadContext call can be canceled using a context.
type ContextReader struct {
	r     io.Reader
	data  chan []byte
	close chan struct{}

	mu  sync.RWMutex
	err error
}

// NewContextReader creates a new ContextReader wrapping r. Callers should not
// use r after creating this ContextReader to avoid loss of data (the last read
// will be lost).
//
// Callers are responsible for closing the ContextReader to release associated
// resources.
func NewContextReader(r io.Reader) *ContextReader {
	cr := &ContextReader{
		r:     r,
		data:  make(chan []byte),
		close: make(chan struct{}),
	}
	go cr.read()
	return cr
}

func (r *ContextReader) setErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		// Keep only the first encountered error.
		return
	}
	r.err = err
}

func (r *ContextReader) getErr() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.err
}

func (r *ContextReader) read() {
	defer close(r.data)

	for {
		// Allocate a new buffer for every read because we need to send it to
		// another goroutine.
		buf := make([]byte, 4*1024) // 4kB, matches Linux page size.
		n, err := r.r.Read(buf)
		r.setErr(err)
		buf = buf[:n]
		if n == 0 {
			return
		}
		select {
		case <-r.close:
			return
		case r.data <- buf:
		}
	}
}

// ReadContext returns the next chunk of output from the reader. If ctx is
// canceled before any data is available, ReadContext will return too. If r
// was closed, ReadContext will return immediately with ErrReaderClosed.
func (r *ContextReader) ReadContext(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.close:
		// Close was called, unblock immediately.
		// r.data might still be blocked if it's blocked on the Read call.
		return nil, r.getErr()
	case buf, ok := <-r.data:
		if !ok {
			// r.data was closed, so the read goroutine has finished.
			// No more data will be available, return the latest error.
			return nil, r.getErr()
		}
		return buf, nil
	}
}

// Close releases the background resources of r. All ReadContext calls will
// unblock immediately.
func (r *ContextReader) Close() {
	select {
	case <-r.close:
		// Already closed, do nothing.
		return
	default:
		close(r.close)
		r.setErr(ErrReaderClosed)
	}
}
