// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
////////////////////////////////////////////////////////////////////////////////

package subtle

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"

	"github.com/google/tink/go/subtle/random"
)

const (
	// AESCTRMinIVSize is the minimum IV size that this implementation supports.
	AESCTRMinIVSize = 12
)

// AESCTR is an implementation of AEAD interface.
type AESCTR struct {
	Key    []byte
	IVSize int
}

// NewAESCTR returns an AESCTR instance.
// The key argument should be the AES key, either 16 or 32 bytes to select
// AES-128 or AES-256.
// ivSize specifies the size of the IV in bytes.
func NewAESCTR(key []byte, ivSize int) (*AESCTR, error) {
	keySize := uint32(len(key))
	if err := ValidateAESKeySize(keySize); err != nil {
		return nil, fmt.Errorf("aes_ctr: %s", err)
	}
	if ivSize < AESCTRMinIVSize || ivSize > aes.BlockSize {
		return nil, fmt.Errorf("aes_ctr: invalid IV size: %d", ivSize)
	}
	return &AESCTR{Key: key, IVSize: ivSize}, nil
}

// Encrypt encrypts plaintext using AES in CTR mode.
// The resulting ciphertext consists of two parts:
// (1) the IV used for encryption and (2) the actual ciphertext.
func (a *AESCTR) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) > maxInt-a.IVSize {
		return nil, fmt.Errorf("aes_ctr: plaintext too long")
	}
	iv := a.newIV()
	stream, err := newCipher(a.Key, iv)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, a.IVSize+len(plaintext))
	if n := copy(ciphertext, iv); n != a.IVSize {
		return nil, fmt.Errorf("aes_ctr: failed to copy IV (copied %d/%d bytes)", n, a.IVSize)
	}

	stream.XORKeyStream(ciphertext[a.IVSize:], plaintext)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext.
func (a *AESCTR) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < a.IVSize {
		return nil, fmt.Errorf("aes_ctr: ciphertext too short")
	}

	iv := ciphertext[:a.IVSize]
	stream, err := newCipher(a.Key, iv)
	if err != nil {
		return nil, err
	}

	plaintext := make([]byte, len(ciphertext)-a.IVSize)
	stream.XORKeyStream(plaintext, ciphertext[a.IVSize:])
	return plaintext, nil
}

// newIV creates a new IV for encryption.
func (a *AESCTR) newIV() []byte {
	return random.GetRandomBytes(uint32(a.IVSize))
}

// newCipher creates a new AES-CTR cipher using the given key, IV and the crypto library.
func newCipher(key, iv []byte) (cipher.Stream, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes_ctr: failed to create block cipher, error: %v", err)
	}

	// If the IV is less than BlockSize bytes we need to pad it with zeros
	// otherwise NewCTR will panic.
	if len(iv) < aes.BlockSize {
		paddedIV := make([]byte, aes.BlockSize)
		if n := copy(paddedIV, iv); n != len(iv) {
			return nil, fmt.Errorf("aes_ctr: failed to pad IV")
		}
		return cipher.NewCTR(block, paddedIV), nil
	}

	return cipher.NewCTR(block, iv), nil
}
