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
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestHeadlessAuthenticationWatcher(t *testing.T) {
	ctx := context.Background()

	t.Parallel()
	identity := newIdentityService(t, clockwork.NewFakeClock())

	watcherCtx, watcherCancel := context.WithCancel(ctx)
	defer watcherCancel()

	watcherClock := clockwork.NewFakeClock()
	w, err := local.NewHeadlessAuthenticationWatcher(watcherCtx, local.HeadlessAuthenticationWatcherConfig{
		Clock:   watcherClock,
		Backend: identity.Backend,
	})
	require.NoError(t, err)

	pubUUID := services.NewHeadlessAuthenticationID([]byte(sshPubKey))

	waitInGoroutine := func(ctx context.Context, t *testing.T, name string, cond func(*types.HeadlessAuthentication) (bool, error)) (chan *types.HeadlessAuthentication, chan error) {
		headlessAuthnCh := make(chan *types.HeadlessAuthentication, 1)
		errC := make(chan error, 1)
		go func() {
			headlessAuthn, err := w.Wait(ctx, name, cond)
			errC <- err
			headlessAuthnCh <- headlessAuthn
		}()
		require.Eventually(t, func() bool { return w.CheckWaiter(name) }, time.Millisecond*100, time.Millisecond*10)

		return headlessAuthnCh, errC
	}

	t.Run("WaitTimeout", func(t *testing.T) {
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Millisecond*10)
		defer waitCancel()

		_, err = w.Wait(waitCtx, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) { return true, nil })
		require.Error(t, err)
		require.Equal(t, waitCtx.Err(), err)
	})

	t.Run("WaitCreateStub", func(t *testing.T) {
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*2)
		defer waitCancel()

		headlessAuthnCh, errC := waitInGoroutine(waitCtx, t, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			return true, nil
		})

		stub, err := identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, identity.DeleteHeadlessAuthentication(ctx, pubUUID)) })

		require.NoError(t, <-errC)
		require.Equal(t, stub, <-headlessAuthnCh)
	})

	t.Run("WaitCompareAndSwap", func(t *testing.T) {
		stub, err := identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, identity.DeleteHeadlessAuthentication(ctx, pubUUID)) })

		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*2)
		defer waitCancel()

		headlessAuthnCh, errC := waitInGoroutine(waitCtx, t, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			return ha.State == types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED, nil
		})

		replace := *stub
		replace.State = types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED
		replace.PublicKey = []byte(sshPubKey)
		replace.User = "user"

		swapped, err := identity.CompareAndSwapHeadlessAuthentication(ctx, stub, &replace)
		require.NoError(t, err)

		require.NoError(t, <-errC)
		require.Equal(t, swapped, <-headlessAuthnCh)
	})

	t.Run("StaleCheck", func(t *testing.T) {
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*2)
		defer waitCancel()

		// Create a waiter that we can block/unblock.
		notifyReceived := make(chan struct{}, 1)
		blockWaiter := make(chan struct{})
		_, errC := waitInGoroutine(waitCtx, t, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			notifyReceived <- struct{}{}
			<-blockWaiter
			return false, nil
		})

		// Create stub and wait for it to be caught by the waiter.
		stub, err := identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)
		<-notifyReceived

		// perform a put to mark the blocked waiter as stale and
		replace := *stub
		replace.PublicKey = []byte(sshPubKey)
		replace.User = "user"
		_, err = identity.CompareAndSwapHeadlessAuthentication(ctx, stub, &replace)
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			ok, err := w.CheckWaiterStale(pubUUID)
			require.NoError(t, err)
			return ok
		}, time.Second, time.Millisecond, "Expected waiter to be marked as stale")

		// delete the headless authentication.
		err = identity.DeleteHeadlessAuthentication(ctx, pubUUID)
		require.NoError(t, err)

		// unblock the waiter. It should perform a stale check and return a not found error.
		close(blockWaiter)
		err = <-errC
		require.True(t, trace.IsNotFound(err), "Expected a not found error from Wait but got %v", err)
	})

	t.Run("WatchReset", func(t *testing.T) {
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*2)
		defer waitCancel()

		headlessAuthnCh, errC := waitInGoroutine(waitCtx, t, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			return true, nil
		})

		// closed watchers should be handled gracefully and reset.
		identity.Backend.CloseWatchers()
		watcherClock.BlockUntil(1)

		// The watcher should notify waiters of missed events.
		stub, err := identity.CreateHeadlessAuthenticationStub(ctx, pubUUID)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, identity.DeleteHeadlessAuthentication(ctx, pubUUID)) })

		watcherClock.Advance(w.MaxRetryPeriod)
		require.NoError(t, <-errC)
		require.Equal(t, stub, <-headlessAuthnCh)
	})

	t.Run("WatcherClosed", func(t *testing.T) {
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Second*2)
		defer waitCancel()

		_, errC := waitInGoroutine(waitCtx, t, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) {
			return true, nil
		})

		watcherCancel()

		// waiters should be notified to close and result in ctx error
		waitErr := <-errC
		require.Error(t, waitErr)
		require.Equal(t, waitErr.Error(), "headless authentication watcher closed")

		// New waiters should be prevented.
		_, err = w.Wait(ctx, pubUUID, func(ha *types.HeadlessAuthentication) (bool, error) { return true, nil })
		require.Error(t, err)
	})
}
