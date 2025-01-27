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

package client

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

type testAuthority struct {
	keygen       *testauthority.Keygen
	tlsCA        *tlsca.CertAuthority
	trustedCerts authclient.TrustedCerts
	clock        clockwork.Clock
}

func newTestAuthority(t *testing.T) testAuthority {
	tlsCA, trustedCerts, err := newSelfSignedCA(CAPriv, "localhost")
	require.NoError(t, err)

	clock := clockwork.NewFakeClock()
	return testAuthority{
		keygen:       testauthority.NewWithClock(clock),
		tlsCA:        tlsCA,
		trustedCerts: trustedCerts,
		clock:        clock,
	}
}

// makeSignedKeyRing helper returns a new user key ring signed by CAPriv key.
func (s *testAuthority) makeSignedKeyRing(t *testing.T, idx KeyRingIndex, makeExpired bool) *KeyRing {
	sshKey, tlsKey, err := cryptosuites.GenerateUserSSHAndTLSKey(context.Background(), func(context.Context) (types.SignatureAlgorithmSuite, error) {
		return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
	})
	require.NoError(t, err)
	sshPriv, err := keys.NewSoftwarePrivateKey(sshKey)
	require.NoError(t, err)
	tlsPriv, err := keys.NewSoftwarePrivateKey(tlsKey)
	require.NoError(t, err)

	allowedLogins := []string{idx.Username, "root"}
	ttl := 20 * time.Minute
	if makeExpired {
		ttl = -ttl
	}

	identity := tlsca.Identity{
		Username: idx.Username,
		Groups:   []string{"groups"},
	}
	subject, err := identity.Subject()
	require.NoError(t, err)
	tlsCert, err := s.tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     s.clock,
		PublicKey: tlsKey.Public(),
		Subject:   subject,
		NotAfter:  s.clock.Now().UTC().Add(ttl),
	})
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(CAPriv)
	require.NoError(t, err)

	cert, err := s.keygen.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:      caSigner,
		PublicUserKey: sshPriv.MarshalSSHPublicKey(),
		TTL:           ttl,
		Identity: sshca.Identity{
			Username:              idx.Username,
			Principals:            allowedLogins,
			PermitAgentForwarding: false,
			PermitPortForwarding:  true,
			GitHubUserID:          "1234567",
			GitHubUsername:        "github-username",
		},
	})
	require.NoError(t, err)

	keyRing := NewKeyRing(sshPriv, tlsPriv)
	keyRing.KeyRingIndex = idx
	keyRing.Cert = cert
	keyRing.TLSCert = tlsCert
	keyRing.TrustedCerts = []authclient.TrustedCerts{s.trustedCerts}
	keyRing.DBTLSCredentials["example-db"] = TLSCredential{
		Cert:       tlsCert,
		PrivateKey: tlsPriv,
	}
	return keyRing
}

func newSelfSignedCA(privateKey []byte, cluster string) (*tlsca.CertAuthority, authclient.TrustedCerts, error) {
	priv, err := keys.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, authclient.TrustedCerts{}, trace.Wrap(err)
	}

	cert, err := tlsca.GenerateSelfSignedCAWithSigner(priv, pkix.Name{
		CommonName:   cluster,
		Organization: []string{cluster},
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
		ClusterName:     cluster,
		TLSCertificates: [][]byte{cert},
		AuthorizedKeys:  [][]byte{ssh.MarshalAuthorizedKey(sshPub)},
	}, nil
}

func newTestFSClientStore(t *testing.T) *Store {
	fsClientStore := NewFSClientStore(t.TempDir())
	return fsClientStore
}

func testEachClientStore(t *testing.T, testFunc func(t *testing.T, clientStore *Store)) {
	t.Run("FS", func(t *testing.T) {
		testFunc(t, newTestFSClientStore(t))
	})

	t.Run("Mem", func(t *testing.T) {
		testFunc(t, NewMemClientStore())
	})
}

