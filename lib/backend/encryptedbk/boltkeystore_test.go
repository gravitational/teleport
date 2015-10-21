package encryptedbk

import (
	"path/filepath"
	"reflect"

	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type KeyStoreSuite struct {
	ks  KeyStore
	dir string
}

var _ = Suite(&KeyStoreSuite{})

func (s *KeyStoreSuite) SetUpSuite(c *C) {
	log.Initialize("console", "WARN")
}

func (s *KeyStoreSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()
	var err error
	s.ks, err = NewKeyStore(filepath.Join(s.dir, "keystore.db"))
	c.Assert(err, IsNil)
}

func (s *KeyStoreSuite) TearDownTest(c *C) {
}

func (s *KeyStoreSuite) TestBasicCRUD(c *C) {
	key1 := encryptor.Key{
		Name:  "name1",
		ID:    "key1",
		Value: []byte("value1"),
	}
	key2 := encryptor.Key{
		Name:  "name2",
		ID:    "key2",
		Value: []byte("value2"),
	}

	c.Assert(s.ks.AddKey(key1), IsNil)
	c.Assert(s.ks.AddKey(key2), IsNil)

	ids, err := s.ks.GetKeys()
	c.Assert(err, IsNil)
	if reflect.DeepEqual(ids[0], key1) {
		c.Assert(ids, DeepEquals, []encryptor.Key{key1, key2})
	} else {
		c.Assert(ids, DeepEquals, []encryptor.Key{key2, key1})
	}

	k1, err := s.ks.GetKey(key1.ID)
	c.Assert(err, IsNil)
	c.Assert(k1, DeepEquals, key1)

	k2, err := s.ks.GetKey(key2.ID)
	c.Assert(err, IsNil)
	c.Assert(k2, DeepEquals, key2)

	c.Assert(s.ks.DeleteKey(key1.ID), IsNil)
	c.Assert(s.ks.HasKey(key1.ID), Equals, false)
	c.Assert(s.ks.HasKey(key2.ID), Equals, true)

	ids, err = s.ks.GetKeys()
	c.Assert(err, IsNil)
	c.Assert(ids, DeepEquals, []encryptor.Key{key2})
}
