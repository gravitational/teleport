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

package local_test

import (
	"context"
	"errors"
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
	watcherCancel context.CancelFunc
	identity      *local.IdentityService
}

func newHeadlessAuthenticationWatcherTestEnv(t *testing.T, clock clockwork.Clock) *headlessAuthenticationWatcherTestEnv {
	identity := newIdentityService(t, clock)

	watcherCtx, watcherCancel := context.WithCancel(context.Background())
	t.Cleanup(watcherCancel)

	w, err := local.NewHeadlessAuthenticationWatcher(watcherCtx, local.HeadlessAuthenticationWatcherConfig{
		Clock:   clock,
		Backend: identity.Backend,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, w.WaitInit(ctx))

	return &headlessAuthenticationWatcherTestEnv{
		watcher:       w,
		watcherCancel: watcherCancel,
		identity:      identity,
	}
}

func TestHeadlessAuthenticationWatcher_Subscribe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))
	username := "username"

	newHeadlessAuthn := func(t *testing.T, s *headlessAuthenticationWatcherTestEnv) *types.HeadlessAuthentication {
		headlessAuthn, err := types.NewHeadlessAuthentication(username, pubUUID, s.watcher.Clock.Now().Add(time.Minute))
		require.NoError(t, err)
		headlessAuthn.User = username
		return headlessAuthn
	}

	t.Run("Updates", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
		headlessAuthn := newHeadlessAuthn(t, s)

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
		assert.NoError(t, err)

		for {
			select {
			case update := <-sub.Updates():
				// We should receive the update.
				require.Equal(t, headlessAuthn, update)
				return
			case <-time.After(time.Second):
				t.Fatal("Expected subscriber to receive an update")
			}
		}
	})

	t.Run("Stale", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
		headlessAuthn := newHeadlessAuthn(t, s)

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// Create a second subscriber to wait for the update below. Since this
		// subscriber was created second, it should receive the update second.
		drain, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(drain.Close)

		// Create 2 updates without servicing the subscriber's Updates channel.
		// The backend update should be dropped in favor of the second update.
		err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
		require.NoError(t, err)

		replace := *headlessAuthn
		replace.SshPublicKey = []byte(sshPubKey)
		replace.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
		swapped, err := s.identity.CompareAndSwapHeadlessAuthentication(ctx, headlessAuthn, &replace)
		require.NoError(t, err)

		// Drain updates and wait for the second update. Depending on the speed
		// of the watcher above, we may see one or both updates.
	DRAIN:
		for {
			select {
			case update := <-drain.Updates():
				if assert.ObjectsAreEqual(swapped, update) {
					break DRAIN
				}
			case <-time.After(time.Second):
				t.Fatal("Expected subscriber to receive an update")
			}
		}

		select {
		case update := <-sub.Updates():
			// The first update should be replaced by the second.
			require.Equal(t, swapped, update)
			return
		default:
			t.Fatal("Expected subscriber to receive an update")
		}
	})

	t.Run("WatchReset", func(t *testing.T) {
		t.Parallel()
		clock := clockwork.NewFakeClock()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clock)
		headlessAuthn := newHeadlessAuthn(t, s)

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// Closed watchers should be handled gracefully and reset.
		s.identity.Backend.CloseWatchers()
		clock.BlockUntil(1)

		err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
		require.NoError(t, err)

		// Reset the watcher. Make sure we are servicing the updates channel first.
		readyForUpdate := make(chan struct{})
		go func() {
			<-readyForUpdate
			clock.Advance(s.watcher.MaxRetryPeriod)
		}()

		readyForUpdate <- struct{}{}
		select {
		case update := <-sub.Updates():
			// We should receive an update of the current backend state on watcher reset.
			require.Equal(t, headlessAuthn, update)
			return
		case <-time.After(time.Second):
			t.Fatal("Expected subscriber to receive an update")
		}
	})
}

