package membk

import (
	"testing"

	"github.com/gravitational/teleport/backend/test"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestMem(t *testing.T) { TestingT(t) }

type MemSuite struct {
	bk    *MemBackend
	suite test.BackendSuite
}

var _ = Suite(&MemSuite{})

func (s *MemSuite) SetUpTest(c *C) {
	// Initiate a backend with a registry
	s.bk = New()

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *MemSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *MemSuite) TestUserKeyCRUD(c *C) {
	s.suite.UserKeyCRUD(c)
}

func (s *MemSuite) TestUserCACRUD(c *C) {
	s.suite.UserCACRUD(c)
}

func (s *MemSuite) TestHostCACRUD(c *C) {
	s.suite.HostCACRUD(c)
}

func (s *MemSuite) TestServerCRUD(c *C) {
	s.suite.ServerCRUD(c)
}

func (s *MemSuite) TestUsersCRUD(c *C) {
	s.suite.UsersCRUD(c)
}

func (s *MemSuite) TestPasswordHashCRUD(c *C) {
	s.suite.PasswordHashCRUD(c)
}

func (s *MemSuite) TestWebSessionCRUD(c *C) {
	s.suite.WebSessionCRUD(c)
}

func (s *MemSuite) TestWebTunCRUD(c *C) {
	s.suite.WebTunCRUD(c)
}

func (s *MemSuite) TestLocking(c *C) {
	s.suite.Locking(c)
}

func (s *MemSuite) TestToken(c *C) {
	s.suite.TokenCRUD(c)
}

func (s *MemSuite) TestRemoteCert(c *C) {
	s.suite.RemoteCertCRUD(c)
}
