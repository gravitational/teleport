/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services_test

import (
	"context"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

var _ types.Events = (*errorWatcher)(nil)

type errorWatcher struct{}

func (e errorWatcher) NewWatcher(context.Context, types.Watch) (types.Watcher, error) {
	return nil, errors.New("watcher error")
}

var _ services.ProxyGetter = (*nopProxyGetter)(nil)

type nopProxyGetter struct{}

func (n nopProxyGetter) GetProxies() ([]types.Server, error) {
	return nil, nil
}

func TestResourceWatcher_Backoff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	w, err := services.NewProxyWatcher(ctx, services.ProxyWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			Clock:          clock,
			MaxRetryPeriod: defaults.MaxWatcherBackoff,
			Client:         &errorWatcher{},
			ResetC:         make(chan time.Duration, 5),
		},
		ProxyGetter: &nopProxyGetter{},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	step := w.MaxRetryPeriod / 5.0
	for i := 0; i < 5; i++ {
		// wait for watcher to reload
		select {
		case duration := <-w.ResetC:
			stepMin := step * time.Duration(i) / 2
			stepMax := step * time.Duration(i+1)

			require.GreaterOrEqual(t, duration, stepMin)
			require.LessOrEqual(t, duration, stepMax)

			// wait for watcher to get to retry.After
			clock.BlockUntil(1)

			// add some extra to the duration to ensure the retry occurs
			clock.Advance(w.MaxRetryPeriod)
		case <-time.After(time.Minute):
			t.Fatalf("timeout waiting for reset")
		}
	}
}

func TestProxyWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Presence
		types.Events
	}

	presence := local.NewPresenceService(bk)
	w, err := services.NewProxyWatcher(ctx, services.ProxyWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Presence: presence,
				Events:   local.NewEventsService(bk),
			},
		},
		ProxiesC: make(chan []types.Server, 10),
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	require.NoError(t, w.WaitInitialization())
	// Add a proxy server.
	proxy := newProxyServer(t, "proxy1", "127.0.0.1:2023")
	require.NoError(t, presence.UpsertProxy(ctx, proxy))

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
	require.NoError(t, presence.UpsertProxy(ctx, proxy2))

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
	require.NoError(t, presence.DeleteProxy(ctx, proxy.GetName()))

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

	// Delete the second proxy.
	require.NoError(t, presence.DeleteProxy(ctx, proxy2.GetName()))

	// Watcher should detect the proxy list change.
	select {
	case changeset := <-w.ProxiesC:
		require.Empty(t, changeset)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}
}

func newProxyServer(t *testing.T, name, addr string) types.Server {
	s, err := types.NewServer(name, types.KindProxy, types.ServerSpecV2{
		Addr:        addr,
		PublicAddrs: []string{addr},
	})
	require.NoError(t, err)
	return s
}

func TestLockWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Access
		types.Events
	}

	access := local.NewAccessService(bk)
	w, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
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

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Access
		types.Events
	}

	access := local.NewAccessService(bk)
	w, err := services.NewLockWatcher(ctx, services.LockWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
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

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
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
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
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
		cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
		cmpopts.EquateEmpty())
}

