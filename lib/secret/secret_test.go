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

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// TestKey checks a various key operations like new key generation and parsing.
func TestKey(t *testing.T) {
	// Keys should be 32-bytes.
	key, err := NewKey()
	require.NoError(t, err)
	require.Len(t, key, 32)

	// ParseKey should be able to load and key and use it to Open something
	// sealed by the original key.
	ciphertext, err := key.Seal([]byte("hello, world"))
	require.NoError(t, err)
	pkey, err := ParseKey([]byte(key.String()))
	require.NoError(t, err)
	plaintext, err := pkey.Open(ciphertext)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(plaintext, []byte("hello, world")))

	// NewKey should always return a new key.
	key1, err := NewKey()
	require.NoError(t, err)
	key2, err := NewKey()
	require.NoError(t, err)
	require.NotEmpty(t, cmp.Diff(key1, key2))
}

// TestSeal makes sure calling Seal on the same data with the same key
// results in different ciphertexts and nonces.
func TestSeal(t *testing.T) {
	key, err := NewKey()
	require.NoError(t, err)

	plaintext := []byte("hello, world")

	ciphertext1, err := key.Seal(plaintext)
	require.NoError(t, err)
	var data1 sealedData
	err = json.Unmarshal(ciphertext1, &data1)
	require.NoError(t, err)

	ciphertext2, err := key.Seal(plaintext)
	require.NoError(t, err)
	var data2 sealedData
	err = json.Unmarshal(ciphertext2, &data2)
	require.NoError(t, err)

	// Ciphertext and nonce for the same plaintext should be different each time
	// Seal is called.
	require.NotEmpty(t, cmp.Diff(data1.Ciphertext, data2.Ciphertext))
	require.NotEmpty(t, cmp.Diff(data1.Nonce, data2.Nonce))

	// The plaintext for both should be the same and match the original.
	plaintext1, err := key.Open(ciphertext1)
	require.NoError(t, err)
	plaintext2, err := key.Open(ciphertext2)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(plaintext, plaintext1))
	require.Empty(t, cmp.Diff(plaintext, plaintext2))
}

// TestOpen makes sure data that was sealed with a key can only be opened
// with the same key.
func TestOpen(t *testing.T) {
	key1, err := NewKey()
	require.NoError(t, err)

	ciphertext, err := key1.Seal([]byte("hello, world"))
	require.NoError(t, err)

	// Trying to call Open with a different key should always fail.
	key2, err := NewKey()
	require.NoError(t, err)
	_, err = key2.Open(ciphertext)
	require.Error(t, err)

	// Calling Open with the same key should work.
	plaintext, err := key1.Open(ciphertext)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(plaintext, []byte("hello, world")))
}