func TestClientStore(t *testing.T) {
	t.Parallel()
	a := newTestAuthority(t)

	testEachClientStore(t, func(t *testing.T, clientStore *Store) {
		t.Parallel()

		idx := KeyRingIndex{
			ProxyHost:   "proxy.example.com",
			ClusterName: "root",
			Username:    "test-user",
		}
		keyRing := a.makeSignedKeyRing(t, idx, false)

		// Add key should add the key and trusted certs to their respective stores.
		err := clientStore.AddKeyRing(keyRing)
		require.NoError(t, err)

		// the key's trusted certs should be added to the trusted certs store.
		retrievedTrustedCerts, err := clientStore.GetTrustedCerts(idx.ProxyHost)
		require.NoError(t, err)
		require.Equal(t, keyRing.TrustedCerts, retrievedTrustedCerts)

		// Getting the key from the key store should have no trusted certs.
		retrievedKeyRing, err := clientStore.KeyStore.GetKeyRing(idx, WithAllCerts...)
		require.NoError(t, err)
		expectKeyRing := keyRing.Copy()
		expectKeyRing.TrustedCerts = nil
		assertEqualKeyRings(t, expectKeyRing, retrievedKeyRing)

		// Getting the key from the client store should fill in the trusted certs.
		retrievedKeyRing, err = clientStore.GetKeyRing(idx, WithAllCerts...)
		require.NoError(t, err)
		assertEqualKeyRings(t, keyRing, retrievedKeyRing)

		var profileDir string
		if fs, ok := clientStore.KeyStore.(*FSKeyStore); ok {
			profileDir = fs.KeyDir
		}

		// Create and save a corresponding profile for the key.
		profile := &profile.Profile{
			WebProxyAddr: idx.ProxyHost + ":3080",
			SiteName:     idx.ClusterName,
			Username:     idx.Username,
		}
		err = clientStore.SaveProfile(profile, true)
		require.NoError(t, err)
		expectStatus, err := profileStatusFromKeyRing(keyRing, profileOptions{
			ProfileName:   profile.Name(),
			WebProxyAddr:  profile.WebProxyAddr,
			ProfileDir:    profileDir,
			Username:      profile.Username,
			SiteName:      profile.SiteName,
			KubeProxyAddr: profile.KubeProxyAddr,
			IsVirtual:     profileDir == "",
		})
		require.NoError(t, err)

		// ReadProfileStatus should prepare a *ProfileStatus using the saved
		// profile and key together.
		profileStatus, err := clientStore.ReadProfileStatus(profile.Name())
		require.NoError(t, err)
		require.Equal(t, expectStatus, profileStatus)

		// FullProfileStatus should return the current profile status, and any
		// other available profiles' statuses.
		otherKey := keyRing.Copy()
		otherKey.ProxyHost = "other.example.com"
		err = clientStore.AddKeyRing(otherKey)
		require.NoError(t, err)

		otherProfile := profile.Copy()
		otherProfile.WebProxyAddr = "other.example.com:3080"
		err = clientStore.SaveProfile(otherProfile, false)
		require.NoError(t, err)

		expectOtherStatus, err := profileStatusFromKeyRing(keyRing, profileOptions{
			ProfileName:   otherProfile.Name(),
			WebProxyAddr:  otherProfile.WebProxyAddr,
			ProfileDir:    profileDir,
			Username:      otherProfile.Username,
			SiteName:      otherProfile.SiteName,
			KubeProxyAddr: otherProfile.KubeProxyAddr,
			IsVirtual:     profileDir == "",
		})
		require.NoError(t, err)

		currentStatus, otherStatuses, err := clientStore.FullProfileStatus()
		require.NoError(t, err)
		require.Equal(t, expectStatus, currentStatus)
		require.Len(t, otherStatuses, 1)
		require.Equal(t, expectOtherStatus, otherStatuses[0])
	})
}

