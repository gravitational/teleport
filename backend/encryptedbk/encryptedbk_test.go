package encryptedbk

import (
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/backend/test"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestEncryptedBk(t *testing.T) { TestingT(t) }

type EncryptedBkSuite struct {
	bk    *EncryptedBackend
	suite test.BackendSuite
	dir   string
}

var _ = Suite(&EncryptedBkSuite{})

func (s *EncryptedBkSuite) SetUpSuite(c *C) {
	log.Initialize("console", "WARN")
}

func (s *EncryptedBkSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	boltBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = newEncryptedBackend(boltBk, Key{ID: "key25", Value: key[:]})
	c.Assert(err, IsNil)

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *EncryptedBkSuite) TearDownTest(c *C) {
}

func (s *EncryptedBkSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *EncryptedBkSuite) TestExpiration(c *C) {
	s.suite.Expiration(c)
}

func (s *EncryptedBkSuite) TestLock(c *C) {
	s.suite.Locking(c)
}

func (s *EncryptedBkSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTl(c)
}

func (s *EncryptedBkSuite) TestDataIsEncrypted(c *C) {
	// saving value
	c.Assert(s.bk.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), 0), IsNil)

	// checking decrypted value
	out, err := s.bk.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")

	// checking value as it saved
	out, err = s.bk.bk.GetVal(append(s.bk.prefix, "a", "b"), "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out) == "val1", Equals, false)

}
