// gobuild: go1.24 && enablesynctest
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
	compactor := &auditCompactor{
		refreshInterval:  1 * time.Second,
		maxDelayInterval: 3 * time.Second,
		emitFn: func(_ context.Context, evnt events.AuditEvent) {
			eventsLock.Lock()
			defer eventsLock.Unlock()
			auditEvents = append(auditEvents, evnt)
		},
		streams: map[streamID]*stream{},
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
			synctest.Wait()
		})

		require.Len(t, auditEvents, 2)
		// Should be compacted to 2 audit events
		// Once compacted, audit events should inherit the timestamp of
		// the first event in the stream
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 640))
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200))
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
			synctest.Wait()
		})

		require.Len(t, auditEvents, 3)
		// Should be compacted to 3 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 100, 325))
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 750))
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 1200))
	})

	t.Run("expirations", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		synctest.Run(func() {
			// 2 sequential reads
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("foo", 1, 100, 100))
			time.Sleep(4 * time.Second)
			// extra sequential read, but it occurred after the configured maxDelayInterval
			// so it should not get compacted with the above reads
			compactor.handleRead(ctx, newReadEvent("foo", 1, 200, 100))

			synctest.Wait()
		})

		require.Len(t, auditEvents, 2)
		// Should be compacted to 3 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200))
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 200, 100))
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
			synctest.Wait()
		})

		require.Len(t, auditEvents, 2)
		// Should be compacted to 2 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 300))
		assert.Contains(t, auditEvents, newWriteEvent("foo", 1, 300, 100))
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
			synctest.Wait()
		})

		require.Len(t, auditEvents, 3)
		// Should be compacted to 3 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 200))
		assert.Contains(t, auditEvents, newReadEvent("foo", 2, 0, 200))
		assert.Contains(t, auditEvents, newReadEvent("bar", 1, 0, 200))
	})

	t.Run("flush", func(t *testing.T) {
		auditEvents = auditEvents[:0]
		ctx := t.Context()
		synctest.Run(func() {

			// Identical offsets and lengths, but different path and/or directoryID
			compactor.handleRead(ctx, newReadEvent("foo", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("bar", 1, 0, 100))
			compactor.handleRead(ctx, newReadEvent("baz", 1, 0, 100))

			compactor.flush(ctx)
			synctest.Wait()
		})

		require.Len(t, auditEvents, 3)
		// Should be compacted to 3 audit events
		assert.Contains(t, auditEvents, newReadEvent("foo", 1, 0, 100))
		assert.Contains(t, auditEvents, newReadEvent("bar", 1, 0, 100))
		assert.Contains(t, auditEvents, newReadEvent("baz", 1, 0, 100))
	})
}
