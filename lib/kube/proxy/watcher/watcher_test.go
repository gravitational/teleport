// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package watcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services/readonly"
)

// mockKubeServerWatcherGetter is a testify mock for the [services.KubernetesServerWatcherGetter] interface.
type mockKubeServerWatcherGetter struct {
	mock.Mock
}

func (m *mockKubeServerWatcherGetter) GetKubernetesServers(ctx context.Context) ([]types.KubeServer, error) {
	args := m.Called(ctx)
	return args.Get(0).([]types.KubeServer), args.Error(1)
}

func (m *mockKubeServerWatcherGetter) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	args := m.Called(ctx, watch)
	if w := args.Get(0); w != nil {
		return w.(types.Watcher), args.Error(1)
	}
	return nil, args.Error(1)
}

// fakeWatcher is a mocked implemenation of the [types.Watcher] interface that allows sending events and simulating errors.
type fakeWatcher struct {
	events chan types.Event
	done   chan struct{}
	err    error

	once sync.Once
}

func newFakeWatcher(buffer int) *fakeWatcher {
	return &fakeWatcher{
		events: make(chan types.Event, buffer),
		done:   make(chan struct{}),
	}
}

func (f *fakeWatcher) Events() <-chan types.Event {
	return f.events
}

func (f *fakeWatcher) Done() <-chan struct{} {
	return f.done
}

func (f *fakeWatcher) Error() error {
	return f.err
}

func (f *fakeWatcher) Close() error {
	f.once.Do(func() {
		close(f.done)
		close(f.events)
	})
	return nil
}

func (f *fakeWatcher) send(event types.Event) {
	f.events <- event
}

func (f *fakeWatcher) closeWithError(err error) {
	f.err = err
	f.Close()
}

// newTestKubeServer is a testing helper to create a new Kubernetes server with the given name and hostID.
func newTestKubeServer(t *testing.T, name, hostID string) types.KubeServer {
	t.Helper()
	return newTestKubeServerWithRevsion(t, name, hostID, 0)
}

func newTestKubeServerWithRevsion(t *testing.T, name, hostID string, revsion int) types.KubeServer {
	t.Helper()
	s, err := types.NewKubernetesServerV3(
		types.Metadata{
			Name:        name,
			Description: hostID,
			Revision:    fmt.Sprintf("%d", revsion),
		},
		types.KubernetesServerSpecV3{
			HostID: hostID,
			Cluster: &types.KubernetesClusterV3{
				Metadata: types.Metadata{Name: name},
				Spec:     types.KubernetesClusterSpecV3{},
			},
		},
	)

	require.NoError(t, err)
	return s
}

func testProxyKubeServerWatcherStartsWithFaultyPrimarySynctest(t *testing.T) {
	ctx := t.Context()
	noopFilter := func(k readonly.KubeServer) bool { return true }

	primary := &mockKubeServerWatcherGetter{}
	fallback := &mockKubeServerWatcherGetter{}

	var calls int32

	primary.On("NewWatcher", mock.Anything, mock.Anything).
		Return(nil, context.DeadlineExceeded).
		Run(func(args mock.Arguments) {
			atomic.AddInt32(&calls, 1)
		})

	fallback.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{newTestKubeServer(t, "foo", "bar")}, nil).Twice()

	cfg := ProxyKubeServerWatcherConfig{
		Component:        teleport.ComponentProxy,
		AccessPoint:      primary,
		FallbackGetter:   fallback,
		MaxRetryPeriod:   time.Second * 10,
		PrimaryTimeout:   time.Second,
		FallbackInterval: time.Second,
	}

	w, err := NewProxyKubeServerWatcher(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(w.Close)

	waitCh := make(chan error)
	defer close(waitCh)

	go func() {
		// This should block until the watcher is ready or closed.
		waitCh <- w.WaitInitialization()
	}()

	require.False(t, w.hot.Load(), "Watcher starts cold")
	require.False(t, w.IsInitialized())

	time.Sleep(2 * time.Second)
	require.False(t, w.IsInitialized())
	require.False(t, w.hot.Load(), "Watcher should not be hot since primary is failing")

	srvs, err := w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, srvs, 1)
	require.Equal(t, "foo", srvs[0].GetName())

	time.Sleep(1 * time.Second)

	var wg sync.WaitGroup

	type result struct {
		srvs []types.KubeServer
		err  error
	}

	// Concurrent access will only result in one call to fallback
	results := make(chan result, 64)
	for range 64 {
		wg.Go(func() {
			srvs, err := w.CurrentResourcesWithFilter(ctx, noopFilter)
			results <- result{srvs: srvs, err: err}
		})
	}

	wg.Wait()
	close(results)
	for res := range results {
		require.NoError(t, res.err)
		require.Len(t, res.srvs, 1)
		require.Equal(t, "foo", res.srvs[0].GetName())
	}

	w.Close() // closes context

	waitErr := <-waitCh
	require.Error(t, waitErr)
	require.ErrorContains(t, waitErr, "context closing")

	select {
	case <-w.Done():
	default:
		t.Fatal("Watcher did not close in time")
	}

	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2))
	primary.AssertExpectations(t)
	fallback.AssertExpectations(t)
}

