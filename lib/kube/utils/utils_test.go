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

	"github.com/gravitational/teleport/api/types"
	"github.com/stretchr/testify/require"
)

func TestCheckOrSetKubeCluster(t *testing.T) {
	t.Parallel()
	ctx := context.TODO()

	tests := []struct {
		desc        string
		services    []types.Server
		kubeCluster string
		teleCluster string
		want        string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			desc: "valid cluster name",
			services: []types.Server{
				kubeService("k8s-1", "k8s-2"),
				kubeService("k8s-3", "k8s-4"),
			},
			kubeCluster: "k8s-4",
			teleCluster: "zzz-tele-cluster",
			want:        "k8s-4",
			assertErr:   require.NoError,
		},
		{
			desc: "invalid cluster name",
			services: []types.Server{
				kubeService("k8s-1", "k8s-2"),
				kubeService("k8s-3", "k8s-4"),
			},
			kubeCluster: "k8s-5",
			teleCluster: "zzz-tele-cluster",
			assertErr:   require.Error,
		},
		{
			desc:        "no registered clusters",
			services:    []types.Server{},
			kubeCluster: "k8s-1",
			teleCluster: "zzz-tele-cluster",
			assertErr:   require.Error,
		},
		{
			desc:        "no registered clusters and empty cluster provided",
			services:    []types.Server{},
			kubeCluster: "",
			teleCluster: "zzz-tele-cluster",
			assertErr:   require.Error,
		},
		{
			desc: "no cluster provided, default to first alphabetically",
			services: []types.Server{
				kubeService("k8s-1", "k8s-2"),
				kubeService("k8s-3", "k8s-4"),
			},
			kubeCluster: "",
			teleCluster: "zzz-tele-cluster",
			want:        "k8s-1",
			assertErr:   require.NoError,
		},
		{
			desc: "no cluster provided, default to teleport cluster name",
			services: []types.Server{
				kubeService("k8s-1", "k8s-2"),
				kubeService("k8s-3", "zzz-tele-cluster", "k8s-4"),
			},
			kubeCluster: "",
			teleCluster: "zzz-tele-cluster",
			want:        "zzz-tele-cluster",
			assertErr:   require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := CheckOrSetKubeCluster(ctx, mockKubeServicesPresence(tt.services), tt.kubeCluster, tt.teleCluster)
			tt.assertErr(t, err)
			require.Equal(t, got, tt.want)
		})
	}
}

type mockKubeServicesPresence []types.Server

func (p mockKubeServicesPresence) GetKubeServices(context.Context) ([]types.Server, error) {
	return p, nil
}

func kubeService(kubeClusters ...string) types.Server {
	var ks []*types.KubernetesCluster
	for _, kc := range kubeClusters {
		ks = append(ks, &types.KubernetesCluster{Name: kc})
	}
	return &types.ServerV2{
		Spec: types.ServerSpecV2{
			KubernetesClusters: ks,
		},
	}
}
