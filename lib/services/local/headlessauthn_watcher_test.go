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

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestHeadlessAuthenticationWatcher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))

	type testSuite struct {
		watcher       *local.HeadlessAuthenticationWatcher
		watcherClock  clockwork.FakeClock
		watcherCancel context.CancelFunc
		identity      *local.IdentityService
		buf           *backend.CircularBuffer
	}

	newSuite := func(t *testing.T) *testSuite {
		identity := newIdentityService(t, clockwork.NewFakeClock())

		// use a standalone buffer as a watcher service.
		buf := backend.NewCircularBuffer()
		buf.SetInit()

		watcherCtx, watcherCancel := context.WithCancel(ctx)
		t.Cleanup(func() {
			watcherCancel()
		})

		watcherClock := clockwork.NewFakeClock()
		w, err := local.NewHeadlessAuthenticationWatcher(watcherCtx, local.HeadlessAuthenticationWatcherConfig{
			Clock:          watcherClock,
			WatcherService: buf,
			Backend:        identity.Backend,
		})
		require.NoError(t, err)

		return &testSuite{
			watcher:       w,
			watcherClock:  watcherClock,
			watcherCancel: watcherCancel,
			identity:      identity,
			buf:           buf,
		}
	}

	waitInGoroutine := func(ctx context.Context, t *testing.T, watcher *local.HeadlessAuthenticationWatcher, name string, cond func(*types.HeadlessAuthentication) (bool, error)) (chan *types.HeadlessAuthentication, chan error) {
		headlessAuthnCh := make(chan *types.HeadlessAuthentication, 1)
		errC := make(chan error, 1)
		go func() {
			headlessAuthn, err := watcher.Wait(ctx, name, cond)
			errC <- err
			headlessAuthnCh <- headlessAuthn
		}()
		return headlessAuthnCh, errC
	}

	t.Run("WaitEventWithConditionMet", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*5)
		defer waitCancel()

		firstEmitReceived := make(chan struct{})
		headlessAuthnCh, errC := waitInGoroutine(waitCtx, t, s.watcher, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			select {
			case <-firstEmitReceived:
			default:
				close(firstEmitReceived)
			}

			return ha.User != "", nil
		})

		// Emit put event that doesn't pass the condition (user not set). The waiter should ignore these events.
		stub, err := types.NewHeadlessAuthenticationStub(pubUUID, time.Now())
		require.NoError(t, err)
		stub.User = "user"
		item, err := local.MarshalHeadlessAuthenticationToItem(stub)
		require.NoError(t, err)

		// The waiter may miss put events during initialization, so we continuously emit them until one is caught.
		require.Eventually(t, func() bool {
			s.buf.Emit(backend.Event{
				Type: types.OpPut,
				Item: *item,
			})
			select {
			case <-firstEmitReceived:
				return true
			default:
				return false
			}
		}, time.Second, time.Millisecond*100)

		require.NoError(t, <-errC)
		require.Equal(t, stub, <-headlessAuthnCh)
	})

	t.Run("WaitEventWithConditionUnmet", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*5)
		defer waitCancel()

		firstEmitReceived := make(chan struct{})
		headlessAuthnCh, errC := waitInGoroutine(waitCtx, t, s.watcher, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			select {
			case <-firstEmitReceived:
			default:
				close(firstEmitReceived)
			}

			return ha.User != "", nil
		})

		// Emit put event that doesn't pass the condition (user not set). The waiter should ignore these events.
		stub, err := types.NewHeadlessAuthenticationStub(pubUUID, time.Now())
		require.NoError(t, err)
		item, err := local.MarshalHeadlessAuthenticationToItem(stub)
		require.NoError(t, err)

		// The waiter may miss put events during initialization, so we continuously emit them until one is caught.
		require.Eventually(t, func() bool {
			s.buf.Emit(backend.Event{
				Type: types.OpPut,
				Item: *item,
			})
			select {
			case <-firstEmitReceived:
				return true
			default:
				return false
			}
		}, time.Second, time.Millisecond*100)

		// Ensure that the waiter did not finish with the condition unmet.
		select {
		case err := <-errC:
			t.Errorf("Expected waiter to continue but instead the waiter returned with err: %v", err)
		default:
			waitCancel()
		}

		require.Error(t, <-errC)
		require.Nil(t, <-headlessAuthnCh)
	})

	t.Run("WaitBackend", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t)

		stub, err := s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*5)
		defer waitCancel()

		// Wait should immediately check the backend and return the existing headless authentication stub.
		headlessAuthn, err := s.watcher.Wait(waitCtx, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			return true, nil
		})

		require.NoError(t, err)
		require.Equal(t, stub, headlessAuthn)
	})

	t.Run("WaitTimeout", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Millisecond*10)
		defer waitCancel()

		_, err := s.watcher.Wait(waitCtx, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) { return true, nil })
		require.Error(t, err)
		require.Equal(t, waitCtx.Err(), err)
	})

	t.Run("StaleCheck", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*5)
		defer waitCancel()

		// Create a waiter that we can block/unblock.
		firstEmitReceived := make(chan struct{})
		blockWaiter := make(chan struct{})
		_, blockedWaiterErrC := waitInGoroutine(waitCtx, t, s.watcher, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			select {
			case <-firstEmitReceived:
			default:
				close(firstEmitReceived)
				// we only block the first event received, incase additional events get to this waiter.
				<-blockWaiter
			}
			return false, nil
		})

		// Emit stub put event and wait for it to be caught by the waiter.
		stub, err := types.NewHeadlessAuthenticationStub(pubUUID, time.Now())
		require.NoError(t, err)
		item, err := local.MarshalHeadlessAuthenticationToItem(stub)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			s.buf.Emit(backend.Event{
				Type: types.OpPut,
				Item: *item,
			})
			select {
			case <-firstEmitReceived:
				return true
			default:
				return false
			}
		}, time.Second, time.Millisecond*100)

		// Create a second waiter to catch a second put event.
		_, freeWaiterErrC := waitInGoroutine(waitCtx, t, s.watcher, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			return true, nil
		})

		require.Eventually(t, func() bool {
			s.buf.Emit(backend.Event{
				Type: types.OpPut,
				Item: *item,
			})
			select {
			case <-freeWaiterErrC:
				return true
			default:
				return false
			}
		}, time.Second, time.Millisecond*100)

		// unblock the waiter. It should perform a stale check and return a not found error.
		close(blockWaiter)
		err = <-blockedWaiterErrC
		require.True(t, trace.IsNotFound(err), "Expected a not found error from Wait but got %v", err)
	})

	t.Run("WatchReset", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*5)
		defer waitCancel()

		firstEmitReceived := make(chan struct{})
		headlessAuthnCh, errC := waitInGoroutine(waitCtx, t, s.watcher, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			select {
			case <-firstEmitReceived:
			default:
				close(firstEmitReceived)
			}

			return services.ValidateHeadlessAuthentication(ha) == nil, nil
		})

		stub, err := s.identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)

		item, err := local.MarshalHeadlessAuthenticationToItem(stub)
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			s.buf.Emit(backend.Event{
				Type: types.OpPut,
				Item: *item,
			})
			select {
			case <-firstEmitReceived:
				return true
			default:
				return false
			}
		}, time.Second, time.Millisecond*100)

		// closed watchers should be handled gracefully and reset.
		s.buf.Clear()
		s.watcherClock.BlockUntil(1)

		// The watcher should notify waiters of backend state on watcher reset.
		replace := *stub
		replace.PublicKey = []byte(sshPubKey)
		replace.User = "user"
		swapped, err := s.identity.CompareAndSwapHeadlessAuthentication(ctx, stub, &replace)
		require.NoError(t, err)

		s.watcherClock.Advance(s.watcher.MaxRetryPeriod)
		require.NoError(t, <-errC)
		require.Equal(t, swapped, <-headlessAuthnCh)
	})

	t.Run("WatcherClosed", func(t *testing.T) {
		t.Parallel()
		s := newSuite(t)

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*5)
		defer waitCancel()

		_, errC := waitInGoroutine(waitCtx, t, s.watcher, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			return true, nil
		})

		s.watcherCancel()

		// waiters should be notified to close and result in ctx error
		waitErr := <-errC
		require.Error(t, waitErr)
		require.Equal(t, waitErr.Error(), "headless authentication watcher closed")

		// New waiters should be prevented.
		_, err := s.watcher.Wait(ctx, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) { return true, nil })
		require.Error(t, err)
	})
}