func TestProxyKubeServerWatcher_StartsWithFaultyPrimary(t *testing.T) {
	synctest.Test(t, testProxyKubeServerWatcherStartsWithFaultyPrimarySynctest)
}

func testWatcherProcessesEventsSynctest(t *testing.T) {
	ctx := t.Context()
	noopFilter := func(k readonly.KubeServer) bool { return true }

	fw := newFakeWatcher(10)
	t.Cleanup(func() { fw.Close() })
	primary := &mockKubeServerWatcherGetter{}
	fallback := &mockKubeServerWatcherGetter{}

	// In the happy path we expect a single call to the primary to pre-warm the cache.
	primary.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{
			newTestKubeServer(t, "initial", "host1"),
		}, nil).Once()

	watcherReady := make(chan time.Time)
	primary.On("NewWatcher", mock.Anything, mock.Anything).Return(fw, nil).WaitUntil(watcherReady).Once()

	fallback.On("GetKubernetesServers", mock.Anything).Return([]types.KubeServer{}, nil).Once()

	w, err := NewProxyKubeServerWatcher(ctx, ProxyKubeServerWatcherConfig{
		Component:      teleport.ComponentProxy,
		AccessPoint:    primary,
		FallbackGetter: fallback,
		PrimaryTimeout: time.Second,
		MaxRetryPeriod: time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	synctest.Wait()

	// Single call to backup since watcher is not ready.
	resources, err := w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Empty(t, resources, "Watcher should start with empty cache before warm-up")

	watcherReady <- time.Now() // unblock watcher
	fw.send(types.Event{Type: types.OpInit})
	require.NoError(t, w.WaitInitialization())
	require.True(t, w.IsInitialized())
	require.True(t, w.hot.Load(), "Watcher should be hot after receiving OpInit event")

	fw.send(types.Event{Type: types.OpPut, Resource: newTestKubeServer(t, "new", "host2")})
	synctest.Wait()

	resources, err = w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	// Test that incorrect resource kinds are ignored.
	fw.send(types.Event{Type: types.OpPut, Resource: &types.DatabaseServerV3{Kind: "oops"}})
	synctest.Wait()

	// Test that resource conversion errors are handled gracefully.
	fw.send(types.Event{Type: types.OpPut, Resource: &types.DatabaseServerV3{Kind: types.KindKubeServer}})
	synctest.Wait()

	// Invalid op types handled gracefully.
	fw.send(types.Event{Type: types.OpInvalid, Resource: newTestKubeServer(t, "new", "host2")})
	synctest.Wait()

	fw.send(types.Event{Type: types.OpDelete, Resource: newTestKubeServer(t, "new", "host2")})
	synctest.Wait()

	resources, err = w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	require.Equal(t, "initial", resources[0].GetName())

	primary.AssertExpectations(t)
	fallback.AssertExpectations(t)
}

func TestProxyKubeServerWatcher_ProcessesEvents(t *testing.T) {
	synctest.Test(t, testWatcherProcessesEventsSynctest)
}

func testProxyKubeServerWatcherRetryWatchAfterTimeoutSynctest(t *testing.T) {
	ctx := t.Context()

	fw1 := newFakeWatcher(20)
	fw2 := newFakeWatcher(20)

	primary := &mockKubeServerWatcherGetter{}
	fallback := &mockKubeServerWatcherGetter{}

	primary.On("NewWatcher", mock.Anything, mock.Anything).
		Return(fw1, nil).Once()
	primary.On("NewWatcher", mock.Anything, mock.Anything).
		Return(fw2, nil).Once()
	primary.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{}, nil).Twice()

	w, _ := NewProxyKubeServerWatcher(ctx, ProxyKubeServerWatcherConfig{
		Component:      teleport.ComponentProxy,
		AccessPoint:    primary,
		FallbackGetter: fallback,
		PrimaryTimeout: 10 * time.Second,
	})
	t.Cleanup(w.Close)

	fw1.send(types.Event{Type: types.OpInit})
	synctest.Wait()

	fw1.closeWithError(context.DeadlineExceeded)
	synctest.Wait()
	// Initially the cache is still hot
	require.True(t, w.hot.Load(), "expected watcher to be hot before primary timeout")
	time.Sleep(10*time.Second + time.Millisecond)
	require.False(t, w.hot.Load(), "expected watcher to be cold after primary timeout")

	fw2.send(types.Event{Type: types.OpInit})
	synctest.Wait()

	require.True(t, w.hot.Load(), "expected watcher to be hot")

	noopFilter := func(k readonly.KubeServer) bool { return true }

	// Verify no calls to the back up are made.
	srvs, err := w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Empty(t, srvs)

	primary.AssertExpectations(t)
	fallback.AssertExpectations(t)

}

