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

	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

type EncryptedBkSuite struct {
	bk    *EncryptedBackend
	suite test.BackendSuite
	dir   string
	key   encryptor.Key
}

var _ = Suite(&EncryptedBkSuite{})

func (s *EncryptedBkSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
	var err error
	s.key, err = encryptor.GenerateGPGKey("key01")
	c.Assert(err, IsNil)
}

func (s *EncryptedBkSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	boltBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = newEncryptedBackend(boltBk, s.key, s.key,
		[]encryptor.Key{s.key.Public()})
	c.Assert(err, IsNil)

	s.suite.ChangesC = make(chan interface{})
	s.suite.B = s.bk
}

func (s *EncryptedBkSuite) TearDownTest(c *C) {
}

func (s *EncryptedBkSuite) TestBasicCRUD(c *C) {
	s.suite.BasicCRUD(c)
}

func (s *EncryptedBkSuite) TestCompareAndSwap(c *C) {
	s.suite.CompareAndSwap(c)
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
