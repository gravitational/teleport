/*
Copyright 2021 Gravitational, Inc.

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

package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestProxyWatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
	})
	require.NoError(t, err)

	type client struct {
		services.Presence
		types.Events
	}

	presence := local.NewPresenceService(bk)
	w, err := services.NewProxyWatcher(ctx, services.ProxyWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:   "test",
			RetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Presence: presence,
				Events:   local.NewEventsService(bk),
			},
		},
		ProxiesC: make(chan []types.Server, 10),
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// Since no proxy is yet present, the ProxyWatcher should immediately
	// yield back to its retry loop.
	select {
	case <-w.ResetC:
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for ProxyWatcher reset.")
	}

	// Add a proxy server.
	proxy := newProxyServer(t, "proxy1", "127.0.0.1:2023")
	require.NoError(t, presence.UpsertProxy(proxy))

	// The first event is always the current list of proxies.
	select {
	case changeset := <-w.ProxiesC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], proxy))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add a second proxy.
	proxy2 := newProxyServer(t, "proxy2", "127.0.0.1:2023")
	require.NoError(t, presence.UpsertProxy(proxy2))

	// Watcher should detect the proxy list change.
	select {
	case changeset := <-w.ProxiesC:
		require.Len(t, changeset, 2)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}

	// Delete the first proxy.
	require.NoError(t, presence.DeleteProxy(proxy.GetName()))

	// Watcher should detect the proxy list change.
	select {
	case changeset := <-w.ProxiesC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], proxy2))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}
}

func newProxyServer(t *testing.T, name, addr string) types.Server {
	s, err := types.NewServer(name, types.KindProxy, types.ServerSpecV2{
		Addr:       addr,
		PublicAddr: addr,
	})
	require.NoError(t, err)
	return s
}

func TestLockWatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Access
		types.Events
	}

	access := local.NewAccessService(bk)
	w, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:   "test",
			RetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Access: access,
				Events: local.NewEventsService(bk),
			},
			Clock: clock,
		},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// Subscribe to lock watcher updates.
	target := types.LockTarget{Node: "node"}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target))
	sub, err := w.Subscribe(ctx, target)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sub.Close()) })

	// Add an *expired* lock matching the subscription target.
	pastTime := clock.Now().Add(-time.Minute)
	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target:  target,
		Expires: &pastTime,
	})
	require.NoError(t, err)
	require.NoError(t, access.UpsertLock(ctx, lock))
	select {
	case event := <-sub.Events():
		t.Fatalf("Unexpected event: %v.", event)
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(time.Second):
	}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target))

	// Update the lock so it becomes in force.
	futureTime := clock.Now().Add(time.Minute)
	lock.SetLockExpiry(&futureTime)
	require.NoError(t, access.UpsertLock(ctx, lock))
	select {
	case event := <-sub.Events():
		require.Equal(t, types.OpPut, event.Type)
		receivedLock, ok := event.Resource.(types.Lock)
		require.True(t, ok)
		require.Empty(t, resourceDiff(receivedLock, lock))
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}
	expectLockInForce(t, lock, w.CheckLockInForce(constants.LockingModeBestEffort, target))

	// Delete the lock.
	require.NoError(t, access.DeleteLock(ctx, lock.GetName()))
	select {
	case event := <-sub.Events():
		require.Equal(t, types.OpDelete, event.Type)
		require.Equal(t, event.Resource.GetName(), lock.GetName())
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target))

	// Add a lock matching a different target.
	target2 := types.LockTarget{User: "user"}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target2))
	lock2, err := types.NewLock("test-lock2", types.LockSpecV2{
		Target: target2,
	})
	require.NoError(t, err)
	require.NoError(t, access.UpsertLock(ctx, lock2))
	select {
	case event := <-sub.Events():
		t.Fatalf("Unexpected event: %v.", event)
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(time.Second):
	}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target))
	expectLockInForce(t, lock2, w.CheckLockInForce(constants.LockingModeBestEffort, target2))
}

func TestLockWatcherSubscribeWithEmptyTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Access
		types.Events
	}

	access := local.NewAccessService(bk)
	w, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:   "test",
			RetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Access: access,
				Events: local.NewEventsService(bk),
			},
			Clock: clock,
		},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	select {
	case <-w.LoopC:
	case <-time.After(15 * time.Second):
		t.Fatal("Timeout waiting for LockWatcher loop.")
	}

	// Subscribe to lock watcher updates with an empty target.
	target := types.LockTarget{Node: "node"}
	sub, err := w.Subscribe(ctx, target, types.LockTarget{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sub.Close()) })

	// Add a lock matching one of the subscription targets.
	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: target,
	})
	require.NoError(t, err)
	require.NoError(t, access.UpsertLock(ctx, lock))
	select {
	case event := <-sub.Events():
		require.Equal(t, types.OpPut, event.Type)
		receivedLock, ok := event.Resource.(types.Lock)
		require.True(t, ok)
		require.Empty(t, resourceDiff(receivedLock, lock))
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}

	// Add a lock matching *none* of the subscription targets.
	target2 := types.LockTarget{User: "user"}
	lock2, err := types.NewLock("test-lock2", types.LockSpecV2{
		Target: target2,
	})
	require.NoError(t, err)
	require.NoError(t, access.UpsertLock(ctx, lock2))
	select {
	case event := <-sub.Events():
		t.Fatalf("Unexpected event: %v.", event)
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(time.Second):
	}
}

func TestLockWatcherStale(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
		Clock:            clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Access
		types.Events
	}

	access := local.NewAccessService(bk)
	events := &withUnreliability{Events: local.NewEventsService(bk)}
	w, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:   "test",
			RetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Access: access,
				Events: events,
			},
			Clock: clock,
		},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	select {
	case <-w.LoopC:
	case <-time.After(15 * time.Second):
		t.Fatal("Timeout waiting for LockWatcher loop.")
	}

	// Subscribe to lock watcher updates.
	target := types.LockTarget{Node: "node"}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target))
	require.NoError(t, w.CheckLockInForce(constants.LockingModeStrict, target))
	sub, err := w.Subscribe(ctx, target)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sub.Close()) })

	// Close the underlying watcher. Until LockMaxStaleness is exceeded, no error
	// should be returned.
	events.setUnreliable(true)
	bk.CloseWatchers()
	select {
	case event := <-sub.Events():
		t.Fatalf("Unexpected event: %v.", event)
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(2 * time.Second):
	}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target))
	require.NoError(t, w.CheckLockInForce(constants.LockingModeStrict, target))

	// Advance the clock to exceed LockMaxStaleness.
	clock.Advance(defaults.LockMaxStaleness + time.Second)
	select {
	case event := <-sub.Events():
		require.Equal(t, types.OpUnreliable, event.Type)
	case <-sub.Done():
		t.Fatal("Lock watcher subscription has unexpectedly exited.")
	case <-time.After(15 * time.Second):
		t.Fatal("Timeout waiting for OpUnreliable.")
	}
	require.NoError(t, w.CheckLockInForce(constants.LockingModeBestEffort, target))
	expectLockInForce(t, nil, w.CheckLockInForce(constants.LockingModeStrict, target))

	// Add a lock matching the subscription target.
	lock, err := types.NewLock("test-lock", types.LockSpecV2{
		Target: target,
	})
	require.NoError(t, err)
	require.NoError(t, access.UpsertLock(ctx, lock))

	// Make the event stream reliable again. That should broadcast any matching
	// locks added in the meantime.
	events.setUnreliable(false)
	clock.Advance(time.Second)
ExpectPut:
	for {
		select {
		case event := <-sub.Events():
			// There might be additional OpUnreliable events in the queue.
			if event.Type == types.OpUnreliable {
				continue ExpectPut
			}
			require.Equal(t, types.OpPut, event.Type)
			receivedLock, ok := event.Resource.(types.Lock)
			require.True(t, ok)
			require.Empty(t, resourceDiff(receivedLock, lock))
			break ExpectPut
		case <-sub.Done():
			t.Fatal("Lock watcher subscription has unexpectedly exited.")
		case <-time.After(15 * time.Second):
			t.Fatal("Timeout waiting for OpPut.")
		}
	}
	expectLockInForce(t, lock, w.CheckLockInForce(constants.LockingModeBestEffort, target))
	expectLockInForce(t, lock, w.CheckLockInForce(constants.LockingModeStrict, target))
}

type withUnreliability struct {
	types.Events
	rw         sync.RWMutex
	unreliable bool
}

func (e *withUnreliability) setUnreliable(u bool) {
	e.rw.Lock()
	defer e.rw.Unlock()
	e.unreliable = u
}

func (e *withUnreliability) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	e.rw.RLock()
	defer e.rw.RUnlock()
	if e.unreliable {
		return nil, trace.ConnectionProblem(nil, "")
	}
	return e.Events.NewWatcher(ctx, watch)
}

func expectLockInForce(t *testing.T, expectedLock types.Lock, err error) {
	require.Error(t, err)
	errLock := err.(trace.Error).GetFields()["lock-in-force"]
	if expectedLock != nil {
		require.Empty(t, resourceDiff(expectedLock, errLock.(types.Lock)))
	} else {
		require.Nil(t, errLock)
	}
}

func resourceDiff(res1, res2 types.Resource) string {
	return cmp.Diff(res1, res2,
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		cmpopts.EquateEmpty())
}

// TestDatabaseWatcher tests that database resource watcher properly receives
// and dispatches updates to database resources.
func TestDatabaseWatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
	})
	require.NoError(t, err)

	type client struct {
		services.Databases
		types.Events
	}

	databasesService := local.NewDatabasesService(bk)
	w, err := services.NewDatabaseWatcher(ctx, services.DatabaseWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:   "test",
			RetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Databases: databasesService,
				Events:    local.NewEventsService(bk),
			},
		},
		DatabasesC: make(chan types.Databases, 10),
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// Initially there are no databases so watcher should send an empty list.
	select {
	case changeset := <-w.DatabasesC:
		require.Len(t, changeset, 0)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add a database.
	database1 := newDatabase(t, "db1")
	require.NoError(t, databasesService.CreateDatabase(ctx, database1))

	// The first event is always the current list of databases.
	select {
	case changeset := <-w.DatabasesC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], database1))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add a second database.
	database2 := newDatabase(t, "db2")
	require.NoError(t, databasesService.CreateDatabase(ctx, database2))

	// Watcher should detect the database list change.
	select {
	case changeset := <-w.DatabasesC:
		require.Len(t, changeset, 2)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}

	// Delete the first database.
	require.NoError(t, databasesService.DeleteDatabase(ctx, database1.GetName()))

	// Watcher should detect the database list change.
	select {
	case changeset := <-w.DatabasesC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], database2))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}
}

func newDatabase(t *testing.T, name string) types.Database {
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: name,
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)
	return database
}

// TestAppWatcher tests that application resource watcher properly receives
// and dispatches updates.
func TestAppWatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	bk, err := lite.NewWithConfig(ctx, lite.Config{
		Path:             t.TempDir(),
		PollStreamPeriod: 200 * time.Millisecond,
	})
	require.NoError(t, err)

	type client struct {
		services.Apps
		types.Events
	}

	appService := local.NewAppService(bk)
	w, err := services.NewAppWatcher(ctx, services.AppWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:   "test",
			RetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Apps:   appService,
				Events: local.NewEventsService(bk),
			},
		},
		AppsC: make(chan types.Apps, 10),
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// Initially there are no apps so watcher should send an empty list.
	select {
	case changeset := <-w.AppsC:
		require.Len(t, changeset, 0)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add an app.
	app1 := newApp(t, "app1")
	require.NoError(t, appService.CreateApp(ctx, app1))

	// The first event is always the current list of apps.
	select {
	case changeset := <-w.AppsC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], app1))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add a second app.
	app2 := newApp(t, "app2")
	require.NoError(t, appService.CreateApp(ctx, app2))

	// Watcher should detect the app list change.
	select {
	case changeset := <-w.AppsC:
		require.Len(t, changeset, 2)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}

	// Delete the first app.
	require.NoError(t, appService.DeleteApp(ctx, app1.GetName()))

	// Watcher should detect the database list change.
	select {
	case changeset := <-w.AppsC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], app2))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}
}

func newApp(t *testing.T, name string) types.Application {
	app, err := types.NewAppV3(types.Metadata{
		Name: name,
	}, types.AppSpecV3{
		URI: "localhost",
	})
	require.NoError(t, err)
	return app
}
