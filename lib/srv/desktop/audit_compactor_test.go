//go:build go1.24 && enablesynctest

package desktop

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReadEvent(path string, directory directoryID, offset uint64, length uint32, t time.Time) *events.DesktopSharedDirectoryRead {
	return &events.DesktopSharedDirectoryRead{
		Metadata: events.Metadata{
			Time: t,
		},
		Path:        path,
		DirectoryID: uint32(directory),
		Offset:      offset,
		Length:      length,
	}
}

func newWriteEvent(path string, directory directoryID, offset uint64, length uint32, t time.Time) *events.DesktopSharedDirectoryWrite {
	return &events.DesktopSharedDirectoryWrite{
		Metadata: events.Metadata{
			Time: t,
		},
		Path:        path,
		DirectoryID: uint32(directory),
		Offset:      offset,
		Length:      length,
	}
}

func TestAuditCompactor(t *testing.T) {
	auditEvents := []events.AuditEvent{}
	eventsLock := sync.Mutex{}
	compactor := &auditCompactor{
		refreshInterval:  1 * time.Second,
		maxDelayInterval: 3 * time.Second,
		emitFn: func(_ context.Context, evnt events.AuditEvent) {
			eventsLock.Lock()
			defer eventsLock.Unlock()
			auditEvents = append(auditEvents, evnt)
		},
		streams: map[streamId]*stream{},
	}

	t.Run("basic", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		baseTime := time.Now()
		synctest.Run(func() {
			// Read sequence A
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100, baseTime))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100, baseTime.Add(1*time.Second)))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 100, baseTime.Add(2*time.Second)))
			// Read sequence B
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100, baseTime.Add(4*time.Second)))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100, baseTime.Add(5*time.Second)))
			// Read sequence A continued
			compactor.handleRead(ctx, newReadEvent("foo", 1, 300, 200, baseTime.Add(6*time.Second)))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 500, 50, baseTime.Add(7*time.Second)))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 550, 90, baseTime.Add(8*time.Second)))
			synctest.Wait()
		})

		require.Len(t, auditEvents, 2)
		// Should be compacted to 2 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 640, baseTime))
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200, baseTime.Add(4*time.Second)))
	})

	t.Run("expirations", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		baseTime := time.Now()
		synctest.Run(func() {
			// 2 sequential reads
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100, baseTime))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100, baseTime.Add(1*time.Second)))
			time.Sleep(4 * time.Second)
			// extra sequential read, but it occurred after the configured maxDelayInterval
			// so it should not get compacted with the above reads
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 100, baseTime.Add(5*time.Second)))

			synctest.Wait()
		})

		require.Len(t, auditEvents, 2)
		// Should be compacted to 3 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200, baseTime))
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 200, 100, baseTime.Add(5*time.Second)))
	})

	t.Run("mix-reads-writes", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		baseTime := time.Now()
		synctest.Run(func() {
			// 3 sequential reads
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100, baseTime))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100, baseTime.Add(1*time.Second)))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 100, baseTime.Add(2*time.Second)))
			// same file and directory, and looks sequential, but it's a write
			compactor.handleWrite(ctx, newWriteEvent("foo", 1, 300, 50, baseTime.Add(3*time.Second)))
			compactor.handleWrite(ctx, newWriteEvent("foo", 1, 350, 50, baseTime.Add(4*time.Second)))
			synctest.Wait()
		})

		require.Len(t, auditEvents, 2)
		// Should be compacted to 2 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 300, baseTime))
		assert.Contains(t, auditEvents, newWriteEvent("foo", 1, 300, 100, baseTime.Add(3*time.Second)))
	})

	t.Run("mix-files-and-directories", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		baseTime := time.Now()
		synctest.Run(func() {
			// Identical offsets and lengths, but different path and/or directoryID
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100, baseTime))
			compactor.handleRead(ctx, newReadEvent("foo", 2, 0, 100, baseTime))
			compactor.handleRead(ctx, newReadEvent("bar", 1, 0, 100, baseTime))

			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100, baseTime.Add(1*time.Second)))
			compactor.handleRead(ctx, newReadEvent("foo", 2, 100, 100, baseTime.Add(1*time.Second)))
			compactor.handleRead(ctx, newReadEvent("bar", 1, 100, 100, baseTime.Add(1*time.Second)))
			synctest.Wait()
		})

		require.Len(t, auditEvents, 3)
		// Should be compacted to 3 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200, baseTime))
		assert.Contains(t, auditEvents, newReadEvent("foo", 2, 0, 200, baseTime))
		assert.Contains(t, auditEvents, newReadEvent("bar", 1, 0, 200, baseTime))
	})

	t.Run("flush", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		baseTime := time.Now()
		synctest.Run(func() {

			// Identical offsets and lengths, but different path and/or directoryID
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100, baseTime))
			compactor.handleRead(ctx, newReadEvent("bar", 1, 0, 100, baseTime))
			compactor.handleRead(ctx, newReadEvent("baz", 1, 0, 100, baseTime))

			compactor.flush(ctx)
			synctest.Wait()
		})

		require.Len(t, auditEvents, 3)
		// Should be compacted to 3 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 100, baseTime))
		assert.Contains(t, auditEvents, newReadEvent("bar", 1, 0, 100, baseTime))
		assert.Contains(t, auditEvents, newReadEvent("baz", 1, 0, 100, baseTime))
	})
}
