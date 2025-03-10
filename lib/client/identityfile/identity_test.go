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

package identityfile

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
)

func newSelfSignedCA(priv crypto.Signer) (*tlsca.CertAuthority, authclient.TrustedCerts, error) {
	cert, err := tlsca.GenerateSelfSignedCAWithSigner(priv, pkix.Name{
		CommonName:   "root",
		Organization: []string{"localhost"},
	}, nil, defaults.CATTL)
	if err != nil {
		return nil, authclient.TrustedCerts{}, trace.Wrap(err)
	}
	ca, err := tlsca.FromCertAndSigner(cert, priv)
	if err != nil {
		return nil, authclient.TrustedCerts{}, trace.Wrap(err)
	}
	sshPub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return nil, authclient.TrustedCerts{}, trace.Wrap(err)
	}
	return ca, authclient.TrustedCerts{
		ClusterName:     "root",
		TLSCertificates: [][]byte{cert},
		AuthorizedKeys:  [][]byte{ssh.MarshalAuthorizedKey(sshPub)},
	}, nil
}

func newClientKeyRing(t *testing.T, modifiers ...func(*tlsca.Identity)) *client.KeyRing {
	// Some formats only support RSA (certain DBs, PPK files).
	privateKey, err := keys.ParsePrivateKey(fixtures.PEMBytes["rsa"])
	require.NoError(t, err)

	ff, tc, err := newSelfSignedCA(privateKey)
	require.NoError(t, err)
	keygen := testauthority.New()

	clock := clockwork.NewRealClock()
	identity := tlsca.Identity{
		Username: "testuser",
		Groups:   []string{"groups"},
	}
	for _, mod := range modifiers {
		mod(&identity)
	}

	subject, err := identity.Subject()
	require.NoError(t, err)

	tlsCert, err := ff.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: privateKey.Public(),
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(defaults.CATTL),
	})
	require.NoError(t, err)

	signer, err := keys.ParsePrivateKey([]byte(fixtures.SSHCAPrivateKey))
	require.NoError(t, err)
	caSigner, err := ssh.NewSignerFromKey(signer)
	require.NoError(t, err)

	certificate, err := keygen.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:      caSigner,
		PublicUserKey: ssh.MarshalAuthorizedKey(privateKey.SSHPublicKey()),
		Identity: sshca.Identity{
			Username:   "testuser",
			Principals: []string{"testuser"},
		},
	})
	require.NoError(t, err)

	// Identity files use a single key for SSH and TLS.
	keyRing := client.NewKeyRing(privateKey, privateKey)
	keyRing.KeyRingIndex = client.KeyRingIndex{
		ProxyHost:   "localhost",
		Username:    "testuser",
		ClusterName: "root",
	}
	keyRing.Cert = certificate
	keyRing.TLSCert = tlsCert
	keyRing.TrustedCerts = []authclient.TrustedCerts{tc}

	return keyRing
}

