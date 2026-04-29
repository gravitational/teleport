package plugins

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta"
	"github.com/gravitational/teleport/e/lib/okta/leader"
	"github.com/gravitational/teleport/e/lib/plugins/factory"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
)

func newOktaPlugin(orgURL string) *types.PluginV1 {
	return &types.PluginV1{
		Metadata: types.Metadata{Name: "test-okta"},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: &types.PluginOktaSettings{
					OrgUrl:       orgURL,
					SyncSettings: &types.PluginOktaSyncSettings{},
				},
			},
		},
	}
}

// newTestPresence creates a memory-backed presence service for use in synctest.
func newTestPresence(t *testing.T) *local.PresenceService {
	t.Helper()
	clock := clockwork.NewRealClock()
	backend, err := memory.New(memory.Config{Clock: clock})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, backend.Close()) })

	presence := local.NewPresenceService(backend)
	t.Cleanup(func() { require.NoError(t, presence.Close()) })
	return presence
}

// runOktaLock starts executeWithOktaLock in a goroutine with the given
// presence service and plugin delegate. It returns a done channel that is
// closed when executeWithOktaLock returns.
func runOktaLock(ctx context.Context, presence *local.PresenceService, pluginFunc factory.Delegate) chan error {
	plugin := newOktaPlugin("https://test.okta.example.com")
	delegate := executeWithOktaLock(presence, "test-holder", plugin, pluginFunc, slog.Default())
	done := make(chan error, 1)
	go func() {
		done <- delegate(ctx)
	}()
	return done
}

