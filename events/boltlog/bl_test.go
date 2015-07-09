package boltlog

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/events/test"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestBolt(t *testing.T) { TestingT(t) }

type BoltLogSuite struct {
	l     *BoltLog
	suite test.EventSuite
	dir   string
}

var _ = Suite(&BoltLogSuite{})

func (s *BoltLogSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.l, err = New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)

	s.suite.L = s.l
}

func (s *BoltLogSuite) TearDownTest(c *C) {
	c.Assert(s.l.Close(), IsNil)
}

func (s *BoltLogSuite) TestEventsCRUD(c *C) {
	s.suite.EventsCRUD(c)
}
