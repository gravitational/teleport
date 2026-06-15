/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package events

import (
	"container/heap"
	"context"
	"io"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// MergeStreams merges N individually time-sorted event streams into a single
// stream ordered by event time, using a k-way merge with a min-heap.
//
// The inputs are the same channel types returned by SessionStreamer.StreamSessionEvents.
// The output follows the same contract: events are sent on the first returned channel
// until it is closed; any fatal error is sent on the error channel before it is closed.
//
// len(streams) must equal len(errs). Passing mismatched slices panics.
func MergeStreams(
	ctx context.Context,
	streams []<-chan apievents.AuditEvent,
	errs []<-chan error,
) (<-chan apievents.AuditEvent, <-chan error) {
	if len(streams) != len(errs) {
		panic("MergeStreams: len(streams) must equal len(errs)")
	}

	outEvts := make(chan apievents.AuditEvent, 64)
	outErr := make(chan error, 1)

	go func() {
		// Close outEvts before outErr so consumers that drain outEvts first
		// (the standard contract) see a closed events channel before checking
		// the error channel. LIFO defers: outErr registered first runs last.
		defer close(outErr)
		defer close(outEvts)

		h := &eventHeap{}
		heap.Init(h)

		// Seed: pull the first event from each stream sequentially.
		for i := range streams {
			evt, err := nextEvent(ctx, streams[i], errs[i])
			if err == io.EOF {
				continue // stream is empty
			}
			if err != nil {
				outErr <- err
				return
			}
			heap.Push(h, heapItem{event: evt, idx: i})
		}

		for h.Len() > 0 {
			item := heap.Pop(h).(heapItem)

			select {
			case outEvts <- item.event:
			case <-ctx.Done():
				outErr <- ctx.Err()
				return
			}

			evt, err := nextEvent(ctx, streams[item.idx], errs[item.idx])
			if err == io.EOF {
				continue // this stream is exhausted; don't re-push
			}
			if err != nil {
				outErr <- err
				return
			}
			heap.Push(h, heapItem{event: evt, idx: item.idx})
		}
	}()

	return outEvts, outErr
}

// nextEvent reads one event from a stream, respecting errors and context cancellation.
// Returns io.EOF when the events channel is closed with no error.
//
// The error channel is only read when the events channel is already closed:
// a non-nil error on errs is returned as-is; a nil error or a closed channel
// both map to io.EOF. This ordering ensures buffered events are never skipped
// due to a ready error channel.
func nextEvent(ctx context.Context, evts <-chan apievents.AuditEvent, errs <-chan error) (apievents.AuditEvent, error) {
	select {
	case evt, ok := <-evts:
		if !ok {
			// Events channel closed. Check for a pending error without blocking.
			select {
			case err := <-errs:
				if err != nil {
					return nil, err
				}
				return nil, io.EOF
			default:
				return nil, io.EOF
			}
		}
		return evt, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// heapItem holds a buffered event alongside the index of the stream it came from.
type heapItem struct {
	event apievents.AuditEvent
	idx   int
}

// eventHeap is a min-heap of heapItems ordered by event time (earliest first).
type eventHeap []heapItem

func (h eventHeap) Len() int           { return len(h) }
func (h eventHeap) Less(i, j int) bool { return h[i].event.GetTime().Before(h[j].event.GetTime()) }
func (h eventHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *eventHeap) Push(x any) { *h = append(*h, x.(heapItem)) }

func (h *eventHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