// TestDatabaseWatcher tests that database resource watcher properly receives
// and dispatches updates to database resources.
func TestDatabaseWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Databases
		types.Events
	}

	databasesService := local.NewDatabasesService(bk)
	w, err := services.NewDatabaseWatcher(ctx, services.DatabaseWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
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
		require.Empty(t, changeset)
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
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Apps
		types.Events
	}

	appService := local.NewAppService(bk)
	w, err := services.NewAppWatcher(ctx, services.AppWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
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
		require.Empty(t, changeset)
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

func newApp(t *testing.T, name string) *types.AppV3 {
	app, err := types.NewAppV3(types.Metadata{
		Name: name,
	}, types.AppSpecV3{
		URI: "localhost",
	})
	require.NoError(t, err)
	return app
}

func TestCertAuthorityWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Trust
		types.Events
	}

	caService := local.NewCAService(bk)
	w, err := services.NewCertAuthorityWatcher(ctx, services.CertAuthorityWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
			Client: &client{
				Trust:  caService,
				Events: local.NewEventsService(bk),
			},
			Clock: clock,
		},
		Types: []types.CertAuthType{types.HostCA, types.UserCA, types.DatabaseCA, types.OpenSSHCA},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	waitForEvent := func(t *testing.T, sub types.Watcher, caType types.CertAuthType, clusterName string, op types.OpType) {
		select {
		case event := <-sub.Events():
			require.Equal(t, types.KindCertAuthority, event.Resource.GetKind())
			require.Equal(t, string(caType), event.Resource.GetSubKind())
			require.Equal(t, clusterName, event.Resource.GetName())
			require.Equal(t, op, event.Type)
			require.Empty(t, sub.Events()) // no more events.
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for event")
		}
	}

	ensureNoEvents := func(t *testing.T, sub types.Watcher) {
		select {
		case event := <-sub.Events():
			t.Fatalf("Unexpected event: %v.", event)
		case <-sub.Done():
			t.Fatal("CA watcher subscription has unexpectedly exited.")
		case <-time.After(time.Second):
		}
	}

	t.Run("Subscribe all", func(t *testing.T) {
		// Use nil CertAuthorityFilter to subscribe all events from the watcher.
		sub, err := w.Subscribe(ctx, nil)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sub.Close()) })

		// Create a CA and ensure we receive the event.
		ca := newCertAuthority(t, "test", types.HostCA)
		require.NoError(t, caService.UpsertCertAuthority(ctx, ca))
		waitForEvent(t, sub, types.HostCA, "test", types.OpPut)

		// Delete a CA and ensure we receive the event.
		require.NoError(t, caService.DeleteCertAuthority(ctx, ca.GetID()))
		waitForEvent(t, sub, types.HostCA, "test", types.OpDelete)

		// Create a CA with a type that the watcher is NOT receiving and ensure
		// we DO NOT receive the event.
		signer := newCertAuthority(t, "test", types.JWTSigner)
		require.NoError(t, caService.UpsertCertAuthority(ctx, signer))
		ensureNoEvents(t, sub)
	})

	t.Run("Subscribe with filter", func(t *testing.T) {
		sub, err := w.Subscribe(ctx,
			types.CertAuthorityFilter{
				types.HostCA: "test",
				types.UserCA: types.Wildcard,
			},
		)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sub.Close()) })

		// Receives one HostCA event, matched by type and specific cluster name.
		require.NoError(t, caService.UpsertCertAuthority(ctx, newCertAuthority(t, "test", types.HostCA)))
		waitForEvent(t, sub, types.HostCA, "test", types.OpPut)

		// Receives one UserCA event, matched by type and wildcard cluster name.
		require.NoError(t, caService.UpsertCertAuthority(ctx, newCertAuthority(t, "unknown", types.UserCA)))
		waitForEvent(t, sub, types.UserCA, "unknown", types.OpPut)

		// Should NOT receive any HostCA events from another cluster.
		require.NoError(t, caService.UpsertCertAuthority(ctx, newCertAuthority(t, "unknown", types.HostCA)))
		// Should NOT receive any DatabaseCA events.
		require.NoError(t, caService.UpsertCertAuthority(ctx, newCertAuthority(t, "test", types.DatabaseCA)))
		ensureNoEvents(t, sub)
	})
}

func newCertAuthority(t *testing.T, name string, caType types.CertAuthType) types.CertAuthority {
	ta := testauthority.New()
	priv, pub, err := ta.GenerateKeyPair()
	require.NoError(t, err)

	// CA for cluster1 with 1 key pair.
	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: name}, nil, time.Minute)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: name,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{
				{
					PrivateKey:     priv,
					PrivateKeyType: types.PrivateKeyType_RAW,
					PublicKey:      pub,
				},
			},
			TLS: []*types.TLSKeyPair{
				{
					Cert: cert,
					Key:  key,
				},
			},
			JWT: []*types.JWTKeyPair{
				{
					PublicKey:  []byte(fixtures.JWTSignerPublicKey),
					PrivateKey: []byte(fixtures.JWTSignerPrivateKey),
				},
			},
		},
	})
	require.NoError(t, err)
	return ca
}

type unhealthyWatcher struct{}

func (f unhealthyWatcher) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return nil, trace.LimitExceeded("too many watchers")
}

