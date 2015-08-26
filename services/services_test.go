package services

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/gravitational/teleport/backend/boltbk"
)

func TestBolt(t *testing.T) { TestingT(t) }

type BoltSuite struct {
	bk    *boltbk.BoltBackend
	suite *ServicesTestSuite
	dir   string
}

var _ = Suite(&BoltSuite{})

func (s *BoltSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *BoltSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.bk, err = boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.suite = NewServicesTestSuite(s.bk)
}

func (s *BoltSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *BoltSuite) TestUserKeyCRUD(c *C) {
	s.suite.UserKeyCRUD(c)
}

func (s *BoltSuite) TestUserCACRUD(c *C) {
	s.suite.UserCACRUD(c)
}

func (s *BoltSuite) TestHostCACRUD(c *C) {
	s.suite.HostCACRUD(c)
}

func (s *BoltSuite) TestServerCRUD(c *C) {
	s.suite.ServerCRUD(c)
}

func (s *BoltSuite) TestUsersCRUD(c *C) {
	s.suite.UsersCRUD(c)
}

func (s *BoltSuite) TestPasswordHashCRUD(c *C) {
	s.suite.PasswordHashCRUD(c)
}

func (s *BoltSuite) TestWebSessionCRUD(c *C) {
	s.suite.WebSessionCRUD(c)
}

func (s *BoltSuite) TestWebTunCRUD(c *C) {
	s.suite.WebTunCRUD(c)
}

func (s *BoltSuite) TestLocking(c *C) {
	s.suite.Locking(c)
}

func (s *BoltSuite) TestToken(c *C) {
	s.suite.TokenCRUD(c)
}

func (s *BoltSuite) TestRemoteCert(c *C) {
	s.suite.RemoteCertCRUD(c)
}
