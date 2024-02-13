/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
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
