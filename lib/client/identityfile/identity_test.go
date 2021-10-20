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

package identityfile

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/api/v8/utils/keypaths"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/stretchr/testify/require"
)

func TestWrite(t *testing.T) {
	outputDir := t.TempDir()
	key := &client.Key{
		Cert:    []byte("cert"),
		TLSCert: []byte("tls-cert"),
		Priv:    []byte("priv"),
		Pub:     []byte("pub"),
		TrustedCA: []auth.TrustedCerts{{
			TLSCertificates: [][]byte{[]byte("ca-cert")},
		}},
	}
	cfg := WriteConfig{Key: key}

	// test OpenSSH-compatible identity file creation:
	cfg.OutputPath = filepath.Join(outputDir, "openssh")
	cfg.Format = FormatOpenSSH
	_, err := Write(cfg)
	require.NoError(t, err)

	// key is OK:
	out, err := ioutil.ReadFile(cfg.OutputPath)
	require.NoError(t, err)
	require.Equal(t, string(out), "priv")

	// cert is OK:
	out, err = ioutil.ReadFile(keypaths.IdentitySSHCertPath(cfg.OutputPath))
	require.NoError(t, err)
	require.Equal(t, string(out), "cert")

	// test standard Teleport identity file creation:
	cfg.OutputPath = filepath.Join(outputDir, "file")
	cfg.Format = FormatFile
	_, err = Write(cfg)
	require.NoError(t, err)

	// key+cert are OK:
	out, err = ioutil.ReadFile(cfg.OutputPath)
	require.NoError(t, err)
	require.Equal(t, string(out), "priv\ncert\ntls-cert\nca-cert\n")

	// Test kubeconfig creation.
	cfg.OutputPath = filepath.Join(outputDir, "kubeconfig")
	cfg.Format = FormatKubernetes
	cfg.KubeProxyAddr = "far.away.cluster"
	_, err = Write(cfg)
	require.NoError(t, err)
	assertKubeconfigContents(t, cfg.OutputPath, key.ClusterName, "far.away.cluster")
}

func TestKubeconfigOverwrite(t *testing.T) {
	key := &client.Key{
		Cert:    []byte("cert"),
		TLSCert: []byte("tls-cert"),
		Priv:    []byte("priv"),
		Pub:     []byte("pub"),
		TrustedCA: []auth.TrustedCerts{{
			TLSCertificates: [][]byte{[]byte("ca-cert")},
		}},
	}

	// First write an ssh key to the file.
	cfg := WriteConfig{
		OutputPath:           filepath.Join(t.TempDir(), "out"),
		Format:               FormatFile,
		Key:                  key,
		OverwriteDestination: true,
	}
	_, err := Write(cfg)
	require.NoError(t, err)

	// Write a kubeconfig to the same file path. It should be overwritten.
	cfg.Format = FormatKubernetes
	cfg.KubeProxyAddr = "far.away.cluster"
	_, err = Write(cfg)
	require.NoError(t, err)
	assertKubeconfigContents(t, cfg.OutputPath, key.ClusterName, "far.away.cluster")

	// Write a kubeconfig for a different cluster to the same file path. It
	// should be overwritten.
	cfg.KubeProxyAddr = "other.cluster"
	_, err = Write(cfg)
	require.NoError(t, err)
	assertKubeconfigContents(t, cfg.OutputPath, key.ClusterName, "other.cluster")
}

func assertKubeconfigContents(t *testing.T, path, clusterName, serverAddr string) {
	t.Helper()

	kc, err := kubeconfig.Load(path)
	require.NoError(t, err)

	require.Len(t, kc.AuthInfos, 1)
	require.Len(t, kc.Contexts, 1)
	require.Len(t, kc.Clusters, 1)
	require.Equal(t, kc.Clusters[clusterName].Server, serverAddr)
}
