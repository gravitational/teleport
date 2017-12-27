/*
Copyright 2016 Gravitational, Inc.

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
	"crypto/rsa"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"time"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
	"gopkg.in/check.v1"
)

type KeyStoreTestSuite struct {
	storeDir string
	store    *FSLocalKeyStore
	keygen   *testauthority.Keygen
	tlsca    *tlsca.CertAuthority
}

var _ = check.Suite(&KeyStoreTestSuite{})

func newSelfSignedCA(privateKey []byte) (*tlsca.CertAuthority, error) {
	rsaKey, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, cert, err := tlsca.GenerateSelfSignedCAWithPrivateKey(rsaKey.(*rsa.PrivateKey), pkix.Name{
		CommonName:   "localhost",
		Organization: []string{"localhost"},
	}, nil, defaults.CATTL)
	return tlsca.New(cert, key)
}

func (s *KeyStoreTestSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	var err error
	s.keygen = testauthority.New()
	s.storeDir = c.MkDir()
	s.store, err = NewFSLocalKeyStore(s.storeDir)
	c.Assert(err, check.IsNil)
	c.Assert(s.store, check.NotNil)
	c.Assert(utils.IsDir(s.store.KeyDir), check.Equals, true)

	s.tlsca, err = newSelfSignedCA(CAPriv)
	c.Assert(err, check.IsNil)
}

func (s *KeyStoreTestSuite) TearDownSuite(c *check.C) {
	os.RemoveAll(s.storeDir)
}

func (s *KeyStoreTestSuite) SetUpTest(c *check.C) {
	os.RemoveAll(s.store.KeyDir)
}

func (s *KeyStoreTestSuite) TestListKeys(c *check.C) {
	const keyNum = 5
	// add 5 keys for "bob"
	keys := make([]Key, keyNum)
	for i := 0; i < keyNum; i++ {
		key := s.makeSignedKey(c, false)
		host := fmt.Sprintf("host-%v", i)
		s.store.AddKey(host, "bob", key)
		key.ProxyHost = host
		keys[i] = *key
	}
	// add 1 key for "sam"
	samKey := s.makeSignedKey(c, false)
	s.store.AddKey("sam.host", "sam", samKey)

	// read all bob keys:
	keys2, err := s.store.GetKeys("bob")
	c.Assert(err, check.IsNil)
	c.Assert(keys2, check.HasLen, keyNum)
	c.Assert(keys2, check.DeepEquals, keys)

	// read sam's key and make sure it's the same:
	keys, err = s.store.GetKeys("sam")
	c.Assert(err, check.IsNil)
	c.Assert(keys, check.HasLen, 1)
	c.Assert(samKey.Cert, check.DeepEquals, keys[0].Cert)
	c.Assert(samKey.Pub, check.DeepEquals, keys[0].Pub)
}

func (s *KeyStoreTestSuite) TestKeyCRUD(c *check.C) {
	key := s.makeSignedKey(c, false)

	// add key:
	err := s.store.AddKey("host.a", "bob", key)
	c.Assert(err, check.IsNil)

	// load back and compare:
	keyCopy, err := s.store.GetKey("host.a", "bob")
	c.Assert(err, check.IsNil)
	c.Assert(key.EqualsTo(keyCopy), check.Equals, true)

	// Delete & verify that its' gone
	err = s.store.DeleteKey("host.a", "bob")
	c.Assert(err, check.IsNil)
	keyCopy, err = s.store.GetKey("host.a", "bob")
	c.Assert(err, check.NotNil)
	c.Assert(trace.IsNotFound(err), check.Equals, true)

	// Delete non-existing
	err = s.store.DeleteKey("non-existing-host", "non-existing-user")
	c.Assert(err, check.NotNil)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}

func (s *KeyStoreTestSuite) TestKeyExpiration(c *check.C) {
	// make two keys: one is current, and the expire one
	good := s.makeSignedKey(c, false)
	expired := s.makeSignedKey(c, true)

	s.store.AddKey("good.host", "sam", good)
	s.store.AddKey("expired.host", "sam", expired)

	// get all keys back. only "good" key should be returned:
	keys, _ := s.store.GetKeys("sam")
	c.Assert(keys, check.HasLen, 1)
	c.Assert(keys[0].EqualsTo(good), check.Equals, true)
}

func (s *KeyStoreTestSuite) TestKnownHosts(c *check.C) {
	os.MkdirAll(s.store.KeyDir, 0777)
	pub, _, _, _, err := ssh.ParseAuthorizedKey(CAPub)
	c.Assert(err, check.IsNil)

	_, p2, _ := s.keygen.GenerateKeyPair("")
	pub2, _, _, _, _ := ssh.ParseAuthorizedKey(p2)

	err = s.store.AddKnownHostKeys("example.com", []ssh.PublicKey{pub})
	c.Assert(err, check.IsNil)
	err = s.store.AddKnownHostKeys("example.com", []ssh.PublicKey{pub2})
	c.Assert(err, check.IsNil)
	err = s.store.AddKnownHostKeys("example.org", []ssh.PublicKey{pub2})
	c.Assert(err, check.IsNil)

	keys, err := s.store.GetKnownHostKeys("")
	c.Assert(err, check.IsNil)
	c.Assert(keys, check.HasLen, 3)
	c.Assert(keys, check.DeepEquals, []ssh.PublicKey{pub, pub2, pub2})

	// check against dupes:
	before, _ := s.store.GetKnownHostKeys("")
	s.store.AddKnownHostKeys("example.org", []ssh.PublicKey{pub2})
	s.store.AddKnownHostKeys("example.org", []ssh.PublicKey{pub2})
	after, _ := s.store.GetKnownHostKeys("")
	c.Assert(len(before), check.Equals, len(after))

	// check by hostname:
	keys, _ = s.store.GetKnownHostKeys("badhost")
	c.Assert(len(keys), check.Equals, 0)
	keys, _ = s.store.GetKnownHostKeys("example.org")
	c.Assert(len(keys), check.Equals, 1)
	c.Assert(sshutils.KeysEqual(keys[0], pub2), check.Equals, true)
}

// makeSIgnedKey helper returns all 3 components of a user key (signed by CAPriv key)
func (s *KeyStoreTestSuite) makeSignedKey(c *check.C, makeExpired bool) *Key {
	var (
		err             error
		priv, pub, cert []byte
	)
	priv, pub, _ = s.keygen.GenerateKeyPair("")
	username := "vincento"
	allowedLogins := []string{username, "root"}
	ttl := time.Duration(time.Minute * 20)
	if makeExpired {
		ttl = -ttl
	}

	// reuse the same RSA keys for SSH and TLS keys
	cryptoPubKey, err := sshutils.CryptoPublicKey(pub)
	c.Assert(err, check.IsNil)
	clock := clockwork.NewRealClock()
	identity := tlsca.Identity{
		Username: username,
	}
	tlsCert, err := s.tlsca.GenerateCertificate(tlsca.CertificateRequest{
		Clock:     clock,
		PublicKey: cryptoPubKey,
		Subject:   identity.Subject(),
		NotAfter:  clock.Now().UTC().Add(ttl),
	})
	c.Assert(err, check.IsNil)

	cert, err = s.keygen.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey: CAPriv,
		PublicUserKey:       pub,
		Username:            username,
		AllowedLogins:       allowedLogins,
		TTL:                 ttl,
		PermitAgentForwarding: false,
		PermitPortForwarding:  true,
	})
	c.Assert(err, check.IsNil)
	return &Key{
		Priv:    priv,
		Pub:     pub,
		Cert:    cert,
		TLSCert: tlsCert,
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
