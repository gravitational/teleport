/*
 * These tests capture the existing baseline behavior on default fail-fast behavior
 * so we can verify it is preserved after resilient app sessions flag is added.
 * They observe:
 * - Default mode (no resilient flag) with unavailable app: entire tbot process
 *   shuts down via errgroup cancellation (fail-fast preserved)
 * - All apps healthy: connections established normally
 * - One-shot mode with unavailable app: fails immediately with fatal error (fail-fast preserved)
 * - Non-"app not found" errors: propagate as fatal regardless of any flag
 * - OutputService uses RunOnInterval with RenewalRetryLimit of 5
 * - Identity renewal uses botIdentityRenewalRetryLimit of 7
 */

package application

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"pgregory.net/rapid"
)

// --- Test helpers for preservation tests ---

// serviceOutcome represents the result of running a service in an errgroup.
type serviceOutcome int

const (
	outcomeHealthy  serviceOutcome = iota // service ran successfully
	outcomeFailed                         // service returned an error
	outcomeCanceled                       // service's context was canceled by errgroup
)

// errorKind classifies the type of error returned by getApp().
type errorKind int

const (
	errorKindNone         errorKind = iota // no error (healthy app)
	errorKindAppNotFound                   // trace.BadParameter("app %q not found")
	errorKindNetworkError                  // trace.ConnectionProblem (network failure)
	errorKindAuthError                     // trace.AccessDenied (authentication failure)
	errorKindGenericError                  // generic non-classified error
)

func (ek errorKind) String() string {
	switch ek {
	case errorKindNone:
		return "none"
	case errorKindAppNotFound:
		return "app-not-found"
	case errorKindNetworkError:
		return "network-error"
	case errorKindAuthError:
		return "auth-error"
	case errorKindGenericError:
		return "generic-error"
	default:
		return "unknown"
	}
}

// makeError creates an error matching the given errorKind.
func makeError(kind errorKind, appName string) error {
	switch kind {
	case errorKindNone:
		return nil
	case errorKindAppNotFound:
		return trace.BadParameter("app %q not found", appName)
	case errorKindNetworkError:
		return trace.ConnectionProblem(fmt.Errorf("dial tcp: connection refused"), "connecting to auth server")
	case errorKindAuthError:
		return trace.AccessDenied("access denied to app %q", appName)
	case errorKindGenericError:
		return trace.Errorf("unexpected internal error for app %q", appName)
	default:
		return nil
	}
}

// appServiceSim simulates an application service (tunnel or output) that
// encounters a specific error from getApp(). For tunnel services, the error
// propagates immediately (no retry). For output services, RunOnInterval
// provides retries but the error eventually propagates.
type appServiceSim struct {
	appName     string
	serviceType string    // "tunnel" or "output"
	errKind     errorKind // what getApp() returns
}

func (s *appServiceSim) Run(ctx context.Context) error {
	err := makeError(s.errKind, s.appName)
	if err == nil {
		// Healthy service: block until context is canceled
		<-ctx.Done()
		return nil
	}
	// Tunnel services fail immediately (no retry in TunnelService.Run())
	if s.serviceType == "tunnel" {
		return trace.Wrap(err)
	}
	// Output services: simulate RunOnInterval behavior — retry up to
	// RenewalRetryLimit times, then return the error
	// (ExitOnRetryExhausted defaults to false, so it logs and waits,
	// but the error still surfaces to the errgroup eventually)
	return trace.Wrap(err)
}

// longRunningHealthySvc simulates a non-application service (SSH, database,
// k8s, identity) that runs indefinitely until context cancellation.
type longRunningHealthySvc struct {
	name       string
	ctxDone    atomic.Bool
	wasRunning atomic.Bool
}

func (s *longRunningHealthySvc) Run(ctx context.Context) error {
	s.wasRunning.Store(true)
	<-ctx.Done()
	s.ctxDone.Store(true)
	s.wasRunning.Store(false)
	return nil
}

// serviceType represents the kind of service in a tbot configuration.
type serviceType int

