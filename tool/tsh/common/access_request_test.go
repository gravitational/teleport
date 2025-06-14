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

package common

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/entitlements"
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
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.K8s: {Enabled: true},
			},
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
						{"nginx-1", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-1", rootClusterName, rootKubeCluster)},
						{"nginx-2", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-2", rootClusterName, rootKubeCluster)},
						{"test", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/test", rootClusterName, rootKubeCluster)},
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
						{"nginx-1", "dev", "", fmt.Sprintf("/%s/kube:ns:pods/%s/dev/nginx-1", rootClusterName, rootKubeCluster)},
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
						{"nginx-1", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-1", leafClusterName, leafKubeCluster)},
						{"nginx-2", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-2", leafClusterName, leafKubeCluster)},
						{"test", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/test", leafClusterName, leafKubeCluster)},
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
						{"nginx-1", "default", "", fmt.Sprintf("/%s/kube:ns:pods/%s/default/nginx-1", leafClusterName, leafKubeCluster)},
						{"nginx-1", "dev", "", fmt.Sprintf("/%s/kube:ns:pods/%s/dev/nginx-1", leafClusterName, leafKubeCluster)},
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			homePath, _ := mustLoginLegacy(t, s, tc.args.teleportCluster)
			captureStdout := new(bytes.Buffer)
			err := Run(
				ctx,
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

func TestShowRequestTable(t *testing.T) {
	createdAtTime := time.Now()
	expiresTime := time.Now().Add(time.Hour)
	assumeStartTime := createdAtTime.Add(30 * time.Minute)
	tests := []struct {
		name        string
		reqs        []types.AccessRequest
		wantPresent []string
	}{
		{
			name: "Access Requests without assume time",
			reqs: []types.AccessRequest{
				&types.AccessRequestV3{
					Metadata: types.Metadata{
						Name:    "someName",
						Expires: &expiresTime,
					},
					Spec: types.AccessRequestSpecV3{
						User:  "someUser",
						Roles: []string{"role1", "role2"},

						Expires:    expiresTime,
						SessionTTL: expiresTime,
						Created:    createdAtTime,
					},
				},
			},
			wantPresent: []string{"someName", "someUser", "role1,role2"},
		},
		{
			name: "Access Requests with assume time",
			reqs: []types.AccessRequest{
				&types.AccessRequestV3{
					Metadata: types.Metadata{
						Name:    "someName",
						Expires: &expiresTime,
					},
					Spec: types.AccessRequestSpecV3{
						User:  "someUser",
						Roles: []string{"role1", "role2"},

						Expires:         expiresTime,
						SessionTTL:      expiresTime,
						AssumeStartTime: &assumeStartTime,
						Created:         createdAtTime,
					},
				},
			},
			wantPresent: []string{"someName", "someUser", "role1,role2", assumeStartTime.UTC().Format(time.RFC822)},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			captureStdout := new(bytes.Buffer)
			cf := &CLIConf{
				OverrideStdout: captureStdout,
			}
			err := showRequestTable(cf, tc.reqs)
			require.NoError(t, err)
			for _, wanted := range tc.wantPresent {
				require.Contains(t, captureStdout.String(), wanted)
			}
		})
	}
}
