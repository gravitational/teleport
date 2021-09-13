/*
Copyright 2016-2020 Gravitational, Inc.

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

package client

import (
	"context"
	"crypto/rsa"
	"crypto/x509/pkix"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keypaths"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/crypto/ssh"
)

func TestListKeys(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	const keyNum = 5

	// add 5 keys for "bob"
	keys := make([]Key, keyNum)
	for i := 0; i < keyNum; i++ {
		idx := KeyIndex{fmt.Sprintf("host-%v", i), "bob", "root"}
		key := s.makeSignedKey(t, idx, false)
		require.NoError(t, s.addKey(key))
		keys[i] = *key
	}
	// add 1 key for "sam"
	samIdx := KeyIndex{"sam.host", "sam", "root"}
	samKey := s.makeSignedKey(t, samIdx, false)
	require.NoError(t, s.addKey(samKey))

	// read all bob keys:
	for i := 0; i < keyNum; i++ {
		keys2, err := s.store.GetKey(keys[i].KeyIndex, WithSSHCerts{}, WithDBCerts{})
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(*keys2, keys[i], cmpopts.EquateEmpty()))
	}

	// read sam's key and make sure it's the same:
	skey, err := s.store.GetKey(samIdx, WithSSHCerts{})
	require.NoError(t, err)
	require.Equal(t, samKey.Cert, skey.Cert)
	require.Equal(t, samKey.Pub, skey.Pub)
}

func TestKeyCRUD(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	idx := KeyIndex{"host.a", "bob", "root"}
	key := s.makeSignedKey(t, idx, false)

	// add key:
	err := s.addKey(key)
	require.NoError(t, err)

	// load back and compare:
	keyCopy, err := s.store.GetKey(idx, WithSSHCerts{}, WithDBCerts{})
	require.NoError(t, err)
	key.ProxyHost = keyCopy.ProxyHost
	require.Empty(t, cmp.Diff(key, keyCopy, cmpopts.EquateEmpty()))
	require.Len(t, key.DBTLSCerts, 1)

	// Delete just the db cert, reload & verify it's gone
	err = s.store.DeleteUserCerts(idx, WithDBCerts{})
	require.NoError(t, err)
	keyCopy, err = s.store.GetKey(idx, WithSSHCerts{}, WithDBCerts{})
	require.NoError(t, err)
	key.DBTLSCerts = nil
	require.Empty(t, cmp.Diff(key, keyCopy, cmpopts.EquateEmpty()))

	// Delete & verify that it's gone
	err = s.store.DeleteKey(idx)
	require.NoError(t, err)
	_, err = s.store.GetKey(idx)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	// Delete non-existing
	err = s.store.DeleteKey(KeyIndex{ProxyHost: "non-existing-host", Username: "non-existing-user"})
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))
}

func TestDeleteAll(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	// generate keys
	idxFoo := KeyIndex{"proxy.example.com", "foo", "root"}
	keyFoo := s.makeSignedKey(t, idxFoo, false)
	idxBar := KeyIndex{"proxy.example.com", "bar", "root"}
	keyBar := s.makeSignedKey(t, idxBar, false)

	// add keys
	err := s.addKey(keyFoo)
	require.NoError(t, err)
	err = s.addKey(keyBar)
	require.NoError(t, err)

	// check keys exist
	_, err = s.store.GetKey(idxFoo)
	require.NoError(t, err)
	_, err = s.store.GetKey(idxBar)
	require.NoError(t, err)

	// delete all keys
	err = s.store.DeleteKeys()
	require.NoError(t, err)

	// verify keys are gone
	_, err = s.store.GetKey(idxFoo)
	require.True(t, trace.IsNotFound(err))
	_, err = s.store.GetKey(idxBar)
	require.Error(t, err)
}

func TestKnownHosts(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	err := os.MkdirAll(s.store.KeyDir, 0777)
	require.NoError(t, err)
	pub, _, _, _, err := ssh.ParseAuthorizedKey(CAPub)
	require.NoError(t, err)

	_, p2, _ := s.keygen.GenerateKeyPair("")
	pub2, _, _, _, _ := ssh.ParseAuthorizedKey(p2)

	err = s.store.AddKnownHostKeys("example.com", "proxy.example.com", []ssh.PublicKey{pub})
	require.NoError(t, err)
	err = s.store.AddKnownHostKeys("example.com", "proxy.example.com", []ssh.PublicKey{pub2})
	require.NoError(t, err)
	err = s.store.AddKnownHostKeys("example.org", "proxy.example.org", []ssh.PublicKey{pub2})
	require.NoError(t, err)

	keys, err := s.store.GetKnownHostKeys("")
	require.NoError(t, err)
	require.Len(t, keys, 3)
	require.Equal(t, keys, []ssh.PublicKey{pub, pub2, pub2})

	// check against dupes:
	before, _ := s.store.GetKnownHostKeys("")
	err = s.store.AddKnownHostKeys("example.org", "proxy.example.org", []ssh.PublicKey{pub2})
	require.NoError(t, err)
	err = s.store.AddKnownHostKeys("example.org", "proxy.example.org", []ssh.PublicKey{pub2})
	require.NoError(t, err)
	after, _ := s.store.GetKnownHostKeys("")
	require.Equal(t, len(before), len(after))

	// check by hostname:
	keys, _ = s.store.GetKnownHostKeys("badhost")
	require.Equal(t, len(keys), 0)
	keys, _ = s.store.GetKnownHostKeys("example.org")
	require.Equal(t, len(keys), 1)
	require.True(t, apisshutils.KeysEqual(keys[0], pub2))

	// check for proxy and wildcard as well:
	keys, _ = s.store.GetKnownHostKeys("proxy.example.org")
	require.Equal(t, 1, len(keys))
	require.True(t, apisshutils.KeysEqual(keys[0], pub2))
	keys, _ = s.store.GetKnownHostKeys("*.example.org")
	require.Equal(t, 1, len(keys))
	require.True(t, apisshutils.KeysEqual(keys[0], pub2))
}

// TestCheckKey makes sure Teleport clients can load non-RSA algorithms in
// normal operating mode.
func TestCheckKey(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	idx := KeyIndex{"host.a", "bob", "root"}
	key := s.makeSignedKey(t, idx, false)

	// Swap out the key with a ECDSA SSH key.
	ellipticCertificate, _, err := utils.CreateEllipticCertificate("foo", ssh.UserCert)
	require.NoError(t, err)
	key.Cert = ssh.MarshalAuthorizedKey(ellipticCertificate)

	err = s.addKey(key)
	require.NoError(t, err)

	_, err = s.store.GetKey(idx)
	require.NoError(t, err)
}

// TestProxySSHConfig tests proxy client SSH config function
// that generates SSH client configuration for proxy tunnel connections
func TestProxySSHConfig(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	idx := KeyIndex{"host.a", "bob", "root"}
	key := s.makeSignedKey(t, idx, false)

	caPub, _, _, _, err := ssh.ParseAuthorizedKey(CAPub)
	require.NoError(t, err)

	err = s.store.AddKnownHostKeys("127.0.0.1", idx.ProxyHost, []ssh.PublicKey{caPub})
	require.NoError(t, err)

	clientConfig, err := key.ProxyClientSSHConfig(s.store)
	require.NoError(t, err)

	called := atomic.NewInt32(0)
	handler := sshutils.NewChanHandlerFunc(func(_ context.Context, _ *sshutils.ConnectionContext, nch ssh.NewChannel) {
		called.Inc()
		nch.Reject(ssh.Prohibited, "nothing to see here")
	})

	hostPriv, hostPub, err := s.keygen.GenerateKeyPair("")
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(CAPriv)
	require.NoError(t, err)

	hostCert, err := s.keygen.GenerateHostCert(services.HostCertParams{
		CASigner:      caSigner,
		CASigningAlg:  defaults.CASignatureAlgorithm,
		PublicHostKey: hostPub,
		HostID:        "127.0.0.1",
		NodeName:      "127.0.0.1",
		ClusterName:   "host-cluster-name",
		Role:          types.RoleNode,
	})
	require.NoError(t, err)

	hostSigner, err := sshutils.NewSigner(hostPriv, hostCert)
	require.NoError(t, err)

	srv, err := sshutils.NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		handler,
		[]ssh.Signer{hostSigner},
		sshutils.AuthMethods{
			PublicKey: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
				certChecker := utils.CertChecker{
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
	require.Equal(t, int(called.Load()), 1)
}

// TestCheckKeyFIPS makes sure Teleport clients don't load invalid
// certificates while in FIPS mode.
func TestCheckKeyFIPS(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	// This test only runs in FIPS mode.
	if !isFIPS() {
		t.Skip("This test only runs in FIPS mode.")
	}

	idx := KeyIndex{"host.a", "bob", "root"}
	key := s.makeSignedKey(t, idx, false)

	// Swap out the key with a ECDSA SSH key.
	ellipticCertificate, _, err := utils.CreateEllipticCertificate("foo", ssh.UserCert)
	require.NoError(t, err)
	key.Cert = ssh.MarshalAuthorizedKey(ellipticCertificate)

	err = s.addKey(key)
	require.NoError(t, err)

	// Should return trace.BadParameter error because only RSA keys are supported.
	_, err = s.store.GetKey(idx)
	require.True(t, trace.IsBadParameter(err))
}

func TestGetTrustedCertsPEM_nonCertificateBlocks(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	// Make sure we behave correctly if someone writes a non-CERTIFICATE block to
	// certs.pem. During regular use this shouldn't happen, but a bug was lurking
	// around here.
	proxy := "proxy.example.com"
	certsFile := keypaths.TLSCAsPath(s.storeDir, proxy)
	err := os.MkdirAll(filepath.Dir(certsFile), 0700)
	require.NoError(t, err)
	err = ioutil.WriteFile(certsFile, []byte(`-----BEGIN RSA PUBLIC KEY-----
MIIBCgKCAQEAp2eO39fYnpUI4PplyoS/bHrr5Yiy98t+1sdDwGIG01UPlkxAxzIi
VVQmel1NrSh4lF4t3b8KUUNM+5pk241F7Olr/4DIRTPQHDGWO0nciEieZ8IpFigz
kUQRvKjNIw4zZbZSsZu0QE7hCU6O8VwEwSFrEsCCrPw4+28pp2IEYOqe0chZosO/
6kXdJa/ZjC/Edjep1XVdoM+BSFXR5qwY4WtU/Ha4SNRbaktzMZgrkOLgD5TALGoN
DYxXLyVgxD6BvRxlaQft75Bwg1KJ6nKqYAAtu/Me98BXDt+1GFwltLsjeY68untS
hRdXE63PXwAfzj0P/H4qWsFfwdeCo/fuIQIDAQAB
-----END RSA PUBLIC KEY-----`), 0600)
	require.NoError(t, err)

	blocks, err := s.store.GetTrustedCertsPEM(proxy)
	require.Empty(t, blocks)
	require.NoError(t, err)
}

type keyStoreTest struct {
	storeDir  string
	store     *FSLocalKeyStore
	keygen    *testauthority.Keygen
	tlsCA     *tlsca.CertAuthority
	tlsCACert auth.TrustedCerts
}

func (s *keyStoreTest) addKey(key *Key) error {
	if err := s.store.AddKey(key); err != nil {
		return err
	}
	// Also write the trusted CA certs for the host.
	return s.store.SaveTrustedCerts(key.ProxyHost, []auth.TrustedCerts{s.tlsCACert})
}

// makeSignedKey helper returns all 3 components of a user key (signed by CAPriv key)
func (s *keyStoreTest) makeSignedKey(t *testing.T, idx KeyIndex, makeExpired bool) *Key {
	var (
		err             error
		priv, pub, cert []byte
	)
	priv, pub, _ = s.keygen.GenerateKeyPair("")
	allowedLogins := []string{idx.Username, "root"}
	ttl := 20 * time.Minute
	if makeExpired {
		ttl = -ttl
	}

	// reuse the same RSA keys for SSH and TLS keys
	cryptoPubKey, err := sshutils.CryptoPublicKey(pub)
	require.NoError(t, err)
	clock := clockwork.NewRealClock()
	identity := tlsca.Identity{
		Username: idx.Username,
	}
	subject, err := identity.Subject()
	require.NoError(t, err)
	tlsCert, err := s.tlsCA.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPubKey,
		Subject:   subject,
		NotAfter:  clock.Now().UTC().Add(ttl),
	})
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(CAPriv)
	require.NoError(t, err)

	cert, err = s.keygen.GenerateUserCert(services.UserCertParams{
		CASigner:              caSigner,
		CASigningAlg:          defaults.CASignatureAlgorithm,
		PublicUserKey:         pub,
		Username:              idx.Username,
		AllowedLogins:         allowedLogins,
		TTL:                   ttl,
		PermitAgentForwarding: false,
		PermitPortForwarding:  true,
	})
	require.NoError(t, err)
	return &Key{
		KeyIndex:   idx,
		Priv:       priv,
		Pub:        pub,
		Cert:       cert,
		TLSCert:    tlsCert,
		TrustedCA:  []auth.TrustedCerts{s.tlsCACert},
		DBTLSCerts: map[string][]byte{"example-db": tlsCert},
	}
}

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

func newTest(t *testing.T) (keyStoreTest, func()) {
	dir, err := ioutil.TempDir("", "teleport-keystore")
	require.NoError(t, err)

	store, err := NewFSLocalKeyStore(dir)
	require.NoError(t, err)

	s := keyStoreTest{
		keygen:   testauthority.New(),
		storeDir: dir,
		store:    store,
	}
	require.True(t, utils.IsDir(s.store.KeyDir))

	s.tlsCA, s.tlsCACert, err = newSelfSignedCA(CAPriv)
	require.NoError(t, err)

	return s, func() {
		os.RemoveAll(dir)
	}
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

func TestMemLocalKeyStore(t *testing.T) {
	s, cleanup := newTest(t)
	defer cleanup()

	// create keystore
	dir := t.TempDir()
	keystore, err := NewMemLocalKeyStore(dir)
	require.NoError(t, err)

	// create a test key
	idx := KeyIndex{"test.com", "test", "root"}
	key := s.makeSignedKey(t, idx, false)

	// add the test key to the memory store
	err = keystore.AddKey(key)
	require.NoError(t, err)

	// check that the key exists in the store
	retrievedKey, err := keystore.GetKey(idx)
	require.NoError(t, err)
	require.Equal(t, key, retrievedKey)

	// delete the key
	err = keystore.DeleteKey(idx)
	require.NoError(t, err)

	// check that the key doesn't exist in the store
	retrievedKey, err = keystore.GetKey(idx)
	require.Error(t, err)
	require.Nil(t, retrievedKey)

	// add it again
	err = keystore.AddKey(key)
	require.NoError(t, err)

	// check for the key, now without cluster name
	retrievedKey, err = keystore.GetKey(KeyIndex{idx.ProxyHost, idx.Username, ""})
	require.NoError(t, err)
	require.Equal(t, key, retrievedKey)

	// delete all keys
	err = keystore.DeleteKeys()
	require.NoError(t, err)

	// verify it's deleted
	retrievedKey, err = keystore.GetKey(idx)
	require.Error(t, err)
	require.Nil(t, retrievedKey)
}

func TestMatchesWildcard(t *testing.T) {
	// Not a wildcard pattern.
	require.False(t, matchesWildcard("foo.example.com", "example.com"))

	// Not a match.
	require.False(t, matchesWildcard("foo.example.org", "*.example.com"))

	// Too many levels deep.
	require.False(t, matchesWildcard("a.b.example.com", "*.example.com"))

	// Single-part hostnames never match.
	require.False(t, matchesWildcard("example", "*.example.com"))
	require.False(t, matchesWildcard("example", "*.example"))
	require.False(t, matchesWildcard("example", "example"))
	require.False(t, matchesWildcard("example", "*."))

	// Valid wildcard matches.
	require.True(t, matchesWildcard("foo.example.com", "*.example.com"))
	require.True(t, matchesWildcard("bar.example.com", "*.example.com"))
	require.True(t, matchesWildcard("bar.example.com.", "*.example.com"))
	require.True(t, matchesWildcard("bar.foo", "*.foo"))
}
