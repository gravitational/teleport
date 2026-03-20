// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import "sync"

// buffer adapted from go/src/fmt/print.go
type buffer []byte

// Having an initial size gives a dramatic speedup.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 1024)
		return (*buffer)(&b)
	},
}

func newBuffer() *buffer {
	return bufPool.Get().(*buffer)
}

func (b *buffer) Len() int {
	return len(*b)
}

func (b *buffer) SetLen(n int) {
	*b = (*b)[:n]
}

func (b *buffer) Free() {
	// To reduce peak allocation, return only smaller buffers to the pool.
	const maxBufferSize = 16 << 10
	if cap(*b) <= maxBufferSize {
		*b = (*b)[:0]
		bufPool.Put(b)
	}
}

func (b *buffer) Reset() {
	*b = (*b)[:0]
}

func (b *buffer) Write(p []byte) (int, error) {
	*b = append(*b, p...)
	return len(p), nil
}

func (b *buffer) WriteString(s string) (int, error) {
	*b = append(*b, s...)
	return len(s), nil
}

func (b *buffer) WriteByte(c byte) error {
	*b = append(*b, c)
	return nil
}

func (b *buffer) String() string {
	return string(*b)
}