func TestProxyKubeServerWatcher_RetryWatchAfterTimeout(t *testing.T) {
	synctest.Test(t, testProxyKubeServerWatcherRetryWatchAfterTimeoutSynctest)
}

func testProxyKubeServerWatcherRecoversAfterTimeoutSynctest(t *testing.T) {
	ctx := t.Context()

	fw := newFakeWatcher(1)

	primary := &mockKubeServerWatcherGetter{}
	fallback := &mockKubeServerWatcherGetter{}

	primary.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{}, nil).Once().NotBefore(
		primary.On("NewWatcher", mock.Anything, mock.Anything).
			Return(fw, nil).
			Once(),
	)

	w, err := NewProxyKubeServerWatcher(ctx, ProxyKubeServerWatcherConfig{
		Component:      teleport.ComponentProxy,
		AccessPoint:    primary,
		FallbackGetter: fallback,
		PrimaryTimeout: 20 * time.Millisecond,
		MaxRetryPeriod: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	time.Sleep(21 * time.Millisecond)

	require.False(t, w.hot.Load())
	fw.send(types.Event{Type: types.OpInit})
	synctest.Wait()

	require.True(t, w.hot.Load())

	primary.AssertExpectations(t)
	fallback.AssertExpectations(t)
}

func TestProxyKubeServerWatcher_RecoversAfterTimeout(t *testing.T) {
	synctest.Test(t, testProxyKubeServerWatcherRecoversAfterTimeoutSynctest)
}

func testProxyKubeServerWatcherKeepsStaleServersSynctest(t *testing.T) {
	ctx := t.Context()
	noopFilter := func(k readonly.KubeServer) bool { return true }
	const numberOfEvents = 256
	fw := newFakeWatcher(512)
	t.Cleanup(func() { fw.Close() })
	primary := &mockKubeServerWatcherGetter{}
	fallback := &mockKubeServerWatcherGetter{}

	primary.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{}, nil).Once().NotBefore(
		primary.On("NewWatcher", mock.Anything, mock.Anything).
			Return(fw, nil).
			Once(),
	)

	w, err := NewProxyKubeServerWatcher(ctx, ProxyKubeServerWatcherConfig{
		Component:        teleport.ComponentProxy,
		AccessPoint:      primary,
		FallbackGetter:   fallback,
		PrimaryTimeout:   time.Second,
		FallbackInterval: time.Second,
		MaxRetryPeriod:   time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	fw.send(types.Event{Type: types.OpInit})
	synctest.Wait()
	require.True(t, w.hot.Load(), "Watcher should be hot after receiving OpInit event")

	for i := range numberOfEvents {
		srv := newTestKubeServerWithRevsion(t, "foo", "host", i)
		fw.send(types.Event{Type: types.OpPut, Resource: srv})

		if i%16 == 0 {
			// simulate some batching.
			synctest.Wait()
		}
	}

	synctest.Wait()

	srvs, err := w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, srvs, 1)
	require.Equal(t, "foo", srvs[0].GetName())
	require.Equal(t, fmt.Sprintf("%d", numberOfEvents-1), srvs[0].GetRevision())

	fw.send(types.Event{Type: types.OpDelete, Resource: srvs[0]})
	synctest.Wait()

	srvs, err = w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Empty(t, srvs)

	expectedByName := make(map[string]types.KubeServer)
	for i := range numberOfEvents {
		srv := newTestKubeServer(t, fmt.Sprintf("kube-%d", i), "host")
		expectedByName[srv.GetName()] = srv
		fw.send(types.Event{Type: types.OpPut, Resource: srv})

		if i%16 == 0 {
			synctest.Wait()
		}
	}

	synctest.Wait()

	srvs, err = w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, srvs, numberOfEvents)

	for _, srv := range srvs {
		expected, ok := expectedByName[srv.GetName()]
		require.True(t, ok, "unexpected server name: %s", srv.GetName())
		require.Equal(t, expected, srv)
		delete(expectedByName, srv.GetName())
	}

	primary.On("NewWatcher", mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded)

	// simulate failing fallback
	fallback.On("GetKubernetesServers", mock.Anything).Return([]types.KubeServer{}, context.DeadlineExceeded).Once()

	fw.closeWithError(context.DeadlineExceeded)
	require.True(t, w.hot.Load(), "Watcher remains hot for grace period")
	synctest.Wait()

	srvs, err = w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, srvs, numberOfEvents)

	time.Sleep(10 * time.Second) // exceed max staleness.
	require.False(t, w.hot.Load(), "Watcher is not hot after max staleness is exceeded")

	srvs, err = w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, srvs, numberOfEvents)

	// Simulate recovery of the primary.
	fw2 := newFakeWatcher(512)
	primary.On("GetKubernetesServers", mock.Anything).
		Return(srvs, nil).Once().NotBefore(
		primary.On("NewWatcher", mock.Anything, mock.Anything).Unset(),
		primary.On("NewWatcher", mock.Anything, mock.Anything).
			Return(fw2, nil).
			Once(),
	)

	fw2.send(types.Event{Type: types.OpInit})
	time.Sleep(10 * time.Second) // exceed retry staleness.

	require.True(t, w.hot.Load(), "Watcher should be hot after new watcher is created and receives OpInit")

	// Calls should not be routed to fallback.
	srvs, err = w.CurrentResourcesWithFilter(ctx, noopFilter)
	require.NoError(t, err)
	require.Len(t, srvs, numberOfEvents)

	primary.AssertExpectations(t)
	fallback.AssertExpectations(t)
}

