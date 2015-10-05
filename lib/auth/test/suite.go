// package test contains CA authority acceptance test suite
package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/lib/auth"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestAuth(t *testing.T) { TestingT(t) }

type AuthSuite struct {
	A auth.Authority
}

func (s *AuthSuite) GenerateKeypairEmptyPass(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateKeypairPass(c *C) {
	_, pub, err := s.A.GenerateKeyPair("pass1")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	// TODO(klizhentas) test the private key actually
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateHostCert(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := s.A.GenerateHostCert(priv, pub, "auth", "auth.example.com", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateUserCert(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := s.A.GenerateUserCert(priv, pub, "user", "user", time.Hour)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}
