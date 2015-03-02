package openssh

import (
	"testing"

	"github.com/gravitational/teleport/auth/test"

	. "gopkg.in/check.v1"
)

func TestO(t *testing.T) { TestingT(t) }

type OSuite struct {
	suite *test.AuthSuite
}

var _ = Suite(&OSuite{})

func (s *OSuite) SetUpSuite(c *C) {
	s.suite = &test.AuthSuite{A: New()}
}

func (s *OSuite) TestGenerateKeypairEmptyPass(c *C) {
	s.suite.GenerateKeypairEmptyPass(c)
}

func (s *OSuite) TestGenerateKeypairPass(c *C) {
	s.suite.GenerateKeypairPass(c)
}

func (s *OSuite) TestGenerateHostCert(c *C) {
	s.suite.GenerateHostCert(c)
}

func (s *OSuite) TestGenerateUserCert(c *C) {
	s.suite.GenerateUserCert(c)
}
