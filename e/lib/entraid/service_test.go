package entraid

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/testing/integration"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

// fakeDirectoryReconciler is a no-op reconciler that returns the given error.
type fakeDirectoryReconciler struct {
	timesCalled    int64
	err            error
	importedUsers  int
	importedGroups int
}

func (r *fakeDirectoryReconciler) ImportedUsers() int {
	return r.importedUsers
}

func (r *fakeDirectoryReconciler) ImportedGroups() int {
	return r.importedGroups
}

func (r *fakeDirectoryReconciler) Reconcile(ctx context.Context) error {
	atomic.AddInt64(&r.timesCalled, 1)
	return trace.Wrap(r.err)
}

// fakeTAGSynchronizer that "does nothing" until the context is canceled.
type fakeTAGSynchronizer struct{}

func (r *fakeTAGSynchronizer) Run(ctx context.Context) error {
	<-ctx.Done()
	return trace.Wrap(ctx.Err())
}

func TestServiceRetryAndCancelation(t *testing.T) {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	semaphoreSvc := local.NewPresenceService(backend)
	clock := clockwork.NewFakeClock()

	directoryReconciler := &fakeDirectoryReconciler{
		err: errors.New("something bad happened"),
	}

	svc := &Service{
		clock:                   clock,
		log:                     logtest.NewLogger(),
		pluginStatusSink:        &integration.FakeStatusSink{},
		semaphoreSvc:            semaphoreSvc,
		hostID:                  "foo",
		directoryReconciler:     directoryReconciler,
		accessGraphSynchronizer: &fakeTAGSynchronizer{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	returned := make(chan error, 1)
	go func() {
		returned <- svc.Run(ctx)
	}()

	// Expect directory reconciler to get called once.
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&directoryReconciler.timesCalled) == 1
	}, time.Second, time.Second/100)

	// Advance to the next sync interval
	clock.Advance(syncInterval)

	// Expect directory reconciler to get called a second time.
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&directoryReconciler.timesCalled) == 2
	}, time.Second, time.Second/100)

	cancel()

	select {
	case err := <-returned:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		require.Fail(t, "expected Run() to return after canceling the context, but it did not")
	}
}

func TestDirectoryReconcilerStatus(t *testing.T) {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	semaphoreSvc := local.NewPresenceService(backend)
	clock := clockwork.NewFakeClock()

	directoryReconciler := &fakeDirectoryReconciler{
		importedUsers:  34,
		importedGroups: 12,
	}

	statusSink := &integration.FakeStatusSink{}

	svc := &Service{
		clock:                   clock,
		log:                     logtest.NewLogger(),
		pluginStatusSink:        statusSink,
		semaphoreSvc:            semaphoreSvc,
		hostID:                  "foo",
		directoryReconciler:     directoryReconciler,
		accessGraphSynchronizer: &fakeTAGSynchronizer{},
	}

	ctx := t.Context()

	go func() {
		svc.Run(ctx)
	}()

	// Expect directory reconciler to get called once.
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&directoryReconciler.timesCalled) == 1
	}, time.Second, time.Second/100)

	// Expect a "running" status and details.
	require.Eventually(t, func() bool {
		return statusSink.Get() != nil
	}, time.Second, time.Second/100)
	status := statusSink.Get()
	require.Equal(t, types.PluginStatusCode_RUNNING, status.GetCode())
	entraStatus := status.GetEntraId()
	require.NotNil(t, entraStatus)
	require.Equal(t, uint32(34), entraStatus.ImportedUsers)
	require.Equal(t, uint32(12), entraStatus.ImportedGroups)

	// Mock an error and advance to the next sync interval
	directoryReconciler.err = errors.New("something bad happened")
	clock.Advance(syncInterval)

	// Expect directory reconciler to get called a second time.
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&directoryReconciler.timesCalled) == 2
	}, time.Second, time.Second/100)

	// Expect an error status with a message, and details to remain.
	require.Eventually(t, func() bool {
		return statusSink.Get().GetCode() == types.PluginStatusCode_OTHER_ERROR
	}, time.Second, time.Second/100)

	status = statusSink.Get()
	require.Contains(t, status.GetErrorMessage(), "completed with partial success")
	statusV1, ok := status.(*types.PluginStatusV1)
	require.True(t, ok, "expected type PluginStatusV1 but got %T", statusV1)
	require.Contains(t, statusV1.LastRawError, directoryReconciler.err.Error())
	entraStatus = statusSink.Get().GetEntraId()
	require.NotNil(t, entraStatus)
	require.Equal(t, uint32(34), entraStatus.ImportedUsers)
	require.Equal(t, uint32(12), entraStatus.ImportedGroups)
}
