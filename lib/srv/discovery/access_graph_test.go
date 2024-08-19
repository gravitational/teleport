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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
)

func TestServer_updateDiscoveryConfigStatus(t *testing.T) {
	testErr := "test error"
	clock := clockwork.NewFakeClock()
	type args struct {
		fetchers []aws_sync.AWSSync
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
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						count:               1,
						discoveryConfigName: "test",
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:               "DISCOVERY_CONFIG_STATE_RUNNING",
						ErrorMessage:        nil,
						DiscoveredResources: 1,
						LastSyncTime:        clock.Now(),
					},
				},
			},
		},

		{
			name: "test updateDiscoveryConfigStatus with pushError",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						count:               1,
						discoveryConfigName: "test",
					},
				},
				pushErr: errors.New(testErr),
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:               "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage:        &testErr,
						DiscoveredResources: 1,
						LastSyncTime:        clock.Now(),
					},
				},
			},
		},
		{
			name: "test updateDiscoveryConfigStatus with error",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						count:               1,
						discoveryConfigName: "test",
						err:                 errors.New(testErr),
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:               "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage:        &testErr,
						DiscoveredResources: 1,
						LastSyncTime:        clock.Now(),
					},
				},
			},
		},
		{
			name: "discar reports for non-discovery config results",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						count: 1,
					},
				},
			},
			want: map[string][]discoveryconfig.Status{},
		},
		{
			name: "test updateDiscoveryConfigStatus pre-run",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						discoveryConfigName: "test",
					},
				},
				preRun: true,
			},
			want: map[string][]discoveryconfig.Status{
				"test": {
					{
						State:               "DISCOVERY_CONFIG_STATE_SYNCING",
						ErrorMessage:        nil,
						DiscoveredResources: 0,
						LastSyncTime:        clock.Now(),
					},
				},
			},
		},
		{
			name: "test multiple aws sync fetchers",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						discoveryConfigName: "test1",
						count:               1,
					},
					&fakeFetcher{
						discoveryConfigName: "test1",
						count:               1,
					},
					&fakeFetcher{
						discoveryConfigName: "test2",
						count:               1,
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test1": {
					{
						State:               "DISCOVERY_CONFIG_STATE_RUNNING",
						ErrorMessage:        nil,
						DiscoveredResources: 2,
						LastSyncTime:        clock.Now(),
					},
				},
				"test2": {
					{
						State:               "DISCOVERY_CONFIG_STATE_RUNNING",
						ErrorMessage:        nil,
						DiscoveredResources: 1,
						LastSyncTime:        clock.Now(),
					},
				},
			},
		},
		{
			name: "merge two errors",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						discoveryConfigName: "test1",
						err:                 fmt.Errorf("error in fetcher 1"),
					},
					&fakeFetcher{
						discoveryConfigName: "test1",
						err:                 fmt.Errorf("error in fetcher 2"),
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test1": {
					{
						State:        "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage: stringPointer("error in fetcher 1\nerror in fetcher 2"),
						LastSyncTime: clock.Now(),
					},
				},
			},
		},
		{
			name: "reports error if at least one fetcher fails",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						discoveryConfigName: "test1",
						err:                 fmt.Errorf("error in fetcher 1"),
					},
					&fakeFetcher{
						discoveryConfigName: "test1",
						count:               2,
					},
				},
			},
			want: map[string][]discoveryconfig.Status{
				"test1": {
					{
						State:               "DISCOVERY_CONFIG_STATE_ERROR",
						ErrorMessage:        stringPointer("error in fetcher 1"),
						DiscoveredResources: 2,
						LastSyncTime:        clock.Now(),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessPoint := newFakeAccessPoint()
			s := &Server{
				ctx: context.Background(),
				Config: &Config{
					AccessPoint: accessPoint,
					clock:       clock,
				},
				awsSyncStatus: awsSyncStatus{},
			}

			if tt.args.preRun {
				s.awsSyncStatus.iterationStarted(tt.args.fetchers, s.clock.Now())
			} else {
				s.awsSyncStatus.iterationFinished(tt.args.fetchers, tt.args.pushErr, s.clock.Now())
			}

			for _, discoveryConfigName := range s.awsSyncStatus.discoveryConfigs() {
				s.updateDiscoveryConfigStatus(discoveryConfigName)
			}

			require.Equal(t, tt.want, accessPoint.reports)
		})
	}
}

func stringPointer(s string) *string {
	return &s
}

type fakeFetcher struct {
	aws_sync.AWSSync
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
