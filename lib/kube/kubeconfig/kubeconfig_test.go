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

package kubeconfig

import (
	"bytes"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
)

func setup(t *testing.T) (string, clientcmdapi.Config) {
	f, err := os.CreateTemp("", "kubeconfig")
	if err != nil {
		t.Fatalf("failed to create temp kubeconfig file: %v", err)
	}
	defer f.Close()

	// Note: LocationOfOrigin and Extensions would be automatically added on
	// clientcmd.Write below. Set them explicitly so we can compare
	// initialConfig against loaded config.
	//
	// TODO: use a comparison library that can ignore individual fields.
	kubeconfigPath := f.Name()
	t.Cleanup(func() { os.Remove(kubeconfigPath) })

	initialConfig := clientcmdapi.Config{
		CurrentContext: "dev",
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster-1": {
				CertificateAuthority: "fake-ca-file",
				Server:               "https://1.2.3.4",
				LocationOfOrigin:     f.Name(),
				Extensions:           map[string]runtime.Object{},
			},
			"cluster-2": {
				InsecureSkipTLSVerify: true,
				Server:                "https://1.2.3.5",
				LocationOfOrigin:      f.Name(),
				Extensions:            map[string]runtime.Object{},
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"developer": {
				ClientCertificate: "fake-client-cert",
				ClientKey:         "fake-client-key",
				LocationOfOrigin:  f.Name(),
				Extensions:        map[string]runtime.Object{},
			},
			"admin": {
				Username:         "admin",
				Password:         "hunter1",
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
			"support": {
				Exec: &clientcmdapi.ExecConfig{
					Command: "/bin/get_creds",
					Args:    []string{"--role=support"},
				},
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"dev": {
				Cluster:          "cluster-2",
				AuthInfo:         "developer",
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
			"prod": {
				Cluster:          "cluster-1",
				AuthInfo:         "admin",
				LocationOfOrigin: f.Name(),
				Extensions:       map[string]runtime.Object{},
			},
		},
		Preferences: clientcmdapi.Preferences{
			Extensions: map[string]runtime.Object{},
		},
		Extensions: map[string]runtime.Object{},
	}

	initialContent, err := clientcmd.Write(initialConfig)
	require.NoError(t, err)

	if _, err := f.Write(initialContent); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}

	return kubeconfigPath, initialConfig
}

func TestLoad(t *testing.T) {
	kubeconfigPath, initialConfig := setup(t)
	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	require.Equal(t, initialConfig, *config)
}

func TestSave(t *testing.T) {
	kubeconfigPath, _ := setup(t)
	cfg := clientcmdapi.Config{
		CurrentContext: "a",
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				CertificateAuthority: "fake-ca-file",
				Server:               "https://1.2.3.4",
				LocationOfOrigin:     kubeconfigPath,
				Extensions:           map[string]runtime.Object{},
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				LocationOfOrigin: kubeconfigPath,
				Extensions:       map[string]runtime.Object{},
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"a": {
				Cluster:          "cluster",
				AuthInfo:         "user",
				LocationOfOrigin: kubeconfigPath,
				Extensions:       map[string]runtime.Object{},
			},
		},
		Preferences: clientcmdapi.Preferences{
			Extensions: map[string]runtime.Object{},
		},
		Extensions: map[string]runtime.Object{},
	}

	err := Save(kubeconfigPath, cfg)
	require.NoError(t, err)

	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	require.Equal(t, cfg, *config)
}

func TestUpdate(t *testing.T) {
	const (
		clusterName = "teleport-cluster"
		clusterAddr = "https://1.2.3.6:3080"
	)
	kubeconfigPath, initialConfig := setup(t)
	creds, caCertPEM, err := genUserKeyRing("localhost")
	require.NoError(t, err)
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	}, false)
	require.NoError(t, err)

	wantConfig := initialConfig.DeepCopy()
	wantConfig.Contexts[wantConfig.CurrentContext].Extensions = map[string]runtime.Object{
		selectedExtension: nil,
	}
	wantConfig.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   clusterAddr,
		CertificateAuthorityData: caCertPEM,
		LocationOfOrigin:         kubeconfigPath,
		Extensions:               map[string]runtime.Object{},
	}
	wantConfig.AuthInfos[clusterName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: creds.TLSCert,
		ClientKeyData:         creds.TLSPrivateKey.PrivateKeyPEM(),
		LocationOfOrigin:      kubeconfigPath,
		Extensions:            map[string]runtime.Object{},
	}
	wantConfig.Contexts[clusterName] = &clientcmdapi.Context{
		Cluster:          clusterName,
		AuthInfo:         clusterName,
		LocationOfOrigin: kubeconfigPath,
		Extensions:       map[string]runtime.Object{},
	}
	wantConfig.CurrentContext = clusterName

	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	require.Equal(t, wantConfig, config)
}

