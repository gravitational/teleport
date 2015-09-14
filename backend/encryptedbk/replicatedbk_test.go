package encryptedbk

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/backend/test"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestReplicatedBk(t *testing.T) { TestingT(t) }

type ReplicatedBkSuite struct {
	bk    *ReplicatedBackend
	suite test.BackendSuite
	dir   string
}

var _ = Suite(&ReplicatedBkSuite{})

func (s *ReplicatedBkSuite) SetUpSuite(c *C) {
	log.Initialize("console", "INFO")
}

func (s *ReplicatedBkSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	boltBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = NewReplicatedBackend(boltBk, filepath.Join(s.dir, "keysDB"), false)
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

func (s *ReplicatedBkSuite) TestValueAndTTL(c *C) {
	s.suite.ValueAndTTl(c)
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

func (s *ReplicatedBkSuite) TestSeveralKeys(c *C) {
	c.Assert(s.bk.UpsertVal([]string{"a1"}, "b1", []byte("val1"), 0), IsNil)
	c.Assert(s.bk.DeleteEncryptingKey("key0"), NotNil)

	c.Assert(s.bk.GenerateEncryptingKey("key2", true), IsNil)
	val, err := s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	c.Assert(s.bk.UpsertVal([]string{"a2"}, "b2", []byte("val2"), 0), IsNil)
	c.Assert(s.bk.GenerateEncryptingKey("key2", true), NotNil)

	key0, err := s.bk.GetEncryptingKey("key0")
	c.Assert(err, IsNil)

	c.Assert(s.bk.DeleteEncryptingKey("key0"), IsNil)
	val, err = s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	val, err = s.bk.GetVal([]string{"a2"}, "b2")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val2")
	c.Assert(s.bk.UpsertVal([]string{"a3"}, "b3", []byte("val3"), 0), IsNil)

	c.Assert(s.bk.GenerateEncryptingKey("key3", true), IsNil)
	val, err = s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	c.Assert(s.bk.UpsertVal([]string{"a4"}, "b4", []byte("val4"), 0), IsNil)

	localKeys, err := s.bk.GetAllEncryptingKeys()
	c.Assert(err, IsNil)
	localIDs := make([]string, len(localKeys))
	for i, _ := range localKeys {
		localIDs[i] = localKeys[i].ID
	}
	remoteIDs, err := s.bk.GetRemoteEncryptingKeys()
	c.Assert(err, IsNil)
	c.Assert(localIDs, DeepEquals, []string{"key2", "key3"})
	c.Assert(remoteIDs, DeepEquals, []string{"key2", "key3"})

	_, err = s.bk.GetVal([]string{"a10"}, "b10")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	c.Assert(s.bk.AddEncryptingKey(key0, true), IsNil)
	c.Assert(s.bk.AddEncryptingKey(key0, true), NotNil)
	val, err = s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	c.Assert(s.bk.UpsertVal([]string{"a5"}, "b5", []byte("val5"), 0), IsNil)

	localKeys, err = s.bk.GetAllEncryptingKeys()
	c.Assert(err, IsNil)
	localIDs = make([]string, len(localKeys))
	for i, _ := range localKeys {
		localIDs[i] = localKeys[i].ID
	}
	remoteIDs, err = s.bk.GetRemoteEncryptingKeys()
	c.Assert(err, IsNil)
	c.Assert(localIDs, DeepEquals, []string{"key0", "key2", "key3"})
	c.Assert(remoteIDs, DeepEquals, []string{"key0", "key2", "key3"})

	keysAreEqual1 := reflect.DeepEqual(localKeys[0].Value, localKeys[1].Value)
	keysAreEqual2 := reflect.DeepEqual(localKeys[0].Value, localKeys[2].Value)
	keysAreEqual3 := reflect.DeepEqual(localKeys[2].Value, localKeys[1].Value)
	c.Assert(keysAreEqual1, Equals, false)
	c.Assert(keysAreEqual2, Equals, false)
	c.Assert(keysAreEqual3, Equals, false)

	c.Assert(s.bk.DeleteEncryptingKey("key2"), IsNil)
	c.Assert(s.bk.DeleteEncryptingKey("key0"), IsNil)
	c.Assert(s.bk.DeleteEncryptingKey("key3"), NotNil)
}

func (s *ReplicatedBkSuite) TestSeveralAuthServers(c *C) {
	c.Assert(s.bk.UpsertVal([]string{"a1"}, "b1", []byte("val1"), 0), IsNil)
	c.Assert(s.bk.GenerateEncryptingKey("key/2", true), IsNil)
	key2, err := s.bk.GetEncryptingKey("key/2")
	c.Assert(err, IsNil)

	bk2, err := NewReplicatedBackend(s.bk.baseBk, filepath.Join(s.dir, "keysDB_2"), false)
	c.Assert(err, IsNil)

	localKeys, err := bk2.GetAllEncryptingKeys()
	c.Assert(err, IsNil)
	localIDs := make([]string, len(localKeys))
	for i, _ := range localKeys {
		localIDs[i] = localKeys[i].ID
	}
	remoteIDs, err := bk2.GetRemoteEncryptingKeys()
	c.Assert(err, IsNil)
	c.Assert(localIDs, DeepEquals, []string{})
	c.Assert(remoteIDs, DeepEquals, []string{"key/2", "key0"})

	x := 5
	go func() {
		time.Sleep(1 * time.Second)
		x = 7
		c.Assert(bk2.AddEncryptingKey(key2, true), IsNil)
	}()

	val, err := bk2.GetVal([]string{"a1"}, "b1")
	c.Assert(x, Equals, 7)
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")

	c.Assert(bk2.UpsertVal([]string{"a2"}, "b2", []byte("val2"), 0), NotNil)

	_, err = s.bk.GetVal([]string{"a2"}, "b2")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
	c.Assert(bk2.DeleteEncryptingKey("key/2"), NotNil)

	// making bk2 master

	c.Assert(bk2.DeleteEncryptingKey("key0"), IsNil)

	// after restarting bk2 should be master
	bk2.keyStorage.(*boltbk.BoltBackend).Close()
	bk2, err = NewReplicatedBackend(s.bk.baseBk, filepath.Join(s.dir, "keysDB_2"), false)

	val, err = bk2.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")

	c.Assert(bk2.UpsertVal([]string{"a2"}, "b2", []byte("val2"), 0), IsNil)
}