// TestNodeWatcherFallback validates that calling GetNodes on
// a NodeWatcher will pull data from the cache if the resourceWatcher
// run loop is unhealthy due to issues creating a types.Watcher.
func TestNodeWatcherFallback(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Presence
		types.Events
	}

	presence := local.NewPresenceService(bk)
	w, err := services.NewNodeWatcher(ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client: &client{
				Presence: presence,
				Events:   unhealthyWatcher{},
			},
			MaxStaleness: time.Minute,
		},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// Add some servers.
	nodes := make([]types.Server, 0, 5)
	for i := 0; i < 5; i++ {
		node := newNodeServer(t, fmt.Sprintf("node%d", i), fmt.Sprintf("hostname%d", i), "127.0.0.1:2023", i%2 == 0)
		_, err = presence.UpsertNode(ctx, node)
		require.NoError(t, err)
		nodes = append(nodes, node)
	}

	require.Empty(t, w.NodeCount())
	require.False(t, w.IsInitialized())

	got := w.GetNodes(ctx, func(n services.Node) bool {
		return true
	})
	require.Len(t, nodes, len(got))

	require.Len(t, nodes, w.NodeCount())
	require.False(t, w.IsInitialized())
}

func TestNodeWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Presence
		types.Events
	}

	presence := local.NewPresenceService(bk)
	w, err := services.NewNodeWatcher(ctx, services.NodeWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client: &client{
				Presence: presence,
				Events:   local.NewEventsService(bk),
			},
			MaxStaleness: time.Minute,
		},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	require.NoError(t, w.WaitInitialization())
	// Add some node servers.
	nodes := make([]types.Server, 0, 5)
	for i := 0; i < 5; i++ {
		node := newNodeServer(t, fmt.Sprintf("node%d", i), fmt.Sprintf("hostname%d", i), "127.0.0.1:2023", i%2 == 0)
		_, err = presence.UpsertNode(ctx, node)
		require.NoError(t, err)
		nodes = append(nodes, node)
	}

	require.Eventually(t, func() bool {
		filtered := w.GetNodes(ctx, func(n services.Node) bool {
			return true
		})
		return len(filtered) == len(nodes)
	}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive nodes.")

	require.Len(t, w.GetNodes(ctx, func(n services.Node) bool { return n.GetUseTunnel() }), 3)

	require.NoError(t, presence.DeleteNode(ctx, apidefaults.Namespace, nodes[0].GetName()))

	require.Eventually(t, func() bool {
		filtered := w.GetNodes(ctx, func(n services.Node) bool {
			return true
		})
		return len(filtered) == len(nodes)-1
	}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive nodes.")

	require.Empty(t, w.GetNodes(ctx, func(n services.Node) bool { return n.GetName() == nodes[0].GetName() }))
}

func newNodeServer(t *testing.T, name, hostname, addr string, tunnel bool) types.Server {
	s, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{
		Addr:      addr,
		UseTunnel: tunnel,
		Hostname:  hostname,
	})
	require.NoError(t, err)
	return s
}

func TestKubeServerWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.Presence
		types.Events
	}

	presence := local.NewPresenceService(bk)
	w, err := services.NewKubeServerWatcher(ctx, services.KubeServerWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client: &client{
				Presence: presence,
				Events:   local.NewEventsService(bk),
			},
			MaxStaleness: time.Minute,
		},
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	require.NoError(t, w.WaitInitialization())
	newKubeServer := func(t *testing.T, name, addr, hostID string) types.KubeServer {
		kube, err := types.NewKubernetesClusterV3(
			types.Metadata{
				Name: name,
			},
			types.KubernetesClusterSpecV3{})
		require.NoError(t, err)
		server, err := types.NewKubernetesServerV3FromCluster(kube, addr, hostID)
		require.NoError(t, err)
		return server
	}

	// Add some kube servers.
	kubeServers := make([]types.KubeServer, 0, 5)
	for i := 0; i < 5; i++ {
		kubeServer := newKubeServer(t, fmt.Sprintf("kube_cluster-%d", i), "addr", fmt.Sprintf("host-%d", i))
		_, err = presence.UpsertKubernetesServer(ctx, kubeServer)
		require.NoError(t, err)
		kubeServers = append(kubeServers, kubeServer)
	}

	require.Eventually(t, func() bool {
		filtered, err := w.GetKubernetesServers(context.Background())
		assert.NoError(t, err)
		return len(filtered) == len(kubeServers)
	}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive kube servers.")

	// Test filtering by cluster name.
	filtered, err := w.GetKubeServersByClusterName(context.Background(), kubeServers[0].GetName())
	require.NoError(t, err)
	require.Len(t, filtered, 1)

	// Test Deleting a kube server.
	require.NoError(t, presence.DeleteKubernetesServer(ctx, kubeServers[0].GetHostID(), kubeServers[0].GetName()))
	require.Eventually(t, func() bool {
		kube, err := w.GetKubernetesServers(context.Background())
		assert.NoError(t, err)
		return len(kube) == len(kubeServers)-1
	}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive the delete event.")

	filtered, err = w.GetKubeServersByClusterName(context.Background(), kubeServers[0].GetName())
	require.Error(t, err)
	require.Empty(t, filtered)

	// Test adding a kube server with the same name as an existing one.
	kubeServer := newKubeServer(t, kubeServers[1].GetName(), "addr", uuid.NewString())
	_, err = presence.UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		filtered, err := w.GetKubeServersByClusterName(context.Background(), kubeServers[1].GetName())
		assert.NoError(t, err)
		return len(filtered) == 2
	}, time.Second, time.Millisecond, "Timeout waiting for watcher to the new registered kube server.")

	// Test deleting all kube servers with the same name.
	filtered, err = w.GetKubeServersByClusterName(context.Background(), kubeServers[1].GetName())
	assert.NoError(t, err)
	for _, server := range filtered {
		require.NoError(t, presence.DeleteKubernetesServer(ctx, server.GetHostID(), server.GetName()))
	}
	require.Eventually(t, func() bool {
		filtered, err := w.GetKubeServersByClusterName(context.Background(), kubeServers[1].GetName())
		return len(filtered) == 0 && err != nil
	}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive the two delete events.")

	require.NoError(t, presence.DeleteAllKubernetesServers(ctx))
	require.Eventually(t, func() bool {
		filtered, err := w.GetKubernetesServers(context.Background())
		assert.NoError(t, err)
		return len(filtered) == 0
	}, time.Second, time.Millisecond, "Timeout waiting for watcher to receive all delete events.")
}

// TestAccessRequestWatcher tests that access request resource watcher properly receives
// and dispatches updates to access request resources.
func TestAccessRequestWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.DynamicAccessCore
		types.Events
	}

	dynamicAccessService := local.NewDynamicAccessService(bk)
	w, err := services.NewAccessRequestWatcher(ctx, services.AccessRequestWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
			Client: &client{
				DynamicAccessCore: dynamicAccessService,
				Events:            local.NewEventsService(bk),
			},
		},
		AccessRequestsC: make(chan types.AccessRequests, 10),
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// Initially there are no access requests so watcher should send an empty list.
	select {
	case changeset := <-w.AccessRequestsC:
		require.Empty(t, changeset)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add an access request.
	accessRequest1 := newAccessRequest(t, uuid.NewString())
	accessRequest1, err = dynamicAccessService.CreateAccessRequestV2(ctx, accessRequest1)
	require.NoError(t, err)

	// The first event is always the current list of access requests.
	select {
	case changeset := <-w.AccessRequestsC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], accessRequest1))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add a second access request.
	accessRequest2 := newAccessRequest(t, uuid.NewString())
	accessRequest2, err = dynamicAccessService.CreateAccessRequestV2(ctx, accessRequest2)
	require.NoError(t, err)

	// Watcher should detect the access request list change.
	select {
	case changeset := <-w.AccessRequestsC:
		require.Len(t, changeset, 2)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the second event.")
	}

	// Change the second access request
	accessRequest2.SetState(types.RequestState_APPROVED)
	require.NoError(t, dynamicAccessService.UpsertAccessRequest(ctx, accessRequest2))

	// Watcher should detect the access request list change.
	select {
	case changeset := <-w.AccessRequestsC:
		require.Len(t, changeset, 2)
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the updated event.")
	}

	// Delete the first access request.
	require.NoError(t, dynamicAccessService.DeleteAccessRequest(ctx, accessRequest1.GetName()))

	// Watcher should detect the access request list change.
	select {
	case changeset := <-w.AccessRequestsC:
		require.Len(t, changeset, 1)
		require.Empty(t, resourceDiff(changeset[0], accessRequest2))
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the update event.")
	}
}