// TestProxySSHConfig tests proxy client SSH config function
// that generates SSH client configuration for proxy tunnel connections
func TestProxySSHConfig(t *testing.T) {
	t.Parallel()
	auth := newTestAuthority(t)

	testEachClientStore(t, func(t *testing.T, clientStore *Store) {
		t.Parallel()

		idx := KeyRingIndex{"host.a", "bob", "root"}
		keyRing := auth.makeSignedKeyRing(t, idx, false)

		caPub, _, _, _, err := ssh.ParseAuthorizedKey(CAPub)
		require.NoError(t, err)

		err = clientStore.AddKeyRing(keyRing)
		require.NoError(t, err)

		firsthost := "127.0.0.1"
		err = clientStore.AddTrustedHostKeys(idx.ProxyHost, firsthost, caPub)
		require.NoError(t, err)

		retrievedKeyRing, err := clientStore.GetKeyRing(idx, WithSSHCerts{})
		require.NoError(t, err)

		clientConfig, err := retrievedKeyRing.ProxyClientSSHConfig(firsthost)
		require.NoError(t, err)

		var called atomic.Int32
		handler := sshutils.NewChanHandlerFunc(func(_ context.Context, _ *sshutils.ConnectionContext, nch ssh.NewChannel) {
			called.Add(1)
			nch.Reject(ssh.Prohibited, "nothing to see here")
		})

		hostPriv, hostPub, err := auth.keygen.GenerateKeyPair()
		require.NoError(t, err)

		caSigner, err := ssh.ParsePrivateKey(CAPriv)
		require.NoError(t, err)

		hostCert, err := auth.keygen.GenerateHostCert(sshca.HostCertificateRequest{
			CASigner:      caSigner,
			PublicHostKey: hostPub,
			HostID:        "127.0.0.1",
			NodeName:      "127.0.0.1",
			Identity: sshca.Identity{
				ClusterName: "host-cluster-name",
				SystemRole:  types.RoleNode,
			},
		})
		require.NoError(t, err)

		hostSigner, err := sshutils.NewSigner(hostPriv, hostCert)
		require.NoError(t, err)

		srv, err := sshutils.NewServer(
			"test",
			utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
			handler,
			sshutils.StaticHostSigners(hostSigner),
			sshutils.AuthMethods{
				PublicKey: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
					certChecker := apisshutils.CertChecker{
						CertChecker: ssh.CertChecker{
							IsUserAuthority: func(cert ssh.PublicKey) bool {
								// Makes sure that user presented key signed by or with trusted authority.
								return apisshutils.KeysEqual(caPub, cert)
							},
						},
					}
					return certChecker.Authenticate(conn, key)
				},
			},
		)
		require.NoError(t, err)
		require.NoError(t, srv.Start())
		defer srv.Close()

		clt, err := ssh.Dial("tcp", srv.Addr(), clientConfig)
		require.NoError(t, err)
		defer clt.Close()

		// Call new session to initiate opening new channel. This should get
		// rejected and fail.
		_, err = clt.NewSession()
		require.Error(t, err)
		require.Equal(t, 1, int(called.Load()))

		_, spub, err := testauthority.New().GenerateKeyPair()
		require.NoError(t, err)
		caPub22, _, _, _, err := ssh.ParseAuthorizedKey(spub)
		require.NoError(t, err)
		err = clientStore.AddTrustedHostKeys(idx.ProxyHost, "second-host", caPub22)
		require.NoError(t, err)

		// The ProxyClientSSHConfig should create configuration that validates server authority only based on
		// second-host instead of all known hosts.
		retrievedKeyRing, err = clientStore.GetKeyRing(idx, WithSSHCerts{})
		require.NoError(t, err)
		clientConfig, err = retrievedKeyRing.ProxyClientSSHConfig("second-host")
		require.NoError(t, err)

		// ssh server cert doesn't match second-host user known host thus connection should fail.
		_, err = ssh.Dial("tcp", srv.Addr(), clientConfig)
		require.Error(t, err)
	})
}

