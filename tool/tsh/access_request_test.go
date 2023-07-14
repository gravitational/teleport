/*
Copyright 2023 Gravitational, Inc.

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

package main

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAccessRequestSearch(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Kubernetes: true,
		},
	},
	)
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })
	ctx := context.Background()
	const (
		rootClusterName = "root-cluster"
		rootKubeCluster = "first-cluster"
		leafClusterName = "leaf-cluster"
		leafKubeCluster = "second-cluster"
	)
	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.ClusterName.SetClusterName(rootClusterName)
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newKubeConfigFile(t, rootKubeCluster)
		}),
		withLeafCluster(),
		withLeafConfigFunc(
			func(cfg *servicecfg.Config) {
				cfg.Auth.ClusterName.SetClusterName(leafClusterName)
				cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				cfg.Kube.Enabled = true
				cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
				cfg.Kube.KubeconfigPath = newKubeConfigFile(t, leafKubeCluster)
			},
		),
		withValidationFunc(func(s *suite) bool {
			rootClusters, err := s.root.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			leafClusters, err := s.leaf.GetAuthServer().GetKubernetesServers(ctx)
			require.NoError(t, err)
			return len(rootClusters) >= 1 && len(leafClusters) >= 1
		}),
	)

	// We create another kube_server in the leaf cluster to test that we can
	// deduplicate the results when multiple replicas of the same resource
	// exist.
	kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name: leafKubeCluster,
	}, types.KubernetesClusterSpecV3{})
	require.NoError(t, err)
	kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, "host", uuid.New().String())
	require.NoError(t, err)
	_, err = s.leaf.GetAuthServer().UpsertKubernetesServer(ctx, kubeServer)
	require.NoError(t, err)

	type args struct {
		teleportCluster string
		kind            string
		extraArgs       []string
	}
	tests := []struct {
		name      string
		args      args
		wantTable func() string
	}{
		{
			name: "list pods in root cluster for default namespace",
			args: args{
				teleportCluster: rootClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", rootKubeCluster)},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "default", "", fmt.Sprintf("/%s/pod/%s/default/nginx-1", rootClusterName, rootKubeCluster)},
						{"nginx-2", "default", "", fmt.Sprintf("/%s/pod/%s/default/nginx-2", rootClusterName, rootKubeCluster)},
						{"test", "default", "", fmt.Sprintf("/%s/pod/%s/default/test", rootClusterName, rootKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list pods in root cluster for dev namespace with search",
			args: args{
				teleportCluster: rootClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", rootKubeCluster), "--kube-namespace=dev", "--search=nginx-1"},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "dev", "", fmt.Sprintf("/%s/pod/%s/dev/nginx-1", rootClusterName, rootKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list pods in leaf cluster for default namespace",
			args: args{
				teleportCluster: leafClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", leafKubeCluster)},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "default", "", fmt.Sprintf("/%s/pod/%s/default/nginx-1", leafClusterName, leafKubeCluster)},
						{"nginx-2", "default", "", fmt.Sprintf("/%s/pod/%s/default/nginx-2", leafClusterName, leafKubeCluster)},
						{"test", "default", "", fmt.Sprintf("/%s/pod/%s/default/test", leafClusterName, leafKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list pods in leaf cluster for all namespaces",
			args: args{
				teleportCluster: leafClusterName,
				extraArgs:       []string{fmt.Sprintf("--kube-cluster=%v", leafKubeCluster), "--all-kube-namespaces", "--search=nginx-1"},
				kind:            types.KindKubePod,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Namespace", "Labels", "Resource ID"},
					[][]string{
						{"nginx-1", "default", "", fmt.Sprintf("/%s/pod/%s/default/nginx-1", leafClusterName, leafKubeCluster)},
						{"nginx-1", "dev", "", fmt.Sprintf("/%s/pod/%s/dev/nginx-1", leafClusterName, leafKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "list kube clusters in leaf cluster",
			args: args{
				teleportCluster: leafClusterName,
				kind:            types.KindKubernetesCluster,
			},
			wantTable: func() string {
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Name", "Hostname", "Labels", "Resource ID"},
					[][]string{
						{leafKubeCluster, "", "", fmt.Sprintf("/%s/kube_cluster/%s", leafClusterName, leafKubeCluster)},
					},
					"Labels")
				return table.AsBuffer().String()
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			homePath, _ := mustLogin(t, s, tc.args.teleportCluster)
			captureStdout := new(bytes.Buffer)
			err := Run(
				context.Background(),
				append([]string{
					"--insecure",
					"request",
					"search",
					fmt.Sprintf("--kind=%s", tc.args.kind),
				},
					tc.args.extraArgs...,
				),
				setCopyStdout(captureStdout),
				setHomePath(homePath),
			)
			require.NoError(t, err)
			// We append a newline to the expected output to esnure that the table
			// does not contain any more rows than expected.
			require.Contains(t, captureStdout.String(), tc.wantTable()+"\n")
		})
	}
}
