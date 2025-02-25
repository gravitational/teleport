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
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
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
			var loadedKubeClusters kubeconfig.LocalProxyClusters
			wantLocalProxyKubeConfigLocation := filepath.Join(tshHome, uuid.NewString())

			closeFn, actualKubeConfigLocation, err := maybeStartKubeLocalProxy(cf,
				withKubectlArgs(test.inputArgs),
				func(o *kubeLocalProxyOpts) {
					// Fake makeAndStartKubeLocalProxyFunc instead of making a
					// real kube local proxy.
					o.makeAndStartKubeLocalProxyFunc = func(_ *CLIConf, _ *clientcmdapi.Config, clusters kubeconfig.LocalProxyClusters) (func(), string, error) {
						localProxyCreated = true
						loadedKubeClusters = clusters
						return func() {}, wantLocalProxyKubeConfigLocation, nil
					}
				},
			)
			require.NoError(t, err)
			defer closeFn()

			require.Equal(t, test.wantLocalProxy, localProxyCreated)
			if test.wantLocalProxy {
				require.ElementsMatch(t, test.wantKubeClusters, loadedKubeClusters)
				require.Equal(t, wantLocalProxyKubeConfigLocation, actualKubeConfigLocation)
			}
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
			inputArgs:   []string{"kubectl", "get", "po", "-A", "--context", "test-context"},
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
			inputArgs:  []string{"kubectl", "--kubeconfig", "oldpath", "version"},
			expectArgs: []string{"kubectl", "--kubeconfig", "newpath", "version"},
		},
		{
			name:       "kubeconfig equal flag",
			inputPath:  "newpath",
			inputArgs:  []string{"--debug", "kubectl", "version", "--kubeconfig=oldpath"},
			expectArgs: []string{"--debug", "kubectl", "version", "--kubeconfig=newpath"},
		},
	}

	for _, test := range tests {
		outputArgs := overwriteKubeconfigFlagInArgs(test.inputArgs, test.inputPath)
		require.Equal(t, test.expectArgs, outputArgs)
	}
}

func Test_overwriteKubeconfigFlagInEnv(t *testing.T) {
	t.Parallel()

	inputEnv := []string{
		"foo=bar",
		"KUBECONFIG=old-path",
		"bar=foo",
	}
	wantEnv := []string{
		"foo=bar",
		"bar=foo",
		"KUBECONFIG=new-path",
	}
	require.Equal(t, wantEnv, overwriteKubeconfigInEnv(inputEnv, "new-path"))
}

func mustSetupKubeconfig(t *testing.T, tshHome, kubeCluster string) string {
	kubeconfigLocation := filepath.Join(tshHome, "kubeconfig")
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	priv, err := keys.NewSoftwarePrivateKey(key)
	require.NoError(t, err)
	err = kubeconfig.Update(kubeconfigLocation, kubeconfig.Values{
		TeleportClusterName: "localhost",
		ClusterAddr:         "https://localhost:443",
		KubeClusters:        []string{kubeCluster},
		Credentials: &client.KeyRing{
			TLSPrivateKey: priv,
			TLSCert:       []byte(fixtures.TLSCACertPEM),
			TrustedCerts: []authclient.TrustedCerts{{
				TLSCertificates: [][]byte{[]byte(fixtures.TLSCACertPEM)},
			}},
		},
		SelectCluster: kubeCluster,
	}, false)
	require.NoError(t, err)
	return kubeconfigLocation
}
