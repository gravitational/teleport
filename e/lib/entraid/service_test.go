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
