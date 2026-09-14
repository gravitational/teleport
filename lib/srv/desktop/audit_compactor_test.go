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
	"reflect"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/gravitational/teleport/api/types/events"
	sliceutils "github.com/gravitational/teleport/lib/utils/slices"
)

func newReadEvent(path string, directory directoryID, offset uint64, length uint32) *events.DesktopSharedDirectoryRead {
	return &events.DesktopSharedDirectoryRead{
		Path:        path,
		DirectoryID: uint32(directory),
		Offset:      offset,
		Length:      length,
	}
}

func newWriteEvent(path string, directory directoryID, offset uint64, length uint32) *events.DesktopSharedDirectoryWrite {
	return &events.DesktopSharedDirectoryWrite{
		Path:        path,
		DirectoryID: uint32(directory),
		Offset:      offset,
		Length:      length,
	}
}

type directoryEvent struct {
	DirectoryID uint32
	Offset      uint64
	Length      uint64
}

type eventCase struct {
	// A base read/write event
	Base directoryEvent
	// The result of splitting the base event
	// into some number of compatible events.
	Split []directoryEvent
}

type eventCases []eventCase

func (e eventCases) allEvents() (all []directoryEvent) {
	for _, evnt := range e {
		all = append(all, evnt.Split...)
	}
	return
}

// Generate what we'll call "Base" events. These are reads/writes
// which we will later split into many smaller read/write events.
func generateBaseEvents(minDirectoryID, maxDirectoryID uint32, minLength uint64) *rapid.Generator[directoryEvent] {
	maxLength := 1024 * 1024 * 1024 // 1GiB
	return rapid.MakeCustom[directoryEvent](rapid.MakeConfig{
		Fields: map[reflect.Type]map[string]*rapid.Generator[any]{
			reflect.TypeFor[directoryEvent](): {
				"DirectoryID": rapid.Uint32Range(minDirectoryID, maxDirectoryID).AsAny(),
				"Offset":      rapid.Uint64Range(0, uint64(math.MaxUint32-maxLength)).AsAny(),
				"Length":      rapid.Uint64Range(minLength, uint64(maxLength)).AsAny(),
			},
		},
	})
}

// Generate 'eventCase' instances from base 'directoryEvents' instances.
// Divide up each base event into some number of subEvents to simulate a large read/write
// being randomly split into N sub-reads/sub-writes.
func genEventCaseEx(baseGen *rapid.Generator[directoryEvent]) *rapid.Generator[eventCase] {
	return rapid.Custom(func(t *rapid.T) eventCase {
		baseEvent := baseGen.Draw(t, "base events")

		// Cannot split events with length < 2
		if baseEvent.Length < 2 {
			return eventCase{
				Base:  baseEvent,
				Split: []directoryEvent{baseEvent},
			}
		}

		// Select N distinct pivots in the range [1, eventLength).
		// Note: rapid.Uint64Range is *inclusive*.
		// Limit to 32 pivots (events can only be split into 33 chunks max).
		const maxPivots = 32
		pivotCount := int(min(baseEvent.Length-1, maxPivots))
		pivots := rapid.SliceOfNDistinct(rapid.Uint64Range(1, baseEvent.Length-1), 1, pivotCount, rapid.ID).Draw(t, "pivots")
		slices.Sort(pivots)

		// Ex: length: 100 pivots: [10, 25, 55, 72, 87]
		// yields lengths: 10, 15, 30, 17, 15, 13
		length := baseEvent.Length
		evnts := []directoryEvent{}
		relativeOffset := uint64(0)

		for _, pivot := range pivots {
			evnts = append(evnts, directoryEvent{
				DirectoryID: baseEvent.DirectoryID,
				Offset:      baseEvent.Offset + relativeOffset,
				Length:      pivot - relativeOffset,
			})
			relativeOffset += pivot - relativeOffset
		}

		// And the remainder
		evnts = append(evnts, directoryEvent{
			DirectoryID: baseEvent.DirectoryID,
			Offset:      baseEvent.Offset + relativeOffset,
			Length:      length - relativeOffset,
		})

		return eventCase{
			Base:  baseEvent,
			Split: evnts,
		}
	})
}

func runCompaction(ctx context.Context, evnts []directoryEvent, maxEvents int) []directoryEvent {
	auditEvents := []events.AuditEvent{}
	eventsLock := sync.Mutex{}
	const refreshInterval = 1 * time.Second
	const maxDelayInterval = 3 * time.Second
	compactor := &auditCompactor{
		maxEventsPerBucket: maxEvents,
		refreshInterval:    refreshInterval,
		maxDelayInterval:   maxDelayInterval,
		emitFn: func(_ context.Context, event events.AuditEvent) {
			eventsLock.Lock()
			defer eventsLock.Unlock()
			auditEvents = append(auditEvents, event)
		},
		buckets:  map[fileOperationsKey]*fileOperationsBucket{},
		flushing: map[*fileOperationsBucket]struct{}{},
	}

	for _, event := range evnts {
		compactor.handleEvent(ctx, &readEvent{newReadEvent("foo", directoryID(event.DirectoryID), event.Offset, uint32(event.Length))})
	}
	// Compact
	compactor.flush(ctx)
	// Transform audit events to directoryEvent
	return sliceutils.Map(auditEvents, func(evnt events.AuditEvent) directoryEvent {
		readEvent := evnt.(*events.DesktopSharedDirectoryRead)
		return directoryEvent{
			DirectoryID: readEvent.DirectoryID,
			Offset:      readEvent.Offset,
			Length:      uint64(readEvent.Length),
		}
	})
}