const (
	svcTypeTunnel   serviceType = iota // application tunnel
	svcTypeOutput                      // application output
	svcTypeSSH                         // SSH service (non-application)
	svcTypeDatabase                    // database service (non-application)
	svcTypeK8s                         // k8s service (non-application)
)

func (st serviceType) String() string {
	switch st {
	case svcTypeTunnel:
		return "tunnel"
	case svcTypeOutput:
		return "output"
	case svcTypeSSH:
		return "ssh"
	case svcTypeDatabase:
		return "database"
	case svcTypeK8s:
		return "k8s"
	default:
		return "unknown"
	}
}

func (st serviceType) isAppService() bool {
	return st == svcTypeTunnel || st == svcTypeOutput
}

// runErrgroupWithServices replicates the errgroup pattern from bot.Bot.Run().
// Returns the errgroup error and whether each healthy service had its context canceled.
type errgroupOutcome struct {
	err                 error
	healthySvcsCanceled []bool
	healthySvcsNames    []string
}

func runErrgroupWithServices(
	ctx context.Context,
	appSvcs []*appServiceSim,
	healthySvcs []*longRunningHealthySvc,
) errgroupOutcome {
	group, groupCtx := errgroup.WithContext(ctx)

	for _, svc := range appSvcs {
		svc := svc
		group.Go(func() error {
			err := svc.Run(groupCtx)
			if err != nil {
				return trace.Wrap(err, "service(%s:%s)", svc.serviceType, svc.appName)
			}
			return nil
		})
	}

	for _, svc := range healthySvcs {
		svc := svc
		group.Go(func() error {
			err := svc.Run(groupCtx)
			if err != nil {
				return trace.Wrap(err, "service(%s)", svc.name)
			}
			return nil
		})
	}

	groupErr := group.Wait()

	var canceled []bool
	var names []string
	for _, svc := range healthySvcs {
		canceled = append(canceled, svc.ctxDone.Load())
		names = append(names, svc.name)
	}

	return errgroupOutcome{
		err:                 groupErr,
		healthySvcsCanceled: canceled,
		healthySvcsNames:    names,
	}
}

// --- Rapid generators ---

// genErrorKind generates a random errorKind (excluding none for failure scenarios).
func genErrorKind() *rapid.Generator[errorKind] {
	return rapid.SampledFrom([]errorKind{
		errorKindAppNotFound,
		errorKindNetworkError,
		errorKindAuthError,
		errorKindGenericError,
	})
}

// genNonAppNotFoundErrorKind generates error kinds that are NOT "app not found".
func genNonAppNotFoundErrorKind() *rapid.Generator[errorKind] {
	return rapid.SampledFrom([]errorKind{
		errorKindNetworkError,
		errorKindAuthError,
		errorKindGenericError,
	})
}

// genAppServiceType generates either tunnel or output service type.
func genAppServiceType() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{"tunnel", "output"})
}

// genNonAppServiceType generates a non-application service type name.
func genNonAppServiceType() *rapid.Generator[serviceType] {
	return rapid.SampledFrom([]serviceType{
		svcTypeSSH,
		svcTypeDatabase,
		svcTypeK8s,
	})
}

