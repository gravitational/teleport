package fs

import (
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"gopkg.in/check.v1"
)

type Suite struct {
	dirName string
	bk      backend.Backend
}

var _ = check.Suite(&Suite{})

// bootstrap check.v1:
func TestFSBackend(t *testing.T) { check.TestingT(t) }

func (s *Suite) SetUpSuite(c *check.C) {
	dirName := c.MkDir()
	bk, err := FromJSON(fmt.Sprintf(`{ "path": "%s" }`, dirName))

	c.Assert(err, check.IsNil)
	c.Assert(bk.Path, check.Equals, dirName)
	c.Assert(utils.IsDir(bk.Path), check.Equals, true)

	s.bk = bk
}

func (s *Suite) TestLocking(c *check.C) {
	fmt.Println("Check locking", s.dirName)
}

func (s *Suite) TestCreateAndRead(c *check.C) {
	path := []string{"one", "two"}

	// must succeed:
	err := s.bk.CreateVal(path, "key", []byte("original"), backend.Forever)
	c.Assert(err, check.IsNil)

	// must get 'already exists' error
	err = s.bk.CreateVal(path, "key", []byte("failed-write"), backend.Forever)
	c.Assert(trace.IsAlreadyExists(err), check.Equals, true)

	// read back the original:
	val, err := s.bk.GetVal(path, "key")
	c.Assert(err, check.IsNil)
	c.Assert(string(val), check.Equals, "original")

	// upsert:
	err = s.bk.UpsertVal(path, "key", []byte("new-value"), backend.Forever)
	c.Assert(err, check.IsNil)

	// read back the new value:
	val, err = s.bk.GetVal(path, "key")
	c.Assert(err, check.IsNil)
	c.Assert(string(val), check.Equals, "new-value")
}

func (s *Suite) TestListDelete(c *check.C) {
	root := []string{"root"}
	kid := []string{"root", "kid"}

	// create two entries in root:
	s.bk.CreateVal(root, "one", []byte("1"), backend.Forever)
	s.bk.CreateVal(root, "two", []byte("2"), backend.Forever)

	// create one entry in the kid:
	s.bk.CreateVal(kid, "three", []byte("3"), backend.Forever)

	// list the root (should get 2 back):
	kids, err := s.bk.GetKeys(root)
	c.Assert(err, check.IsNil)
	c.Assert(kids, check.HasLen, 2)
	c.Assert(kids[0], check.Equals, "one")
	c.Assert(kids[1], check.Equals, "two")

	// list the kid (should get 1)
	kids, err = s.bk.GetKeys(kid)
	c.Assert(err, check.IsNil)
	c.Assert(kids, check.HasLen, 1)
	c.Assert(kids[0], check.Equals, "three")

	// delete one of the kids:
	err = s.bk.DeleteKey(kid, "three")
	c.Assert(err, check.IsNil)
	kids, err = s.bk.GetKeys(kid)
	c.Assert(kids, check.HasLen, 0)

	// try to delete non-existing key:
	err = s.bk.DeleteKey(kid, "three")
	c.Assert(trace.IsNotFound(err), check.Equals, true)

	// try to delete the root bucket:
	err = s.bk.DeleteBucket(root, "kid")
	c.Assert(err, check.IsNil)

	// try to list non-existing:
	_, err = s.bk.GetKeys(kid)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}
