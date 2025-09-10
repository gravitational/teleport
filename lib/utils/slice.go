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
	"bytes"
	"sync"
)

// SlicePool manages a pool of slices
// in attempts to manage memory in go more efficiently
// and avoid frequent allocations
type SlicePool interface {
	// Zero zeroes slice
	Zero(b []byte)
	// Get returns a new or already allocated slice
	Get() []byte
	// Put returns slice back to the pool
	Put(b []byte)
	// Size returns a slice size
	Size() int64
}

// NewSliceSyncPool returns a new slice pool, using sync.Pool
// of pre-allocated or newly allocated slices of the predefined size and capacity
func NewSliceSyncPool(sliceSize int64) *SliceSyncPool {
	s := &SliceSyncPool{
		sliceSize: sliceSize,
		zeroSlice: make([]byte, sliceSize),
	}
	s.New = func() any {
		slice := make([]byte, s.sliceSize)
		return &slice
	}
	return s
}

// SliceSyncPool is a sync pool of slices (usually large)
// of the same size to optimize memory usage, see sync.Pool for more details
type SliceSyncPool struct {
	sync.Pool
	sliceSize int64
	zeroSlice []byte
}

// Zero zeroes slice of any length
func (s *SliceSyncPool) Zero(b []byte) {
	if len(b) <= len(s.zeroSlice) {
		// zero all bytes in the slice to avoid
		// data lingering in memory
		copy(b, s.zeroSlice[:len(b)])
	} else {
		// use working, but less optimal implementation
		for i := range b {
			b[i] = 0
		}
	}
}

// Get returns a new or already allocated slice
func (s *SliceSyncPool) Get() []byte {
	pslice := s.Pool.Get().(*[]byte)
	return *pslice
}

// Put returns slice back to the pool
func (s *SliceSyncPool) Put(b []byte) {
	s.Zero(b)
	s.Pool.Put(&b)
}

// Size returns a slice size
func (s *SliceSyncPool) Size() int64 {
	return s.sliceSize
}

// NewBufferSyncPool returns a new instance of sync pool of bytes.Buffers
// that creates new buffers with preallocated underlying buffer of size
func NewBufferSyncPool(size int64) *BufferSyncPool {
	return &BufferSyncPool{
		size: size,
		Pool: sync.Pool{
			New: func() any {
				return bytes.NewBuffer(make([]byte, size))
			},
		},
	}
}

// BufferSyncPool is a sync pool of bytes.Buffer
type BufferSyncPool struct {
	sync.Pool
	size int64
}

// Put resets the buffer (does not free the memory)
// and returns it back to the pool. Users should be careful
// not to use the buffer (e.g. via Bytes) after it was returned
func (b *BufferSyncPool) Put(buf *bytes.Buffer) {
	buf.Reset()
	b.Pool.Put(buf)
}

// Get returns a new or already allocated buffer
func (b *BufferSyncPool) Get() *bytes.Buffer {
	return b.Pool.Get().(*bytes.Buffer)
}

// Size returns default allocated buffer size
func (b *BufferSyncPool) Size() int64 {
	return b.size
}

// FromSlice converts the provided slice to a map using the key function
// to determine the appropriate key per entry. If any duplicates
// exist in the slice, then the entry with the lowest index is used.
func FromSlice[T any](r []T, key func(T) string) map[string]T {
	out := make(map[string]T, len(r))

	// there may be duplicate resources in the input list.
	// by iterating from end to start, the first resource of given name wins.
	for i := len(r) - 1; i >= 0; i-- {
		res := r[i]
		out[key(res)] = res
	}

	return out
}