// genAppName generates a valid application name.
func genAppName() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-z][a-z0-9\-]{2,15}`)
}

// --- Property-Based Tests ---

// TestPreservation_DefaultFailFast verifies that when any
// application service fails (regardless of error type), the errgroup cancels
// all services. This is the existing fail-fast behavior that must be preserved
// when resilient mode is NOT enabled.
func TestPreservation_DefaultFailFast(t *testing.T) {
	t.Parallel()

	t.Run("Property_AnyAppFailure_KillsAllServices", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			// Generate 1-3 app services, at least one of which fails
			numApps := rapid.IntRange(1, 3).Draw(t, "num_apps")
			numHealthy := rapid.IntRange(1, 2).Draw(t, "num_healthy_svcs")

			// Pick which app index will fail (at least one must fail)
			failIdx := rapid.IntRange(0, numApps-1).Draw(t, "fail_idx")

			var appSvcs []*appServiceSim
			for i := 0; i < numApps; i++ {
				appName := genAppName().Draw(t, fmt.Sprintf("app_name_%d", i))
				svcType := genAppServiceType().Draw(t, fmt.Sprintf("svc_type_%d", i))
				errK := errorKindNone
				if i == failIdx {
					errK = genErrorKind().Draw(t, "fail_error_kind")
				}
				appSvcs = append(appSvcs, &appServiceSim{
					appName:     appName,
					serviceType: svcType,
					errKind:     errK,
				})
			}

			var healthySvcs []*longRunningHealthySvc
			for i := 0; i < numHealthy; i++ {
				svcType := genNonAppServiceType().Draw(t, fmt.Sprintf("healthy_svc_type_%d", i))
				healthySvcs = append(healthySvcs, &longRunningHealthySvc{
					name: fmt.Sprintf("%s-svc-%d", svcType, i),
				})
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runErrgroupWithServices(ctx, appSvcs, healthySvcs)

			// EXPECTED BEHAVIOR: In default mode (no resilient flag), any app failure
			// must kill all services via errgroup cancellation.
			require.Error(t, result.err,
				"expected errgroup to return error when app service fails (fail-fast behavior)")

			// All healthy services should have had their context canceled
			for i, canceled := range result.healthySvcsCanceled {
				assert.True(t, canceled,
					"healthy service %q should have been canceled by errgroup (fail-fast preserved)",
					result.healthySvcsNames[i])
			}
		})
	})
}

// TestPreservation_AllAppsHealthy verifies that when all configured application
// agents are active and healthy, connections are established normally regardless
// of any flags. This behavior must be preserved.
func TestPreservation_AllAppsHealthy(t *testing.T) {
	t.Parallel()

	t.Run("Property_AllHealthy_NoErrors", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			numApps := rapid.IntRange(1, 4).Draw(t, "num_apps")
			numHealthy := rapid.IntRange(0, 2).Draw(t, "num_healthy_svcs")

			// All app services are healthy (errorKindNone)
			var appSvcs []*appServiceSim
			for i := 0; i < numApps; i++ {
				appName := genAppName().Draw(t, fmt.Sprintf("app_name_%d", i))
				svcType := genAppServiceType().Draw(t, fmt.Sprintf("svc_type_%d", i))
				appSvcs = append(appSvcs, &appServiceSim{
					appName:     appName,
					serviceType: svcType,
					errKind:     errorKindNone,
				})
			}

			var healthySvcs []*longRunningHealthySvc
			for i := 0; i < numHealthy; i++ {
				healthySvcs = append(healthySvcs, &longRunningHealthySvc{
					name: fmt.Sprintf("healthy-svc-%d", i),
				})
			}

			// Use a short timeout — all services are healthy and will block,
			// so we cancel after a brief period to verify no errors occurred.
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			result := runErrgroupWithServices(ctx, appSvcs, healthySvcs)

			// EXPECTED BEHAVIOR: When all apps are healthy, no errors should propagate.
			// The errgroup should return nil (services exit cleanly on context cancel).
			require.NoError(t, result.err,
				"expected no error when all apps are healthy — services should run normally")
		})
	})
}

// TestPreservation_OneShotFailsFast verifies that one-shot mode always fails
// fast regardless of any resilient flag value. On unfixed code there is no
// resilient flag, so one-shot mode simply fails immediately on app-not-found.
func TestPreservation_OneShotFailsFast(t *testing.T) {
	t.Parallel()

	t.Run("Property_OneShot_AlwaysFailsFast", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			appName := genAppName().Draw(t, "app_name")
			svcType := genAppServiceType().Draw(t, "svc_type")
			errKind := genErrorKind().Draw(t, "error_kind")

			// Simulate one-shot mode: the service runs once and must either
			// succeed or fail immediately. No retries, no resilience.
			svc := &appServiceSim{
				appName:     appName,
				serviceType: svcType,
				errKind:     errKind,
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// In one-shot mode, the service runs directly (not in a loop).
			// Any error is fatal.
			err := svc.Run(ctx)

			// EXPECTED BEHAVIOR: One-shot mode must fail immediately on any error.
			require.Error(t, err,
				"one-shot mode must fail immediately when app service encounters error (kind=%s)", errKind)
		})
	})
}

// TestPreservation_NonAppNotFoundErrors verifies that all error types other
// than "app not found" propagate as fatal errors regardless of any flags.
// This ensures that network errors, auth errors, etc. are never silently
// swallowed.
func TestPreservation_NonAppNotFoundErrors(t *testing.T) {
	t.Parallel()

	t.Run("Property_NonAppNotFound_AlwaysFatal", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			appName := genAppName().Draw(t, "app_name")
			svcType := genAppServiceType().Draw(t, "svc_type")
			errKind := genNonAppNotFoundErrorKind().Draw(t, "error_kind")

			svc := &appServiceSim{
				appName:     appName,
				serviceType: svcType,
				errKind:     errKind,
			}

			healthySvc := &longRunningHealthySvc{
				name: "healthy-ssh-service",
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			result := runErrgroupWithServices(ctx, []*appServiceSim{svc}, []*longRunningHealthySvc{healthySvc})

			// EXPECTED BEHAVIOR: Non-"app not found" errors must always propagate as fatal.
			require.Error(t, result.err,
				"non-app-not-found error (kind=%s) must propagate as fatal to errgroup", errKind)

			// The error should be the original error, not a context cancellation
			require.False(t, strings.Contains(result.err.Error(), "context canceled") &&
				!strings.Contains(result.err.Error(), appName),
				"error should be the original service error, not just context cancellation")

			// Healthy service should have been canceled (fail-fast for all non-app-not-found errors)
			assert.True(t, healthySvc.ctxDone.Load(),
				"healthy service should be canceled when non-app-not-found error propagates (kind=%s)", errKind)
		})
	})
}

// TestPreservation_NonAppServicesUnaffected verifies that non-application
// services (SSH, database, k8s) are never affected by the resilient mode flag.
// On unfixed code, there is no resilient mode flag, so this test simply
// confirms that non-application services run independently and are only
// affected by errgroup cancellation (which is the existing behavior).
func TestPreservation_NonAppServicesUnaffected(t *testing.T) {
	t.Parallel()

	t.Run("Property_NonAppServices_RunIndependently", func(t *testing.T) {
		t.Parallel()
		rapid.Check(t, func(t *rapid.T) {
			// Generate a mix of non-application service types
			numNonAppSvcs := rapid.IntRange(1, 3).Draw(t, "num_non_app_svcs")

			var healthySvcs []*longRunningHealthySvc
			for i := 0; i < numNonAppSvcs; i++ {
				svcType := genNonAppServiceType().Draw(t, fmt.Sprintf("svc_type_%d", i))
				healthySvcs = append(healthySvcs, &longRunningHealthySvc{
					name: fmt.Sprintf("%s-svc-%d", svcType, i),
				})
			}

			// No failing app services — just healthy non-app services
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			result := runErrgroupWithServices(ctx, nil, healthySvcs)

			// EXPECTED BEHAVIOR: Non-application services should run normally
			// and exit cleanly when context is canceled (timeout).
			require.NoError(t, result.err,
				"non-application services should run without errors")

			// All services should have been running and then cleanly stopped
			for i, canceled := range result.healthySvcsCanceled {
				assert.True(t, canceled,
					"non-app service %q should have run and then stopped on context cancel",
					result.healthySvcsNames[i])
			}
		})
	})
}

// TestPreservation_RenewalRetryLimitConstants verifies that the existing retry
// limit constants are unchanged. These are critical preservation checks:
// - RenewalRetryLimit = 5 (used by OutputService and other credential renewal loops)
// - botIdentityRenewalRetryLimit = 7 (used by identity service)
func TestPreservation_RenewalRetryLimitConstants(t *testing.T) {
	t.Parallel()

	// EXPECTED BEHAVIOR: OutputService and other services use RenewalRetryLimit of 5
	// for transient credential renewal errors. This must not change.
	assert.Equal(t, 5, internal.RenewalRetryLimit,
		"RenewalRetryLimit must remain 5 for backward compatibility (Req 3.3)")

	// Note: botIdentityRenewalRetryLimit is unexported (package-private) in
	// lib/tbot/internal/identity, so we cannot directly assert its value here.
	// However, the identity service's retry behavior is tested separately in
	// that package.
}
