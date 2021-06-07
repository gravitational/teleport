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
	"crypto/hmac"
	"fmt"
	"hash"

	"github.com/google/tink/go/subtle"
)

const (
	minHMACKeySizeInBytes = uint32(16)
)

// HMACPRF is a type that can be used to compute several HMACs with the same key material.
type HMACPRF struct {
	h   func() hash.Hash
	key []byte
}

// NewHMACPRF creates a new HMACPRF object and initializes it with the correct key material.
func NewHMACPRF(hashAlg string, key []byte) (*HMACPRF, error) {
	h := &HMACPRF{}
	hashFunc := subtle.GetHashFunc(hashAlg)
	if hashFunc == nil {
		return nil, fmt.Errorf("hmac: invalid hash algorithm")
	}
	h.h = hashFunc
	h.key = key
	return h, nil
}

// ValidateHMACPRFParams validates parameters of HMAC constructor.
func ValidateHMACPRFParams(hash string, keySize uint32) error {
	// validate key size
	if keySize < minHMACKeySizeInBytes {
		return fmt.Errorf("key too short")
	}
	if subtle.GetHashFunc(hash) == nil {
		return fmt.Errorf("invalid hash function")
	}
	return nil
}

// ComputePRF computes the HMAC for the given key and data, returning outputLength bytes.
func (h HMACPRF) ComputePRF(data []byte, outputLength uint32) ([]byte, error) {
	mac := hmac.New(h.h, h.key)
	if outputLength > uint32(mac.Size()) {
		return nil, fmt.Errorf("outputLength must be between 0 and %d", mac.Size())
	}
	mac.Write(data)
	return mac.Sum(nil)[:outputLength], nil
}