// BenchmarkLoadKeysToKubeFromStore benchmarks the namesake function used in the
// `tsh kube credentials` command called by kubectl. It should be reasonably
// fast to avoid adding latency to all kubectl calls. It should tolerate being
// called many times in parallel.
func BenchmarkLoadKeysToKubeFromStore(b *testing.B) {
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(b, err)

	template := x509.Certificate{
		Subject: pkix.Name{
			CommonName: "k8scluster",
		},
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, key.Public(), key)
	require.NoError(b, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	require.NotEmpty(b, certPEM)

	keyPEM, err := keys.MarshalPrivateKey(key)
	require.NoError(b, err)
	privateKey, err := keys.NewPrivateKey(key, keyPEM)
	require.NoError(b, err)

	kubeCred := TLSCredential{
		PrivateKey: privateKey,
		Cert:       certPEM,
	}

	dir := b.TempDir()
	fsKeyStore := NewFSKeyStore(dir)

	keyRing := &KeyRing{
		KeyRingIndex: KeyRingIndex{
			ProxyHost:   "teleport.example.com",
			Username:    "tester",
			ClusterName: "teleportcluster",
		},
		TLSPrivateKey:      privateKey,
		SSHPrivateKey:      privateKey,
		TLSCert:            certPEM,
		KubeTLSCredentials: make(map[string]TLSCredential, 10),
	}

	kubeClusterNames := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		kubeClusterName := fmt.Sprintf("kubecluster-%d", i)
		keyRing.KubeTLSCredentials[kubeClusterName] = kubeCred
		kubeClusterNames = append(kubeClusterNames, kubeClusterName)
	}

	err = fsKeyStore.AddKeyRing(keyRing)
	require.NoError(b, err)

	b.Run("LoadKeysToKubeFromStore", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			wg.Add(len(kubeClusterNames))
			for _, kubeClusterName := range kubeClusterNames {
				go func() {
					defer wg.Done()
					keyPEM, certPEM, err := LoadKeysToKubeFromStore(&profile.Profile{
						SiteName: "teleport.example.com",
						Username: "tester",
					}, dir, "teleportcluster", kubeClusterName)
					require.NoError(b, err)
					require.NotEmpty(b, certPEM)
					require.NotEmpty(b, keyPEM)
				}()
			}
			wg.Wait()
		}
	})

	// Compare against a naive GetKeyRing call which loads the key and cert for
	// all active kube clusters, not just the one requested.
	b.Run("GetKeyRing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			wg.Add(len(kubeClusterNames))
			for _, kubeClusterName := range kubeClusterNames {
				go func() {
					defer wg.Done()
					keyRing, err := fsKeyStore.GetKeyRing(keyRing.KeyRingIndex, WithKubeCerts{})
					require.NoError(b, err)
					require.NotNil(b, keyRing.KubeTLSCredentials[kubeClusterName].PrivateKey)
					require.NotEmpty(b, keyRing.KubeTLSCredentials[kubeClusterName].Cert)
				}()
			}
			wg.Wait()
		}
	})
}

