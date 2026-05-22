package entraid

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/entraid/directory"
	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/msgraph"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// fakeDirectoryReconciler is a no-op reconciler that returns the given error.
type fakeDirectoryReconciler struct {
	timesCalled int64

	mu  sync.Mutex
	err error

	importedUsers  int
	importedGroups int
}

func (r *fakeDirectoryReconciler) Reconcile(ctx context.Context, _ mdmsync.SyncMode) (directory.Result, error) {
	atomic.AddInt64(&r.timesCalled, 1)

	r.mu.Lock()
	err := r.err
	r.mu.Unlock()

	out := directory.Result{
		ImportedUsers:  r.importedUsers,
		ImportedGroups: r.importedGroups,
	}
	return out, trace.Wrap(err)
}

func (r *fakeDirectoryReconciler) setErr(err error) {
	r.mu.Lock()
	r.err = err
	r.mu.Unlock()
}

// fakeTAGSynchronizer that "does nothing" until the context is canceled.
type fakeTAGSynchronizer struct{}

func (r *fakeTAGSynchronizer) Run(ctx context.Context) error {
	<-ctx.Done()
	return trace.Wrap(ctx.Err())
}

func TestServiceRetryAndCancelation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		backend, err := memory.New(memory.Config{})
		require.NoError(t, err)

		semaphoreSvc := local.NewPresenceService(backend)
		clock := clockwork.NewRealClock()

		directoryReconciler := &fakeDirectoryReconciler{
			err: errors.New("something bad happened"),
		}

		shedules, err := newScheduler(SyncIntervals{
			Full:  DefaultFullSyncInterval,
			Delta: 0,
		})
		require.NoError(t, err)
		svc := &Service{
			clock:                   clock,
			log:                     logtest.NewLogger(),
			pluginStatusSink:        &integration.FakeStatusSink{},
			semaphoreSvc:            semaphoreSvc,
			hostID:                  "foo",
			directoryReconciler:     directoryReconciler,
			accessGraphSynchronizer: &fakeTAGSynchronizer{},
			syncIntervals:           shedules,
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		returned := make(chan error, 1)
		go func() {
			returned <- svc.Run(ctx)
		}()

		synctest.Wait()
		// Expect directory reconciler to get called once.
		require.Equal(t, int64(1), atomic.LoadInt64(&directoryReconciler.timesCalled))

		// Advance to the next sync interval
		time.Sleep(DefaultFullSyncInterval)
		synctest.Wait()
		// Expect directory reconciler to get called a second time.
		require.Equal(t, int64(2), atomic.LoadInt64(&directoryReconciler.timesCalled))

		cancel()
		synctest.Wait()
		require.ErrorIs(t, <-returned, context.Canceled)
	})
}

func TestDirectoryReconcilerStatus(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		backend, err := memory.New(memory.Config{})
		require.NoError(t, err)
		semaphoreSvc := local.NewPresenceService(backend)
		clock := clockwork.NewRealClock()

		directoryReconciler := &fakeDirectoryReconciler{
			importedUsers:  34,
			importedGroups: 12,
		}

		statusSink := &integration.FakeStatusSink{}

		shedules, err := newScheduler(SyncIntervals{
			Full:  DefaultFullSyncInterval,
			Delta: 0,
		})
		require.NoError(t, err)

		svc := &Service{
			clock:                   clock,
			log:                     logtest.NewLogger(),
			pluginStatusSink:        statusSink,
			semaphoreSvc:            semaphoreSvc,
			hostID:                  "foo",
			directoryReconciler:     directoryReconciler,
			accessGraphSynchronizer: &fakeTAGSynchronizer{},
			syncIntervals:           shedules,
		}

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		go func() {
			svc.Run(ctx)
		}()

		synctest.Wait()
		// Expect directory reconciler to get called once.
		require.Equal(t, int64(1), atomic.LoadInt64(&directoryReconciler.timesCalled))

		// Expect a "running" status and details.
		status := statusSink.Get()
		require.Equal(t, types.PluginStatusCode_RUNNING, status.GetCode())
		entraStatus := status.GetEntraId()
		require.NotNil(t, entraStatus)
		require.Equal(t, uint32(34), entraStatus.ImportedUsers)
		require.Equal(t, uint32(12), entraStatus.ImportedGroups)

		// Mock an error and advance to the next sync interval
		syncErr := errors.New("something bad happened")
		directoryReconciler.setErr(syncErr)
		time.Sleep(DefaultFullSyncInterval)
		synctest.Wait()
		// Expect directory reconciler to get called a second time.
		require.Equal(t, int64(2), atomic.LoadInt64(&directoryReconciler.timesCalled))

		// Expect an error status with a message, and details to remain.
		status = statusSink.Get()
		require.Equal(t, types.PluginStatusCode_OTHER_ERROR, status.GetCode())
		require.Contains(t, status.GetErrorMessage(), "completed with partial success")
		statusV1, ok := status.(*types.PluginStatusV1)
		require.True(t, ok, "expected type PluginStatusV1 but got %T", statusV1)
		require.Contains(t, statusV1.LastRawError, syncErr.Error())
		entraStatus = statusSink.Get().GetEntraId()
		require.NotNil(t, entraStatus)
		require.Equal(t, uint32(34), entraStatus.ImportedUsers)
		require.Equal(t, uint32(12), entraStatus.ImportedGroups)
	})
}

