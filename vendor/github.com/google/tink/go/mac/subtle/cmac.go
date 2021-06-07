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
	"crypto/subtle"
	"fmt"

	subtleprf "github.com/google/tink/go/prf/subtle"

	// Placeholder for internal crypto/subtle allowlist, please ignore.
)

const (
	minCMACKeySizeInBytes         = 16
	recommendedCMACKeySizeInBytes = uint32(32)
	minTagLengthInBytes           = uint32(10)
	maxTagLengthInBytes           = uint32(16)
)

// AESCMAC represents an AES-CMAC struct that implements the MAC interface.
type AESCMAC struct {
	prf       *subtleprf.AESCMACPRF
	tagLength uint32
}

// NewAESCMAC creates a new AESCMAC object that implements the MAC interface.
func NewAESCMAC(key []byte, tagLength uint32) (*AESCMAC, error) {
	if len(key) < minCMACKeySizeInBytes {
		return nil, fmt.Errorf("Only 256 but keys are allowed with AES-CMAC")
	}
	if tagLength < minTagLengthInBytes {
		return nil, fmt.Errorf("Tag length %d is shorter than minimum tag length %d", tagLength, minTagLengthInBytes)
	}
	if tagLength > maxTagLengthInBytes {
		return nil, fmt.Errorf("Tag length %d is longer than maximum tag length %d", tagLength, minTagLengthInBytes)
	}
	ac := &AESCMAC{}
	var err error
	ac.prf, err = subtleprf.NewAESCMACPRF(key)
	if err != nil {
		return nil, fmt.Errorf("Could not create AES-CMAC prf: %v", err)
	}
	ac.tagLength = tagLength
	return ac, nil
}

// ComputeMAC computes message authentication code (MAC) for code data.
func (a AESCMAC) ComputeMAC(data []byte) ([]byte, error) {
	return a.prf.ComputePRF(data, a.tagLength)
}

// VerifyMAC returns nil if mac is a correct authentication code (MAC) for data,
// otherwise it returns an error.
func (a AESCMAC) VerifyMAC(mac, data []byte) error {
	computed, err := a.prf.ComputePRF(data, a.tagLength)
	if err != nil {
		return fmt.Errorf("Could not compute MAC: %v", err)
	}
	if subtle.ConstantTimeCompare(mac, computed) != 1 {
		return fmt.Errorf("CMAC: Invalid MAC")
	}
	return nil
}

// ValidateCMACParams validates the parameters for an AES-CMAC against the recommended parameters.
func ValidateCMACParams(keySize, tagSize uint32) error {
	if keySize != recommendedCMACKeySizeInBytes {
		return fmt.Errorf("Only %d sized keys are allowed with Tink's AES-CMAC", recommendedCMACKeySizeInBytes)
	}
	if tagSize < minTagLengthInBytes {
		return fmt.Errorf("Tag size too short")
	}
	if tagSize > maxTagLengthInBytes {
		return fmt.Errorf("Tag size too long")
	}
	return nil
}
