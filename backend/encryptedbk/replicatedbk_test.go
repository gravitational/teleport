package encryptedbk

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/backend/test"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestReplicatedBk(t *testing.T) { TestingT(t) }

type ReplicatedBkSuite struct {
	bk    *ReplicatedBackend
	suite test.BackendSuite
	dir   string
}

var _ = Suite(&ReplicatedBkSuite{})

func (s *ReplicatedBkSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	boltBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = New(boltBk, filepath.Join(s.dir, "keysDB"))
	c.Assert(err, IsNil)

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *ReplicatedBkSuite) TearDownTest(c *C) {
}

func (s *ReplicatedBkSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *ReplicatedBkSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *ReplicatedBkSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *ReplicatedBkSuite) TestDataIsReplicated(c *C) {
	// saving value
	c.Assert(s.bk.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), 0), IsNil)

	// checking decrypted value
	out, err := s.bk.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")

	// checking value as it saved
	out, err = s.bk.baseBk.GetVal(append(s.bk.ebk[0].prefix, "a", "b"), "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out) == "val1", Equals, false)

}
