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
	"crypto/subtle"
	"fmt"

	// Placeholder for internal crypto/subtle allowlist, please ignore.
)

const (
	mul                = 0x87
	pad                = byte(0x80)
	recommendedKeySize = uint32(32)
)

// AESCMACPRF is a type that can be used to compute several CMACs with the same key material.
type AESCMACPRF struct {
	bc               cipher.Block
	subkey1, subkey2 []byte
}

// NewAESCMACPRF creates a new AESCMACPRF object and initializes it with the correct key material.
func NewAESCMACPRF(key []byte) (*AESCMACPRF, error) {
	aesCmac := &AESCMACPRF{}
	var err error
	aesCmac.bc, err = aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("Could not obtain cipher: %v", err)
	}
	bs := aesCmac.bc.BlockSize()
	zeroBlock := make([]byte, bs)

	// Generate Subkeys
	aesCmac.subkey1 = make([]byte, bs)
	aesCmac.subkey2 = make([]byte, bs)
	aesCmac.bc.Encrypt(aesCmac.subkey1, zeroBlock)
	mulByX(aesCmac.subkey1)
	copy(aesCmac.subkey2, aesCmac.subkey1)
	mulByX(aesCmac.subkey2)
	return aesCmac, nil
}

// ValidateAESCMACPRFParams checks that the key is the recommended size for AES-CMAC.
func ValidateAESCMACPRFParams(keySize uint32) error {
	if keySize != recommendedKeySize {
		return fmt.Errorf("Recommended key size for AES-CMAC is %d, but %d given", recommendedKeySize, keySize)
	}
	return nil
}

// ComputePRF computes the AES-CMAC for the given key and data, returning outputLength bytes.
// The timing of this function will only depend on len(data), and not leak any additional information about the key or the data.
func (a AESCMACPRF) ComputePRF(data []byte, outputLength uint32) ([]byte, error) {
	// Setup
	bs := a.bc.BlockSize()
	if outputLength > uint32(bs) {
		return nil, fmt.Errorf("outputLength must be between 0 and %d", bs)
	}

	// Pad
	flag := false
	n := len(data)/bs + 1
	// if only depends on len(data).
	if len(data) > 0 && len(data)%bs == 0 {
		n--
		flag = true
	}
	mLast := make([]byte, bs)
	mLastStart := (n - 1) * bs
	for i := 0; i < bs; i++ {
		// if depends on mLastStart and len(data), which depend on len(data)
		if i+mLastStart < len(data) {
			mLast[i] = data[i+mLastStart]
		} else if i+mLastStart == len(data) {
			mLast[i] = pad
		}
		// if only depends on flag, which depends on len(data)
		if flag {
			mLast[i] ^= a.subkey1[i]
		} else {
			mLast[i] ^= a.subkey2[i]
		}
	}
	input := make([]byte, bs)
	output := make([]byte, bs)
	for i := 0; i < n; i++ {
		// if depends on n, which depends on len(data)
		if i+1 == n {
			copy(input, mLast)
		} else {
			copy(input, data[i*bs:(i+1)*bs])
		}
		for j := 0; j < bs; j++ {
			input[j] ^= output[j]
		}
		a.bc.Encrypt(output, input)
	}
	return output[:outputLength], nil
}

func mulByX(block []byte) {
	bs := len(block)
	v := int(block[0] >> 7)
	for i := 0; i < bs-1; i++ {
		block[i] = block[i]<<1 | block[i+1]>>7
	}
	block[bs-1] = (block[bs-1] << 1) ^ byte(subtle.ConstantTimeSelect(v, mul, 0x00))
}
