package encryptedbk

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/backend/boltbk"
	"github.com/gravitational/teleport/backend/test"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestReplicatedBk(t *testing.T) { TestingT(t) }

type ReplicatedBkSuite struct {
	bk    *MasterReplicatedBackend
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
	s.bk, err = NewMasterReplicatedBackend(boltBk, filepath.Join(s.dir, "keysDB"), nil)
	c.Assert(err, IsNil)

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *ReplicatedBkSuite) TearDownTest(c *C) {
}

func (s *ReplicatedBkSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *ReplicatedBkSuite) TestCompareAndSwap(c *C) {
	s.suite.CompareAndSwap(c)
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

func (s *ReplicatedBkSuite) TestDataIsEncrypted(c *C) {
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

	keys, err := s.bk.GetLocalSealKeys()
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 1)
	key0ID := keys[0].ID

	c.Assert(s.bk.DeleteSealKey(key0ID), NotNil)

	val, err := s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")

	c.Assert(len(s.bk.signCheckingKeys), Equals, 1)

	key2, err := s.bk.GenerateSealKey("key2")

	///c.Assert(s.bk.ebk[1].encryptor.AddSignCheckingKey(keys[0]), IsNil) ///
	val, err = s.bk.ebk[1].GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")

	c.Assert(s.bk.ebk[0].VerifySign(), IsNil)
	c.Assert(s.bk.ebk[1].VerifySign(), IsNil)

	c.Assert(s.bk.RewriteData(), IsNil)

	c.Assert(err, IsNil)
	c.Assert(len(key2.ID) > 0, Equals, true)
	c.Assert(key2.Name, Equals, "key2")
	val, err = s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	c.Assert(s.bk.UpsertVal([]string{"a2"}, "b2", []byte("val2"), 0), IsNil)
	_, err = s.bk.GenerateSealKey("key2")
	c.Assert(err, NotNil)

	key0, err := s.bk.GetLocalSealKey(key0ID)
	c.Assert(err, IsNil)

	c.Assert(len(s.bk.signCheckingKeys), Equals, 2)

	//_, err = s.bk.GenerateSealKey("key5") //
	//c.Assert(err, IsNil)                  //

	c.Assert(s.bk.DeleteSealKey(key0ID), IsNil)
	//c.Assert(s.bk.DeleteSealKey(key2.ID), IsNil)
	c.Assert(len(s.bk.signCheckingKeys), Equals, 1)
	//val, err = s.bk.GetVal([]string{"a1"}, "b1")
	//c.Assert(string(val), Equals, "val1")
	//c.Assert(err, NotNil)

	//c.Assert(s.bk.RewriteData(), IsNil)

	val, err = s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	val, err = s.bk.GetVal([]string{"a2"}, "b2")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val2")
	c.Assert(s.bk.UpsertVal([]string{"a3"}, "b3", []byte("val3"), 0), IsNil)

	key3, err := s.bk.GenerateSealKey("key3")
	c.Assert(err, IsNil)
	c.Assert(len(key3.ID) > 0, Equals, true)
	val, err = s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	c.Assert(s.bk.UpsertVal([]string{"a4"}, "b4", []byte("val4"), 0), IsNil)

	localKeys, err := s.bk.GetLocalSealKeys()
	c.Assert(err, IsNil)
	localIDs := make([]string, len(localKeys))
	for i, _ := range localKeys {
		localIDs[i] = localKeys[i].ID
	}
	if localIDs[0] == key2.ID {
		c.Assert(localIDs, DeepEquals, []string{key2.ID, key3.ID})
	} else {
		c.Assert(localIDs, DeepEquals, []string{key3.ID, key2.ID})
	}

	remoteKeys, err := s.bk.GetClusterPublicSealKeys()
	c.Assert(err, IsNil)
	remoteIDs := make([]string, len(remoteKeys))
	for i, _ := range remoteKeys {
		remoteIDs[i] = remoteKeys[i].ID
	}
	if remoteIDs[0] == key2.ID {
		c.Assert(remoteIDs, DeepEquals, []string{key2.ID, key3.ID})
	} else {
		c.Assert(remoteIDs, DeepEquals, []string{key3.ID, key2.ID})
	}

	_, err = s.bk.GetVal([]string{"a10"}, "b10")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	c.Assert(s.bk.AddSealKey(key0), IsNil)
	c.Assert(s.bk.AddSealKey(key0), NotNil)
	val, err = s.bk.GetVal([]string{"a1"}, "b1")
	c.Assert(err, IsNil)
	c.Assert(string(val), Equals, "val1")
	c.Assert(s.bk.UpsertVal([]string{"a5"}, "b5", []byte("val5"), 0), IsNil)

	localKeys, err = s.bk.GetLocalSealKeys()
	c.Assert(err, IsNil)
	localIDs = make([]string, len(localKeys))
	for i, _ := range localKeys {
		localIDs[i] = localKeys[i].ID
	}
	remoteKeys, err = s.bk.GetClusterPublicSealKeys()
	c.Assert(err, IsNil)
	remoteIDs = make([]string, len(remoteKeys))
	for i, _ := range remoteKeys {
		remoteIDs[i] = remoteKeys[i].ID
	}

	expectedIDs := []string{key0.ID, key2.ID, key3.ID}
	sort.Strings(expectedIDs)
	sort.Strings(localIDs)
	sort.Strings(remoteIDs)

	c.Assert(localIDs, DeepEquals, expectedIDs)
	c.Assert(remoteIDs, DeepEquals, expectedIDs)

	c.Assert(s.bk.DeleteSealKey(key2.ID), IsNil)
	c.Assert(s.bk.DeleteSealKey(key0.ID), IsNil)
	c.Assert(s.bk.DeleteSealKey(key3.ID), NotNil)
}

func (s *ReplicatedBkSuite) TestSeveralAuthServers(c *C) {
	/*c.Assert(s.bk.UpsertVal([]string{"a1"}, "b1", []byte("val1"), 0), IsNil)

	bk2, err := NewMasterReplicatedBackend(s.bk.baseBk, filepath.Join(s.dir, "keysDB_2"), nil)
	c.Assert(err, IsNil)


	key2description, err := s.bk.GenerateSealKey("key/2")
	c.Assert(err, IsNil)
	key2, err := s.bk.GetLocalSealKey(key2description.ID)
	c.Assert(err, IsNil)


	localKeys, err := bk2.GetLocalSealKeys()
	c.Assert(err, IsNil)
	localIDs := make([]string, len(localKeys))
	for i, _ := range localKeys {
		localIDs[i] = localKeys[i].ID
	}
	remoteKeys, err := bk2.GetClusterSealKeys()
	c.Assert(err, IsNil)
	remoteIDs := make([]string, len(remoteKeys))
	for i, _ := range localKeys {
		localIDs[i] = localKeys[i].ID
	}
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

	c.Assert(bk2.UpsertVal([]string{"a2"}, "b2", []byte("val2"), 0), IsNil)*/
}
