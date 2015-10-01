package boltbk

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/backend/test"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestBolt(t *testing.T) { TestingT(t) }

type BoltSuite struct {
	bk    *BoltBackend
	suite test.BackendSuite
	dir   string
}

var _ = Suite(&BoltSuite{})

func (s *BoltSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.bk, err = New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *BoltSuite) TearDownTest(c *C) {
	c.Assert(s.bk.Close(), IsNil)
}

func (s *BoltSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *BoltSuite) TestCompareAndSwap(c *C) {
	s.suite.CompareAndSwap(c)
}

func (s *BoltSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *BoltSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *BoltSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTl(c)
}
