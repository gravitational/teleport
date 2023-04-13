/*
Copyright 2023 Gravitational, Inc.

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

package local_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type headlessAuthenticationWatcherTestEnv struct {
	watcher       *local.HeadlessAuthenticationWatcher
	watcherClock  clockwork.FakeClock
	watcherCancel context.CancelFunc
	identity      *local.IdentityService
}

func newHeadlessAuthenticationWatcherTestEnv(t *testing.T) *headlessAuthenticationWatcherTestEnv {
	identity := newIdentityService(t, clockwork.NewFakeClock())

	watcherCtx, watcherCancel := context.WithCancel(context.Background())
	t.Cleanup(watcherCancel)

	watcherClock := clockwork.NewFakeClock()
	w, err := local.NewHeadlessAuthenticationWatcher(watcherCtx, local.HeadlessAuthenticationWatcherConfig{
		Clock:   watcherClock,
		Backend: identity.Backend,
	})
	require.NoError(t, err)

	return &headlessAuthenticationWatcherTestEnv{
		watcher:       w,
		watcherClock:  watcherClock,
		watcherCancel: watcherCancel,
		identity:      identity,
	}
}

func TestHeadlessAuthenticationWatcher_Subscribe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))

	t.Run("Updates", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// Make an update.
		_, err = s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		ticker := time.NewTicker(time.Second)
		t.Cleanup(ticker.Stop)

		for {
			select {
			case <-sub.Updates():
				// We should receive the update below.
				return
			case <-sub.Stale():
				// We may miss the update but get marked as stale, this is also fine.
				return
			case <-ticker.C:
				t.Error("Expected subscriber to receive an update")
				return
			}
		}
	})

	t.Run("Stale", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// Make an update without servicing the subscriber's Updates channel.
		_, err = s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		ticker := time.NewTicker(time.Second)
		t.Cleanup(ticker.Stop)

		select {
		case <-sub.Stale():
		case <-ticker.C:
			t.Fatal("Expected subscriber to be marked as stale")
		}
	})

	t.Run("WatchReset", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// Closed watchers should be handled gracefully and reset.
		s.identity.Backend.CloseWatchers()
		s.watcherClock.BlockUntil(1)

		stub, err := s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		ticker := time.NewTicker(time.Second)
		t.Cleanup(ticker.Stop)

		// Reset the watcher.
		s.watcherClock.Advance(s.watcher.MaxRetryPeriod)

		for {
			select {
			case update := <-sub.Updates():
				// We should receive an update of the current backend state on watcher reset.
				assert.Equal(t, stub, update)
				return
			case <-sub.Stale():
				// We may miss the update but get marked as stale, this is also fine.
				return
			case <-ticker.C:
				t.Error("Expected subscriber to receive an update")
				return
			}
		}
	})
}

func TestHeadlessAuthenticationWatcher_WaitForUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))

	t.Run("ConditionMet", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		t.Cleanup(cancel)

		headlessAuthnCh := make(chan *types.HeadlessAuthentication, 1)
		errC := make(chan error, 1)
		go func() {
			ha, err := s.watcher.WaitForUpdate(ctx, sub, func(ha *types.HeadlessAuthentication) (bool, error) {
				return ha.User != "", nil
			})
			headlessAuthnCh <- ha
			errC <- err
		}()

		// Make an update that passes the condition.
		stub, err := s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		replace := *stub
		replace.User = "user"
		replace.PublicKey = []byte(sshPubKey)
		s.identity.CompareAndSwapHeadlessAuthentication(ctx, stub, &replace)

		require.NoError(t, <-errC)
		require.Equal(t, &replace, <-headlessAuthnCh)
	})

	t.Run("ConditionUnmet", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		headlessAuthnCh := make(chan *types.HeadlessAuthentication, 1)
		errC := make(chan error, 1)
		go func() {
			ha, err := s.watcher.WaitForUpdate(ctx, sub, func(ha *types.HeadlessAuthentication) (bool, error) {
				return ha.User != "", nil
			})
			errC <- err
			headlessAuthnCh <- ha
		}()

		// Make an update that doesn't pass the condition (user not set). The waiter should ignore these events.
		_, err = s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		// Ensure that the waiter did not finish with the condition unmet.
		select {
		case err := <-errC:
			t.Fatalf("Expected waiter to continue but instead the waiter returned with err: %v", err)
		default:
			cancel()
		}

		require.Error(t, <-errC)
		require.Nil(t, <-headlessAuthnCh)
	})

	t.Run("InitialBackendCheck", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		stub, err := s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
		t.Cleanup(waitCancel)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// WaitForUpdate should immediately check the backend and return the existing headless authentication stub.
		headlessAuthn, err := s.watcher.WaitForUpdate(waitCtx, sub, func(ha *types.HeadlessAuthentication) (bool, error) {
			return true, nil
		})

		require.NoError(t, err)
		require.Equal(t, stub, headlessAuthn)
	})

	t.Run("Timeout", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Millisecond)
		t.Cleanup(waitCancel)

		_, err = s.watcher.WaitForUpdate(waitCtx, sub, func(ha *types.HeadlessAuthentication) (bool, error) { return true, nil })
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("WatcherClosed", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t)

		sub, err := s.watcher.Subscribe(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		errC := make(chan error)
		go func() {
			_, err := s.watcher.WaitForUpdate(ctx, sub, func(ha *types.HeadlessAuthentication) (bool, error) {
				return true, nil
			})
			errC <- err
		}()

		s.watcherCancel()

		// WaitForUpdate should end with closed watcher error.
		waitErr := <-errC
		require.Error(t, waitErr)
		require.ErrorIs(t, waitErr, local.HeadlessAuthenticationWatcherClosedErr)

		// New subscribers should be prevented.
		_, err = s.watcher.Subscribe(ctx, pubUUID)
		require.Error(t, err)
	})
}
