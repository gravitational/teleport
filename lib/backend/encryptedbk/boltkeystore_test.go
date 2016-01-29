/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package encryptedbk

import (
	"path/filepath"
	"reflect"

	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"

	. "gopkg.in/check.v1"
)

type KeyStoreSuite struct {
	ks  KeyStore
	dir string
}

var _ = Suite(&KeyStoreSuite{})

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

func (s *KeyStoreSuite) TestKeyToString(c *C) {
	key1 := encryptor.Key{
		Name:  "name3",
		ID:    "key3",
		Value: []byte("value3"),
	}

	b64key, err := KeyToString(key1)
	c.Assert(err, IsNil)

	key2, err := KeyFromString(b64key)
	c.Assert(err, IsNil)
	c.Assert(key2.Name, Equals, key1.Name)
	c.Assert(key2.ID, Equals, key1.ID)
	c.Assert(key2.Value, DeepEquals, key2.Value)
}
