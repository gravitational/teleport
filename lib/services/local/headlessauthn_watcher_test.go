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
		headlessAuthn, err := types.NewHeadlessAuthenticationStub(pubUUID, s.watcher.Clock.Now().Add(time.Minute))
		require.NoError(t, err)
		headlessAuthn.User = username
		return headlessAuthn
	}

	for _, tc := range []struct {
		name          string
		subscribeName string
	}{
		{
			name:          "id",
			subscribeName: pubUUID,
		}, {
			name:          "username",
			subscribeName: username,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Updates", func(t *testing.T) {
				t.Parallel()
				s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
				headlessAuthn := newHeadlessAuthn(t, s)

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
				require.NoError(t, err)
				t.Cleanup(sub.Close)

				// Make an update. Make sure we are servicing the updates channel first.
				readyForUpdate := make(chan struct{})
				go func() {
					<-readyForUpdate
					err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
					assert.NoError(t, err)
				}()

				for {
					select {
					case update := <-sub.Updates():
						// We should receive the update.
						require.Equal(t, headlessAuthn, update)
						return
					case <-sub.Stale():
						t.Fatal("Expected subscriber to not be marked as stale")
					case <-time.After(time.Second):
						t.Fatal("Expected subscriber to receive an update")
					case readyForUpdate <- struct{}{}:
					}
				}
			})

			t.Run("Stale", func(t *testing.T) {
				t.Parallel()
				s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
				headlessAuthn := newHeadlessAuthn(t, s)

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
				require.NoError(t, err)
				t.Cleanup(sub.Close)

				// Make 2 updates without servicing the subscriber's Updates channel.
				// The second update will be dropped and result in the subscriber being
				// marked as stale.
				err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
				require.NoError(t, err)
				err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
				require.NoError(t, err)

				select {
				case <-sub.Stale():
				case <-time.After(time.Second):
					t.Fatal("Expected subscriber to be marked as stale")
				}
			})

			t.Run("WatchReset", func(t *testing.T) {
				t.Parallel()
				clock := clockwork.NewFakeClock()
				s := newHeadlessAuthenticationWatcherTestEnv(t, clock)
				headlessAuthn := newHeadlessAuthn(t, s)

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
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
				case <-sub.Stale():
					t.Fatal("Expected subscriber to not be marked as stale")
				case <-time.After(time.Second):
					t.Fatal("Expected subscriber to receive an update")
				}
			})
		})
	}
}

func TestHeadlessAuthenticationWatcher_WaitForUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))
	username := "username"

	newHeadlessAuthn := func(t *testing.T, s *headlessAuthenticationWatcherTestEnv) *types.HeadlessAuthentication {
		headlessAuthn, err := types.NewHeadlessAuthenticationStub(pubUUID, s.watcher.Clock.Now().Add(time.Minute))
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

	for _, tc := range []struct {
		name          string
		subscribeName string
	}{
		{
			name:          "id",
			subscribeName: pubUUID,
		}, {
			name:          "username",
			subscribeName: username,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("ConditionMet", func(t *testing.T) {
				t.Parallel()
				s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())
				headlessAuthn := newHeadlessAuthn(t, s)

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
				require.NoError(t, err)
				t.Cleanup(sub.Close)

				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				t.Cleanup(cancel)

				headlessAuthnCh := make(chan *types.HeadlessAuthentication, 1)
				errC := make(chan error, 1)
				go func() {
					ha, err := s.watcher.WaitForUpdate(ctx, sub, conditionFunc)
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

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
				require.NoError(t, err)
				t.Cleanup(sub.Close)

				// Make an update that doesn't pass the condition (pending).
				// The waiter should ignore this update and timeout.
				err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
				require.NoError(t, err)

				ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
				t.Cleanup(cancel)

				_, err = s.watcher.WaitForUpdate(ctx, sub, conditionFunc)
				require.Error(t, err)
				require.ErrorIs(t, err, context.DeadlineExceeded)

				// Make an update that causes the condition to error (denied).
				// The waiter should return the condition error during the initial backend check.
				headlessAuthn.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED
				err = s.identity.UpsertHeadlessAuthentication(ctx, headlessAuthn)
				require.NoError(t, err)

				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				t.Cleanup(cancel)

				_, err = s.watcher.WaitForUpdate(ctx, sub, conditionFunc)
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

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
				require.NoError(t, err)
				t.Cleanup(sub.Close)

				// WaitForUpdate should immediately check the backend and return the existing headless authentication.
				headlessAuthnUpdate, err := s.watcher.WaitForUpdate(waitCtx, sub, conditionFunc)

				require.NoError(t, err)
				require.Equal(t, headlessAuthn, headlessAuthnUpdate)
			})

			t.Run("Timeout", func(t *testing.T) {
				t.Parallel()
				s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
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
				s := newHeadlessAuthenticationWatcherTestEnv(t, clockwork.NewFakeClock())

				sub, err := s.watcher.Subscribe(ctx, tc.subscribeName)
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
				require.ErrorIs(t, waitErr, local.ErrHeadlessAuthenticationWatcherClosed)

				// New subscribers should be prevented.
				_, err = s.watcher.Subscribe(ctx, tc.subscribeName)
				require.Error(t, err)
			})
		})
	}
}
