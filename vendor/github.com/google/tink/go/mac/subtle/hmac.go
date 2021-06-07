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

// Package subtle provides subtle implementations of the MAC primitive.
package subtle

import (
	"crypto/hmac"
	"errors"
	"fmt"
	"hash"

	"github.com/google/tink/go/subtle"
)

const (
	// Minimum key size in bytes.
	minKeySizeInBytes = uint32(16)

	// Minimum tag size in bytes. This provides minimum 80-bit security strength.
	minTagSizeInBytes = uint32(10)
)

var errHMACInvalidInput = errors.New("HMAC: invalid input")

// HMAC implementation of interface tink.MAC
type HMAC struct {
	HashFunc func() hash.Hash
	Key      []byte
	TagSize  uint32
}

// NewHMAC creates a new instance of HMAC with the specified key and tag size.
func NewHMAC(hashAlg string, key []byte, tagSize uint32) (*HMAC, error) {
	keySize := uint32(len(key))
	if err := ValidateHMACParams(hashAlg, keySize, tagSize); err != nil {
		return nil, fmt.Errorf("hmac: %s", err)
	}
	hashFunc := subtle.GetHashFunc(hashAlg)
	if hashFunc == nil {
		return nil, fmt.Errorf("hmac: invalid hash algorithm")
	}
	return &HMAC{
		HashFunc: hashFunc,
		Key:      key,
		TagSize:  tagSize,
	}, nil
}

// ValidateHMACParams validates parameters of HMAC constructor.
func ValidateHMACParams(hash string, keySize uint32, tagSize uint32) error {
	// validate tag size
	digestSize, err := subtle.GetHashDigestSize(hash)
	if err != nil {
		return err
	}
	if tagSize > digestSize {
		return fmt.Errorf("tag size too big")
	}
	if tagSize < minTagSizeInBytes {
		return fmt.Errorf("tag size too small")
	}
	// validate key size
	if keySize < minKeySizeInBytes {
		return fmt.Errorf("key too short")
	}
	return nil
}

// ComputeMAC computes message authentication code (MAC) for the given data.
func (h *HMAC) ComputeMAC(data []byte) ([]byte, error) {
	mac := hmac.New(h.HashFunc, h.Key)
	if _, err := mac.Write(data); err != nil {
		return nil, err
	}
	tag := mac.Sum(nil)
	return tag[:h.TagSize], nil
}

// VerifyMAC verifies whether the given MAC is a correct message authentication
// code (MAC) the given data.
func (h *HMAC) VerifyMAC(mac []byte, data []byte) error {
	expectedMAC, err := h.ComputeMAC(data)
	if err != nil {
		return err
	}
	if hmac.Equal(expectedMAC, mac) {
		return nil
	}
	return errors.New("HMAC: invalid MAC")
}