func TestWrite(t *testing.T) {
	keyRing := newClientKeyRing(t)

	outputDir := t.TempDir()
	cfg := WriteConfig{KeyRing: keyRing}

	// test OpenSSH-compatible identity file creation:
	cfg.OutputPath = filepath.Join(outputDir, "openssh")
	cfg.Format = FormatOpenSSH
	_, err := Write(context.Background(), cfg)
	require.NoError(t, err)

	// key is OK:
	out, err := os.ReadFile(cfg.OutputPath)
	require.NoError(t, err)
	require.Equal(t, string(out), string(keyRing.SSHPrivateKey.PrivateKeyPEM()))

	// cert is OK:
	out, err = os.ReadFile(keypaths.IdentitySSHCertPath(cfg.OutputPath))
	require.NoError(t, err)
	require.Equal(t, string(out), string(keyRing.Cert))

	// test standard Teleport identity file creation:
	cfg.OutputPath = filepath.Join(outputDir, "file")
	cfg.Format = FormatFile
	_, err = Write(context.Background(), cfg)
	require.NoError(t, err)

	// key+cert are OK:
	out, err = os.ReadFile(cfg.OutputPath)
	require.NoError(t, err)

	knownHosts, err := sshutils.MarshalKnownHost(sshutils.KnownHost{
		Hostname:      keyRing.ClusterName,
		ProxyHost:     keyRing.ProxyHost,
		AuthorizedKey: keyRing.TrustedCerts[0].AuthorizedKeys[0],
	})
	require.NoError(t, err)

	wantArr := [][]byte{
		keyRing.TLSPrivateKey.PrivateKeyPEM(),
		keyRing.Cert,
		keyRing.TLSCert,
		[]byte(knownHosts),
		bytes.Join(keyRing.TrustedCerts[0].TLSCertificates, []byte{}),
	}
	want := string(bytes.Join(wantArr, nil))
	require.Equal(t, want, string(out))

	// Test kubeconfig creation.
	cfg.OutputPath = filepath.Join(outputDir, "kubeconfig")
	cfg.Format = FormatKubernetes
	cfg.KubeProxyAddr = "far.away.cluster"
	cfg.KubeTLSServerName = constants.KubeTeleportProxyALPNPrefix + "far.away.cluster"
	_, err = Write(context.Background(), cfg)
	require.NoError(t, err)
	assertKubeconfigContents(t, cfg.OutputPath, keyRing.ClusterName, "far.away.cluster", cfg.KubeTLSServerName)
}

// Assert that the kubeconfig writer only writes to the supplied filesystem
// abstraction, and not to the system
func TestWriteKubeOnlyWritesToWriter(t *testing.T) {
	keyRing := newClientKeyRing(t)
	outputDir := t.TempDir()

	fs := NewInMemoryConfigWriter()
	cfg := WriteConfig{
		KeyRing: keyRing,
		Writer:  fs,
	}

	cfg.OutputPath = filepath.Join(outputDir, "kubeconfig")
	cfg.Format = FormatOpenSSH
	cfg.KubeProxyAddr = "far.away.cluster"
	cfg.KubeTLSServerName = constants.KubeTeleportProxyALPNPrefix + "far.away.cluster"
	files, err := Write(context.Background(), cfg)
	require.NoError(t, err)

	// Assert that none of the listed files
	for _, fn := range files {
		// assert that no such file exists on the system filesystem
		_, err := os.Stat(fn)
		require.Error(t, err)

		// assert that the file exists is in the filesystem abstraction
		require.Contains(t, fs.files, fn)
	}

	// Assert that nothing has written to the temp dir without it being added to
	// the returned file list
	actualFiles, err := os.ReadDir(outputDir)
	require.NoError(t, err)
	require.Empty(t, actualFiles)
}

func TestWriteAllFormats(t *testing.T) {
	for _, format := range KnownFileFormats {
		t.Run(string(format), func(t *testing.T) {
			keyRing := newClientKeyRing(t)

			cfg := WriteConfig{
				OutputPath:          path.Join(t.TempDir(), "identity"),
				KeyRing:             keyRing,
				WindowsDesktopCerts: map[string][]byte{"windows-user": []byte("cert data")},
				Format:              format,
			}

			// extra fields for kubernetes
			if format == FormatKubernetes {
				cfg.KubeProxyAddr = "far.away.cluster"
				cfg.KubeTLSServerName = constants.KubeTeleportProxyALPNPrefix + "far.away.cluster"
			}

			// for cockroach, output path should be a directory
			if format == FormatCockroach {
				cfg.OutputPath = t.TempDir()
			}

			files, err := Write(context.Background(), cfg)
			require.NoError(t, err)
			for _, file := range files {
				require.True(t, strings.HasPrefix(file, cfg.OutputPath))
			}
			require.NotEmpty(t, files)
		})
	}
}