func genEventCase(mustBeCompactible bool) *rapid.Generator[eventCase] {
	if mustBeCompactible {
		// Require minimum length of two to guarantee
		// that generated events can be compacted.
		return genEventCaseEx(generateBaseEvents(1, 1, 2))
	}
	return genEventCaseEx(generateBaseEvents(1, 1, 0))
}

func TestProperty_AuditCompactor_HalvesCompactibleFileOperations(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		evnts := rapid.SliceOfN(genEventCase(true), 10, 100).Draw(t, "events")
		allEvents := eventCases(evnts).allEvents()

		compactedEvents := runCompaction(t.Context(), rapid.Permutation(allEvents).Draw(t, "shuffled"), 0)

		// Event count should be reduced by at least half
		if len(compactedEvents) > len(allEvents)/2 {
			t.Errorf("compaction should reduce event count by at least half. pre-compaction: %d, post-compaction: %d, eventCase: %v", len(allEvents), len(compactedEvents), evnts)
		}
	})
}

func TestProperty_AuditCompactor_PreservesFileOperationLengths(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		evnts := rapid.SliceOfN(genEventCase(false), 10, 100).Draw(t, "events")
		allEvents := eventCases(evnts).allEvents()
		totalBaseReads := uint64(0)
		for _, evnt := range evnts {
			totalBaseReads += evnt.Base.Length
		}

		compactedEvents := runCompaction(t.Context(), rapid.Permutation(allEvents).Draw(t, "shuffled"), 0)

		// Count the total bytes.
		totalCompactedReads := uint64(0)
		for _, evnt := range compactedEvents {
			totalCompactedReads += evnt.Length
		}

		// Compaction must not alter the total number of bytes read.
		if totalCompactedReads != totalBaseReads {
			t.Errorf("Compacted reads != original reads. Got: %d, expected: %d", totalCompactedReads, totalBaseReads)
		}
	})
}

func TestProperty_AuditCompactor_HonorsMaxCompaction(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		evnts := rapid.SliceOfN(genEventCase(true), 10, 100).Draw(t, "events")
		allEvents := eventCases(evnts).allEvents()

		// Only allow 1 event per bucket.
		compactedEvents := runCompaction(t.Context(), rapid.Permutation(allEvents).Draw(t, "shuffled"), 1)

		// Compaction should not be possible with 'maxEventsPerBucket' set to 1.
		if len(compactedEvents) > len(allEvents) {
			t.Errorf("compaction should not be possible. pre-compaction: %d, post-compaction: %d, eventCase: %v", len(allEvents), len(compactedEvents), evnts)
		}
	})
}

