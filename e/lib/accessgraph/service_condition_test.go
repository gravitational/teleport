package accessgraph

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/clusterconfig"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

type fakeWatcher struct {
	events chan types.Event
	done   chan struct{}
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{
		events: make(chan types.Event, 10),
		done:   make(chan struct{}),
	}
}

func (w *fakeWatcher) Events() <-chan types.Event { return w.events }
func (w *fakeWatcher) Done() <-chan struct{}      { return w.done }
func (w *fakeWatcher) Error() error               { return nil }
func (w *fakeWatcher) Close() error {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	return nil
}

type fakeConditionWatcher struct {
	watcher  *fakeWatcher
	errOnNew error
}

func (f *fakeConditionWatcher) NewWatcher(_ context.Context, _ types.Watch) (types.Watcher, error) {
	if f.errOnNew != nil {
		return nil, f.errOnNew
	}
	return f.watcher, nil
}

type sequentialConditionWatcher struct {
	watchers []*fakeWatcher
	idx      int
}

func (s *sequentialConditionWatcher) NewWatcher(_ context.Context, _ types.Watch) (types.Watcher, error) {
	w := s.watchers[s.idx]
	s.idx++
	return w, nil
}

func newAccessGraphSettingsEvent(t *testing.T, demoMode clusterconfigv1.AccessGraphDemoMode) types.Event {
	t.Helper()
	settings, err := clusterconfig.NewAccessGraphSettings(clusterconfigv1.AccessGraphSettingsSpec_builder{
		DemoMode:          demoMode,
		SecretsScanConfig: clusterconfigv1.AccessGraphSecretsScanConfig_ACCESS_GRAPH_SECRETS_SCAN_CONFIG_DISABLED,
	}.Build())
	require.NoError(t, err)
	return types.Event{Type: types.OpPut, Resource: types.Resource153ToLegacy(settings)}
}

func noopModules() modules.Modules { return &modulestest.Modules{} }

func modulesWithEntitlement(kind entitlements.EntitlementKind) modules.Modules {
	return &modulestest.Modules{
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				kind: {Enabled: true},
			},
		},
	}
}

type conditionResult struct {
	retry bool
	err   error
}

func runWatchAccessGraphEntitlementOnce(ctx context.Context, cw conditionWatcher, mods modules.Modules) conditionResult {
	retry, err := watchAccessGraphEntitlementOnce(ctx, slog.Default(), cw, mods)
	return conditionResult{retry, err}
}

// TestWatchAccessGraphEntitlementOnce covers all return paths of watchAccessGraphEntitlementOnce.
func TestWatchAccessGraphEntitlementOnce(t *testing.T) {
	t.Parallel()

	t.Run("pre-canceled ctx returns error", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			r := runWatchAccessGraphEntitlementOnce(ctx, &fakeConditionWatcher{watcher: newFakeWatcher()}, noopModules())
			require.False(t, r.retry)
			require.ErrorIs(t, r.err, context.Canceled)
		})
	})

	t.Run("watcher creation error signals retry", func(t *testing.T) {
		t.Parallel()
		r := runWatchAccessGraphEntitlementOnce(context.Background(), &fakeConditionWatcher{errOnNew: context.DeadlineExceeded}, noopModules())
		require.True(t, r.retry)
		require.NoError(t, r.err)
	})

	t.Run("demo mode event with entitlement signals condition met", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			w := newFakeWatcher()
			w.events <- newAccessGraphSettingsEvent(t, clusterconfigv1.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED)
			r := runWatchAccessGraphEntitlementOnce(context.Background(), &fakeConditionWatcher{watcher: w}, modulesWithEntitlement(entitlements.AccessGraphDemoMode))
			require.False(t, r.retry)
			require.NoError(t, r.err)
		})
	})

	t.Run("watcher done with canceled ctx returns error", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			w := newFakeWatcher()
			close(w.done)
			r := runWatchAccessGraphEntitlementOnce(ctx, &fakeConditionWatcher{watcher: w}, noopModules())
			require.False(t, r.retry)
			require.ErrorIs(t, r.err, context.Canceled)
		})
	})

	t.Run("5-minute timer with AccessGraph entitlement signals condition met", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			errc := make(chan conditionResult, 1)
			go func() {
				errc <- runWatchAccessGraphEntitlementOnce(context.Background(), &fakeConditionWatcher{watcher: newFakeWatcher()}, modulesWithEntitlement(entitlements.AccessGraph))
			}()
			synctest.Wait()
			r := <-errc
			require.False(t, r.retry)
			require.NoError(t, r.err)
		})
	})

	t.Run("watcher done without ctx error signals retry", func(t *testing.T) {
		t.Parallel()
		w := newFakeWatcher()
		close(w.done)
		r := runWatchAccessGraphEntitlementOnce(context.Background(), &fakeConditionWatcher{watcher: w}, noopModules())
		require.True(t, r.retry)
		require.NoError(t, r.err)
	})

	// Events that must be ignored: disabled demo mode, enabled demo mode without the
	// entitlement, and non-OpPut event types. In each case the function must keep
	// blocking until the context is canceled.
	for _, tc := range []struct {
		name  string
		event types.Event
		mods  modules.Modules
	}{
		{"demo mode disabled event", newAccessGraphSettingsEvent(t, clusterconfigv1.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_DISABLED), noopModules()},
		{"demo mode enabled without entitlement", newAccessGraphSettingsEvent(t, clusterconfigv1.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED), noopModules()},
		{"non-OpPut event", types.Event{Type: types.OpDelete}, noopModules()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				w := newFakeWatcher()
				w.events <- tc.event
				errc := make(chan error, 1)
				go func() {
					_, err := watchAccessGraphEntitlementOnce(ctx, slog.Default(), &fakeConditionWatcher{watcher: w}, tc.mods)
					errc <- err
				}()
				synctest.Wait()
				cancel()
				require.ErrorIs(t, <-errc, context.Canceled)
			})
		})
	}
}

// TestWaitForAccessGraphEntitlement covers the retry loop in waitForAccessGraphEntitlement.
func TestWaitForAccessGraphEntitlement(t *testing.T) {
	t.Parallel()

	t.Run("retries after watcher done then succeeds on second watcher", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			first, second := newFakeWatcher(), newFakeWatcher()
			errc := make(chan error, 1)
			go func() {
				errc <- waitForAccessGraphEntitlement(context.Background(), slog.Default(),
					&sequentialConditionWatcher{watchers: []*fakeWatcher{first, second}},
					modulesWithEntitlement(entitlements.AccessGraphDemoMode))
			}()
			close(first.done)
			synctest.Wait()
			second.events <- newAccessGraphSettingsEvent(t, clusterconfigv1.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED)
			require.NoError(t, <-errc)
		})
	})

	t.Run("ctx canceled during 1-minute retry delay returns error", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			w := newFakeWatcher()
			errc := make(chan error, 1)
			go func() {
				errc <- waitForAccessGraphEntitlement(ctx, slog.Default(), &fakeConditionWatcher{watcher: w}, noopModules())
			}()
			close(w.done)
			synctest.Wait()
			cancel()
			require.ErrorIs(t, <-errc, context.Canceled)
		})
	})
}
