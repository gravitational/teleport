/*
Copyright 2018-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlbk

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestDriver executes the backend compliance suite for a driver. A single
// backend is created so connections remain open for all subtests.
func TestDriver(t *testing.T, driver Driver) {
	// Create test configuration.
	fakeClock := clockwork.NewFakeClock()
	cfg := driver.Config()
	cfg.Clock = fakeClock
	cfg.PurgePeriod = time.Minute
	cfg.RetryTimeout = time.Minute
	cfg.PollStreamPeriod = time.Millisecond * 300

	// Init Backend
	bk, err := newWithConfig(context.Background(), driver, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { bk.Close() })

	// Start background process.
	err = bk.start(context.Background())
	require.NoError(t, err)

	// Run test suite.
	t.Run("Backend Compliance Suite", func(t *testing.T) {
		newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
			opts, err := test.ApplyOptions(options)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}

			if opts.MirrorMode {
				return nil, nil, test.ErrMirrorNotSupported
			}

			bk := &testBackend{Backend: bk}
			bk.buf = backend.NewCircularBuffer(backend.BufferCapacity(bk.BufferSize))
			bk.buf.SetInit()
			return bk, fakeClock, nil
		}
		test.RunBackendComplianceSuite(t, newBackend)
	})

	// Stop background routine for the remaining tests.
	bk.closeFn()
	<-bk.bgDone
	bk.closeCtx, bk.closeFn = context.WithCancel(context.Background())

	// Purge tests the background's purge function.
	t.Run("Purge", func(t *testing.T) {
		// - Create 4 items (a, b, c, d)
		//   - a/c are active
		//   - b/d have expired
		// - Call purge with d-1 event ID
		// - Confirm:
		//   - b item removed (no event or lease)
		//   - b/d leases removed (expired)
		//   - a event removed (before DefaultEventsTTL)

		// Create items
		createItem := func(tx Tx, key string, expires time.Time) backend.Item {
			item := backend.Item{Key: []byte(key), Expires: expires, Value: []byte("value")}
			item.ID = tx.InsertItem(item)
			tx.UpsertLease(item)
			tx.InsertEvent(types.OpPut, item)
			return item
		}
		tx := bk.db.Begin(context.Background())
		a := createItem(tx, "/purgetest/a", time.Time{}) // active
		bk.Config.Clock.(clockwork.FakeClock).Advance(backend.DefaultEventsTTL + time.Second)
		b := createItem(tx, "/purgetest/b", fakeClock.Now().Add(-time.Second))               // expired
		c := createItem(tx, "/purgetest/c", fakeClock.Now().Add(backend.DefaultEventsTTL*2)) // active
		d := createItem(tx, "/purgetest/d", fakeClock.Now().Add(-time.Second))               // expired with event
		require.Greater(t, tx.GetLastEventID(), int64(0))
		require.NoError(t, tx.Commit())

		// Purge
		require.NoError(t, bk.purge())

		// Validate results.
		tx = bk.db.ReadOnly(context.Background())
		t.Cleanup(func() { tx.Commit() })

		// Get a single event so we can cover getEventsRemaining.
		var fromEventID int64
		events := tx.GetEvents(fromEventID, 1)
		require.Greater(t, events.LastID, fromEventID)
		require.Equal(t, 2, events.Remaining)
		require.Equal(t, 1, len(events.BackendEvents))
		require.Equal(t, b.Key, events.BackendEvents[0].Item.Key)

		// Get the rest of the events.
		fromEventID = events.LastID
		events = tx.GetEvents(fromEventID, 10)
		require.Greater(t, events.LastID, fromEventID)
		require.Equal(t, 0, events.Remaining)
		require.Equal(t, 2, len(events.BackendEvents))
		require.Equal(t, c.Key, events.BackendEvents[0].Item.Key)
		require.Equal(t, d.Key, events.BackendEvents[1].Item.Key)

		// Assert leases exist or not.
		require.True(t, tx.LeaseExists(a.Key))
		require.False(t, tx.LeaseExists(b.Key))
		require.True(t, tx.LeaseExists(c.Key))
		require.False(t, tx.LeaseExists(d.Key))

		// Validate a range query returns the correct items. This joins the item
		// and lease tables so we can test both at the same time.
		items := tx.GetItemRange(a.Key, d.Key, 10)
		require.Equal(t, 2, len(items))
		require.Equal(t, items[0].Key, a.Key)
		require.Equal(t, items[1].Key, c.Key)
	})

	// Poll tests the backend poll function's ability to reset the buffer when it
	// falls behind emitting events due to latency.
	t.Run("Poll", func(t *testing.T) {
		// - Configure backend so a single event is emitted at a time and the
		//   buffer is reset when there are two or more events remaining.
		// - Create three items/events and detect that the watcher is closed.
		// - Add a fourth item and detect that the buffer emits it and skips all
		//   previous items.

		backupConfig := *bk.Config
		t.Cleanup(func() { *bk.Config = backupConfig })

		bk.buf = backend.NewCircularBuffer(backend.BufferCapacity(bk.BufferSize))
		bk.buf.SetInit()

		// Setup watcher to receive events.
		createWatcher := func() backend.Watcher {
			watcher, err := bk.NewWatcher(context.Background(), backend.Watch{Name: "PollTest"})
			require.NoError(t, err)
			select {
			case event := <-watcher.Events():
				require.Equal(t, types.OpInit, event.Type)
			case <-watcher.Done():
				t.Fatal("watcher done unexpectedly")
			}
			return watcher
		}
		watcher := createWatcher()

		// Update config to trigger buffer reset due to latency emitting events.
		// Formula: <events remaining>/(BufferSize/2)*PollStreamPeriod > EventsTTL
		bk.BufferSize = 2 // emit 1 event at a time
		bk.EventsTTL = time.Second
		bk.PurgePeriod = time.Second
		bk.PollStreamPeriod = time.Second

		// Insert three events. Poll will get first event and detect 2 remaining.
		createEvent := func(tx Tx, key string) backend.Item {
			item := backend.Item{Key: []byte(key), Value: []byte("value")}
			item.ID = tx.InsertItem(item)
			tx.InsertEvent(types.OpPut, item)
			return item
		}
		tx := bk.db.Begin(context.Background())
		createEvent(tx, "/polltest/a")
		createEvent(tx, "/polltest/b")
		createEvent(tx, "/polltest/c")
		require.NoError(t, tx.Commit())

		// First poll call should detect latency and reset the buffer.
		lastEventID, err := bk.poll(0)
		require.NoError(t, err)
		require.Greater(t, lastEventID, int64(0)) // points to "c" event
		select {
		case <-watcher.Done():
			// OK: buffer reset closed watcher.
		case event := <-watcher.Events():
			require.Failf(t, "expected watcher to close", "received %+v", event)
		}

		// lastEventID should now be set to "c" event.
		// Adding a new "d" item should emit an event for "d" and not "b".
		watcher = createWatcher()
		fromEventID := lastEventID
		tx = bk.db.Begin(context.Background())
		d := createEvent(tx, "/polltest/d")
		require.NoError(t, tx.Commit())
		lastEventID, err = bk.poll(fromEventID)
		require.NoError(t, err)
		require.Greater(t, lastEventID, fromEventID)
		select {
		case event := <-watcher.Events():
			require.Equal(t, types.OpPut, event.Type)
			require.Equal(t, d.Key, event.Item.Key)
		case <-watcher.Done():
			require.Fail(t, "watcher done unexpectedly")
		}
	})
}

// testBackend wraps Backend overriding Close.
type testBackend struct {
	*Backend
}

// Close only the buffer so buffer watchers are notified of close events.
func (b *testBackend) Close() error {
	return b.buf.Close()
}