func TestProxyKubeServerWatcher_KeepsStaleServers(t *testing.T) {
	synctest.Test(t, testProxyKubeServerWatcherKeepsStaleServersSynctest)
}

func TestProxyKubeServerWatcher_MaybeFetchFromUpstreamDoesNotOverwriteHotCache(t *testing.T) {
	ctx := t.Context()

	primaryServer := newTestKubeServer(t, "primary", "host")
	fallbackServer := newTestKubeServer(t, "fallback", "host")

	fallback := &mockKubeServerWatcherGetter{}
	fetchStarted := make(chan struct{})
	continueFetch := make(chan struct{})
	fallback.On("GetKubernetesServers", mock.Anything).
		Run(func(args mock.Arguments) {
			close(fetchStarted)
			<-continueFetch
		}).
		Return([]types.KubeServer{fallbackServer}, nil).Once()

	cfg := ProxyKubeServerWatcherConfig{
		FallbackGetter:   fallback,
		FallbackInterval: 0,
	}
	w := &ProxyKubeServerWatcher{
		ProxyKubeServerWatcherConfig: cfg,
		current: map[serverKey]types.KubeServer{
			kubeServerKey(primaryServer): primaryServer,
		},
	}

	w.hot.Store(false)

	done := make(chan error, 1)
	go func() {
		done <- w.maybeFetchFromUpstream(ctx)
	}()

	<-fetchStarted
	w.hot.Store(true)
	close(continueFetch)

	require.NoError(t, <-done)

	w.rw.RLock()
	defer w.rw.RUnlock()
	require.Len(t, w.current, 1)
	srv, ok := w.current[kubeServerKey(primaryServer)]
	require.True(t, ok)
	require.Equal(t, primaryServer.GetName(), srv.GetName())

	fallback.AssertExpectations(t)
}

