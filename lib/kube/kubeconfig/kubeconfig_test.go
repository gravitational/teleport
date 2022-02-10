// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubeconfig

import (
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func setup(t *testing.T) (string, clientcmdapi.Config) {
	f, err := ioutil.TempFile("", "kubeconfig")
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
	creds, caCertPEM, err := genUserKey()
	require.NoError(t, err)
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	})
	require.NoError(t, err)

	wantConfig := initialConfig.DeepCopy()
	wantConfig.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   clusterAddr,
		CertificateAuthorityData: caCertPEM,
		LocationOfOrigin:         kubeconfigPath,
		Extensions:               map[string]runtime.Object{},
	}
	wantConfig.AuthInfos[clusterName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: creds.TLSCert,
		ClientKeyData:         creds.Priv,
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
	)
	kubeconfigPath, initialConfig := setup(t)
	creds, caCertPEM, err := genUserKey()
	require.NoError(t, err)
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
		Exec: &ExecValues{
			TshBinaryPath: tshPath,
			KubeClusters:  []string{kubeCluster},
			Env: map[string]string{
				homeEnvVar: home,
			},
		},
	})
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
			Args: []string{"kube", "credentials",
				fmt.Sprintf("--kube-cluster=%s", kubeCluster),
				fmt.Sprintf("--teleport-cluster=%s", clusterName),
			},
			Env: []clientcmdapi.ExecEnvVar{{Name: homeEnvVar, Value: home}},
		},
	}
	wantConfig.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:          clusterName,
		AuthInfo:         contextName,
		LocationOfOrigin: kubeconfigPath,
		Extensions:       map[string]runtime.Object{},
	}

	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	require.Equal(t, wantConfig, config)
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
	creds, caCertPEM, err := genUserKey()
	require.NoError(t, err)
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
		ProxyAddr:           proxy,
		Exec: &ExecValues{
			TshBinaryPath: tshPath,
			KubeClusters:  []string{kubeCluster},
			Env: map[string]string{
				homeEnvVar: home,
			},
		},
	})
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
			Args: []string{"kube", "credentials",
				fmt.Sprintf("--kube-cluster=%s", kubeCluster),
				fmt.Sprintf("--teleport-cluster=%s", clusterName),
				fmt.Sprintf("--proxy=%s", proxy),
			},
			Env: []clientcmdapi.ExecEnvVar{{Name: homeEnvVar, Value: home}},
		},
	}
	wantConfig.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:          clusterName,
		AuthInfo:         contextName,
		LocationOfOrigin: kubeconfigPath,
		Extensions:       map[string]runtime.Object{},
	}

	config, err := Load(kubeconfigPath)
	require.NoError(t, err)
	require.Equal(t, wantConfig, config)
}

func TestRemove(t *testing.T) {
	const (
		clusterName = "teleport-cluster"
		clusterAddr = "https://1.2.3.6:3080"
	)
	kubeconfigPath, initialConfig := setup(t)
	creds, _, err := genUserKey()
	require.NoError(t, err)

	// Add teleport-generated entries to kubeconfig.
	err = Update(kubeconfigPath, Values{
		TeleportClusterName: clusterName,
		ClusterAddr:         clusterAddr,
		Credentials:         creds,
	})
	require.NoError(t, err)

	// Remove those generated entries from kubeconfig.
	err = Remove(kubeconfigPath, clusterName)
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
	})
	require.NoError(t, err)

	// This time, explicitly switch CurrentContext to "prod".
	// Remove should preserve this CurrentContext!
	config, err = Load(kubeconfigPath)
	require.NoError(t, err)
	config.CurrentContext = "prod"
	err = Save(kubeconfigPath, *config)
	require.NoError(t, err)

	// Remove teleport-generated entries from kubeconfig.
	err = Remove(kubeconfigPath, clusterName)
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

func genUserKey() (*client.Key, []byte, error) {
	caKey, caCert, err := tlsca.GenerateSelfSignedCA(pkix.Name{
		CommonName:   "localhost",
		Organization: []string{"localhost"},
	}, nil, defaults.CATTL)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	ca, err := tlsca.FromKeys(caCert, caKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	keygen := testauthority.New()
	priv, pub, err := keygen.GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cryptoPub, err := sshutils.CryptoPublicKey(pub)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clock := clockwork.NewRealClock()
	tlsCert, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPub,
		Subject: pkix.Name{
			CommonName: "teleport-user",
		},
		NotAfter: clock.Now().UTC().Add(time.Minute),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &client.Key{
		Priv:    priv,
		Pub:     pub,
		TLSCert: tlsCert,
		TrustedCA: []auth.TrustedCerts{{
			TLSCertificates: [][]byte{caCert},
		}},
	}, caCert, nil
}
