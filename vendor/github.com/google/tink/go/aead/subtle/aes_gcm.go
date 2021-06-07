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
	"github.com/google/tink/go/tink"
)

const (
	// AESGCMIVSize is the only IV size that this implementation supports.
	AESGCMIVSize = 12
	// AESGCMTagSize is the only tag size that this implementation supports.
	AESGCMTagSize          = 16
	maxAESGCMPlaintextSize = (1 << 36) - 32
)

// AESGCM is an implementation of AEAD interface.
type AESGCM struct {
	Key []byte
}

// Assert that AESGCM implements the AEAD interface.
var _ tink.AEAD = (*AESGCM)(nil)

// NewAESGCM returns an AESGCM instance.
// The key argument should be the AES key, either 16 or 32 bytes to select
// AES-128 or AES-256.
func NewAESGCM(key []byte) (*AESGCM, error) {
	keySize := uint32(len(key))
	if err := ValidateAESKeySize(keySize); err != nil {
		return nil, fmt.Errorf("aes_gcm: %s", err)
	}
	return &AESGCM{Key: key}, nil
}

// Encrypt encrypts pt with aad as additional authenticated data.
// The resulting ciphertext consists of two parts:
// (1) the IV used for encryption and (2) the actual ciphertext.
//
// Note: AES-GCM implementation of crypto library always returns ciphertext with
// 128-bit tag.
func (a *AESGCM) Encrypt(pt, aad []byte) ([]byte, error) {
	// Although Seal() function already checks for plaintext length,
	// this check is repeated here to avoid panic.
	if len(pt) > maxPtSize() {
		return nil, fmt.Errorf("aes_gcm: plaintext too long")
	}
	cipher, err := a.newCipher(a.Key)
	if err != nil {
		return nil, err
	}
	iv := a.newIV()
	ct := cipher.Seal(nil, iv, pt, aad)
	return append(iv, ct...), nil
}

// Decrypt decrypts ct with aad as the additional authenticated data.
func (a *AESGCM) Decrypt(ct, aad []byte) ([]byte, error) {
	if len(ct) < AESGCMIVSize+AESGCMTagSize {
		return nil, fmt.Errorf("aes_gcm: ciphertext too short")
	}
	cipher, err := a.newCipher(a.Key)
	if err != nil {
		return nil, err
	}
	iv := ct[:AESGCMIVSize]
	pt, err := cipher.Open(nil, iv, ct[AESGCMIVSize:], aad)
	if err != nil {
		return nil, fmt.Errorf("aes_gcm: %s", err)
	}
	return pt, nil
}

// newIV creates a new IV for encryption.
func (a *AESGCM) newIV() []byte {
	return random.GetRandomBytes(AESGCMIVSize)
}

var errCipher = fmt.Errorf("aes_gcm: initializing cipher failed")

// newCipher creates a new AES-GCM cipher using the given key and the crypto library.
func (a *AESGCM) newCipher(key []byte) (cipher.AEAD, error) {
	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, errCipher
	}
	ret, err := cipher.NewGCM(aesCipher)
	if err != nil {
		return nil, errCipher
	}
	return ret, nil
}

func maxPtSize() int {
	x := maxInt - AESGCMIVSize - AESGCMTagSize
	if x > maxAESGCMPlaintextSize {
		return maxAESGCMPlaintextSize
	}
	return x
}