func TestUpdateWithExec(t *testing.T) {
	const (
		clusterName = "teleport-cluster"
		clusterAddr = "https://1.2.3.6:3080"
		tshPath     = "/path/to/tsh"
		kubeCluster = "my-cluster"
		homeEnvVar  = "TELEPORT_HOME"
		home        = "/alt/home"
		namespace   = "kubeNamespace"
	)

	creds, caCertPEM, err := genUserKeyRing("localhost")
	require.NoError(t, err)

	tests := []struct {
		name                string
		namespace           string
		impersonatedUser    string
		impersonatedGroups  []string
		overrideContextName string
	}{
		{
			name:               "config with namespace selection",
			impersonatedUser:   "",
			impersonatedGroups: nil,
			namespace:          namespace,
		},
		{
			name:               "config without impersonation",
			impersonatedUser:   "",
			impersonatedGroups: nil,
		},
		{
			name:               "config with user impersonation",
			impersonatedUser:   "user1",
			impersonatedGroups: nil,
		},
		{
			name:               "config with group impersonation",
			impersonatedUser:   "",
			impersonatedGroups: []string{"group1", "group2"},
		},
		{
			name:               "config with user and group impersonation",
			impersonatedUser:   "user",
			impersonatedGroups: []string{"group1", "group2"},
		},
		{
			name:                "config with custom context name",
			impersonatedUser:    "",
			impersonatedGroups:  nil,
			namespace:           namespace,
			overrideContextName: "custom-context-name",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeconfigPath, initialConfig := setup(t)
			err = Update(kubeconfigPath, Values{
				TeleportClusterName: clusterName,
				ClusterAddr:         clusterAddr,
				Credentials:         creds,
				Impersonate:         tt.impersonatedUser,
				ImpersonateGroups:   tt.impersonatedGroups,
				Namespace:           tt.namespace,
				KubeClusters:        []string{kubeCluster},
				Exec: &ExecValues{
					TshBinaryPath: tshPath,
					Env: map[string]string{
						homeEnvVar: home,
					},
				},
				OverrideContext: tt.overrideContextName,
			}, false)
			require.NoError(t, err)

			wantConfig := initialConfig.DeepCopy()
			contextName := ContextName(clusterName, kubeCluster)
			authInfoName := contextName
			if tt.overrideContextName != "" {
				contextName = tt.overrideContextName
			}
			wantConfig.Clusters[clusterName] = &clientcmdapi.Cluster{
				Server:                   clusterAddr,
				CertificateAuthorityData: caCertPEM,
				LocationOfOrigin:         kubeconfigPath,
				Extensions:               map[string]runtime.Object{},
			}
			wantConfig.AuthInfos[authInfoName] = &clientcmdapi.AuthInfo{
				LocationOfOrigin:  kubeconfigPath,
				Extensions:        map[string]runtime.Object{},
				Impersonate:       tt.impersonatedUser,
				ImpersonateGroups: tt.impersonatedGroups,
				Exec: &clientcmdapi.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1beta1",
					Command:    tshPath,
					Args: []string{
						"kube", "credentials",
						fmt.Sprintf("--kube-cluster=%s", kubeCluster),
						fmt.Sprintf("--teleport-cluster=%s", clusterName),
					},
					Env:             []clientcmdapi.ExecEnvVar{{Name: homeEnvVar, Value: home}},
					InteractiveMode: clientcmdapi.IfAvailableExecInteractiveMode,
				},
			}
			wantConfig.Contexts[contextName] = &clientcmdapi.Context{
				Cluster:          clusterName,
				AuthInfo:         authInfoName,
				LocationOfOrigin: kubeconfigPath,
				Extensions: map[string]runtime.Object{
					teleportKubeClusterNameExtension: &runtime.Unknown{
						Raw:         []byte(fmt.Sprintf("%q", kubeCluster)),
						ContentType: "application/json",
					},
				},
				Namespace: tt.namespace,
			}
			config, err := Load(kubeconfigPath)
			require.NoError(t, err)
			require.Equal(t, wantConfig, config)
		},
		)
	}
}

