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
	"fmt"
	"hash"
	"io"

	"golang.org/x/crypto/hkdf"
	"github.com/google/tink/go/subtle"
)

const (
	// We use a somewhat larger minimum key size than usual, because PRFs might be
	// used by many users, in which case the security can degrade by a factor
	// depending on the number of users. (Discussed for example in
	// https://eprint.iacr.org/2012/159)
	minHKDFKeySizeInBytes = uint32(32)
)

// HKDFPRF is a type that can be used to compute several HKDFs with the same key material.
type HKDFPRF struct {
	h    func() hash.Hash
	key  []byte
	salt []byte
}

// NewHKDFPRF creates a new HKDFPRF object and initializes it with the correct key material.
func NewHKDFPRF(hashAlg string, key []byte, salt []byte) (*HKDFPRF, error) {
	h := &HKDFPRF{}
	hashFunc := subtle.GetHashFunc(hashAlg)
	if hashFunc == nil {
		return nil, fmt.Errorf("hkdf: invalid hash algorithm")
	}
	h.h = hashFunc
	h.key = key
	h.salt = salt
	return h, nil
}

// ValidateHKDFPRFParams validates parameters of HKDF constructor.
func ValidateHKDFPRFParams(hash string, keySize uint32, salt []byte) error {
	// validate key size
	if keySize < minHKDFKeySizeInBytes {
		return fmt.Errorf("key too short")
	}
	if subtle.GetHashFunc(hash) == nil {
		return fmt.Errorf("invalid hash function")
	}
	if hash != "SHA256" && hash != "SHA512" {
		return fmt.Errorf("Only SHA-256 and SHA-512 currently allowed for HKDF")
	}
	return nil
}

// ComputePRF computes the HKDF for the given key and data, returning outputLength bytes.
func (h HKDFPRF) ComputePRF(data []byte, outputLength uint32) ([]byte, error) {
	kdf := hkdf.New(h.h, h.key, h.salt, data)
	output := make([]byte, outputLength)
	_, err := io.ReadAtLeast(kdf, output, int(outputLength))
	if err != nil {
		return nil, fmt.Errorf("Error computing HKDF: %v", err)
	}
	return output[:outputLength], nil
}
