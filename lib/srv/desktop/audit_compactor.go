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
	"context"
	"math"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types/events"
)

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
	IsSuccess() bool
}

// fileOperationsBucket identifies a set of reads/writes
// to a particular file within some period of time.
type fileOperationsBucket struct {
	expireTime time.Time
	// events are stored in a map where the key is the event's offset + length.
	// This allows us to easily combine new events as they're added to the bucket.
	events map[uint64][]fileOperationEvent
	// Some events should not be compacted, such as those containing error details.
	nonCompactable []fileOperationEvent
	timer          *time.Timer
	done           chan struct{}
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
}

func newAuditCompactor(refreshInterval, maxDelayInterval time.Duration, emitFn func(context.Context, events.AuditEvent)) auditCompactor {
	return auditCompactor{
		refreshInterval:  refreshInterval,
		maxDelayInterval: maxDelayInterval,
		emitFn:           emitFn,
		buckets:          map[fileOperationsKey]*fileOperationsBucket{},
	}
}

func (s *fileOperationsBucket) emitEvents(ctx context.Context, emitFn func(ctx context.Context, event events.AuditEvent)) {
	for _, events := range s.events {
		for _, event := range events {
			emitFn(ctx, event.Base())
		}
	}

	for _, event := range s.nonCompactable {
		emitFn(ctx, event.Base())
	}
}

func (s *fileOperationsBucket) addEvent(event fileOperationEvent) {
	if !event.IsSuccess() {
		// Do not compact events indicating failed file IO.
		s.nonCompactable = append(s.nonCompactable, event)
		return
	}

	offset := event.GetOffset()
	length := event.GetLength()
	nextOffset := offset + length
	if baseEvents, ok := s.events[offset]; ok {
		baseEvent := baseEvents[0]
		baseEvent.SetLength(baseEvent.GetLength() + length)
		delete(s.events, offset)
		s.events[nextOffset] = append(s.events[nextOffset], baseEvent)
	} else {
		s.events[nextOffset] = append(s.events[nextOffset], event)
	}
}

func (a *auditCompactor) handleEvent(ctx context.Context, event fileOperationEvent) {
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
			// Update the current bucket. It is a continuation of the current bucket
			// and the timer has not yet fired for it.
			bucket.addEvent(event)
			// Reset the timer either to the refresh interval, or until
			// the buckets's expiration time
			bucket.timer.Reset(time.Duration(math.Min(float64(a.refreshInterval), float64(time.Until(bucket.expireTime)))))
			newBucket = false
		} else {
			// The timer has already fired. Stop tracking this bucket.
			// A new bucket will be created below to handle this event.
			delete(a.buckets, key)
		}
	}

	// We need to create a new bucket due to one of the following:
	//   - We are not tracking any such bucket yet.
	//   - We were tracking this bucket but the timer has already fired.
	if newBucket {
		bucket := &fileOperationsBucket{
			done:       make(chan struct{}),
			expireTime: time.Now().Add(a.maxDelayInterval),
			events:     map[uint64][]fileOperationEvent{},
		}
		bucket.addEvent(event)
		bucket.timer = time.AfterFunc(a.refreshInterval, func() {
			// Close done channel so that the 'flush' function can
			// verify that this goroutine has completed its work.
			defer close(bucket.done)
			a.bucketsLock.Lock()
			delete(a.buckets, key)
			a.bucketsLock.Unlock()
			bucket.emitEvents(ctx, a.emitFn)

		})
		a.buckets[key] = bucket
	}
}

// flush immediately compacts and emits audit events for all
// unexpired buckets and blocks until completion.
func (a *auditCompactor) flush(ctx context.Context) {
	a.bucketsLock.Lock()
	wait := []chan struct{}{}
	for bucketKey, bucket := range a.buckets {
		if bucket.timer.Stop() {
			// If we successfully stop the timer before it fires,
			// go ahead and emit the audit event.
			bucket.emitEvents(ctx, a.emitFn)
			delete(a.buckets, bucketKey)
		} else {
			// The timer was already firing, so wait until
			// the emitFn as been executed by the underlying goroutine.
			wait = append(wait, bucket.done)
		}
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
func (r *readEvent) IsWriteEvent() bool          { return true }
func (r *readEvent) GetDirectoryID() directoryID { return directoryID(r.DirectoryID) }
func (r *readEvent) Base() events.AuditEvent     { return r.DesktopSharedDirectoryRead }
func (r *readEvent) IsSuccess() bool             { return r.Success }

type writeEvent struct {
	*events.DesktopSharedDirectoryWrite
}

func (r *writeEvent) SetLength(len uint64)        { r.Length = toUint32(len) }
func (r *writeEvent) GetLength() uint64           { return uint64(r.Length) }
func (r *writeEvent) GetOffset() uint64           { return r.Offset }
func (r *writeEvent) GetPath() string             { return r.Path }
func (r *writeEvent) IsWriteEvent() bool          { return false }
func (r *writeEvent) GetDirectoryID() directoryID { return directoryID(r.DirectoryID) }
func (r *writeEvent) Base() events.AuditEvent     { return r.DesktopSharedDirectoryWrite }
func (r *writeEvent) IsSuccess() bool             { return r.Success }

func (a *auditCompactor) handleRead(ctx context.Context, event *events.DesktopSharedDirectoryRead) {
	a.handleEvent(ctx, &readEvent{DesktopSharedDirectoryRead: event})
}

func (a *auditCompactor) handleWrite(ctx context.Context, event *events.DesktopSharedDirectoryWrite) {
	a.handleEvent(ctx, &writeEvent{DesktopSharedDirectoryWrite: event})
}
