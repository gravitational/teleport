// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package itertools

import (
	"errors"
)

// ErrCannotReduceBatchSize is returned when an attempt is made to reduce the batch size
// below the minimum allowed size.
var ErrCannotReduceBatchSize = errors.New("cannot reduce batch size further")

// DynamicBatchSizeIterator provides an iterator over batches of items with dynamic size adjustment.
// It supports batch size reduction for retry scenarios via the ReduceSize() method.
type DynamicBatchSizeIterator[T any] struct {
	items        []T
	currentSize  int
	currentChunk []T
}

// DynamicBatchSize creates a new iterator that yields batches of items with dynamic size adjustment.
//
// Example usage:
//
//	chunks := itertools.DynamicBatchSize(items, 1000)
//	for chunks.Next() {
//	    chunk := chunks.Chunk()
//	    if err := processBatch(chunk); err != nil {
//	        if shouldReduceSize(err) {
//	            if err := chunks.ReduceSize(); err != nil {
//	                return err
//	            }
//	        }
//	    }
//	}
func DynamicBatchSize[T any](items []T, defaultChunkSize int) *DynamicBatchSizeIterator[T] {
	if defaultChunkSize <= 0 {
		defaultChunkSize = 1
	}
	return &DynamicBatchSizeIterator[T]{
		items:       items,
		currentSize: defaultChunkSize,
	}
}

// Next advances the iterator to the next batch. Returns true if there is a batch available,
// false if the iterator is exhausted. If ReduceSize() was called on the previous iteration,
// Next() will retry the current batch with the reduced size.
func (it *DynamicBatchSizeIterator[T]) Next() bool {
	// If currentChunk is not nil, advance past it
	if it.currentChunk != nil {
		it.items = it.items[len(it.currentChunk):]
	}

	if len(it.items) == 0 {
		return false
	}

	end := min(it.currentSize, len(it.items))
	it.currentChunk = it.items[:end]
	return true
}

// Chunk returns the current batch of items. Should only be called after Next() returns true.
func (it *DynamicBatchSizeIterator[T]) Chunk() []T {
	return it.currentChunk
}

// ReduceSize reduces the batch size by half and signals that the current batch should be retried.
// Returns ErrCannotReduceBatchSize if the batch size cannot be reduced further.
func (it *DynamicBatchSizeIterator[T]) ReduceSize() error {
	if len(it.currentChunk) <= 1 {
		return ErrCannotReduceBatchSize
	}
	it.currentSize = (it.currentSize + 1) / 2
	it.currentChunk = nil
	return nil
}
