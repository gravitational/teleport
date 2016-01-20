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
	"testing"
)

func TestEncryptor(t *testing.T) { TestingT(t) }

type gpgSuite struct {
	E *GPGEncryptor
}

var _ = Suite(&gpgSuite{})

func (s *gpgSuite) SetUpTest(c *C) {
	key, err := GenerateGPGKey("key1")
	c.Assert(err, IsNil)

	s.E, err = NewGPGEncryptor(key)
	c.Assert(err, IsNil)

	err = s.E.SetSignKey(key)
	c.Assert(err, IsNil)
	err = s.E.AddSignCheckingKey(key)
	c.Assert(err, IsNil)
}

func (s *gpgSuite) TestEncryptDecrypt(c *C) {
	s1 := "first string"
	s2 := "second"
	s3 := "third text"

	e1, err := s.E.Encrypt([]byte(s1))
	c.Assert(err, IsNil)
	e11, err := s.E.Encrypt([]byte(s1))
	c.Assert(err, IsNil)
	e2, err := s.E.Encrypt([]byte(s2))
	c.Assert(err, IsNil)
	e3, err := s.E.Encrypt([]byte(s3))
	c.Assert(err, IsNil)

	c.Assert(s1 == string(e1), Equals, false)
	c.Assert(s2 == string(e2), Equals, false)
	c.Assert(s3 == string(e3), Equals, false)

	c.Assert(string(e1) == string(e2), Equals, false)
	c.Assert(string(e1) == string(e3), Equals, false)
	c.Assert(string(e2) == string(e3), Equals, false)

	d1, err := s.E.Decrypt([]byte(e1))
	c.Assert(err, IsNil)
	d11, err := s.E.Decrypt([]byte(e11))
	c.Assert(err, IsNil)
	d2, err := s.E.Decrypt([]byte(e2))
	c.Assert(err, IsNil)
	d22, err := s.E.Decrypt([]byte(e2))
	c.Assert(err, IsNil)
	d3, err := s.E.Decrypt([]byte(e3))
	c.Assert(err, IsNil)

	c.Assert(s1, DeepEquals, string(d1))
	c.Assert(s1, DeepEquals, string(d11))
	c.Assert(s2, DeepEquals, string(d2))
	c.Assert(s2, DeepEquals, string(d22))
	c.Assert(s3, DeepEquals, string(d3))
}

