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

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
)

func Test_maybeStartKubeLocalProxy(t *testing.T) {
	t.Parallel()

	tshHome := t.TempDir()
	kubeCluster := "kube1"
	kubeconfigLocation := mustSetupKubeconfig(t, tshHome, kubeCluster)

	tests := []struct {
		name             string
		inputProfile     *profile.Profile
		inputArgs        []string
		wantLocalProxy   bool
		wantKubeClusters kubeconfig.LocalProxyClusters
	}{
		{
			name:           "no profile",
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
				KubeCluster:     kubeCluster,
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.inputProfile != nil {
				err := client.NewFSProfileStore(tshHome).SaveProfile(test.inputProfile, true)
				require.NoError(t, err)
			}

			cf := &CLIConf{
				HomePath: tshHome,
				Proxy:    "localhost",
			}

			var localProxyCreated bool
			kubeconfigLocationForLocalProxy := path.Join(tshHome, uuid.NewString())

			// Fake makeAndStartKubeLocalProxyFunc instead of making a real
			// kube local proxy. Verify the loaded kube cluster is correct.
			verifyKubeCluster := func(o *kubeLocalProxyOpts) {
				o.makeAndStartKubeLocalProxyFunc = func(_ *CLIConf, _ *clientcmdapi.Config, clusters kubeconfig.LocalProxyClusters) (func(), string, error) {
					localProxyCreated = true
					require.True(t, test.wantLocalProxy, "makeAndStartKubeLocalProxy should only be called if local proxy is required.")
					require.ElementsMatch(t, test.wantKubeClusters, clusters)
					return func() {}, kubeconfigLocationForLocalProxy, nil
				}
			}
			// Fake os.Setenv and verify the env value.
			verifyEnv := func(o *kubeLocalProxyOpts) {
				o.setEnvFunc = func(key, value string) error {
					require.Equal(t, "KUBECONFIG", key)
					require.Equal(t, kubeconfigLocationForLocalProxy, value)
					return nil
				}
			}

			closeFn, err := maybeStartKubeLocalProxy(cf,
				withKubectlArgs(test.inputArgs),
				verifyKubeCluster,
				verifyEnv,
			)
			require.NoError(t, err)
			defer closeFn()

			// Make sure makeAndStartKubeLocalProxyFunc is called if local proxy is required.
			require.Equal(t, test.wantLocalProxy, localProxyCreated)
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
			name:        "kubectl get pod",
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

func Test_overwriteKubeconfigFlagInArgs(t *testing.T) {
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
			inputArgs:  []string{"kubectl", "version"},
			expectArgs: []string{"kubectl", "version"},
		},
		{
			name:       "kubeconfig flag",
			inputPath:  "newpath",
			inputArgs:  []string{"kubectl", "version", "--kubeconfig", "oldpath"},
			expectArgs: []string{"kubectl", "version", "--kubeconfig", "newpath"},
		},
		{
			name:       "kubeconfig equal flag",
			inputPath:  "newpath",
			inputArgs:  []string{"kubectl", "version", "--kubeconfig=oldpath"},
			expectArgs: []string{"kubectl", "version", "--kubeconfig=newpath"},
		},
	}

	for _, test := range tests {
		overwriteKubeconfigFlagInArgs(test.inputArgs, test.inputPath)
		require.Equal(t, test.expectArgs, test.inputArgs)
	}
}

func mustSetupKubeconfig(t *testing.T, tshHome, kubeCluster string) string {
	kubeconfigLocation := path.Join(tshHome, "kubeconfig")
	priv, err := keys.ParsePrivateKey([]byte(fixtures.SSHCAPrivateKey))
	require.NoError(t, err)
	err = kubeconfig.Update(kubeconfigLocation, kubeconfig.Values{
		TeleportClusterName: "localhost",
		ClusterAddr:         "https://localhost:443",
		KubeClusters:        []string{kubeCluster},
		Credentials: &client.Key{
			PrivateKey: priv,
			TLSCert:    []byte(fixtures.TLSCACertPEM),
			TrustedCerts: []auth.TrustedCerts{{
				TLSCertificates: [][]byte{[]byte(fixtures.TLSCACertPEM)},
			}},
		},
		SelectCluster: kubeCluster,
	}, false)
	require.NoError(t, err)
	return kubeconfigLocation
}
