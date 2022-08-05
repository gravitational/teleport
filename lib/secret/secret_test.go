/*
Copyright 2019 Gravitational, Inc.

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

package secret

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestSecret(t *testing.T) { check.TestingT(t) }

type SecretSuite struct{}

var _ = check.Suite(&SecretSuite{})

// TestKey checks a various key operations like new key generation and parsing.
func (s *SecretSuite) TestKey(c *check.C) {
	// Keys should be 32-bytes.
	key, err := NewKey()
	c.Assert(err, check.IsNil)
	c.Assert(key, check.HasLen, 32)

	// ParseKey should be able to load and key and use it to Open something
	// sealed by the original key.
	ciphertext, err := key.Seal([]byte("hello, world"))
	c.Assert(err, check.IsNil)
	pkey, err := ParseKey([]byte(key.String()))
	c.Assert(err, check.IsNil)
	plaintext, err := pkey.Open(ciphertext)
	c.Assert(err, check.IsNil)
	c.Assert(plaintext, check.DeepEquals, []byte("hello, world"))

	// NewKey should always return a new key.
	key1, err := NewKey()
	c.Assert(err, check.IsNil)
	key2, err := NewKey()
	c.Assert(err, check.IsNil)
	c.Assert(key1, check.Not(check.DeepEquals), key2)
}

// TestSeal makes sure calling Seal on the same data with the same key
// results in different ciphertexts and nonces.
func (s *SecretSuite) TestSeal(c *check.C) {
	key, err := NewKey()
	c.Assert(err, check.IsNil)

	plaintext := []byte("hello, world")

	ciphertext1, err := key.Seal(plaintext)
	c.Assert(err, check.IsNil)
	var data1 sealedData
	err = json.Unmarshal(ciphertext1, &data1)
	c.Assert(err, check.IsNil)

	ciphertext2, err := key.Seal(plaintext)
	c.Assert(err, check.IsNil)
	var data2 sealedData
	err = json.Unmarshal(ciphertext2, &data2)
	c.Assert(err, check.IsNil)

	// Ciphertext and nonce for the same plaintext should be different each time
	// Seal is called.
	c.Assert(data1.Ciphertext, check.Not(check.DeepEquals), data2.Ciphertext)
	c.Assert(data1.Nonce, check.Not(check.DeepEquals), data2.Nonce)

	// The plaintext for both should be the same and match the original.
	plaintext1, err := key.Open(ciphertext1)
	c.Assert(err, check.IsNil)
	plaintext2, err := key.Open(ciphertext2)
	c.Assert(err, check.IsNil)
	c.Assert(plaintext, check.DeepEquals, plaintext1)
	c.Assert(plaintext, check.DeepEquals, plaintext2)
}

// TestOpen makes sure data that was sealed with a key can only be opened
// with the same key.
func (s *SecretSuite) TestOpen(c *check.C) {
	key1, err := NewKey()
	c.Assert(err, check.IsNil)

	ciphertext, err := key1.Seal([]byte("hello, world"))
	c.Assert(err, check.IsNil)

	// Trying to call Open with a different key should always fail.
	key2, err := NewKey()
	c.Assert(err, check.IsNil)
	_, err = key2.Open(ciphertext)
	c.Assert(err, check.NotNil)

	// Calling Open with the same key should work.
	plaintext, err := key1.Open(ciphertext)
	c.Assert(err, check.IsNil)
	c.Assert(plaintext, check.DeepEquals, []byte("hello, world"))
}
