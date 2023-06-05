/*
Copyright 2022 Gravitational, Inc.

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
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/asciitable"
	kubeserver "github.com/gravitational/teleport/lib/kube/proxy/testing/kube_server"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestKube(t *testing.T) {
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	pack := setupKubeTestPack(t)
	t.Run("list kube", pack.testListKube)
	t.Run("proxy kube", pack.testProxyKube)
}

type kubeTestPack struct {
	*suite

	rootClusterName  string
	leafClusterName  string
	rootKubeCluster1 string
	rootKubeCluster2 string
	leafKubeCluster  string
	serviceLabels    map[string]string
	formatedLabels   string
}

func setupKubeTestPack(t *testing.T) *kubeTestPack {
	t.Helper()

	ctx := context.Background()
	rootKubeCluster1 := "root-cluster"
	rootKubeCluster2 := "first-cluster"
	leafKubeCluster := "leaf-cluster"
	serviceLabels := map[string]string{
		"label1": "val1",
		"ultra_long_label_for_teleport_kubernetes_service_list_kube_clusters_method": "ultra_long_label_value_for_teleport_kubernetes_service_list_kube_clusters_method",
	}
	formatedLabels := formatServiceLabels(serviceLabels)

	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Kube.Enabled = true
			cfg.Kube.ListenAddr = utils.MustParseAddr(localListenerAddr())
			cfg.Kube.KubeconfigPath = newKubeConfigFile(t, rootKubeCluster1, rootKubeCluster2)
			cfg.Kube.StaticLabels = serviceLabels
		}),
		withLeafCluster(),
		withLeafConfigFunc(
			func(cfg *servicecfg.Config) {
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
			return len(rootClusters) >= 2 && len(leafClusters) >= 1
		}),
	)

	mustLoginSetEnv(t, s)
	return &kubeTestPack{
		suite:            s,
		rootClusterName:  s.root.Config.Auth.ClusterName.GetClusterName(),
		leafClusterName:  s.leaf.Config.Auth.ClusterName.GetClusterName(),
		rootKubeCluster1: rootKubeCluster1,
		rootKubeCluster2: rootKubeCluster2,
		leafKubeCluster:  leafKubeCluster,
		serviceLabels:    serviceLabels,
		formatedLabels:   formatedLabels,
	}
}

func (p *kubeTestPack) testListKube(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantTable func() string
	}{
		{
			name: "default mode with truncated table",
			args: nil,
			wantTable: func() string {
				// p.rootKubeCluster2 ("first-cluster") should appear before
				// p.rootKubeCluster1 ("root-cluster") after sorting.
				table := asciitable.MakeTableWithTruncatedColumn(
					[]string{"Kube Cluster Name", "Labels", "Selected"},
					[][]string{{p.rootKubeCluster2, p.formatedLabels, ""}, {p.rootKubeCluster1, p.formatedLabels, ""}},
					"Labels")
				return table.AsBuffer().String()
			},
		},
		{
			name: "show complete list of labels",
			args: []string{"--verbose"},
			wantTable: func() string {
				table := asciitable.MakeTable(
					[]string{"Kube Cluster Name", "Labels", "Selected"},
					[]string{p.rootKubeCluster2, p.formatedLabels, ""},
					[]string{p.rootKubeCluster1, p.formatedLabels, ""})
				return table.AsBuffer().String()
			},
		},
		{
			name: "show headless table",
			args: []string{"--quiet"},
			wantTable: func() string {
				table := asciitable.MakeHeadlessTable(2)
				table.AddRow([]string{p.rootKubeCluster2, p.formatedLabels, ""})
				table.AddRow([]string{p.rootKubeCluster1, p.formatedLabels, ""})

				return table.AsBuffer().String()
			},
		},
		{
			name: "list all clusters including leaf clusters",
			args: []string{"--all"},
			wantTable: func() string {
				table := asciitable.MakeTable(
					[]string{"Proxy", "Cluster", "Kube Cluster Name", "Labels"},

					[]string{p.root.Config.Proxy.WebAddr.String(), "leaf1", p.leafKubeCluster, ""},
					[]string{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster2, p.formatedLabels},
					[]string{p.root.Config.Proxy.WebAddr.String(), "root", p.rootKubeCluster1, p.formatedLabels},
				)
				return table.AsBuffer().String()
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			captureStdout := new(bytes.Buffer)
			err := Run(
				context.Background(),
				append([]string{
					"--insecure",
					"kube",
					"ls",
				},
					tc.args...,
				),
				setCopyStdout(captureStdout),
			)
			require.NoError(t, err)
			require.Contains(t, captureStdout.String(), tc.wantTable())
		})
	}
}

func newKubeConfigFile(t *testing.T, clusterNames ...string) string {
	tmpDir := t.TempDir()

	kubeConf := clientcmdapi.NewConfig()
	for _, name := range clusterNames {
		kubeConf.Clusters[name] = &clientcmdapi.Cluster{
			Server:                newKubeSelfSubjectServer(t),
			InsecureSkipTLSVerify: true,
		}
		kubeConf.AuthInfos[name] = &clientcmdapi.AuthInfo{}

		kubeConf.Contexts[name] = &clientcmdapi.Context{
			Cluster:  name,
			AuthInfo: name,
		}
	}
	kubeConfigLocation := filepath.Join(tmpDir, "kubeconfig")
	err := clientcmd.WriteToFile(*kubeConf, kubeConfigLocation)
	require.NoError(t, err)
	return kubeConfigLocation
}

func formatServiceLabels(labels map[string]string) string {
	labelSlice := make([]string, 0, len(labels))
	for key, value := range labels {
		labelSlice = append(labelSlice, fmt.Sprintf("%s=%s", key, value))
	}

	sort.Strings(labelSlice)
	return strings.Join(labelSlice, " ")
}

func newKubeSelfSubjectServer(t *testing.T) string {
	srv, err := kubeserver.NewKubeAPIMock()
	require.NoError(t, err)
	t.Cleanup(func() { srv.Close() })

	return srv.URL
}
