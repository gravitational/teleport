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

package desktop

import (
	"cmp"
	"context"
	"math"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types/events"
)

const defaultMaxEventsPerBucket = 1000

// fileOperationsKey uniquely identifies a set of common file operations
type fileOperationsKey struct {
	path        string
	directoryID directoryID
	write       bool
}

// fileOperationEvent is an abstraction of read/write events
// so that we need only one compactor implementation.
type fileOperationEvent interface {
	Base() events.AuditEvent
	IsWriteEvent() bool
	GetDirectoryID() directoryID
	GetPath() string
	GetOffset() uint64
	GetLength() uint64
	SetLength(uint64)
}

// fileOperationsBucket identifies a set of reads/writes
// to a particular file within some period of time.
type fileOperationsBucket struct {
	expireTime time.Time
	events     fileOperationEvents
	timer      *time.Timer
	done       chan struct{}
}

// auditCompactor retains read and write events to a given file for a period of time before
// emitting them to the audit log. Once the timeout period expires, contiguous read/write events are
// compacted into a single audit event and emitted.
type auditCompactor struct {
	// refreshInterval defines how long a bucket should wait for a subsequent
	// file operation to arrive before compacting and emitting its audit event(s).
	refreshInterval time.Duration
	// maxDelayInterval defines the maximum length of time that a bucket should wait
	// before before compacting and emitting its audit event(s)
	// this prevents a slow trickle of read/write events within the refreshInterval from
	// indefinitely delaying audit events from being emitted.
	maxDelayInterval time.Duration
	emitFn           func(context.Context, events.AuditEvent)
	buckets          map[fileOperationsKey]*fileOperationsBucket
	bucketsLock      sync.Mutex
	// flushing is set of buckets that are currently running compaction and emitting events.
	// Used to guarantee that teardown blocks until all timer goroutines are complete.
	flushing map[*fileOperationsBucket]struct{}
	// maxEventsPerBucket is a hard limit on how many events will be aggregated for compaction.
	// The bucket is immediately flushed/compacted upon hitting this limit.
	// zero or negative values are treated as "unlimited".
	maxEventsPerBucket int
}

type fileOperationEntry struct {
	marked bool
	fileOperationEvent
}

type fileOperationEvents []fileOperationEntry

func (f *fileOperationEvents) insert(e fileOperationEntry) {
	*f = append(*f, e)
}

func (f fileOperationEvents) compact() (out []fileOperationEvent) {
	// Sort by offset in ascending order
	slices.SortFunc(f, func(a, c fileOperationEntry) int {
		if a.GetOffset() != c.GetOffset() {
			return cmp.Compare(a.GetOffset(), c.GetOffset())
		}
		// If offsets match, place the larger segment first
		// for greedy matching.
		return cmp.Compare(c.GetLength(), a.GetLength())
	})

	for i := range f {
		currentBase := &f[i]
		if currentBase.marked {
			continue
		}

		currentBase.marked = true
		consecutive := []fileOperationEvent{currentBase.fileOperationEvent}
		nextOffset := currentBase.GetOffset() + currentBase.GetLength()

		// Search the remainder of the list for consecutive segments
		// which are candidates for compaction.
		for j := i + 1; j < len(f); j++ {
			candidateSegment := &f[j]
			if candidateSegment.GetOffset() > nextOffset {
				// No need to continue. There are no more candidate segments.
				break
			}

			if candidateSegment.marked {
				// Skip events that have already been consumed by
				// previous iterations.
				continue
			}

			if candidateSegment.GetOffset() == nextOffset {
				candidateSegment.marked = true
				nextOffset += candidateSegment.GetLength()
				consecutive = append(consecutive, candidateSegment.fileOperationEvent)
			}
		}
		// use the 'compact' helper to actually combine these events into one.
		// It handles cases where our read/write lengths would overflow a uint32.
		out = append(out, compact(consecutive...)...)
	}
	return
}

func newAuditCompactor(refreshInterval, maxDelayInterval time.Duration, maxEventsPerBucket int, emitFn func(context.Context, events.AuditEvent)) auditCompactor {
	return auditCompactor{
		refreshInterval:    refreshInterval,
		maxDelayInterval:   maxDelayInterval,
		maxEventsPerBucket: maxEventsPerBucket,
		emitFn:             emitFn,
		buckets:            map[fileOperationsKey]*fileOperationsBucket{},
		flushing:           map[*fileOperationsBucket]struct{}{},
	}
}

// Assumes that the provided fileOperations are consecutive.
// Typically, returns a slice of length 1, but may return length >1
// if the compacted length exceeds math.MaxUint32.
func compact(op ...fileOperationEvent) []fileOperationEvent {
	if len(op) == 0 {
		return []fileOperationEvent{}
	}

	base := op[0]
	out := []fileOperationEvent{base}
	for _, nextSegment := range op[1:] {
		nextLength := base.GetLength() + nextSegment.GetLength()
		if nextLength > math.MaxUint32 {
			// The edge case where we need to return multiple
			// events
			base = nextSegment
			out = append(out, nextSegment)
			continue
		}
		base.SetLength(nextLength)
	}
	return out
}

func (s *fileOperationsBucket) emitEvents(ctx context.Context, emitFn func(ctx context.Context, event events.AuditEvent)) {
	for _, event := range s.events.compact() {
		emitFn(ctx, event.Base())
	}
}

func (s *fileOperationsBucket) addEvent(event fileOperationEvent) {
	s.events.insert(fileOperationEntry{fileOperationEvent: event})
}