func testProxyKubeServerWatcherDiscardsBadInitEventSynctest(t *testing.T) {
	ctx := t.Context()

	fw1 := newFakeWatcher(20)
	fw2 := newFakeWatcher(20)
	t.Cleanup(func() { fw1.Close() })
	t.Cleanup(func() { fw2.Close() })

	primary := &mockKubeServerWatcherGetter{}
	fallback := &mockKubeServerWatcherGetter{}

	primary.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{}, nil).Once().NotBefore(
		primary.On("NewWatcher", mock.Anything, mock.Anything).
			Return(fw1, nil).
			Once(),
		primary.On("NewWatcher", mock.Anything, mock.Anything).
			Return(fw2, nil).
			Once(),
	)

	w, err := NewProxyKubeServerWatcher(ctx, ProxyKubeServerWatcherConfig{
		Component:      teleport.ComponentProxy,
		AccessPoint:    primary,
		FallbackGetter: fallback,
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	fw1.send(types.Event{Type: types.OpInvalid})
	fw2.send(types.Event{Type: types.OpInit})

	time.Sleep(time.Minute) // wait long enough for retry backoff to init second watcher
	require.True(t, w.hot.Load(), "Watcher should be hot after receiving OpInit event")

	primary.AssertExpectations(t)
	fallback.AssertExpectations(t)
}

func TestProxyKubeServerWatcher_DiscardsBadInitEvent(t *testing.T) {
	synctest.Test(t, testProxyKubeServerWatcherDiscardsBadInitEventSynctest)
}

func testProxyKubeServerWatcherRecoversFromFirstFetchFailSynctest(t *testing.T) {
	ctx := t.Context()

	fw1 := newFakeWatcher(20)
	fw2 := newFakeWatcher(20)
	t.Cleanup(func() { fw1.Close() })
	t.Cleanup(func() { fw2.Close() })

	primary := &mockKubeServerWatcherGetter{}
	fallback := &mockKubeServerWatcherGetter{}

	primary.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{}, context.DeadlineExceeded).Once().NotBefore(
		primary.On("NewWatcher", mock.Anything, mock.Anything).
			Return(fw1, nil).
			Once(),
	)

	primary.On("GetKubernetesServers", mock.Anything).
		Return([]types.KubeServer{}, nil).Once().NotBefore(
		primary.On("NewWatcher", mock.Anything, mock.Anything).
			Return(fw2, nil).
			Once(),
	)

	w, err := NewProxyKubeServerWatcher(ctx, ProxyKubeServerWatcherConfig{
		Component:      teleport.ComponentProxy,
		AccessPoint:    primary,
		FallbackGetter: fallback,
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)
	fw1.send(types.Event{Type: types.OpInit})
	synctest.Wait()
	fw2.send(types.Event{Type: types.OpInit})

	time.Sleep(time.Minute) // wait long enough for retry backoff to init second watcher
	require.True(t, w.hot.Load(), "Watcher should be hot after receiving OpInit event")

	primary.AssertExpectations(t)
	fallback.AssertExpectations(t)
}

func TestProxyKubeServerWatcher_RecoversFromFirstFetchFail(t *testing.T) {
	synctest.Test(t, testProxyKubeServerWatcherRecoversFromFirstFetchFailSynctest)
}

func TestFillEventBuf_DoesNotExceedMaxBufferSize(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	const pageCount = 3
	const bufferSize = 64
	const bufferStartingSize = 16
	const totalEvents = bufferSize * pageCount
	fw := newFakeWatcher(totalEvents)
	t.Cleanup(func() { fw.Close() })

	for i := range totalEvents {
		fw.send(types.Event{Type: types.OpPut, Resource: newTestKubeServer(t, fmt.Sprintf("kube-%d", i), "host")})
	}
	eventBuf := make([]types.Event, 0, bufferStartingSize)
	seen := make(map[string]struct{}, totalEvents)
	for page := range pageCount {
		eventBuf = eventBuf[:0] // caller must reset
		eventBuf = fillEventBuf(ctx, eventBuf, fw, bufferSize)
		require.Len(t, eventBuf, bufferSize, "expected page %d to be full", page)

		pageSeen := make(map[string]struct{}, bufferSize)
		for idx, event := range eventBuf {
			name := event.Resource.GetName()
			require.NotEmpty(t, name)
			require.NotContains(t, pageSeen, name, "duplicate event in page %d", page)
			require.NotContains(t, seen, name, "duplicate event across pages at page %d", page)
			pageSeen[name] = struct{}{}
			seen[name] = struct{}{}

			expectedName := fmt.Sprintf("kube-%d", page*bufferSize+idx)
			require.Equal(t, expectedName, name)
		}
	}

	select {
	case event := <-fw.Events():
		t.Fatalf("expected no remaining events after consuming %d pages, got %s", totalEvents, event.Resource.GetName())
	default:
	}
}