func TestUpdateWithExecAndProxy(t *testing.T) {
	const (
		clusterName = "teleport-cluster"
		clusterAddr = "https://1.2.3.6:3080"
		proxy       = "my-teleport-proxy:3080"
		tshPath     = "/path/to/tsh"
		kubeCluster = "my-cluster"
		homeEnvVar  = "TELEPORT_HOME"
		home        = "/alt/home"
	)
	kubeconfigPath, initialConfig := setup(t)
	creds, caCertPEM, err := genUserKeyRing("localhost")
	require.NoError(t, err)
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
		ProxyAddr:           proxy,
		KubeClusters:        []string{kubeCluster},
		Exec: &ExecValues{
			TshBinaryPath: tshPath,
			Env: map[string]string{
				homeEnvVar: home,
			},
		},
	}, false)
	require.NoError(t, err)

	wantConfig := initialConfig.DeepCopy()
	contextName := ContextName(clusterName, kubeCluster)
	wantConfig.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   clusterAddr,
		CertificateAuthorityData: caCertPEM,
		LocationOfOrigin:         kubeconfigPath,
		Extensions:               map[string]runtime.Object{},
	}
	wantConfig.AuthInfos[contextName] = &clientcmdapi.AuthInfo{
		LocationOfOrigin: kubeconfigPath,
		Extensions:       map[string]runtime.Object{},
		Exec: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Command:    tshPath,
			Args: []string{
				"kube", "credentials",
				fmt.Sprintf("--kube-cluster=%s", kubeCluster),
				fmt.Sprintf("--teleport-cluster=%s", clusterName),
				fmt.Sprintf("--proxy=%s", proxy),
			},
			Env:             []clientcmdapi.ExecEnvVar{{Name: homeEnvVar, Value: home}},
			InteractiveMode: clientcmdapi.IfAvailableExecInteractiveMode,
		},
	}
	wantConfig.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:          clusterName,
		AuthInfo:         contextName,
		LocationOfOrigin: kubeconfigPath,
		Extensions: map[string]runtime.Object{
			teleportKubeClusterNameExtension: &runtime.Unknown{
				Raw:         []byte(fmt.Sprintf("%q", kubeCluster)),
				ContentType: "application/json",
			},
		},
	}

	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	require.Equal(t, wantConfig, config)
}

func TestUpdateLoadAllCAs(t *testing.T) {
	const (
		clusterName     = "teleport-cluster"
		leafClusterName = "leaf-teleport-cluster"
		clusterAddr     = "https://1.2.3.6:3080"
	)
	kubeconfigPath, _ := setup(t)
	creds, _, err := genUserKeyRing("localhost")
	require.NoError(t, err)
	_, leafCACertPEM, err := genUserKeyRing("example.com")
	require.NoError(t, err)
	creds.TrustedCerts[0].ClusterName = clusterName
	creds.TrustedCerts = append(creds.TrustedCerts, authclient.TrustedCerts{
		ClusterName:     leafClusterName,
		TLSCertificates: [][]byte{leafCACertPEM},
	})

	tests := []struct {
		loadAllCAs     bool
		expectedNumCAs int
	}{
		{loadAllCAs: false, expectedNumCAs: 1},
		{loadAllCAs: true, expectedNumCAs: 2},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("LoadAllCAs=%v", tc.loadAllCAs), func(t *testing.T) {
			require.NoError(t, Update(kubeconfigPath, Values{
				TeleportClusterName: clusterName,
				ClusterAddr:         clusterAddr,
				Credentials:         creds,
			}, tc.loadAllCAs))

			config, err := Load(kubeconfigPath)
			require.NoError(t, err)
			numCAs := bytes.Count(config.Clusters[clusterName].CertificateAuthorityData, []byte("-----BEGIN CERTIFICATE-----"))
			require.Equal(t, tc.expectedNumCAs, numCAs)
		})
	}
}

func TestRemoveByClusterName(t *testing.T) {
	const (
		clusterName = "teleport-cluster"
		clusterAddr = "https://1.2.3.6:3080"
	)
	kubeconfigPath, initialConfig := setup(t)

	creds, _, err := genUserKeyRing("localhost")
	require.NoError(t, err)

	// Add teleport-generated entries to kubeconfig.
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	}, false)
	require.NoError(t, err)

	// Remove those generated entries from kubeconfig.
	err = RemoveByClusterName(kubeconfigPath, clusterName)
	require.NoError(t, err)

	// Verify that kubeconfig changed back to the initial state.
	wantConfig := initialConfig.DeepCopy()
	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	// CurrentContext can end up as either of the remaining contexts, as long
	// as it's not the one we just removed.
	require.NotEqual(t, clusterName, config.CurrentContext)
	wantConfig.CurrentContext = config.CurrentContext
	require.Equal(t, wantConfig, config)

	// Add teleport-generated entries to kubeconfig again.
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	}, false)
	require.NoError(t, err)

	// This time, explicitly switch CurrentContext to "prod".
	// Remove should preserve this CurrentContext!
	config, err = Load(kubeconfigPath)
	require.NoError(t, err)
	config.CurrentContext = "prod"
	err = Save(kubeconfigPath, *config)
	require.NoError(t, err)

	// Remove teleport-generated entries from kubeconfig.
	err = RemoveByClusterName(kubeconfigPath, clusterName)
	require.NoError(t, err)

	wantConfig = initialConfig.DeepCopy()
	// CurrentContext should always end up as "prod" because we explicitly set
	// it above and Remove shouldn't touch it unless it matches the cluster
	// being removed.
	wantConfig.CurrentContext = "prod"
	config, err = Load(kubeconfigPath)
	require.NoError(t, err)
	require.Equal(t, wantConfig, config)
}

