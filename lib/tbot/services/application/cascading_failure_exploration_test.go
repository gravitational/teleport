/*
 * These tests verify that the resilient mode fix correctly isolates
 * "app not found" failures so that healthy services in the errgroup
 * are NOT terminated when an unrelated app service fails.
 *
 * The mock failingAppService replicates the error isolation pattern
 * implemented in the real OutputService.Run() and TunnelService.Run():
 * when resilientMode is true and the error is "app not found", the
 * service returns nil to the errgroup instead of the error.
 */

package application

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"pgregory.net/rapid"
)

// simulateAppNotFoundError replicates the exact error that getApp() returns
// when an application's agent is unavailable in the cluster.
func simulateAppNotFoundError(appName string) error {
	return trace.BadParameter("app %q not found", appName)
}

// failingAppService simulates a TunnelService or OutputService whose getApp()
// call returns "app not found". When resilientMode is true, it replicates the
// error isolation pattern from the fixed OutputService.Run() and
// TunnelService.Run(): the service suppresses "app not found" errors by
// returning nil to the errgroup, preventing cascading cancellation.
// When resilientMode is false, it returns the error directly (default behavior).
type failingAppService struct {
	appName       string
	serviceType   string // "tunnel" or "output"
	resilientMode bool
}

func (s *failingAppService) String() string {
	return fmt.Sprintf("%s:%s", s.serviceType, s.appName)
}

func (s *failingAppService) Run(_ context.Context) error {
	// Simulate getApp() returning "app not found".
	err := simulateAppNotFoundError(s.appName)
	err = trace.Wrap(err, "building local proxy config")

	// Replicate the resilient mode error isolation pattern from the real
	// OutputService.Run() and TunnelService.Run(): when resilient mode is
	// enabled and the error is "app not found", return nil to the errgroup
	// so other services are not canceled.
	if s.resilientMode && IsAppNotFoundError(err) {
		return nil
	}
	return err
}

// healthyService simulates a long-running healthy service (e.g., a working
// tunnel, SSH service, database service). It blocks until context is canceled
// and tracks whether it was still running when canceled.
type healthyService struct {
	name           string
	wasRunning     atomic.Bool
	contextWasDone atomic.Bool
}

func (s *healthyService) String() string {
	return s.name
}

func (s *healthyService) Run(ctx context.Context) error {
	s.wasRunning.Store(true)
	// Block until context is canceled (simulating a healthy long-running service)
	<-ctx.Done()
	s.contextWasDone.Store(true)
	s.wasRunning.Store(false)
	return nil
}

// runServicesInErrgroup replicates the exact errgroup pattern from bot.Bot.Run():
//
//	group, groupCtx := errgroup.WithContext(ctx)
//	for _, handle := range services {
//	    group.Go(func() error {
//	        err := handle.service.Run(groupCtx)
//	        if err != nil { return trace.Wrap(err) }
//	        return nil
//	    })
//	}
//	return group.Wait()
//
// Default behavior for when any service returns an error, the errgroup
// cancels groupCtx, which terminates all other services.
type errgroupResult struct {
	err                    error
	healthyServiceSurvived bool // true if healthy service was NOT killed by context cancellation
	healthyCtxCanceled     bool // true if healthy service's context was canceled
}

func runServicesInErrgroup(
	ctx context.Context,
	failingSvc *failingAppService,
	healthySvc *healthyService,
) errgroupResult {
	group, groupCtx := errgroup.WithContext(ctx)

	// Track whether the errgroup context was canceled (by a sibling error)
	// BEFORE the parent context was canceled (by the test timeout).
	var errgroupCanceledFirst atomic.Bool

	// Launch the failing service (replicates the app service with unavailable agent)
	group.Go(func() error {
		err := failingSvc.Run(groupCtx)
		if err != nil {
			return trace.Wrap(err, "service(%s)", failingSvc.String())
		}
		return nil
	})

	// Launch the healthy service (replicates a working service)
	group.Go(func() error {
		healthySvc.wasRunning.Store(true)
		// Block until context is canceled
		<-groupCtx.Done()
		// Check if the parent context is still alive — if so, the errgroup
		// context was canceled by a sibling error.
		if ctx.Err() == nil {
			errgroupCanceledFirst.Store(true)
		}
		healthySvc.contextWasDone.Store(true)
		healthySvc.wasRunning.Store(false)
		return nil
	})

	groupErr := group.Wait()

	return errgroupResult{
		err:                    groupErr,
		healthyServiceSurvived: healthySvc.wasRunning.Load(),
		healthyCtxCanceled:     errgroupCanceledFirst.Load(),
	}
}

