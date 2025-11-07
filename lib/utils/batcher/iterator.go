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

package batcher

import (
	"errors"
	"iter"
)

// ErrCannotReduceBatchSize is returned when an attempt is made to reduce the batch size
// below the minimum allowed size.
var ErrCannotReduceBatchSize = errors.New("cannot reduce batch size further")

// DynamicBatchSizeIter returns an iterator that yields batches of items with dynamic size adjustment.
// It supports batch size reduction for retry scenarios via the Batch.Retry() method.
//
// Example usage:
//
//	for batch := range batcher.DynamicBatchSizeIter(items, 100) {
//	    if err := processBatch(batch.Items); err != nil {
//	        if shouldReduceSize(err) {
//	            if err := batch.ReduceBatchSizeByHalf(); err != nil {
//	                return err
//	            }
//	            continue
//	        }
//	        return err
//	    }
//	}
func DynamicBatchSizeIter[T any](items []T, defaultChunkSize int) iter.Seq[*Batch[T]] {
	return func(yield func(*Batch[T]) bool) {
		if len(items) == 0 {
			return
		}
		state := &batchState{
			currentSize: min(len(items), defaultChunkSize),
			shouldRetry: false,
		}
		for len(items) > 0 {
			end := min(state.currentSize, len(items))
			chunk := items[:end]
			if !yield(&Batch[T]{Items: chunk, state: state}) {
				return
			}
			if state.shouldRetry {
				state.shouldRetry = false
				continue
			}
			items = items[end:]
		}
	}
}

// Batch represents a batch of items with control methods for retry logic.
type Batch[T any] struct {
	Items []T
	// state holds internal state for batch size management
	// needs to be a pointer to allow shared state across flow
	state *batchState
}

// ReduceBatchSizeByHalf reduces the batch size and signals that this batch should be retried
// with a smaller size. Returns an error if the batch size cannot be reduced further.
func (b *Batch[T]) ReduceBatchSizeByHalf() error {
	return b.state.reduceBatchSize()
}

type batchState struct {
	shouldRetry bool
	currentSize int
}

func (bs *batchState) reduceBatchSize() error {
	if bs.currentSize <= 1 {
		return ErrCannotReduceBatchSize
	}
	bs.currentSize = (bs.currentSize + 1) / 2
	bs.shouldRetry = true
	return nil
}
