package machineidv1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/slices"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestAutoUpdateVersionReporter(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := clockwork.NewFakeClockAt(time.Now().UTC())

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	sema := &testSemaphores{acquired: make(chan struct{})}

	autoUpdateService, err := local.NewAutoUpdateService(backend)
	require.NoError(t, err)

	botInstanceService, err := local.NewBotInstanceService(backend, clock)
	require.NoError(t, err)

	// Create a couple of bot instances with different versions.
	createBotInstance := func(t *testing.T, versions []string) {
		t.Helper()

		_, err = botInstanceService.CreateBotInstance(ctx, &machineidv1pb.BotInstance{
			Spec: &machineidv1pb.BotInstanceSpec{
				InstanceId: uuid.NewString(),
				BotName:    "bot-1234",
			},
			Status: &machineidv1pb.BotInstanceStatus{
				LatestHeartbeats: slices.Map(versions, func(v string) *machineidv1pb.BotInstanceStatusHeartbeat {
					return &machineidv1pb.BotInstanceStatusHeartbeat{Version: v}
				}),
			},
		})
		require.NoError(t, err)
	}
	createBotInstance(t, []string{"17.0.0", "18.0.0"})
	createBotInstance(t, []string{"18.1.0"})

	reporter, err := machineidv1.NewAutoUpdateVersionReporter(machineidv1.AutoUpdateVersionReporterConfig{
		Clock:      clock,
		Logger:     logtest.NewLogger(),
		Store:      autoUpdateService,
		Cache:      botInstanceService,
		Semaphores: sema,
		HostUUID:   uuid.NewString(),
	})
	require.NoError(t, err)

	// Run the leader election process. Wait for the semaphore to be acquired.
	require.NoError(t, reporter.Run(ctx))
	select {
	case <-sema.acquired:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for semaphore to be acquired")
	}

	// Trigger the report.
	require.NoError(t, reporter.Report(ctx))

	// Check the report records the bots.
	report, err := autoUpdateService.GetAutoUpdateBotReport(ctx)
	require.NoError(t, err)

	require.Equal(t,
		clock.Now(),
		report.GetSpec().GetTimestamp().AsTime(),
	)

	groups := report.GetSpec().GetGroups()

	// Check the "unmanaged" (no group) group is there.
	unmanagedGroup, ok := groups[""]
	require.True(t, ok)

	v180, ok := unmanagedGroup.Versions["18.0.0"]
	require.True(t, ok)
	require.Equal(t, 1, int(v180.Count))

	v181, ok := unmanagedGroup.Versions["18.1.0"]
	require.True(t, ok)
	require.Equal(t, 1, int(v181.Count))
}

type testSemaphores struct {
	types.Semaphores

	acquired chan struct{}
}

func (s *testSemaphores) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	close(s.acquired)
	return &types.SemaphoreLease{}, nil
}
