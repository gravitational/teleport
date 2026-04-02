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
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/watchers"
)

func TestWatcher(t *testing.T) {
	t.Parallel()

	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)

	// Any service works, ClusterConfig is a relatively simple one.
	clusterConfigService, err := local.NewClusterConfigurationService(mem)
	require.NoError(t, err)
	eventsService := local.NewEventsService(mem)

	watcher := newAuthPrefWatcher(t, eventsService)

	assertHasEvent := func(t *testing.T, wantOP types.OpType, wantRevision string) {
		t.Helper()

		select {
		case e := <-watcher.Events():
			assert.Equal(t, wantOP, e.Type, "Event type mismatch")
			assert.Equal(t, wantRevision, e.Resource.GetRevision(), "Event revision mismatch")
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

	ctx := t.Context()

	// 1st event: OpPut (create).
	pref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:          constants.Local,
		SecondFactors: []types.SecondFactorType{types.SecondFactorType_SECOND_FACTOR_TYPE_OTP},
	})
	require.NoError(t, err)
	pref, err = clusterConfigService.CreateAuthPreference(ctx, pref)
	require.NoError(t, err)
	assertHasEvent(t, types.OpPut, pref.GetRevision())

	// 2st event: OpPut (update).
	pref.SetDefaultSessionTTL(types.Duration(4 * time.Hour))
	pref, err = clusterConfigService.UpdateAuthPreference(ctx, pref)
	require.NoError(t, err)
	assertHasEvent(t, types.OpPut, pref.GetRevision())

	// 3rd event: OpDelete.
	require.NoError(t,
		clusterConfigService.DeleteAuthPreference(ctx),
	)
	assertHasEvent(t, types.OpDelete, "" /* wantRevision */)
}

func TestWatcher_reconnection(t *testing.T) {
	t.Parallel()

	mem, err := memory.New(memory.Config{})
	require.NoError(t, err)

	// Any service works, ClusterConfig is a relatively simple one.
	clusterConfigService, err := local.NewClusterConfigurationService(mem)
	require.NoError(t, err)

	sw := &sourceWrapper{
		source: local.NewEventsService(mem),
	}

	watcher := newAuthPrefWatcher(t, sw)

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

	ctx := t.Context()

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

func TestWatcher_backoff(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		mem, err := memory.New(memory.Config{})
		require.NoError(t, err)

		sw := &sourceWrapper{
			source: local.NewEventsService(mem),
		}
		// Setup so we fail 2 attempts.
		sw.SetAttemptsToFail(2)

		watcher := newAuthPrefWatcher(t, sw)

		waitForHealthy := func(*testing.T) {
			t.Helper()
			select {
			case <-watcher.WaitUntilHealthy():
			case <-time.After(2 * time.Second):
				t.Fatal("Timed out waiting for watcher first connection")
			}
		}

		assertForcedFailure := func(t *testing.T) {
			t.Helper()
			_, err = sw.LastWatcher()
			assert.ErrorIs(t, err, errForcedSourceFailure, "want forced source failure")
		}

		// 1st attempt is immediate.
		// 2nd attempt blocks (1m), then fails.
		synctest.Wait()
		assertForcedFailure(t)
		time.Sleep(1 * time.Minute)
		// 3rd attempt blocks (2m), then works.
		synctest.Wait()
		assertForcedFailure(t)
		time.Sleep(2 * time.Minute)
		waitForHealthy(t)

		// Close the watcher and simulate 3 failures.
		sw.SetAttemptsToFail(3)
		sw.MustWatcher(t).Close()
		// 1st is immediate.
		// 2nd, 3rd, 4th: exponential.
		backoff := 1 * time.Minute
		for range 3 {
			synctest.Wait()
			assertForcedFailure(t)
			time.Sleep(backoff)
			backoff *= 2
		}
		waitForHealthy(t)
	})
}

func newAuthPrefWatcher(
	t *testing.T,
	source watchers.WatcherSource,
) *watchers.Watcher {
	t.Helper()

	watcher, err := watchers.NewWatcher(watchers.WatcherConfig{
		Source:            source,
		EventsChannelSize: 1, // arbitrary
		Watch: &types.Watch{
			Name: "test-watcher",
			Kinds: []types.WatchKind{
				{Kind: types.KindClusterAuthPreference},
			},
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	doneC := make(chan struct{})

	// Start watcher.
	go func() {
		_ = watcher.Run(ctx)
		close(doneC)
	}()
	t.Cleanup(func() {
		cancel()
		<-doneC
	})

	return watcher
}

var errForcedSourceFailure = errors.New("forced source failure")

// sourceWrapper wraps a watchers.WatcherSource and allow manipulating and
// capturing the outcome of NewWatcher.
//
// Useful to test backoff and reconnections.
type sourceWrapper struct {
	source watchers.WatcherSource

	mu             sync.Mutex
	attemptsToFail int
	lastWatcher    types.Watcher
	lastErr        error
}

func (sw *sourceWrapper) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.attemptsToFail > 0 {
		sw.attemptsToFail--
		sw.lastErr = errForcedSourceFailure
		return nil, sw.lastErr
	}

	w, err := sw.source.NewWatcher(ctx, watch)

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

func (sw *sourceWrapper) SetAttemptsToFail(num int) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.attemptsToFail = num
}
