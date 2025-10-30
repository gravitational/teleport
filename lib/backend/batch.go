/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package backend

import (
	"context"
	"slices"

	"github.com/gravitational/trace"
)

// RevisionSetter is an interface for objects that can have their revision updated
// after a successful backend write operation.
type revisionSetter interface {
	SetRevision(rev string)
}

// atomicWriteItem pairs a backend conditional action with an object that needs
// its revision updated after to write succeeds.
type writeItem struct {
	item           Item
	revisionSetter revisionSetter
}

// writeItems represents a collection of items to be written
// to the backend in chunks.
type writeItems []*writeItem

// toConditionalActions extracts the conditional actions from the batch items.
func (w writeItems) toConditionalActions() []ConditionalAction {
	actions := make([]ConditionalAction, 0, len(w))
	for _, v := range w {
		actions = append(actions, ConditionalAction{
			Key:       v.item.Key,
			Condition: Whatever(),
			Action:    Put(v.item),
		})
	}
	return actions
}

// updateRevisions updates the revision on all items in the batch after
// a successful atomic write.
func (w writeItems) updateRevisions(revision string) {
	for _, item := range w {
		item.revisionSetter.SetRevision(revision)
	}
}

// NewBatchWriter creates a new batch writer for performing chunked writes.
// that saves the network roundups.
func NewBatchWriter() *BatchWriter {
	return &BatchWriter{
		items: make(writeItems, 0),
	}
}

// BatchWriter provides a builder pattern for constructing and executing
// batch writes.
type BatchWriter struct {
	items writeItems
}

// Add appends a new conditional action with its associated revision setter to the batch.
// The revisionSetter can be nil if revision tracking is not needed for this item.
func (w *BatchWriter) Add(item Item, revisionSetter revisionSetter) {
	w.items = append(w.items, &writeItem{
		item:           item,
		revisionSetter: revisionSetter,
	})
}

// Execute performs the batch write operation.
func (w *BatchWriter) Execute(ctx context.Context, bk Backend) error {
	switch {
	default:
		return executeBatchWriteViaAtomicPUTWrite(ctx, bk, w.items)
	}
}

func executeBatchWriteViaAtomicPUTWrite(ctx context.Context, bk Backend, items writeItems) error {
	for ch := range slices.Chunk(items, MaxAtomicWriteSize) {
		revision, err := bk.AtomicWrite(ctx, ch.toConditionalActions())
		if err != nil {
			return trace.Wrap(err)
		}
		ch.updateRevisions(revision)
	}
	return nil
}
