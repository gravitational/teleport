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
	"path"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
)

func Test_shouldUseKubeLocalProxy(t *testing.T) {
	t.Parallel()

	tshHome := t.TempDir()
	kubeconfigLocation := mustSetupKubeconfig(t, tshHome)

	tests := []struct {
		name             string
		inputProfile     *profile.Profile
		inputArgs        []string
		wantLocalProxy   bool
		wantKubeClusters kubeconfig.LocalProxyClusters
	}{
		{
			name:           "no profile yet",
			inputArgs:      []string{"kubectl", "--kubeconfig", kubeconfigLocation, "version"},
			wantLocalProxy: false,
		},
		{
			name: "ALPN conn upgrade not required",
			inputProfile: &profile.Profile{
				WebProxyAddr:  "localhost:443",
				KubeProxyAddr: "localhost:443",
			},
			inputArgs:      []string{"kubectl", "--kubeconfig", kubeconfigLocation, "version"},
			wantLocalProxy: false,
		},
		{
			name: "skip kubectl config",
			inputProfile: &profile.Profile{
				WebProxyAddr:                  "localhost:443",
				KubeProxyAddr:                 "localhost:443",
				TLSRoutingConnUpgradeRequired: true,
			},
			inputArgs:      []string{"kubectl", "--kubeconfig", kubeconfigLocation, "config"},
			wantLocalProxy: false,
		},
		{
			name: "no Teleport cluster selected",
			inputProfile: &profile.Profile{
				WebProxyAddr:                  "localhost:443",
				KubeProxyAddr:                 "localhost:443",
				TLSRoutingConnUpgradeRequired: true,
			},
			inputArgs:      []string{"kubectl", "--context=not-found", "--kubeconfig", kubeconfigLocation, "version"},
			wantLocalProxy: false,
		},
		{
			name: "use local proxy",
			inputProfile: &profile.Profile{
				WebProxyAddr:                  "localhost:443",
				KubeProxyAddr:                 "localhost:443",
				TLSRoutingConnUpgradeRequired: true,
			},
			inputArgs:      []string{"kubectl", "--kubeconfig", kubeconfigLocation, "version"},
			wantLocalProxy: true,
			wantKubeClusters: kubeconfig.LocalProxyClusters{{
				TeleportCluster: "localhost",
				KubeCluster:     "kube",
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ps := client.NewFSProfileStore(tshHome)
			if test.inputProfile != nil {
				err := ps.SaveProfile(test.inputProfile, true)
				require.NoError(t, err)
			}

			_, clusters, useLocalProxy := shouldUseKubeLocalProxy(&CLIConf{
				HomePath: tshHome,
				Proxy:    "localhost",
			}, test.inputArgs)
			require.ElementsMatch(t, test.wantKubeClusters, clusters)
			require.Equal(t, test.wantLocalProxy, useLocalProxy)
		})
	}
}

func Test_isKubectlConfigCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		inputArgs   []string
		checkResult require.BoolAssertionFunc
	}{
		{
			name:        "no args",
			inputArgs:   nil,
			checkResult: require.False,
		},
		{
			name:        "not kubectl",
			inputArgs:   []string{"aaa", "config"},
			checkResult: require.False,
		},
		{
			name:        "kubectl config",
			inputArgs:   []string{"kubectl", "--kubeconfig", "path", "config", "use-context", "some-context"},
			checkResult: require.True,
		},
		{
			name:        "kubectl get",
			inputArgs:   []string{"kubectl", "get", "pod", "-n", "config"},
			checkResult: require.False,
		},
	}

	command := makeKubectlCobraCommand()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checkResult(t, isKubectlConfigCommand(command, test.inputArgs))
		})
	}
}

func Test_extractKubeConfigAndContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputArgs      []string
		wantKubeconfig string
		wantContext    string
	}{
		{
			name:           "not enough args",
			inputArgs:      []string{"kubectl"},
			wantKubeconfig: "",
			wantContext:    "",
		},
		{
			name:           "args before subcommand",
			inputArgs:      []string{"kubectl", "--kubeconfig", "test-path", "--context=test-context", "get", "po", "--all-namespaces"},
			wantKubeconfig: "test-path",
			wantContext:    "test-context",
		},
		{
			name:        "args after subcommand",
			inputArgs:   []string{"kubectl", "get", "po", "--context", "test-context"},
			wantContext: "test-context",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualKubeconfig, actualContext := extractKubeConfigAndContext(test.inputArgs)
			require.Equal(t, test.wantKubeconfig, actualKubeconfig)
			require.Equal(t, test.wantContext, actualContext)
		})
	}
}

func Test_overwriteKubeconfigFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		inputPath  string
		inputArgs  []string
		expectArgs []string
	}{
		{
			name:       "no kubeconfig flag",
			inputPath:  "newpath",
			inputArgs:  []string{"kubectl", "get", "po"},
			expectArgs: []string{"kubectl", "get", "po"},
		},
		{
			name:       "kubeconfig flag",
			inputPath:  "newpath",
			inputArgs:  []string{"kubectl", "get", "po", "--kubeconfig", "oldpath"},
			expectArgs: []string{"kubectl", "get", "po", "--kubeconfig", "newpath"},
		},
		{
			name:       "kubeconfig equal flag",
			inputPath:  "newpath",
			inputArgs:  []string{"kubectl", "get", "po", "--kubeconfig=oldpath"},
			expectArgs: []string{"kubectl", "get", "po", "--kubeconfig=newpath"},
		},
	}

	for _, test := range tests {
		overwriteKubeconfigFlag(test.inputArgs, test.inputPath)
		require.Equal(t, test.expectArgs, test.inputArgs)
	}
}

func mustSetupKubeconfig(t *testing.T, tshHome string) string {
	kubeconfigLocation := path.Join(tshHome, "kubeconfig")
	priv, err := keys.ParsePrivateKey([]byte(fixtures.SSHCAPrivateKey))
	require.NoError(t, err)
	err = kubeconfig.Update(kubeconfigLocation, kubeconfig.Values{
		TeleportClusterName: "localhost",
		ClusterAddr:         "https://localhost:443",
		KubeClusters:        []string{"kube"},
		Credentials: &client.Key{
			PrivateKey: priv,
			TLSCert:    []byte(fixtures.TLSCACertPEM),
			TrustedCerts: []auth.TrustedCerts{{
				TLSCertificates: [][]byte{[]byte(fixtures.TLSCACertPEM)},
			}},
		},
		SelectCluster: "kube",
	}, false)
	require.NoError(t, err)
	return kubeconfigLocation
}