func (s *gpgSuite) TestSigning(c *C) {
	key1, err := GenerateGPGKey("key1")
	c.Assert(err, IsNil)
	sign1, err := GenerateGPGKey("sign1")
	c.Assert(err, IsNil)
	sign2, err := GenerateGPGKey("sign2")
	c.Assert(err, IsNil)

	gpg1, err := NewGPGEncryptor(key1)
	c.Assert(err, IsNil)
	gpg2, err := NewGPGEncryptor(key1)
	c.Assert(err, IsNil)
	gpg3, err := NewGPGEncryptor(key1)
	c.Assert(err, IsNil)

	s1 := "first string"

	e1, err := gpg1.Encrypt([]byte(s1))
	c.Assert(err, IsNil)
	_, err = gpg1.Decrypt(e1)
	c.Assert(err, NotNil)

	err = gpg1.SetSignKey(sign1)
	c.Assert(err, IsNil)

	e1, err = gpg1.Encrypt([]byte(s1))
	c.Assert(err, IsNil)
	_, err = gpg1.Decrypt(e1)
	c.Assert(err, NotNil)

	err = gpg1.AddSignCheckingKey(sign2)
	c.Assert(err, IsNil)

	_, err = gpg1.Decrypt(e1)
	c.Assert(err, NotNil)

	err = gpg1.AddSignCheckingKey(sign1)
	c.Assert(err, IsNil)

	d1, err := gpg1.Decrypt(e1)
	c.Assert(err, IsNil)
	c.Assert(string(d1), Equals, s1)

	_, err = gpg2.Decrypt(e1)
	c.Assert(err, NotNil)

	err = gpg2.AddSignCheckingKey(sign1)
	c.Assert(err, IsNil)

	d2, err := gpg2.Decrypt(e1)
	c.Assert(err, IsNil)
	c.Assert(string(d2), Equals, s1)

	err = gpg2.AddSignCheckingKey(sign2)
	c.Assert(err, IsNil)

	d22, err := gpg2.Decrypt(e1)
	c.Assert(err, IsNil)
	c.Assert(string(d22), Equals, s1)

	err = gpg3.AddSignCheckingKey(sign2)
	c.Assert(err, IsNil)

	_, err = gpg3.Decrypt(e1)
	c.Assert(err, NotNil)

	err = gpg3.AddSignCheckingKey(sign2)
	c.Assert(err, IsNil)

	_, err = gpg3.Decrypt(e1)
	c.Assert(err, NotNil)

	err = gpg3.AddSignCheckingKey(sign1)
	c.Assert(err, IsNil)

	d3, err := gpg3.Decrypt(e1)
	c.Assert(err, IsNil)
	c.Assert(string(d3), Equals, s1)

	e2, err := gpg2.Encrypt([]byte(s1))
	c.Assert(err, IsNil)
	e3, err := gpg3.Encrypt([]byte(s1))
	c.Assert(err, IsNil)

	_, err = gpg1.Decrypt(e2)
	c.Assert(err, NotNil)
	_, err = gpg1.Decrypt(e3)
	c.Assert(err, NotNil)

	err = gpg3.DeleteSignCheckingKey(sign2.ID)
	c.Assert(err, IsNil)

	d4, err := gpg3.Decrypt(e1)
	c.Assert(err, IsNil)
	c.Assert(string(d4), Equals, s1)

	err = gpg3.DeleteSignCheckingKey(sign1.ID)
	c.Assert(err, IsNil)

	_, err = gpg3.Decrypt(e1)
	c.Assert(err, NotNil)

	err = gpg1.DeleteSignCheckingKey(sign1.ID)
	c.Assert(err, IsNil)

	_, err = gpg1.Decrypt(e1)
	c.Assert(err, NotNil)

}

func (s *gpgSuite) TestGenerateKey(c *C) {
	key1, err := GenerateGPGKey("key1")
	c.Assert(err, IsNil)
	key2, err := GenerateGPGKey("key2")
	c.Assert(err, IsNil)
	key3, err := GenerateGPGKey("key3")
	c.Assert(err, IsNil)

	c.Assert(key1.Name, Equals, "key1")
	c.Assert(key2.Name, Equals, "key2")
	c.Assert(key3.Name, Equals, "key3")

	c.Assert(reflect.DeepEqual(key1.ID, key2.ID), Equals, false)
	c.Assert(reflect.DeepEqual(key1.ID, key3.ID), Equals, false)
	c.Assert(reflect.DeepEqual(key2.ID, key3.ID), Equals, false)

	c.Assert(reflect.DeepEqual(key1.PrivateValue, key1.PublicValue), Equals, false)
	c.Assert(reflect.DeepEqual(key3.PrivateValue, key2.PublicValue), Equals, false)
	c.Assert(reflect.DeepEqual(key3.PrivateValue, key3.PublicValue), Equals, false)

	c.Assert(reflect.DeepEqual(key1.PrivateValue, key2.PrivateValue), Equals, false)
	c.Assert(reflect.DeepEqual(key1.PrivateValue, key3.PrivateValue), Equals, false)
	c.Assert(reflect.DeepEqual(key2.PrivateValue, key3.PrivateValue), Equals, false)

	c.Assert(reflect.DeepEqual(key1.PublicValue, key2.PublicValue), Equals, false)
	c.Assert(reflect.DeepEqual(key1.PublicValue, key3.PublicValue), Equals, false)
	c.Assert(reflect.DeepEqual(key2.PublicValue, key3.PublicValue), Equals, false)
}
