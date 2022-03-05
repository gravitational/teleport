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
	"bytes"
	"crypto/rsa"
	"crypto/x509/pkix"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/stretchr/testify/require"
)

func newSelfSignedCA(privateKey []byte) (*tlsca.CertAuthority, auth.TrustedCerts, error) {
	rsaKey, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, auth.TrustedCerts{}, trace.Wrap(err)
	}
	cert, err := tlsca.GenerateSelfSignedCAWithSigner(rsaKey.(*rsa.PrivateKey), pkix.Name{
		CommonName:   "localhost",
		Organization: []string{"localhost"},
	}, nil, defaults.CATTL)
	if err != nil {
		return nil, auth.TrustedCerts{}, trace.Wrap(err)
	}
	ca, err := tlsca.FromCertAndSigner(cert, rsaKey.(*rsa.PrivateKey))
	if err != nil {
		return nil, auth.TrustedCerts{}, trace.Wrap(err)
	}
	return ca, auth.TrustedCerts{TLSCertificates: [][]byte{cert}}, nil
}

func newClientKey(t *testing.T) *client.Key {
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	ff, tc, err := newSelfSignedCA(privateKey)
	require.NoError(t, err)
	keygen := testauthority.New()

	cryptoPubKey, err := sshutils.CryptoPublicKey(publicKey)
	require.NoError(t, err)

	clock := clockwork.NewRealClock()
	identity := tlsca.Identity{
		Username: "testuser",
	}

	subject, err := identity.Subject()
	require.NoError(t, err)

	tlsCert, err := ff.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPubKey,
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(defaults.CATTL),
	})
	require.NoError(t, err)

	ta := testauthority.New()
	priv, _, err := ta.GenerateKeyPair("")
	require.NoError(t, err)
	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	certificate, err := keygen.GenerateUserCert(services.UserCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicUserKey: publicKey,
		Username:      "testuser",
	})
	require.NoError(t, err)

	return &client.Key{
		Priv:    privateKey,
		Pub:     publicKey,
		Cert:    certificate,
		TLSCert: tlsCert,
		TrustedCA: []auth.TrustedCerts{
			tc,
		},
		KeyIndex: client.KeyIndex{
			ProxyHost:   "localhost",
			Username:    "testuser",
			ClusterName: "root",
		},
	}
}

func TestWrite(t *testing.T) {
	key := newClientKey(t)

	outputDir := t.TempDir()
	cfg := WriteConfig{Key: key}

	// test OpenSSH-compatible identity file creation:
	cfg.OutputPath = filepath.Join(outputDir, "openssh")
	cfg.Format = FormatOpenSSH
	_, err := Write(cfg)
	require.NoError(t, err)

	// key is OK:
	out, err := ioutil.ReadFile(cfg.OutputPath)
	require.NoError(t, err)
	require.Equal(t, string(out), string(key.Priv))

	// cert is OK:
	out, err = ioutil.ReadFile(keypaths.IdentitySSHCertPath(cfg.OutputPath))
	require.NoError(t, err)
	require.Equal(t, string(out), string(key.Cert))

	// test standard Teleport identity file creation:
	cfg.OutputPath = filepath.Join(outputDir, "file")
	cfg.Format = FormatFile
	_, err = Write(cfg)
	require.NoError(t, err)

	// key+cert are OK:
	out, err = ioutil.ReadFile(cfg.OutputPath)
	require.NoError(t, err)

	wantArr := [][]byte{
		key.Priv,
		{'\n'},
		key.Cert,
		key.TLSCert,
		bytes.Join(key.TLSCAs(), []byte{}),
	}
	want := string(bytes.Join(wantArr, nil))
	require.Equal(t, string(out), want)

	// Test kubeconfig creation.
	cfg.OutputPath = filepath.Join(outputDir, "kubeconfig")
	cfg.Format = FormatKubernetes
	cfg.KubeProxyAddr = "far.away.cluster"
	_, err = Write(cfg)
	require.NoError(t, err)
	assertKubeconfigContents(t, cfg.OutputPath, key.ClusterName, "far.away.cluster")
}

func TestKubeconfigOverwrite(t *testing.T) {
	key := newClientKey(t)

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