func TestKubeconfigOverwrite(t *testing.T) {
	keyRing := newClientKeyRing(t)

	// First write an ssh key to the file.
	cfg := WriteConfig{
		OutputPath:           filepath.Join(t.TempDir(), "out"),
		Format:               FormatFile,
		KeyRing:              keyRing,
		OverwriteDestination: true,
	}
	_, err := Write(context.Background(), cfg)
	require.NoError(t, err)

	// Write a kubeconfig to the same file path. It should be overwritten.
	cfg.Format = FormatKubernetes
	cfg.KubeProxyAddr = "far.away.cluster"
	_, err = Write(context.Background(), cfg)
	require.NoError(t, err)
	assertKubeconfigContents(t, cfg.OutputPath, keyRing.ClusterName, "far.away.cluster", "")

	// Write a kubeconfig for a different cluster to the same file path. It
	// should be overwritten.
	cfg.KubeProxyAddr = "other.cluster"
	cfg.KubeTLSServerName = constants.KubeTeleportProxyALPNPrefix + "other.cluster"
	_, err = Write(context.Background(), cfg)
	require.NoError(t, err)
	assertKubeconfigContents(t, cfg.OutputPath, keyRing.ClusterName, "other.cluster", cfg.KubeTLSServerName)
}

func assertKubeconfigContents(t *testing.T, path, clusterName, serverAddr, kubeTLSName string) {
	t.Helper()

	kc, err := kubeconfig.Load(path)
	require.NoError(t, err)

	require.Len(t, kc.AuthInfos, 1)
	require.Len(t, kc.Contexts, 1)
	require.Len(t, kc.Clusters, 1)
	require.Equal(t, kc.Clusters[clusterName].Server, serverAddr)
	require.Equal(t, kc.Clusters[clusterName].TLSServerName, kubeTLSName)
}

func TestIdentityRead(t *testing.T) {
	t.Parallel()

	// 3 different types of identities
	ids := []string{
		"cert-key.pem", // cert + key concatenated together, cert first
		"key-cert.pem", // cert + key concatenated together, key first
		"key",          // two separate files: key and key-cert.pub
	}
	for _, id := range ids {
		// test reading:
		k, err := KeyRingFromIdentityFile(fixturePath(fmt.Sprintf("certs/identities/%s", id)), "proxy.example.com", "")
		require.NoError(t, err)
		require.NotNil(t, k)

		// test creating an auth method from the key:
		am, err := k.AsAuthMethod()
		require.NoError(t, err)
		require.NotNil(t, am)
	}
	k, err := KeyRingFromIdentityFile(fixturePath("certs/identities/lonekey"), "proxy.example.com", "")
	require.Nil(t, k)
	require.Error(t, err)

	// lets read an identity which includes a CA cert
	k, err = KeyRingFromIdentityFile(fixturePath("certs/identities/key-cert-ca.pem"), "proxy.example.com", "")
	require.NoError(t, err)
	require.NotNil(t, k)

	// prepare the cluster CA separately
	certBytes, err := os.ReadFile(fixturePath("certs/identities/ca.pem"))
	require.NoError(t, err)

	_, hosts, cert, _, _, err := ssh.ParseKnownHosts(certBytes)
	require.NoError(t, err)

	var a net.Addr
	// host auth callback must succeed
	cb, err := k.HostKeyCallback()
	require.NoError(t, err)
	require.NoError(t, cb(hosts[0], a, cert))

	// load an identity which include TLS certificates
	k, err = KeyRingFromIdentityFile(fixturePath("certs/identities/tls.pem"), "proxy.example.com", "")
	require.NoError(t, err)
	require.NotNil(t, k)
	require.NotNil(t, k.TLSCert)

	// generate a TLS client config
	conf, err := k.TeleportClientTLSConfig(nil, []string{"one"})
	require.NoError(t, err)
	require.NotNil(t, conf)
}

func fixturePath(path string) string {
	return "../../../fixtures/" + path
}