func TestOktaLock(t *testing.T) {
	t.Run("AcquiresAndRuns", func(t *testing.T) {
		// The wrapper should acquire the semaphore and start the plugin.
		synctest.Test(t, func(t *testing.T) {
			presence := newTestPresence(t)

			var started atomic.Bool
			pluginFunc := func(ctx context.Context) error {
				started.Store(true)
				<-ctx.Done()
				return nil
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			runOktaLock(ctx, presence, pluginFunc)

			synctest.Wait()
			require.True(t, started.Load(), "plugin should have started")

			// WARNING If you need to align this the oktaSemaphoreName has been modified and this is not
			// backward compatible change.
			const expectedSemaphoreName = "aHR0cHM6Ly90ZXN0Lm9rdGEuZXhhbXBsZS5jb20"
			semaphores, err := presence.GetSemaphores(t.Context(), types.SemaphoreFilter{
				SemaphoreKind: okta.OktaServiceSemaphoreKind,
				SemaphoreName: expectedSemaphoreName,
			})
			require.NoError(t, err)
			require.Len(t, semaphores, 1, "executeWithOktaLock should hold a semaphore named %q", expectedSemaphoreName)

			cancel()
			synctest.Wait()
		})
	})

	t.Run("StopsOnContextCancel", func(t *testing.T) {
		// Canceling the parent context should stop the plugin and return nil.
		synctest.Test(t, func(t *testing.T) {
			presence := newTestPresence(t)

			var exited atomic.Bool
			pluginFunc := func(ctx context.Context) error {
				<-ctx.Done()
				exited.Store(true)
				return nil
			}

			ctx, cancel := context.WithCancel(t.Context())
			done := runOktaLock(ctx, presence, pluginFunc)

			synctest.Wait()

			cancel()
			synctest.Wait()

			require.True(t, exited.Load(), "plugin should have exited")

			err := <-done
			require.NoError(t, err, "executeWithOktaLock should return nil on context cancel")
		})
	})

	t.Run("ReleasesSemaphore", func(t *testing.T) {
		// After the wrapper stops, the semaphore should be available for
		// others to acquire.
		synctest.Test(t, func(t *testing.T) {
			presence := newTestPresence(t)

			pluginFunc := func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			}

			ctx, cancel := context.WithCancel(t.Context())
			done := runOktaLock(ctx, presence, pluginFunc)

			synctest.Wait()

			cancel()
			synctest.Wait()
			<-done

			// The semaphore should be free. Verify by acquiring it.
			semaphoreName := oktaSemaphoreName("https://test.okta.example.com")
			lease, err := presence.AcquireSemaphore(t.Context(), types.AcquireSemaphoreRequest{
				SemaphoreKind: okta.OktaServiceSemaphoreKind,
				SemaphoreName: semaphoreName,
				MaxLeases:     1,
				Expires:       time.Now().Add(time.Minute),
				Holder:        "verify-released",
			})
			require.NoError(t, err, "semaphore should be available after wrapper stops")
			require.NoError(t, presence.CancelSemaphoreLease(t.Context(), *lease))
		})
	})

	t.Run("RestartsAfterError", func(t *testing.T) {
		// If the plugin returns an error, the wrapper should wait for
		// oktaRetryInterval then restart it.
		synctest.Test(t, func(t *testing.T) {
			presence := newTestPresence(t)

			var startCount atomic.Int32
			pluginFunc := func(ctx context.Context) error {
				n := startCount.Add(1)
				if n == 1 {
					return errors.New("transient error")
				}
				<-ctx.Done()
				return nil
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			runOktaLock(ctx, presence, pluginFunc)

			synctest.Wait()
			require.Equal(t, int32(1), startCount.Load(), "plugin should have started once")

			// Advance past the retry interval so the wrapper restarts the plugin.
			time.Sleep(oktaRetryInterval)
			synctest.Wait()

			require.Equal(t, int32(2), startCount.Load(), "plugin should have been restarted after error")

			cancel()
			synctest.Wait()
		})
	})

	t.Run("OnlyOneInstance", func(t *testing.T) {
		// A second wrapper targeting the same org URL should not start its
		// plugin while the first one holds the semaphore.
		synctest.Test(t, func(t *testing.T) {
			presence := newTestPresence(t)

			var startCount atomic.Int32
			makePlugin := func() factory.Delegate {
				return func(ctx context.Context) error {
					startCount.Add(1)
					<-ctx.Done()
					return nil
				}
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			// Start the first instance and let it acquire the semaphore.
			plugin := newOktaPlugin("https://test.okta.example.com")
			first := executeWithOktaLock(presence, "holder-1", plugin, makePlugin(), slog.Default())
			go func() { _ = first(ctx) }()

			// Let the retry loop complete so the first instance acquires the lock.
			time.Sleep(oktaRetryInterval)
			synctest.Wait()
			require.Equal(t, int32(1), startCount.Load(), "first plugin should have started")

			// Start a second instance targeting the same semaphore.
			second := executeWithOktaLock(presence, "holder-2", plugin, makePlugin(), slog.Default())
			go func() { _ = second(ctx) }()

			// Advance past several retry intervals — second should stay blocked.
			time.Sleep(3 * oktaRetryInterval)
			synctest.Wait()
			require.Equal(t, int32(1), startCount.Load(), "only one plugin instance should be running")

			cancel()
			synctest.Wait()
		})
	})
}

// TestOktaLockBackwardCompatibility verifies that executeWithOktaLock
// uses the same semaphore parameters as the old leader.Leader package,
// ensuring backward compatibility during rolling upgrades. Both code
// paths target the same semaphore (kind + name), so holding the lock
// with one prevents the other from acquiring it.
func TestOktaLockBackwardCompatibility(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		clock := clockwork.NewRealClock()
		backend, err := memory.New(memory.Config{Clock: clock})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, backend.Close()) })

		presence := local.NewPresenceService(backend)
		t.Cleanup(func() { require.NoError(t, presence.Close()) })

		orgURL := "https://test.okta.example.com"
		semaphoreName := oktaSemaphoreName(orgURL)

		// Acquire leadership using the old leader.Leader package.
		oldLeader, err := leader.New(leader.Config{
			SemaphoreKind: okta.OktaServiceSemaphoreKind,
			SemaphoreName: semaphoreName,
			HostIDHolder:  "old-instance",
			Clock:         clock,
			Semaphores:    presence,
		})
		require.NoError(t, err)

		oldLeader.Start(t.Context())
		synctest.Wait()

		require.True(t, oldLeader.IsLeader(), "old leader should have acquired the semaphore")

		// Start executeWithOktaLock targeting the same org URL. It should
		// block because the old leader holds the semaphore.
		var pluginStarted atomic.Bool
		pluginFunc := func(ctx context.Context) error {
			pluginStarted.Store(true)
			<-ctx.Done()
			return nil
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		plugin := newOktaPlugin(orgURL)
		delegate := executeWithOktaLock(presence, "new-instance", plugin, pluginFunc, slog.Default())
		go func() { _ = delegate(ctx) }()

		// Advance past several retry intervals — the plugin should still
		// not start because the old leader holds the semaphore.
		time.Sleep(3 * oktaRetryInterval)
		synctest.Wait()
		require.False(t, pluginStarted.Load(), "plugin should not start while old leader holds the semaphore")

		// Stop the old leader and cancel its lease.
		require.NoError(t, oldLeader.Close())
		synctest.Wait()

		semaphores, err := presence.GetSemaphores(t.Context(), types.SemaphoreFilter{
			SemaphoreKind: okta.OktaServiceSemaphoreKind,
			SemaphoreName: semaphoreName,
		})
		require.NoError(t, err)
		require.Len(t, semaphores, 1)
		for _, ref := range semaphores[0].LeaseRefs() {
			_ = presence.CancelSemaphoreLease(t.Context(), types.SemaphoreLease{
				SemaphoreKind: okta.OktaServiceSemaphoreKind,
				SemaphoreName: semaphoreName,
				LeaseID:       ref.LeaseID,
				Expires:       ref.Expires,
			})
		}

		// Advance time so the retry fires and the plugin acquires the semaphore.
		time.Sleep(oktaRetryInterval)
		synctest.Wait()

		require.True(t, pluginStarted.Load(), "plugin should start after old leader released the semaphore")

		cancel()
		synctest.Wait()
	})
}
