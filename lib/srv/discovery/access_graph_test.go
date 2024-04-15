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
	"fmt"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
	aws_sync "github.com/gravitational/teleport/lib/srv/discovery/fetchers/aws-sync"
)

func TestServer_updateDiscoveryConfigStatus(t *testing.T) {
	var testErr = "test error"
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
				pushErr: fmt.Errorf(testErr),
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
			name: "test updateDiscoveryConfigStatus with errpr",
			args: args{
				fetchers: []aws_sync.AWSSync{
					&fakeFetcher{
						count:               1,
						discoveryConfigName: "test",
						err:                 fmt.Errorf(testErr),
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
						count:               1,
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
						DiscoveredResources: 1,
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
			}
			s.updateDiscoveryConfigStatus(tt.args.fetchers, tt.args.pushErr, tt.args.preRun)
			require.Equal(t, tt.want, accessPoint.reports)
		})
	}
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
