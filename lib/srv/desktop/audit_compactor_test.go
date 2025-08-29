//go:build go1.24 && enablesynctest

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
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/events"
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
		buckets: map[fileOperationsKey]*fileOperationsBucket{},
	}

	t.Run("basic", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		synctest.Run(func() {
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

	t.Run("complex", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		synctest.Run(func() {

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
		ctx := t.Context()
		synctest.Run(func() {
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
			assert.Contains(t, auditEvents, newReadEvent("foo", 1, 200, uint32(length*uint32(count))))
			eventsLock.Unlock()

		})

	})

	t.Run("mix-reads-writes", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		synctest.Run(func() {
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
		ctx := t.Context()
		synctest.Run(func() {
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
		ctx := t.Context()
		synctest.Run(func() {
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
			for _ = range len(expectedEvents) {
				assert.False(t, flushDone)
				assert.Contains(t, expectedEvents, <-auditEvents)
				synctest.Wait()
			}
			assert.True(t, flushDone)
		})
	})
}
