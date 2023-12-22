/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

	// Attempting to call aesgcm.Open with a invalid nonce will cause it to panic.
	// To make sure that doesn't happen even handling invalid ciphertext
	// (for example, for legacy secret package or attacker controlled data),
	// reject invalid sized nonces.
	if len(data.Nonce) != aesgcm.NonceSize() {
		return nil, trace.BadParameter("invalid nonce sice, only %v supported", aesgcm.NonceSize())
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
