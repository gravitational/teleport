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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/automaticupgrades"
)

func TestCheckKubeCluster(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kubeServers := []types.KubeServer{
		kubeServer(t, "k8s-1", "server1", "uuuid"),
		kubeServer(t, "k8s-2", "server1", "uuuid"),
		kubeServer(t, "k8s-3", "server1", "uuuid"),
		kubeServer(t, "k8s-4", "server1", "uuuid"),
	}

	tests := []struct {
		desc        string
		services    []types.KubeServer
		kubeCluster string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			desc:        "valid cluster name",
			services:    kubeServers,
			kubeCluster: "k8s-4",
			assertErr:   require.NoError,
		},
		{
			desc:        "invalid cluster name",
			services:    kubeServers,
			kubeCluster: "k8s-5",
			assertErr:   require.Error,
		},
		{
			desc:        "no registered clusters",
			services:    []types.KubeServer{},
			kubeCluster: "k8s-1",
			assertErr:   require.Error,
		},
		{
			desc:        "empty cluster provided",
			services:    kubeServers,
			kubeCluster: "",
			assertErr:   require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := CheckKubeCluster(ctx, mockKubeServicesPresence(tt.services), tt.kubeCluster)
			tt.assertErr(t, err)
		})
	}
}

type mockKubeServicesPresence []types.KubeServer

func (p mockKubeServicesPresence) GetKubernetesServers(context.Context) ([]types.KubeServer, error) {
	return p, nil
}

func kubeServer(t *testing.T, kubeCluster, hostname, hostID string) types.KubeServer {
	cluster, err := types.NewKubernetesClusterV3(types.Metadata{Name: kubeCluster}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	server, err := types.NewKubernetesServerV3FromCluster(cluster, hostname, hostID)
	require.NoError(t, err)
	return server
}

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

			result, err := GetKubeAgentVersion(ctx, p, tt.clusterFeatures, channel)

			tt.errorAssert(t, err)
			require.Equal(t, tt.expectedVersion, result)
		})
	}
}

type pinger struct {
	pingFn func(ctx context.Context) (proto.PingResponse, error)
}

func (p *pinger) Ping(ctx context.Context) (proto.PingResponse, error) {
	return p.pingFn(ctx)
}

func TestExtractAndSortKubeClusterNames(t *testing.T) {
	t.Parallel()

	server1 := kubeServer(t, "watermelon", "server1", "uuuid")

	server2 := kubeServer(t, "watermelon", "server1", "uuuid")

	server3 := kubeServer(t, "banana", "server2", "uuuid2")

	server4 := kubeServer(t, "apple", "server2", "uuuid2")

	server5 := kubeServer(t, "pear", "server2", "uuuid2")

	names := extractAndSortKubeClusterNames(types.KubeServers{server1, server2, server3, server4, server5})
	require.Equal(t, []string{"apple", "banana", "pear", "watermelon"}, names)
}