var (
	CAPriv = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAwBgwn+vkjCcKEr2fbX1mLN555B9amVYfD/fUZBNbXKpHaqYn
lM2WlyRR+xCrU9H/X6xT+wKJs1tsxFbxdBc1RWJtaqz/VpQCjomOulBzwumzB5hT
pJfGblGjkPvpt1zwfmKdpBg0jxXUHHR4u4N6OX0dxd0ImRQ4W9QUtEqzgqToS5u4
iwpeg6i1SoAdHBaSeqYhK9+nGrrJBAl/HVSgvL9tGn/+cQqlOiQz0t61V20+oMBA
P+rOTIiwRXn98iMKFjzVW1HTL5Lwit3oJQX0Lrd/I6tN2De6TJxbbOOkF45V/P/k
nBzbxV0fpnhcvZMnQqg1qdUmNVi6VC1O5qIPiwIDAQABAoIBAEg0T4KtLnkn63dj
41tKeW+AKJ0A1BMy9fYQl7sOM5c/QhzqW5JpPKOPOWl/uIaHNtCFfAOrzoqmYNnk
PFoApztvZeVlJY0rkVJ2jjmmJ/0pzuuZ7Ea/7gxlj2/d4NnVi2hWNR8LIiZudA5G
EWOaZgTZ7KkFDkhL+2s46pdiRNtj7l5FXn2tCh7jmFgKS4m1/QqV9KdE5EjwB2mj
BoP/j4V8O0RM05QpiYX/D5/Rr06tBavwTGW3vz/7OPIbf1el1mjfbLlt3z2tH0A5
BSGB4JEwIZ3+2xlZokHy95OSDzE46TsSzgNx3SDzGRc8UnSZN9yunxnL4ej11WYt
59YmD+ECgYEA3zxrDAtscpoxJSwcSkwqcMdElMK4D/BZw/tE9HhpHx3Pdd5XtMio
CHUkkqxwGJeVIixDjwnl4VfA1s0wy3CtHq6mmwfUviYrH2eqxe5RxNyZOZguk6is
GurZzD+ZfacsEIHyz2fZdnEAIFubu/S6x4TQPGg23oxnQpXXq1vzZFkCgYEA3Emz
W4MXvYWvRdbn+W3onHz/vty9owj/BKSP6giPGrpQFdLs8yoBUw1yTOGqAIfuWMLS
xvjULSlhei5PYD1xM2+B4luxM8K25DlqUpgRVtdmjQ/wxnzlmhDAPIMh7LUtw/6o
JJ+diAKTI86T8tokIL7WFaSvzdrz7/WrZQWkpoMCgYAPVAK1rQMhS10chE7c+yXe
4I/g9w3Ualh/kH1HnAz7yfw4x6+WBkEjc4ezWovH5ICk/A0XgUJ7mp7vIN+82FvK
w4tFEeCVveEwItojBR4wOkV7Iuvvz6EhqAaUc7mCWzw3VfTqMONJsrCjiCbFXSSG
FqSFwVIjLdjZRZitd37a4QKBgQDWfjjTIVlLY9EfWrszZu54+Ul4Sa2pAwh1N9sd
kUnuR33VUjUALGVvvgcOjyieLb1J1iGwNfc7JjDQ7CjD1+/Smn/IrWlksfKtVK6P
T5yKh2BGeEAEtPZHxom4IiM1PdEbJ2oHhxe3qHInCm2KqRdGfysrldjMw6aEfxxt
WEpTCwKBgHLZYgNf/dGgWgw7bVu/k61jxw3yZuU/0marFOPINME/AnTcSAGnkC0S
oDZhaPxjz3+2AHWAjUgW1ltTY8FsJYTOYsvzkYPfya4CgHCLg3D9ss1m4Rc7w5qo
Fa6bvW5jo543NztjlKts7XYVqroMCu0sIMS7R4JGsmw3VJcnnMP2
-----END RSA PRIVATE KEY-----`)

	CAPub = []byte(`ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDAGDCf6+SMJwoSvZ9tfWYs3nnkH1qZVh8P99RkE1tcqkdqpieUzZaXJFH7EKtT0f9frFP7AomzW2zEVvF0FzVFYm1qrP9WlAKOiY66UHPC6bMHmFOkl8ZuUaOQ++m3XPB+Yp2kGDSPFdQcdHi7g3o5fR3F3QiZFDhb1BS0SrOCpOhLm7iLCl6DqLVKgB0cFpJ6piEr36causkECX8dVKC8v20af/5xCqU6JDPS3rVXbT6gwEA/6s5MiLBFef3yIwoWPNVbUdMvkvCK3eglBfQut38jq03YN7pMnFts46QXjlX8/+ScHNvFXR+meFy9kydCqDWp1SY1WLpULU7mog+L ekontsevoy@turing`)
)
