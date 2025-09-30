package machineidv1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"github.com/gravitational/teleport/lib/utils/slices"
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

	autoUpdateService, err := local.NewAutoUpdateService(backend)
	require.NoError(t, err)

	botInstanceService, err := local.NewBotInstanceService(backend, clock)
	require.NoError(t, err)

	// Create a couple of bot instances with different versions.
	createBotInstance := func(t *testing.T, group string, versions []string) {
		t.Helper()

		_, err = botInstanceService.CreateBotInstance(ctx, &machineidv1pb.BotInstance{
			Spec: &machineidv1pb.BotInstanceSpec{
				InstanceId: uuid.NewString(),
				BotName:    "bot-1234",
			},
			Status: &machineidv1pb.BotInstanceStatus{
				LatestHeartbeats: slices.Map(versions, func(v string) *machineidv1pb.BotInstanceStatusHeartbeat {
					heartbeat := &machineidv1pb.BotInstanceStatusHeartbeat{Version: v}
					if group != "" {
						heartbeat.ExternalUpdater = "kube"
						heartbeat.UpdaterInfo = &types.UpdaterV2Info{UpdateGroup: group}
					}
					return heartbeat
				}),
			},
		})
		require.NoError(t, err)
	}
	createBotInstance(t, "", []string{"17.0.0", "18.0.0"})
	createBotInstance(t, "", []string{"18.1.0"})
	createBotInstance(t, "prod", []string{"18.1.0"})
	createBotInstance(t, "stage", []string{"19.0.0-dev"})

	reporter, err := machineidv1.NewAutoUpdateVersionReporter(machineidv1.AutoUpdateVersionReporterConfig{
		Clock:      clock,
		Logger:     logtest.NewLogger(),
		Store:      autoUpdateService,
		Cache:      botInstanceService,
		Semaphores: &testSemaphores{},
		HostUUID:   uuid.NewString(),
	})
	require.NoError(t, err)

	// Run the leader election process. Wait for the semaphore to be acquired.
	require.NoError(t, reporter.Run(ctx))
	select {
	case <-reporter.LeaderCh():
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for semaphore to be acquired")
	}

	// Trigger the report.
	require.NoError(t, reporter.Report(ctx))

	// Check the report records the bots.
	report, err := autoUpdateService.GetAutoUpdateBotReport(ctx)
	require.NoError(t, err)

	diff := cmp.Diff(
		&autoupdatev1pb.AutoUpdateBotReportSpec{
			Timestamp: timestamppb.New(clock.Now()),
			Groups: map[string]*autoupdatev1pb.AutoUpdateBotReportSpecGroup{
				"prod": {
					Versions: map[string]*autoupdatev1pb.AutoUpdateBotReportSpecGroupVersion{
						"18.1.0": {Count: 1},
					},
				},
				"stage": {
					Versions: map[string]*autoupdatev1pb.AutoUpdateBotReportSpecGroupVersion{
						"19.0.0-dev": {Count: 1},
					},
				},

				// Unmanaged (no-group) group.
				"": {
					Versions: map[string]*autoupdatev1pb.AutoUpdateBotReportSpecGroupVersion{
						"18.0.0": {Count: 1},
						"18.1.0": {Count: 1},
					},
				},
			},
		},
		report.GetSpec(),
		protocmp.Transform(),
	)
	if diff != "" {
		t.Fatal(diff)
	}
}

type testSemaphores struct{ types.Semaphores }

func (s *testSemaphores) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return &types.SemaphoreLease{}, nil
}