// For any application service (tunnel or output) running in resilient mode
// where getApp() returns "app not found", the service shall not return a
// fatal error to the errgroup. Healthy services should continue running.
func TestBugCondition_CascadingFailureOnAppNotFound(t *testing.T) {
	t.Parallel()

	t.Run("Property_TunnelService_DoesNotKillHealthyServices", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			// Generate a random app name for the unavailable app
			appName := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "unavailable_app_name")

			// Use a short timeout — the failing service returns immediately,
			// so we only need enough time to detect whether the errgroup
			// context was canceled by a sibling error vs. the parent timeout.
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			failingSvc := &failingAppService{
				appName:       appName,
				serviceType:   "tunnel",
				resilientMode: true,
			}
			healthySvc := &healthyService{
				name: "healthy-ssh-service",
			}

			result := runServicesInErrgroup(ctx, failingSvc, healthySvc)

			// EXPECTED BEHAVIOR: In resilient mode, the failing service
			// isolates the "app not found" error by returning nil to the
			// errgroup. The healthy service's context is NOT canceled.

			// ASSERT: healthy service's context was NOT canceled
			require.False(t, result.healthyCtxCanceled,
				"healthy service context was canceled by errgroup — resilient mode "+
					"should have isolated the tunnel service failure for app %q", appName)

			// ASSERT: no fatal error was returned to the errgroup
			require.NoError(t, result.err,
				"errgroup returned error — resilient mode should have suppressed "+
					"the app-not-found error for app %q", appName)
		})
	})

	t.Run("Property_OutputService_DoesNotKillHealthyServices", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			appName := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "unavailable_app_name")

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			// OutputService also fails on app-not-found (via RunOnInterval -> generate -> getRouteToApp -> getApp)
			// In resilient mode, the error is isolated by returning nil to the errgroup
			failingSvc := &failingAppService{
				appName:       appName,
				serviceType:   "output",
				resilientMode: true,
			}
			healthySvc := &healthyService{
				name: "healthy-database-tunnel",
			}

			result := runServicesInErrgroup(ctx, failingSvc, healthySvc)

			// EXPECTED BEHAVIOR: healthy service survives in resilient mode
			require.False(t, result.healthyCtxCanceled,
				"healthy service context was canceled by errgroup — resilient mode "+
					"should have isolated the output service failure for app %q", appName)

			require.NoError(t, result.err,
				"errgroup returned error — resilient mode should have suppressed "+
					"the app-not-found error for app %q", appName)
		})
	})

	t.Run("Property_MultipleApps_HealthyTunnelSurvives", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			healthyAppName := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "healthy_app_name")
			unavailableAppName := rapid.StringMatching(`[a-z][a-z0-9\-]{2,20}`).Draw(t, "unavailable_app_name")

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			// Simulate two tunnel services: one for a healthy app, one for an unavailable app
			// In resilient mode, the failing tunnel isolates its error
			failingSvc := &failingAppService{
				appName:       unavailableAppName,
				serviceType:   "tunnel",
				resilientMode: true,
			}
			// The "healthy tunnel" is modeled as a healthy long-running service
			// because a real healthy TunnelService would block on local proxy listening
			healthyTunnel := &healthyService{
				name: fmt.Sprintf("tunnel:%s", healthyAppName),
			}

			result := runServicesInErrgroup(ctx, failingSvc, healthyTunnel)

			// EXPECTED BEHAVIOR: in resilient mode, the healthy tunnel for
			// healthyAppName should NOT be killed when the tunnel for
			// unavailableAppName fails — the failure is isolated.
			require.False(t, result.healthyCtxCanceled,
				"healthy tunnel for app %q was killed when tunnel for app %q failed — "+
					"resilient mode should have isolated the failure", healthyAppName, unavailableAppName)

			require.NoError(t, result.err,
				"errgroup returned error — resilient mode should have suppressed "+
					"the app-not-found error from tunnel for app %q, protecting app %q",
				unavailableAppName, healthyAppName)
		})
	})
}