func (a *auditCompactor) handleEvent(ctx context.Context, event fileOperationEvent) {
	// 0 length events must skip compaction entirely
	if event.GetLength() == 0 {
		a.emitFn(ctx, event.Base())
		return
	}

	// File Operations are grouped by directoryID, path, and read vs write
	key := fileOperationsKey{
		write:       event.IsWriteEvent(),
		directoryID: event.GetDirectoryID(),
		path:        event.GetPath(),
	}

	newBucket := true
	a.bucketsLock.Lock()
	defer a.bucketsLock.Unlock()

	if bucket, exists := a.buckets[key]; exists {
		// We're currently tracking this bucket
		// Temporarily stop the timer (if possible)
		alreadyFired := !bucket.timer.Stop()
		if !alreadyFired {
			if len(bucket.events) == a.maxEventsPerBucket {
				// This bucket is full. Add it to the set of flushing buckets
				// and start flushing it asynchronously.
				// We'll create a fresh bucket to handle the current
				// event below.
				delete(a.buckets, key)
				a.flushing[bucket] = struct{}{}
				go a.flushBucket(ctx, bucket)
			} else {
				// Update the current bucket. It is a continuation of the current bucket
				// and the timer has not yet fired for it.
				bucket.addEvent(event)
				// Reset the timer either to the refresh interval, or until
				// the buckets's expiration time
				bucket.timer.Reset(time.Duration(math.Min(float64(a.refreshInterval), float64(time.Until(bucket.expireTime)))))
				newBucket = false
			}
		} else {
			// The timer has already fired. Stop tracking this bucket.
			// A new bucket will be created below to handle this event.
			delete(a.buckets, key)
			a.flushing[bucket] = struct{}{}
		}
	}

	// We need to create a new bucket due to one of the following:
	//   - We are not tracking any such bucket yet.
	//   - We were tracking this bucket but the timer has already fired.
	//   - We were tracking this bucket but it was full.
	if newBucket {
		bucket := &fileOperationsBucket{
			done:       make(chan struct{}),
			expireTime: time.Now().Add(a.maxDelayInterval),
			events:     fileOperationEvents{fileOperationEntry{fileOperationEvent: event}},
		}
		bucket.timer = time.AfterFunc(a.refreshInterval, func() {
			a.bucketsLock.Lock()
			// Remove from the *live* map of buckets
			// and add to the set of 'flushing' buckets.
			delete(a.buckets, key)
			a.flushing[bucket] = struct{}{}
			a.bucketsLock.Unlock()

			a.flushBucket(ctx, bucket)
		})
		a.buckets[key] = bucket
	}
}

// compact and emit bucket events, then remove the bucket from the
// 'flushing' set. Acquires 'bucketsLock'.
func (a *auditCompactor) flushBucket(ctx context.Context, f *fileOperationsBucket) {
	defer close(f.done)
	f.emitEvents(ctx, a.emitFn)
	a.bucketsLock.Lock()
	delete(a.flushing, f)
	a.bucketsLock.Unlock()
}

// flush immediately compacts and emits audit events for all
// unexpired buckets and blocks until completion.
func (a *auditCompactor) flush(ctx context.Context) {
	wait := []chan struct{}{}
	a.bucketsLock.Lock()
	for bucketKey, bucket := range a.buckets {
		if bucket.timer.Stop() {
			// If we successfully stop the timer before it fires,
			// go ahead and emit the audit event.
			delete(a.buckets, bucketKey)
			go a.flushBucket(ctx, bucket)
		}
		wait = append(wait, bucket.done)
	}

	// Some buckets are already running compaction. Wait on them
	// to finish as well.
	for bucket := range a.flushing {
		wait = append(wait, bucket.done)
	}
	// Unlock so that we may unblock timer functions.
	a.bucketsLock.Unlock()
	// Wait for pending timers to complete
	// We use our own "done" channel rather than the timer's
	// because we need to know that the timer's underlying goroutine.
	for _, doneChan := range wait {
		<-doneChan
	}
}

// Adapters for current read/write audit events.

type readEvent struct {
	*events.DesktopSharedDirectoryRead
}

func toUint32(len uint64) uint32 {
	return uint32(min(math.MaxUint32, len))
}

func (r *readEvent) SetLength(len uint64)        { r.Length = toUint32(len) }
func (r *readEvent) GetLength() uint64           { return uint64(r.Length) }
func (r *readEvent) GetOffset() uint64           { return r.Offset }
func (r *readEvent) GetPath() string             { return r.Path }
func (r *readEvent) IsWriteEvent() bool          { return false }
func (r *readEvent) GetDirectoryID() directoryID { return directoryID(r.DirectoryID) }
func (r *readEvent) Base() events.AuditEvent     { return r.DesktopSharedDirectoryRead }

type writeEvent struct {
	*events.DesktopSharedDirectoryWrite
}

func (r *writeEvent) SetLength(len uint64)        { r.Length = toUint32(len) }
func (r *writeEvent) GetLength() uint64           { return uint64(r.Length) }
func (r *writeEvent) GetOffset() uint64           { return r.Offset }
func (r *writeEvent) GetPath() string             { return r.Path }
func (r *writeEvent) IsWriteEvent() bool          { return true }
func (r *writeEvent) GetDirectoryID() directoryID { return directoryID(r.DirectoryID) }
func (r *writeEvent) Base() events.AuditEvent     { return r.DesktopSharedDirectoryWrite }

func (a *auditCompactor) handleRead(ctx context.Context, event *events.DesktopSharedDirectoryRead) {
	a.handleEvent(ctx, &readEvent{DesktopSharedDirectoryRead: event})
}

func (a *auditCompactor) handleWrite(ctx context.Context, event *events.DesktopSharedDirectoryWrite) {
	a.handleEvent(ctx, &writeEvent{DesktopSharedDirectoryWrite: event})
}