func TestAuditCompactor(t *testing.T) {
	auditEvents := []events.AuditEvent{}
	eventsLock := sync.Mutex{}
	const refreshInterval = 1 * time.Second
	const maxDelayInterval = 3 * time.Second
	compactor := &auditCompactor{
		refreshInterval:  refreshInterval,
		maxDelayInterval: maxDelayInterval,
		emitFn: func(_ context.Context, event events.AuditEvent) {
			eventsLock.Lock()
			defer eventsLock.Unlock()
			auditEvents = append(auditEvents, event)
		},
		buckets:  map[fileOperationsKey]*fileOperationsBucket{},
		flushing: map[*fileOperationsBucket]struct{}{},
	}

	t.Run("basic", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			// Read sequence A
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 100))
			// Read sequence B
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			// Read sequence A continued
			compactor.handleRead(ctx, newReadEvent("foo", 1, 300, 200))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 500, 50))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 550, 90))

			compactor.flush(ctx)
			require.Len(t, auditEvents, 2)
			// Should be compacted to 2 audit events
			// Once compacted, audit events should inherit the timestamp of
			// the first event in the stream
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 640))
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200))
		})

	})

	t.Run("overflow", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			// Walk up to and beyond MaxUint32
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, math.MaxUint32-1))
			compactor.handleRead(ctx, newReadEvent("foo", 1, math.MaxUint32-1, 1))
			compactor.handleRead(ctx, newReadEvent("foo", 1, math.MaxUint32, 1))
			compactor.handleRead(ctx, newReadEvent("foo", 1, math.MaxUint32+1, 1))

			compactor.flush(ctx)
			require.Len(t, auditEvents, 2)
			// The 'length' field of the underlying directory read/write audit events is a uint32,
			// so we can't record a length greater than 'math.MaxUint32'. Expect the compaction algorithm
			// to handle this gracefully by breaking the read sequence above into two events. One covering the range
			// [0, math.MaxUint32) and the other [math.MaxUint32, math.MaxUint32+2)
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, math.MaxUint32))
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, math.MaxUint32, 2))
		})
	})

	t.Run("zero-length-event", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			// Create two read events with zero length, but with differing
			// error codes. Neither should get compacted, as zero length
			// events are not eligible for compaction.
			firstEvent := newReadEvent("foo", 1, 0, 0)
			firstEvent.Error = "some error"

			secondEvent := newReadEvent("foo", 1, 0, 0)
			secondEvent.Error = "another error"

			compactor.handleRead(ctx, firstEvent)
			compactor.handleRead(ctx, secondEvent)

			compactor.flush(ctx)
			// events with length zero should be ignored (not compacted)
			require.Len(t, auditEvents, 2)
			assert.Contains(t, auditEvents, firstEvent)
			assert.Contains(t, auditEvents, secondEvent)
		})
	})

	t.Run("complex", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			// Three separate reads (with different lengths) of the same file
			// Read sequence A
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 300, 50))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 350, 75))

			// Read sequence B
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 150))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 350, 400))

			// Read sequence C (does not start at 0)
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 500))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 700, 500))

			compactor.flush(ctx)
			require.Len(t, auditEvents, 3)
			// Should be compacted to 3 audit events
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 100, 325))
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 750))
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 1200))
		})

	})

	t.Run("expirations", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			// 2 sequential reads
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			time.Sleep(refreshInterval - time.Millisecond)
			synctest.Wait()

			// Should not be emitted yet refresh interval has not been exceeded
			eventsLock.Lock()
			assert.Empty(t, auditEvents)
			eventsLock.Unlock()

			// Complete the refreshInterval and we should have an event available
			time.Sleep(time.Millisecond)
			synctest.Wait()
			eventsLock.Lock()
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200))
			eventsLock.Unlock()

			// Continue submitting events just before the refresh interval.
			// Not audit event should be submitted until maxDelayInterval is reached
			auditEvents = auditEvents[:0]
			var elapsedTime time.Duration
			offset := uint64(200)
			const length = uint32(100)

			count := 0
			for elapsedTime < maxDelayInterval {
				compactor.handleRead(ctx, newReadEvent("foo", 1, offset, length))
				time.Sleep(refreshInterval - time.Millisecond)
				synctest.Wait()
				elapsedTime += refreshInterval - time.Millisecond
				offset += uint64(length)
				count++
			}
			// maxDelay should be exeeded by now and we should have
			// a single consolidated event
			eventsLock.Lock()
			require.Len(t, auditEvents, 1)
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 200, length*uint32(count)))
			eventsLock.Unlock()

		})

	})

	t.Run("mix-reads-writes", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			// 3 sequential reads
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 100))
			// same file and directory, and looks sequential, but it's a write
			compactor.handleWrite(ctx, newWriteEvent("foo", 1, 300, 50))
			compactor.handleWrite(ctx, newWriteEvent("foo", 1, 350, 50))

			compactor.flush(ctx)
			require.Len(t, auditEvents, 2)
			// Should be compacted to 2 audit events
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 300))
			assert.Contains(t, auditEvents, newWriteEvent("foo", 1, 300, 100))
		})
	})

	t.Run("mix-files-and-directories", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			// Identical offsets and lengths, but different path and/or directoryID
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 2, 0, 100))
			compactor.handleRead(ctx, newReadEvent("bar", 1, 0, 100))

			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 2, 100, 100))
			compactor.handleRead(ctx, newReadEvent("bar", 1, 100, 100))

			compactor.flush(ctx)
			require.Len(t, auditEvents, 3)
			// Should be compacted to 3 audit events
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200))
			assert.Contains(t, auditEvents, newReadEvent("foo", 2, 0, 200))
			assert.Contains(t, auditEvents, newReadEvent("bar", 1, 0, 200))
		})

	})

	t.Run("racy-flush", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()
			auditEvents := make(chan events.AuditEvent)
			compactor.emitFn = func(_ context.Context, ae events.AuditEvent) {
				auditEvents <- ae
			}
			// Identical offsets and lengths, but different path and/or directoryID
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("bar", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("baz", 1, 0, 100))
			time.Sleep(refreshInterval - 1*time.Nanosecond)
			compactor.handleRead(ctx, newReadEvent("caz", 1, 0, 100))

			// Timers should start firing
			time.Sleep(1 * time.Nanosecond)
			synctest.Wait()

			flushDone := false
			go func() {
				compactor.flush(ctx)
				flushDone = true
			}()

			expectedEvents := []events.AuditEvent{
				newReadEvent("foo", 1, 0, 100),
				newReadEvent("bar", 1, 0, 100),
				newReadEvent("baz", 1, 0, 100),
				newReadEvent("caz", 1, 0, 100),
			}
			for range len(expectedEvents) {
				assert.False(t, flushDone)
				assert.Contains(t, expectedEvents, <-auditEvents)
				synctest.Wait()
			}
			assert.True(t, flushDone)
		})
	})
}