func newAccessRequest(t *testing.T, name string) types.AccessRequest {
	accessRequest, err := types.NewAccessRequest(name, "test-user", "role1")
	accessRequest.SetState(types.RequestState_PENDING)
	require.NoError(t, err)
	return accessRequest
}

// TestOktaAssignmentWatcher tests that Okta assignment resource watcher properly receives
// and dispatches updates to Okta assignment resources.
func TestOktaAssignmentWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	type client struct {
		services.OktaAssignments
		types.Events
	}

	oktaService, err := local.NewOktaService(bk, clock)
	require.NoError(t, err)
	w, err := services.NewOktaAssignmentWatcher(ctx, services.OktaAssignmentWatcherConfig{
		RWCfg: services.ResourceWatcherConfig{
			Component:      "test",
			MaxRetryPeriod: 200 * time.Millisecond,
			Client: &client{
				OktaAssignments: oktaService,
				Events:          local.NewEventsService(bk),
			},
		},
		OktaAssignments:  oktaService,
		PageSize:         1, // Set page size to 1 to exercise pagination logic.
		OktaAssignmentsC: make(chan types.OktaAssignments, 10),
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// Initially there are no assignments so watcher should send an empty list.
	select {
	case changeset := <-w.CollectorChan():
		require.Empty(t, changeset, "initial assignment list should be empty")
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the initial empty event.")
	}

	// Add an assignment.
	a1 := newOktaAssignment(t, uuid.NewString())
	_, err = oktaService.CreateOktaAssignment(ctx, a1)
	require.NoError(t, err)

	// The first event is always the current list of assignments.
	select {
	case changeset := <-w.CollectorChan():
		expected := types.OktaAssignments{a1}
		sortedChangeset := changeset
		sort.Sort(expected)
		sort.Sort(sortedChangeset)

		require.Empty(t,
			cmp.Diff(expected,
				changeset,
				cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
			"should be no differences in the changeset after adding the first assignment")
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the first event.")
	}

	// Add a second assignment.
	a2 := newOktaAssignment(t, uuid.NewString())
	_, err = oktaService.CreateOktaAssignment(ctx, a2)
	require.NoError(t, err)

	// Watcher should detect the assignment list change.
	select {
	case changeset := <-w.CollectorChan():
		expected := types.OktaAssignments{a1, a2}
		sort.Sort(expected)
		sort.Sort(changeset)

		require.Empty(t,
			cmp.Diff(
				expected,
				changeset,
				cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
			"should be no difference in the changeset after adding the second assignment")
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the second event.")
	}

	// Change the second assignment.
	a2.SetExpiry(time.Now().Add(30 * time.Minute))
	_, err = oktaService.UpdateOktaAssignment(ctx, a2)
	require.NoError(t, err)

	// Watcher should detect the assignment list change.
	select {
	case changeset := <-w.CollectorChan():
		expected := types.OktaAssignments{a1, a2}
		sort.Sort(expected)
		sort.Sort(changeset)

		require.Empty(t,
			cmp.Diff(
				expected,
				changeset,
				cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
			"should be no difference in the changeset after update")
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the updated event.")
	}

	// Delete the first assignment.
	require.NoError(t, oktaService.DeleteOktaAssignment(ctx, a1.GetName()))

	// Watcher should detect the Okta assignment list change.
	select {
	case changeset := <-w.CollectorChan():
		expected := types.OktaAssignments{a2}
		sort.Sort(expected)
		sort.Sort(changeset)

		require.Empty(t,
			cmp.Diff(
				expected,
				changeset,
				cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")),
			"should be no difference in the changeset after deleting the first assignment")
	case <-w.Done():
		t.Fatal("Watcher has unexpectedly exited.")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for the delete event.")
	}
}

func newOktaAssignment(t *testing.T, name string) types.OktaAssignment {
	assignment, err := types.NewOktaAssignment(
		types.Metadata{
			Name: name,
		},
		types.OktaAssignmentSpecV1{
			User: "test-user@test.user",
			Targets: []*types.OktaAssignmentTargetV1{
				{
					Type: types.OktaAssignmentTargetV1_APPLICATION,
					Id:   "123456",
				},
			},
			Status: types.OktaAssignmentSpecV1_PENDING,
		},
	)
	require.NoError(t, err)
	return assignment
}
