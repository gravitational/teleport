package internal

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/stretchr/testify/require"
)

func Test_RetryTunnelInitialization(t *testing.T) {
	t.Parallel()

	var initCalled bool
	init := func(_ context.Context) error {
		initCalled = true
		return nil
	}

	registry := readyz.NewRegistry()
	reporter := registry.AddService("application-tunnel", "test")

	err := RetryTunnelInitialization(t.Context(), logtest.NewLogger(), reporter, init)
	require.NoError(t, err)
	require.True(t, initCalled)

	status, ok := registry.ServiceStatus("test")
	require.True(t, ok)
	require.Equal(t, readyz.Initializing, status.Status)
}

func Test_RetryTunnelInitialization_Retries(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		registry := readyz.NewRegistry()
		reporter := registry.AddService("application-tunnel", "test")

		var initCallCount atomic.Int32
		var initReturnNil atomic.Bool

		init := func(_ context.Context) error {
			initCallCount.Add(1)
			if initReturnNil.Load() {
				return nil
			}

			return fmt.Errorf("init function call is failing (%d)", initCallCount.Load())
		}

		result := make(chan error, 1)

		retryCtx, cancelCtx := context.WithCancel(t.Context())
		defer cancelCtx()

		deterministicJitter := func(d time.Duration) time.Duration { return d / 2 }
		go func() {
			result <- retryTunnelInitialization(
				retryCtx,
				logtest.NewLogger(),
				reporter,
				deterministicJitter,
				init)
		}()

		synctest.Wait()

		status, ok := registry.ServiceStatus("test")
		require.True(t, ok)
		require.Equal(t, readyz.Unhealthy, status.Status)
		require.Equal(t, "init function call is failing (1)", status.Reason)

		calls := initCallCount.Load()
		require.Equal(t, int32(1), calls)

		backoffs := []time.Duration{
			500 * time.Millisecond, // attempt 2
			1 * time.Second,        // attempt 3
			2 * time.Second,        // attempt 4
			4 * time.Second,        // attempt 5
			// max is set to 10s, but jitter halves it = 5
			5 * time.Second, // attempt 6
			5 * time.Second, // attempt 7
		}

		// Attempt 1 runs at t=0.
		synctest.Wait()
		require.Equal(t, int32(1), initCallCount.Load())

		for i, d := range backoffs {
			before, after := int32(i+1), int32(i+2)

			// Just before the backoff value: no new attempt yet.
			time.Sleep(d - time.Nanosecond)
			synctest.Wait()
			require.Equal(t, before, initCallCount.Load(),
				"retry #%d triggered before the %s backoff elapsed", i+1, d)

			// At the backoff value: exactly one new attempt.
			time.Sleep(time.Nanosecond)
			synctest.Wait()
			require.Equal(t, after, initCallCount.Load(),
				"retry #%d did not fire after %s", i+1, d)
		}

		select {
		case <-result:
			require.Fail(t, "RetryTunnelInitialization returned when it shouldn't yet")
		default:
		}

		initReturnNil.Store(true)

		select {
		case err := <-result:
			require.NoError(t, err)

		// The "1 minute" is just to ensure the check happens after all the other timed
		// events inside synctest.
		case <-time.After(1 * time.Minute):
			require.Fail(t, "timed out waiting for RetryTunnelInitialization to return")
		}

		// There should have been 7 failed attempts, plus one successful.
		require.Equal(t, int32(8), initCallCount.Load())

		// The RetryTunnelInitialization intentionally never sets the status to Healthy.
		status, ok = registry.ServiceStatus("test")
		require.True(t, ok)
		require.Equal(t, readyz.Unhealthy, status.Status)
		require.Equal(t, "init function call is failing (7)", status.Reason)
	})
}

func Test_RetryTunnelInitialization_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("starting with a canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		registry := readyz.NewRegistry()
		reporter := registry.AddService("application-tunnel", "test")

		err := RetryTunnelInitialization(
			ctx,
			logtest.NewLogger(),
			reporter,
			func(ctx context.Context) error { return nil })

		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)

		// It should not report any status change when the context is canceled.
		status, ok := registry.ServiceStatus("test")
		require.True(t, ok)
		require.Equal(t, readyz.Initializing, status.Status)
	})

	t.Run("mid-init cancellation", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			registry := readyz.NewRegistry()
			reporter := registry.AddService("application-tunnel", "test")

			init := func(ctx context.Context) error {
				<-ctx.Done()
				return errors.New("failed mid-init")
			}

			result := make(chan error, 1)

			go func() {
				result <- RetryTunnelInitialization(
					ctx,
					logtest.NewLogger(),
					reporter,
					init)
			}()

			synctest.Wait()
			cancel()
			synctest.Wait()

			select {
			case err := <-result:
				require.ErrorIs(t, err, context.Canceled)
				require.NotContains(t, err.Error(), "failed mid-init")
			default:
				require.Fail(t, "expected result but there was none")
			}

			// It should not report any status change when the context is canceled.
			status, ok := registry.ServiceStatus("test")
			require.True(t, ok)
			require.Equal(t, readyz.Initializing, status.Status)
		})
	})

	t.Run("context gets canceled during retries", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			init := func(_ context.Context) error {
				return errors.New("init function call is failing")
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			registry := readyz.NewRegistry()
			reporter := registry.AddService("application-tunnel", "test")

			result := make(chan error, 1)

			go func() {
				result <- RetryTunnelInitialization(
					ctx,
					logtest.NewLogger(),
					reporter,
					init)
			}()

			synctest.Wait()

			// Status change is being caused by the failing init function.
			status, ok := registry.ServiceStatus("test")
			require.True(t, ok)
			require.Equal(t, readyz.Unhealthy, status.Status)

			select {
			case <-result:
				require.Fail(t, "RetryTunnelInitialization returned when it shouldn't yet")
			default:
			}

			cancel()

			select {
			case err := <-result:
				require.Error(t, err)
				require.ErrorIs(t, err, context.Canceled)

			case <-time.After(1 * time.Minute):
				require.Fail(t, "timed out waiting for RetryTunnelInitialization to return")
			}
		})
	})
}
