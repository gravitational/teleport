/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package discovery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
)

// newStatusTestServer returns a Server wired up to report DiscoveryConfig
// status against a fake access point.
func newStatusTestServer(clock clockwork.Clock) (*Server, *fakeAccessPoint) {
	accessPoint := newFakeAccessPoint()
	return &Server{
		ctx: context.Background(),
		Config: &Config{
			AccessPoint:  accessPoint,
			ServerID:     "server-id",
			PollInterval: time.Minute,
			clock:        clock,
		},
	}, accessPoint
}

func TestServer_updateDiscoveryConfigStatus(t *testing.T) {
	testErr := "test error"
	clock := clockwork.NewFakeClock()

	baseServerStatusFn := func() map[string]*discoveryconfig.DiscoveryStatusServer {
		return map[string]*discoveryconfig.DiscoveryStatusServer{
			"server-id": {
				DiscoveryStatusServer: &discoveryconfigv1.DiscoveryStatusServer{
					IntegrationSummaries: map[string]*discoveryconfigv1.DiscoverSummary{},
					LastUpdate:           timestamppb.New(clock.Now()),
					PollInterval:         durationpb.New(time.Minute),
				},
			},
		}
	}

	type args struct {
		fetchers []*fakeFetcher
		pushErr  error
		preRun   bool
	}
	tests := []struct {
		name string
		args args
		want map[string][]discoveryconfig.Status
	}{
		{
			name: "test updateDiscoveryConfigStatus",
			args: args{
				fetchers: []*fakeFetcher{
					{
						count:               1,
						discoveryConfigName: "test",
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_RUNNING",
						ErrorMessage:                   nil,
						DiscoveredResources:            1,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},

		{
			name: "test updateDiscoveryConfigStatus with pushError",
			args: args{
				fetchers: []*fakeFetcher{
					{
						count:               1,
						discoveryConfigName: "test",
					},
				},
				pushErr: errors.New(testErr),
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage:                   &testErr,
						DiscoveredResources:            1,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},
		{
			name: "test updateDiscoveryConfigStatus with error",
			args: args{
				fetchers: []*fakeFetcher{
					{
						count:               1,
						discoveryConfigName: "test",
						err:                 errors.New(testErr),
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage:                   &testErr,
						DiscoveredResources:            1,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},
		{
			name: "discar reports for non-discovery config results",
			args: args{
				fetchers: []*fakeFetcher{
					{
						count: 1,
					},
				},
			},
			want: map[string][]discoveryconfig.Status{},
		},
		{
			name: "test updateDiscoveryConfigStatus pre-run",
			args: args{
				fetchers: []*fakeFetcher{
					{
						discoveryConfigName: "test",
					},
				},
				preRun: true,
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_SYNCING",
						ErrorMessage:                   nil,
						DiscoveredResources:            0,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},
		{
			name: "test multiple aws sync fetchers",
			args: args{
				fetchers: []*fakeFetcher{
					{
						discoveryConfigName: "test1",
						count:               1,
					},
					{
						discoveryConfigName: "test1",
						count:               1,
					},
					{
						discoveryConfigName: "test2",
						count:               1,
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test1": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_RUNNING",
						ErrorMessage:                   nil,
						DiscoveredResources:            2,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
				"test2": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_RUNNING",
						ErrorMessage:                   nil,
						DiscoveredResources:            1,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},
		{
			name: "merge two errors",
			args: args{
				fetchers: []*fakeFetcher{
					{
						discoveryConfigName: "test1",
						err:                 fmt.Errorf("error in fetcher 1"),
					},
					{
						discoveryConfigName: "test1",
						err:                 fmt.Errorf("error in fetcher 2"),
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test1": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage:                   stringPointer("error in fetcher 1\nerror in fetcher 2"),
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},
		{
			// Counts are distinct powers of two so that a wrong total
			// identifies which fetchers were counted.
			name: "counts every discovery config fetcher",
			args: args{
				fetchers: []*fakeFetcher{
					{count: 1},
					{discoveryConfigName: "test1", count: 2},
					{count: 4},
					{discoveryConfigName: "test1", count: 8},
					{count: 16},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test1": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_RUNNING",
						ErrorMessage:                   nil,
						DiscoveredResources:            10,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},
		{
			name: "reports error if at least one fetcher fails",
			args: args{
				fetchers: []*fakeFetcher{
					{
						discoveryConfigName: "test1",
						err:                 fmt.Errorf("error in fetcher 1"),
					},
					{
						discoveryConfigName: "test1",
						count:               2,
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test1": {
					{
						State:                          "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage:                   stringPointer("error in fetcher 1"),
						DiscoveredResources:            2,
						LastSyncTime:                   clock.Now(),
						IntegrationDiscoveredResources: make(map[string]*discoveryconfig.IntegrationDiscoveredSummary),
						ServerStatus:                   baseServerStatusFn(),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, accessPoint := newStatusTestServer(clock)
			fetcherStatuses := asFetcherStatusSlice(tt.args.fetchers)
			if tt.args.preRun {
				s.tagAWSSyncStatus.iterationStarted(fetcherStatuses, s.clock.Now())
			} else {
				s.tagAWSSyncStatus.iterationFinished(fetcherStatuses, tt.args.pushErr, s.clock.Now())
			}

			for _, discoveryConfigName := range s.tagAWSSyncStatus.discoveryConfigs() {
				s.updateDiscoveryConfigStatus(discoveryConfigName)
			}

			require.Equal(t, tt.want, accessPoint.reports)
		})
	}
}

// lastReport returns the most recent status reported for a DiscoveryConfig.
func lastReport(t *testing.T, accessPoint *fakeAccessPoint, discoveryConfigName string) discoveryconfig.Status {
	t.Helper()

	reports := accessPoint.reports[discoveryConfigName]
	require.NotEmpty(t, reports, "no status reported for %q", discoveryConfigName)
	return reports[len(reports)-1]
}

// TestTagSyncStatus_reportsCurrentIterationOnly tests that the reported
// status describes the sync that just ran, and nothing earlier: results are
// replaced on each iteration rather than accumulated.
func TestTagSyncStatus_reportsCurrentIterationOnly(t *testing.T) {
	clock := clockwork.NewFakeClock()
	s, accessPoint := newStatusTestServer(clock)

	fetcher := &fakeFetcher{discoveryConfigName: "test", count: 10}
	runIteration := func() {
		fetchers := asFetcherStatusSlice([]*fakeFetcher{fetcher})
		s.tagAWSSyncStatus.iterationStarted(fetchers, s.clock.Now())
		s.tagAWSSyncStatus.iterationFinished(fetchers, nil, s.clock.Now())
		for _, discoveryConfigName := range s.tagAWSSyncStatus.discoveryConfigs() {
			s.updateDiscoveryConfigStatus(discoveryConfigName)
		}
	}

	fetcher.err = errors.New("sync failed")
	runIteration()

	failed := lastReport(t, accessPoint, "test")
	require.Equal(t, "DISCOVERY_CONFIG_STATE_ERROR", failed.State)
	require.Equal(t, stringPointer("sync failed"), failed.ErrorMessage)

	// Subsequent iterations succeed and must not carry the failure, or the
	// resource counts, of the ones before them.
	fetcher.err = nil
	for range 3 {
		clock.Advance(time.Minute)
		runIteration()

		report := lastReport(t, accessPoint, "test")
		require.Equal(t, "DISCOVERY_CONFIG_STATE_RUNNING", report.State)
		require.Nil(t, report.ErrorMessage)
		require.Equal(t, uint64(10), report.DiscoveredResources)
		require.Equal(t, clock.Now(), report.LastSyncTime)
	}
}

// TestTagSyncStatus_cloudsAreIndependent tests the AWS and Azure sync loops
// report into the same DiscoveryConfig without overwriting each other, even
// though they run independently.
func TestTagSyncStatus_cloudsAreIndependent(t *testing.T) {
	clock := clockwork.NewFakeClock()
	s, accessPoint := newStatusTestServer(clock)

	awsFetchers := asFetcherStatusSlice([]*fakeFetcher{{discoveryConfigName: "test", count: 3}})
	azureFetchers := asFetcherStatusSlice([]*fakeFetcher{{discoveryConfigName: "test", count: 5}})

	s.tagAWSSyncStatus.iterationFinished(awsFetchers, nil, s.clock.Now())
	s.tagAzureSyncStatus.iterationFinished(azureFetchers, nil, s.clock.Now())
	s.updateDiscoveryConfigStatus("test")

	require.Equal(t, uint64(8), lastReport(t, accessPoint, "test").DiscoveredResources)

	// An AWS iteration must leave the Azure results in place, and vice versa.
	s.tagAWSSyncStatus.iterationStarted(awsFetchers, s.clock.Now())
	s.tagAWSSyncStatus.iterationFinished(awsFetchers, nil, s.clock.Now())
	s.updateDiscoveryConfigStatus("test")

	require.Equal(t, uint64(8), lastReport(t, accessPoint, "test").DiscoveredResources)

	s.tagAzureSyncStatus.iterationStarted(azureFetchers, s.clock.Now())
	s.tagAzureSyncStatus.iterationFinished(azureFetchers, nil, s.clock.Now())
	s.updateDiscoveryConfigStatus("test")

	require.Equal(t, uint64(8), lastReport(t, accessPoint, "test").DiscoveredResources)
}

// TestTagSyncStatus_forgetsUnsyncedDiscoveryConfigs tests that a
// DiscoveryConfig which is no longer being synced stops being reported on.
func TestTagSyncStatus_forgetsUnsyncedDiscoveryConfigs(t *testing.T) {
	clock := clockwork.NewFakeClock()

	var status tagSyncStatus

	status.iterationFinished(asFetcherStatusSlice([]*fakeFetcher{
		{discoveryConfigName: "test1", count: 1},
		{discoveryConfigName: "test2", count: 2},
	}), nil, clock.Now())
	require.ElementsMatch(t, []string{"test1", "test2"}, status.discoveryConfigs())

	// test2 is gone: only the configs in the current iteration are reported.
	status.iterationStarted(asFetcherStatusSlice([]*fakeFetcher{
		{discoveryConfigName: "test1"},
	}), clock.Now())
	require.Equal(t, []string{"test1"}, status.discoveryConfigs())
}

func stringPointer(s string) *string {
	return &s
}

type fakeFetcher struct {
	aws_sync.Fetcher
	err                 error
	count               uint64
	discoveryConfigName string
}

func (f *fakeFetcher) Status() (uint64, error) {
	return f.count, f.err
}

func (f *fakeFetcher) DiscoveryConfigName() string {
	return f.discoveryConfigName
}

func (f *fakeFetcher) IsFromDiscoveryConfig() bool {
	return f.discoveryConfigName != ""
}