func TestMaybeResetSyncScheduleOnStart(t *testing.T) {
	// fullSyncInterval configured to be greater than
	// DefaultFullSyncInterval which is 5 minutes.
	const fullSyncInterval = time.Hour
	const deltaSyncInterval = 2 * time.Minute
	tests := []struct {
		name         string
		err          error
		syncInterval SyncIntervals
		wantReset    bool
	}{
		{
			name: "nil error should not reset",
			err:  nil,
			syncInterval: SyncIntervals{
				Full:  fullSyncInterval,
				Delta: deltaSyncInterval, // duration > 0 enables delta sync.
			},
			wantReset: false,
		},
		{
			name: "delta setup error should reset",
			err:  msgraph.ErrMissingDeltaLink,
			syncInterval: SyncIntervals{
				Full:  fullSyncInterval,
				Delta: deltaSyncInterval,
			},
			wantReset: true,
		},
		{
			name: "delta api error should reset",
			err: trace.Wrap(&msgraph.GraphError{
				Code: msgraph.ErrCodeSyncStateNotFound,
			}),
			syncInterval: SyncIntervals{
				Full:  fullSyncInterval,
				Delta: deltaSyncInterval,
			},
			wantReset: true,
		},
		{
			name: "unknown error from full sync resets if deltasync is enabled",
			err:  errors.New("random error"),
			syncInterval: SyncIntervals{
				Full:  fullSyncInterval,
				Delta: deltaSyncInterval,
			},
			wantReset: true,
		},
		{
			name: "unknown error should not reset if deltasync is disabled",
			err:  errors.New("random error"),
			syncInterval: SyncIntervals{
				Full:  fullSyncInterval,
				Delta: 0, // disables delta sync
			},
			wantReset: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				scheduler, err := newScheduler(tc.syncInterval)
				require.NoError(t, err)
				deltaEnabled := tc.syncInterval.Delta > 0

				clock := clockwork.NewRealClock()

				directoryReconciler := &fakeDirectoryReconciler{}
				directoryReconciler.setErr(tc.err)

				svc := &Service{
					clock:               clock,
					log:                 logtest.NewLogger(),
					directoryReconciler: directoryReconciler,
					syncIntervals:       scheduler,
					deltaSyncEnabled:    deltaEnabled,
				}

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				go func() {
					// First sync is always a full sync.
					svc.runScheduled(ctx)
				}()

				synctest.Wait()
				// Expect directory reconciler to get called once.
				require.Equal(t, int64(1), atomic.LoadInt64(&directoryReconciler.timesCalled))

				if tc.wantReset {
					// Reset offsets next sync to start after DefaultFullSyncInterval.
					require.Equal(t, DefaultFullSyncInterval, scheduler.NextOffset())

					next := scheduler.Next()
					require.Equal(t, mdmsync.SyncModeFull, next.Mode)
					return
				}

				if deltaEnabled {
					require.Equal(t, tc.syncInterval.Delta, scheduler.NextOffset())
				} else {
					require.Equal(t, tc.syncInterval.Full, scheduler.NextOffset())
				}
			})
		})
	}
}

func TestMaybeResetSyncSchedule_SyncModePartial(t *testing.T) {
	const deltaSyncInterval = 2 * time.Minute
	tests := []struct {
		name      string
		err       error
		wantReset bool
	}{
		{
			name:      "missing delta link error should reset",
			err:       msgraph.ErrMissingDeltaLink,
			wantReset: true,
		},
		{
			name: "delta api error should reset",
			err: trace.Wrap(&msgraph.GraphError{
				Code: msgraph.ErrCodeSyncStateNotFound,
			}),
			wantReset: true,
		},
		{
			name:      "should not reset on unknown error on partial sync mode",
			err:       errors.New("random error"),
			wantReset: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			scheduler, err := newScheduler(SyncIntervals{
				Full:  time.Hour,
				Delta: deltaSyncInterval,
			})
			require.NoError(t, err)

			// Consume first full sync schedule i.e. 0 offset.
			firstSync := scheduler.Next()
			require.Equal(t, mdmsync.SyncModeFull, firstSync.Mode)

			svc := &Service{
				log:              logtest.NewLogger(),
				deltaSyncEnabled: true,
				syncIntervals:    scheduler,
			}

			svc.maybeResetSyncSchedule(t.Context(), tc.err, mdmsync.SyncModePartial)

			if tc.wantReset {
				require.Equal(t, DefaultFullSyncInterval, scheduler.NextOffset())

				next := scheduler.Next()
				require.Equal(t, mdmsync.SyncModeFull, next.Mode)
				return
			}

			require.Equal(t, deltaSyncInterval, scheduler.NextOffset())
		})
	}
}