func TestHeadlessAuthenticationWatcher_WaitForUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))
	username := "username"

	newHeadlessAuthn := func(t *testing.T, s *headlessAuthenticationWatcherTestEnv) *types.HeadlessAuthentication {
		headlessAuthn, err := types.NewHeadlessAuthentication(username, pubUUID, s.watcher.Clock.Now().Add(time.Minute))
		require.NoError(t, err)
		headlessAuthn.User = username
		return headlessAuthn
	}

	deniedErr := errors.New("headless authentication denied")
	conditionFunc := func(ha *types.HeadlessAuthentication) (bool, error) {
		switch ha.State {
		case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_PENDING:
			return false, nil
		case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED:
			return true, nil
		case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED:
			return false, deniedErr
		}
		return false, nil
	}

	t.Run("ConditionMet", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
		headlessAuthn := newHeadlessAuthn(t, s)

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		t.Cleanup(cancel)

		headlessAuthnCh := make(chan *types.HeadlessAuthentication, 1)
		errC := make(chan error, 1)
		go func() {
			ha, err := sub.WaitForUpdate(ctx, conditionFunc)
			headlessAuthnCh <- ha
			errC <- err
		}()

		// Make an update that passes the condition.
		headlessAuthn.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
		err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
		require.NoError(t, err)

		require.NoError(t, <-errC)
		require.Equal(t, headlessAuthn, <-headlessAuthnCh)
	})

	t.Run("ConditionUnmet", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
		headlessAuthn := newHeadlessAuthn(t, s)

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// Make an update that doesn't pass the condition (pending).
		// The waiter should ignore this update and timeout.
		err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
		require.NoError(t, err)

		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		t.Cleanup(cancel)

		_, err = sub.WaitForUpdate(timeoutCtx, conditionFunc)
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)

		// Make an update that causes the condition to error (denied).
		// The waiter should return the condition error during the initial backend check.
		headlessAuthn.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED
		err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
		require.NoError(t, err)

		waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		t.Cleanup(cancel)

		_, err = sub.WaitForUpdate(waitCtx, conditionFunc)
		require.Error(t, err)
		require.ErrorIs(t, err, deniedErr)
	})

	t.Run("InitialBackendCheck", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
		headlessAuthn := newHeadlessAuthn(t, s)

		// Create a headless authentication that passes the condition before starting WaitForUpdate.
		headlessAuthn.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
		err := s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
		require.NoError(t, err)

		waitCtx, waitCancel := context.WithTimeout(ctx, 5*time.Second)
		t.Cleanup(waitCancel)

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		// WaitForUpdate should immediately check the backend and return the existing headless authentication.
		headlessAuthnUpdate, err := sub.WaitForUpdate(waitCtx, conditionFunc)

		require.NoError(t, err)
		require.Equal(t, headlessAuthn, headlessAuthnUpdate)
	})

	t.Run("Timeout", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Millisecond)
		t.Cleanup(waitCancel)

		_, err = sub.WaitForUpdate(waitCtx, func(ha *types.HeadlessAuthentication) (bool, error) { return true, nil })
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("WatcherClosed", func(t *testing.T) {
		t.Parallel()
		s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())

		sub, err := s.watcher.Subscribe(ctx, username, pubUUID)
		require.NoError(t, err)
		t.Cleanup(sub.Close)

		errC := make(chan error)
		go func() {
			_, err := sub.WaitForUpdate(ctx, func(ha *types.HeadlessAuthentication) (bool, error) {
				return true, nil
			})
			errC <- err
		}()

		s.watcherCancel()

		// WaitForUpdate should end with closed watcher error.
		waitErr := <-errC
		require.Error(t, waitErr)
		require.ErrorIs(t, waitErr, local.ErrHeadlessAuthenticationWatcherClosed)

		// New subscribers should be prevented.
		_, err = s.watcher.Subscribe(ctx, username, pubUUID)
		require.Error(t, err)
	})
}