func TestKeyFromIdentityFile(t *testing.T) {
	t.Parallel()
	keyRing := newClientKeyRing(t)

	identityFilePath := filepath.Join(t.TempDir(), "out")

	// First write an ssh key to the file.
	_, err := Write(context.Background(), WriteConfig{
		OutputPath:           identityFilePath,
		Format:               FormatFile,
		KeyRing:              keyRing,
		OverwriteDestination: true,
	})
	require.NoError(t, err)

	const proxyHost = "proxy.example.com"
	const cluster = "cluster"

	t.Run("parsed key unchanged when both proxy and cluster provided", func(t *testing.T) {
		// parsed key is unchanged from original with proxy and cluster provided.
		parsedKeyRing, err := KeyRingFromIdentityFile(identityFilePath, proxyHost, cluster)
		keyRing.ClusterName = cluster
		keyRing.ProxyHost = proxyHost
		require.NoError(t, err)
		require.Equal(t, keyRing, parsedKeyRing)
	})

	t.Run("cluster name defaults if not provided", func(t *testing.T) {
		// Identity file's cluster name defaults to root cluster name.
		parsedKeyRing, err := KeyRingFromIdentityFile(identityFilePath, proxyHost, "")
		keyRing.ClusterName = "root"
		require.NoError(t, err)
		require.Equal(t, keyRing, parsedKeyRing)
	})

	t.Run("proxy host not provided", func(t *testing.T) {
		// Returns error if proxy host is not provided.
		_, err = KeyRingFromIdentityFile(identityFilePath, "", "")
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("kubernetes certificate loaded", func(t *testing.T) {
		k8sCluster := "my-cluster"
		identityFilePath := filepath.Join(t.TempDir(), "out")
		keyRing := newClientKeyRing(t, func(params *tlsca.Identity) {
			params.KubernetesCluster = k8sCluster
		})
		_, err := Write(context.Background(), WriteConfig{
			OutputPath:           identityFilePath,
			Format:               FormatFile,
			KeyRing:              keyRing,
			OverwriteDestination: true,
		})
		require.NoError(t, err)
		parsedKeyRing, err := KeyRingFromIdentityFile(identityFilePath, proxyHost, cluster)
		require.NoError(t, err)
		require.NotNil(t, parsedKeyRing.KubeTLSCredentials[k8sCluster].PrivateKey)
		require.Equal(t, keyRing.TLSCert, parsedKeyRing.KubeTLSCredentials[k8sCluster].Cert)
	})
}

func TestNewClientStoreFromIdentityFile(t *testing.T) {
	t.Parallel()
	keyRing := newClientKeyRing(t)
	keyRing.ProxyHost = "proxy.example.com"
	keyRing.ClusterName = "cluster"

	identityFilePath := filepath.Join(t.TempDir(), "out")

	// First write an ssh key to the file.
	_, err := Write(context.Background(), WriteConfig{
		OutputPath:           identityFilePath,
		Format:               FormatFile,
		KeyRing:              keyRing,
		OverwriteDestination: true,
	})
	require.NoError(t, err)

	clientStore, err := NewClientStoreFromIdentityFile(identityFilePath, keyRing.ProxyHost+":3080", keyRing.ClusterName, nil /*hwKeyService*/)
	require.NoError(t, err)

	currentProfile, err := clientStore.CurrentProfile()
	require.NoError(t, err)
	require.Equal(t, keyRing.ProxyHost, currentProfile)

	retrievedProfile, err := clientStore.GetProfile(currentProfile)
	require.NoError(t, err)
	require.Equal(t, &profile.Profile{
		WebProxyAddr:          keyRing.ProxyHost + ":3080",
		SiteName:              keyRing.ClusterName,
		Username:              keyRing.Username,
		PrivateKeyPolicy:      keys.PrivateKeyPolicyNone,
		MissingClusterDetails: true,
	}, retrievedProfile)

	retrievedKeyRing, err := clientStore.GetKeyRing(keyRing.KeyRingIndex, client.WithAllCerts...)
	require.NoError(t, err)
	require.Equal(t, keyRing, retrievedKeyRing)
}
