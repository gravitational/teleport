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
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/tink/go/tink"
)

// EncryptThenAuthenticate performs an encrypt-then-MAC operation on plaintext
// and additional authenticated data (aad). The MAC is computed over (aad ||
// ciphertext || size of aad). This implementation is based on
// http://tools.ietf.org/html/draft-mcgrew-aead-aes-cbc-hmac-sha2-05.
type EncryptThenAuthenticate struct {
	indCPACipher INDCPACipher
	mac          tink.MAC
	tagSize      int
}

const (
	minTagSizeInBytes = 10
)

// Assert that EncryptThenAuthenticate implements the AEAD interface.
var _ tink.AEAD = (*EncryptThenAuthenticate)(nil)

// uint64ToByte stores a uint64 to a slice of bytes in big endian format.
func uint64ToByte(n uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, n)
	return buf
}

// NewEncryptThenAuthenticate returns a new instance of EncryptThenAuthenticate.
func NewEncryptThenAuthenticate(indCPACipher INDCPACipher, mac tink.MAC, tagSize int) (*EncryptThenAuthenticate, error) {
	if tagSize < minTagSizeInBytes {
		return nil, fmt.Errorf("encrypt_then_authenticate: tag size too small")
	}
	return &EncryptThenAuthenticate{indCPACipher, mac, tagSize}, nil
}

// Encrypt encrypts plaintext with additionalData as additional authenticated
// data. The resulting ciphertext allows for checking authenticity and
// integrity of additional data, but does not guarantee its secrecy.
//
// The plaintext is encrypted with an INDCPACipher, then MAC is computed over
// (additionalData || ciphertext || n) where n is additionalData's length
// in bits represented as a 64-bit bigendian unsigned integer. The final
// ciphertext format is (IND-CPA ciphertext || mac).
func (e *EncryptThenAuthenticate) Encrypt(plaintext, additionalData []byte) ([]byte, error) {
	ciphertext, err := e.indCPACipher.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt_then_authenticate: %v", err)
	}

	toAuthData := append(additionalData, ciphertext...)
	aadSizeInBits := uint64(len(additionalData)) * 8
	toAuthData = append(toAuthData, uint64ToByte(aadSizeInBits)...)

	tag, err := e.mac.ComputeMAC(toAuthData)
	if err != nil {
		return nil, fmt.Errorf("encrypt_then_authenticate: %v", err)
	}

	if len(tag) != e.tagSize {
		return nil, errors.New("encrypt_then_authenticate: invalid tag size")
	}

	ciphertext = append(ciphertext, tag...)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext with additionalData as additional authenticated
// data.
func (e *EncryptThenAuthenticate) Decrypt(ciphertext, additionalData []byte) ([]byte, error) {
	if len(ciphertext) < e.tagSize {
		return nil, errors.New("ciphertext too short")
	}

	// payload contains everything except the tag.
	payload := ciphertext[:len(ciphertext)-e.tagSize]

	// Authenticate the following data:
	// additionalData || payload || aadSizeInBits
	toAuthData := append(additionalData, payload...)
	aadSizeInBits := uint64(len(additionalData)) * 8
	toAuthData = append(toAuthData, uint64ToByte(aadSizeInBits)...)

	err := e.mac.VerifyMAC(ciphertext[len(ciphertext)-e.tagSize:], toAuthData)
	if err != nil {
		return nil, fmt.Errorf("encrypt_then_authenticate: %v", err)
	}

	plaintext, err := e.indCPACipher.Decrypt(payload)
	if err != nil {
		return nil, fmt.Errorf("encrypt_then_authenticate: %v", err)
	}

	return plaintext, nil
}
