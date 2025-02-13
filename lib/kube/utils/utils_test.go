/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/automaticupgrades"
)

func TestGetAgentVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := []struct {
		desc            string
		ping            func(ctx context.Context) (proto.PingResponse, error)
		clusterFeatures proto.Features
		channelVersion  string
		expectedVersion string
		errorAssert     require.ErrorAssertionFunc
	}{
		{
			desc: "ping error",
			ping: func(ctx context.Context) (proto.PingResponse, error) {
				return proto.PingResponse{}, trace.BadParameter("ping error")
			},
			expectedVersion: "",
			errorAssert:     require.Error,
		},
		{
			desc: "no automatic upgrades",
			ping: func(ctx context.Context) (proto.PingResponse, error) {
				return proto.PingResponse{ServerVersion: "1.2.3"}, nil
			},
			expectedVersion: "1.2.3",
			errorAssert:     require.NoError,
		},
		{
			desc: "automatic upgrades",
			ping: func(ctx context.Context) (proto.PingResponse, error) {
				return proto.PingResponse{ServerVersion: "10"}, nil
			},
			clusterFeatures: proto.Features{AutomaticUpgrades: true, Cloud: true},
			channelVersion:  "v1.2.3",
			expectedVersion: "1.2.3",
			errorAssert:     require.NoError,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			p := &pinger{pingFn: tt.ping}

			var channel *automaticupgrades.Channel
			if tt.channelVersion != "" {
				channel = &automaticupgrades.Channel{StaticVersion: tt.channelVersion}
				err := channel.CheckAndSetDefaults()
				require.NoError(t, err)
			}
			releaseChannels := automaticupgrades.Channels{automaticupgrades.DefaultChannelName: channel}

			version, err := GetKubeAgentVersion(ctx, p, tt.clusterFeatures, releaseChannels)

			tt.errorAssert(t, err)
			require.Equal(t, tt.expectedVersion, version)
		})
	}
}

type pinger struct {
	pingFn func(ctx context.Context) (proto.PingResponse, error)
}

func (p *pinger) Ping(ctx context.Context) (proto.PingResponse, error) {
	return p.pingFn(ctx)
}