func TestRemoveByServerAddr(t *testing.T) {
	const (
		rootKubeClusterAddr = "https://root-cluster.example.com"
		rootClusterName     = "root-cluster"
		leafClusterName     = "leaf-cluster"
	)

	kubeconfigPath, initialConfig := setup(t)
	creds, _, err := genUserKeyRing("localhost")
	require.NoError(t, err)

	// Add teleport-generated entries to kubeconfig.
	require.NoError(t, Update(kubeconfigPath, Values{
		TeleportClusterName: rootClusterName,
		ClusterAddr:         rootKubeClusterAddr,
		KubeClusters:        []string{"kube1"},
		Credentials:         creds,
	}, false))
	require.NoError(t, Update(kubeconfigPath, Values{
		TeleportClusterName: leafClusterName,
		ClusterAddr:         rootKubeClusterAddr,
		KubeClusters:        []string{"kube2"},
		Credentials:         creds,
	}, false))

	// Remove those generated entries from kubeconfig.
	err = RemoveByServerAddr(kubeconfigPath, rootKubeClusterAddr)
	require.NoError(t, err)

	// Verify that kubeconfig changed back to the initial state.
	wantConfig := initialConfig.DeepCopy()
	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	// CurrentContext can end up as either of the remaining contexts, as long
	// as it's not the one we just removed.
	require.NotEqual(t, rootClusterName, config.CurrentContext)
	require.NotEqual(t, leafClusterName, config.CurrentContext)
	wantConfig.CurrentContext = config.CurrentContext
	require.Equal(t, wantConfig, config)
}

func genUserKeyRing(hostname string) (*client.KeyRing, []byte, error) {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName:   hostname,
		Organization: []string{hostname},
	}, nil, defaults.CATTL)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ca, err := tlsca.FromKeys(caCert, caKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	priv, err := keys.NewSoftwarePrivateKey(key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clock := clockwork.NewRealClock()
	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: priv.Public(),
		Subject: pkix.Name{
			CommonName: "teleport-user",
		},
		NotAfter: clock.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &client.KeyRing{
		TLSPrivateKey: priv,
		TLSCert:       tlsCert,
		TrustedCerts: []authclient.TrustedCerts{{
			TLSCertificates: [][]byte{caCert},
		}},
	}, caCert, nil
}

func TestKubeClusterFromContext(t *testing.T) {
	type args struct {
		contextName     string
		ctx             *clientcmdapi.Context
		teleportCluster string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "context name is cluster name",
			args: args{
				contextName:     "cluster1",
				ctx:             &clientcmdapi.Context{Cluster: "cluster1"},
				teleportCluster: "cluster1",
			},
			want: "cluster1",
		},
		{
			name: "context name is {teleport-cluster}-cluster name",
			args: args{
				contextName:     "telecluster-cluster1",
				ctx:             &clientcmdapi.Context{Cluster: "cluster1"},
				teleportCluster: "telecluster",
			},
			want: "cluster1",
		},
		{
			name: "context name is {kube-cluster} name",
			args: args{
				contextName:     "cluster1",
				ctx:             &clientcmdapi.Context{Cluster: "telecluster"},
				teleportCluster: "telecluster",
			},
			want: "cluster1",
		},
		{
			name: "kube cluster name is set in extension",
			args: args{
				contextName: "cluster1",
				ctx: &clientcmdapi.Context{
					Cluster: "telecluster",
					Extensions: map[string]runtime.Object{
						teleportKubeClusterNameExtension: &runtime.Unknown{
							Raw: []byte("\"another\""),
						},
					},
				},
				teleportCluster: "telecluster",
			},
			want: "another",
		},
		{
			name: "context isn't from teleport",
			args: args{
				contextName:     "cluster1",
				ctx:             &clientcmdapi.Context{Cluster: "someothercluster"},
				teleportCluster: "telecluster",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := KubeClusterFromContext(tt.args.contextName, tt.args.ctx, tt.args.teleportCluster)
			require.Equal(t, tt.want, got)
		})
	}
}
