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

// Package secret implements a authenticated encryption with associated data
// (AEAD) cipher to be used when symmetric is required in Teleport. The
// underlying cipher is AES-GCM.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
)

type sealedData struct {
	Ciphertext []byte `json:"ciphertext"`
	Nonce      []byte `json:"nonce"`
}

// Key for the symmetric cipher.
type Key []byte

// NewKey generates a new key from a cryptographically secure pseudo-random
// number generator (CSPRNG).
func NewKey() (Key, error) {
	k := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, k)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return Key(k), nil
}

// ParseKey reads in an existing hex encoded key.
func ParseKey(k []byte) (Key, error) {
	key, err := hex.DecodeString(string(k))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return Key(key), nil
}

// Seal will encrypt then authenticate the ciphertext.
func (k Key) Seal(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher([]byte(k))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nonce := make([]byte, aesgcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ciphertext, err := json.Marshal(&sealedData{
		Ciphertext: aesgcm.Seal(nil, nonce, plaintext, nil),
		Nonce:      nonce,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ciphertext, nil
}

// Open will authenticate then decrypt the ciphertext.
func (k Key) Open(ciphertext []byte) ([]byte, error) {
	var data sealedData

	err := json.Unmarshal(ciphertext, &data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plaintext, err := aesgcm.Open(nil, data.Nonce, data.Ciphertext, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return plaintext, nil
}

// String returns the human-readable representation of the key.
func (k Key) String() string {
	return hex.EncodeToString(k)
}
