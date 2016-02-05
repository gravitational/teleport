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
package encryptor

import (
	. "gopkg.in/check.v1"
	"reflect"

	"github.com/gravitational/teleport/lib/utils"
)

type naclSuite struct {
	suite encryptorSuite
}

var _ = Suite(&naclSuite{})

func (s *naclSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
}

func (s *naclSuite) SetUpTest(c *C) {
	key, err := GenerateNaClKey("key1")
	c.Assert(err, IsNil)

	s.suite.E, err = NewNaClEncryptor(key)
	c.Assert(err, IsNil)
}

func (s *naclSuite) TestEncryptDecrypt(c *C) {
	s.suite.EncryptDecrypt(c)
}

func (s *naclSuite) TestGenerateKey(c *C) {
	key1, err := GenerateNaClKey("key1")
	c.Assert(err, IsNil)
	key2, err := GenerateNaClKey("key2")
	c.Assert(err, IsNil)
	key3, err := GenerateNaClKey("key3")
	c.Assert(err, IsNil)

	c.Assert(key1.Name, Equals, "key1")
	c.Assert(key2.Name, Equals, "key2")
	c.Assert(key3.Name, Equals, "key3")

	c.Assert(reflect.DeepEqual(key1.ID, key2.ID), Equals, false)
	c.Assert(reflect.DeepEqual(key1.ID, key3.ID), Equals, false)
	c.Assert(reflect.DeepEqual(key2.ID, key3.ID), Equals, false)

	c.Assert(reflect.DeepEqual(key1.Value, key2.Value), Equals, false)
	c.Assert(reflect.DeepEqual(key1.Value, key3.Value), Equals, false)
	c.Assert(reflect.DeepEqual(key2.Value, key3.Value), Equals, false)
}
