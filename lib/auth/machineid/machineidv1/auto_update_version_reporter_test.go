/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package machineidv1_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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
			Metadata: &headerv1.Metadata{
				Expires: timestamppb.New(clock.Now().Add(1 * time.Hour)),
			},
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
	report, err := autoUpdateService.GetAutoUpdateBotInstanceReport(ctx)
	require.NoError(t, err)

	diff := cmp.Diff(
		&autoupdatev1pb.AutoUpdateBotInstanceReportSpec{
			Timestamp: timestamppb.New(clock.Now()),
			Groups: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroup{
				"prod": {
					Versions: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroupVersion{
						"18.1.0": {Count: 1},
					},
				},
				"stage": {
					Versions: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroupVersion{
						"19.0.0-dev": {Count: 1},
					},
				},

				// Unmanaged (no-group) group.
				"": {
					Versions: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroupVersion{
						"18.0.0": {Count: 1},
						"18.1.0": {Count: 2},
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

func TestEmitInstancesMetric(t *testing.T) {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricBotInstances,
		},
		[]string{
			teleport.TagVersion,
			teleport.TagAutomaticUpdates,
		},
	)

	machineidv1.EmitInstancesMetric(
		&autoupdatev1pb.AutoUpdateBotInstanceReport{
			Spec: &autoupdatev1pb.AutoUpdateBotInstanceReportSpec{
				Groups: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroup{
					"prod": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroupVersion{
							"18.0.0": {Count: 1},
							"19.0.0": {Count: 1},
						},
					},
					"stage": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroupVersion{
							"18.0.0": {Count: 1},
							"19.0.0": {Count: 1},
						},
					},
					"": {
						Versions: map[string]*autoupdatev1pb.AutoUpdateBotInstanceReportSpecGroupVersion{
							"19.0.0": {Count: 123},
							"20.0.0": {Count: 321},
						},
					},
				},
			},
		},
		gauge,
	)

	for _, tc := range []struct {
		version          string
		automaticUpdates bool
		expectedValue    float64
	}{
		{version: "18.0.0", automaticUpdates: true, expectedValue: 2},
		{version: "19.0.0", automaticUpdates: true, expectedValue: 2},
		{version: "19.0.0", automaticUpdates: false, expectedValue: 123},
		{version: "20.0.0", automaticUpdates: false, expectedValue: 321},
	} {
		t.Run(fmt.Sprintf("%s/%v", tc.version, tc.automaticUpdates), func(t *testing.T) {
			metric := gauge.WithLabelValues(tc.version, strconv.FormatBool(tc.automaticUpdates))
			require.InEpsilon(t, tc.expectedValue, testutil.ToFloat64(metric), 0)
		})
	}

}

type testSemaphores struct{ types.Semaphores }

func (s *testSemaphores) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return &types.SemaphoreLease{}, nil
}
