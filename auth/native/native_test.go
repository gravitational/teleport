package native

import (
	"testing"

	"github.com/gravitational/teleport/auth/test"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestNative(t *testing.T) { TestingT(t) }

type NativeSuite struct {
	suite *test.AuthSuite
}

var _ = Suite(&NativeSuite{})

func (s *NativeSuite) SetUpSuite(c *C) {
	s.suite = &test.AuthSuite{A: New()}
}

func (s *NativeSuite) TestGenerateKeypairEmptyPass(c *C) {
	s.suite.GenerateKeypairEmptyPass(c)
}

func (s *NativeSuite) TestGenerateKeypairPass(c *C) {
	s.suite.GenerateKeypairPass(c)
}

func (s *NativeSuite) TestGenerateHostCert(c *C) {
	s.suite.GenerateHostCert(c)
}

func (s *NativeSuite) TestGenerateUserCert(c *C) {
	s.suite.GenerateUserCert(c)
}
