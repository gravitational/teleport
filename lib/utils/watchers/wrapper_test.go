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

package watchers_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/watchers"
)

func TestWrapper(t *testing.T) {
	t.Parallel()

	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)

	// Any service works, ClusterConfig is a relatively simple one.
	clusterConfigService, err := local.NewClusterConfigurationService(mem)
	require.NoError(t, err)
	eventsService := local.NewEventsService(mem)

	watcher, err := watchers.NewWrapper(watchers.WrapperConfig{
		Source:            eventsService,
		EventsChannelSize: 1, // arbitrary
		Watch: &types.Watch{
			Name: "test-watcher",
			Kinds: []types.WatchKind{
				{Kind: types.KindClusterAuthPreference},
			},
		},
	})
	require.NoError(t, err)

	// Start watcher.
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() { _ = watcher.Run(ctx) })
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	assertNoEvent := func(t *testing.T) {
		t.Helper()
		select {
		case e := <-watcher.Events():
			t.Errorf("Got unexpected event: %#v", e)
		case <-time.After(1 * time.Millisecond):
			// OK.
		}
	}

	assertHasEvent := func(t *testing.T, want types.OpType) {
		t.Helper()

		select {
		case e := <-watcher.Events():
			assert.Equal(t, want, e.Type, "Event type mismatch")
		case <-time.After(2 * time.Second):
			t.Error("Timed out waiting for event")
		}
	}

	// Wait for connection, otherwise we can miss the first event if it goes too
	// fast.
	select {
	case <-watcher.WaitUntilHealthy():
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for watcher first connection")
	}

	// 1st event: OpPut (create).
	pref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:          constants.Local,
		SecondFactors: []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_OTP},
	})
	require.NoError(t, err)
	pref, err = clusterConfigService.CreateAuthPreference(ctx, pref)
	require.NoError(t, err)
	assertHasEvent(t, types.OpPut)

	// 2st event: OpPut (update).
	pref.SetDefaultSessionTTL(types.Duration(4 * time.Hour))
	pref, err = clusterConfigService.UpdateAuthPreference(ctx, pref)
	require.NoError(t, err)
	assertHasEvent(t, types.OpPut)

	// 3rd event: OpDelete.
	require.NoError(t,
		clusterConfigService.DeleteAuthPreference(ctx),
	)
	assertHasEvent(t, types.OpDelete)

	// Stop watcher.
	cancel()
	wg.Wait() // Confirm goroutine stopped.

	// Further events are not monitored.
	_, err = clusterConfigService.CreateAuthPreference(ctx, pref)
	require.NoError(t, err)
	assertNoEvent(t)
}

func TestWrapper_reconnection(t *testing.T) {
	t.Parallel()

	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)

	// Any service works, ClusterConfig is a relatively simple one.
	clusterConfigService, err := local.NewClusterConfigurationService(mem)
	require.NoError(t, err)

	sw := &sourceWrapper{
		source: local.NewEventsService(mem),
	}

	watcher, err := watchers.NewWrapper(watchers.WrapperConfig{
		Source:            sw,
		EventsChannelSize: 1, // arbitrary
		Watch: &types.Watch{
			Name: "test-watcher",
			Kinds: []types.WatchKind{
				{Kind: types.KindClusterAuthPreference},
			},
		},
	})
	require.NoError(t, err)

	// Start watcher.
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	wg.Go(func() { _ = watcher.Run(ctx) })
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	waitForHealthy := func(*testing.T) {
		t.Helper()
		select {
		case <-watcher.WaitUntilHealthy():
		case <-time.After(2 * time.Second):
			t.Fatal("Timed out waiting for watcher first connection")
		}
	}

	assertHasEvent := func(t *testing.T, want types.OpType) {
		t.Helper()
		select {
		case e := <-watcher.Events():
			assert.Equal(t, want, e.Type, "Event type mismatch")
		case <-time.After(2 * time.Second):
			t.Error("Timed out waiting for event")
		}
	}

	// Wait until healthy.
	waitForHealthy(t)

	// Force 1st reconnection.
	_ = sw.MustWatcher(t).Close()
	waitForHealthy(t)

	// Fire 1st event.
	pref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:          constants.Local,
		SecondFactors: []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_OTP},
	})
	require.NoError(t, err)
	_, err = clusterConfigService.CreateAuthPreference(ctx, pref)
	require.NoError(t, err)
	assertHasEvent(t, types.OpPut)

	// Force 2nd reconnection.
	_ = sw.MustWatcher(t).Close()
	waitForHealthy(t)

	// Fire 2nd event.
	require.NoError(t,
		clusterConfigService.DeleteAuthPreference(ctx),
	)
	assertHasEvent(t, types.OpDelete)
}

// sourceWrapper wraps a watchers.WatcherSource and captures the returned
// Watcher, so it may be forcefully closed.
//
// Useful to test reconnection.
type sourceWrapper struct {
	source watchers.WatcherSource

	mu          sync.Mutex
	lastWatcher types.Watcher
	lastErr     error
}

func (sw *sourceWrapper) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	w, err := sw.source.NewWatcher(ctx, watch)

	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.lastWatcher = w
	sw.lastErr = err
	return w, err
}

func (sw *sourceWrapper) LastWatcher() (types.Watcher, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.lastErr != nil {
		return nil, sw.lastErr
	}
	if sw.lastWatcher == nil {
		return nil, errors.New("watcher not created yet")
	}
	return sw.lastWatcher, nil
}

func (sw *sourceWrapper) MustWatcher(t *testing.T) types.Watcher {
	t.Helper()

	w, err := sw.LastWatcher()
	require.NoError(t, err)
	return w
}
