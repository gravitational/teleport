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
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"math"

	// Placeholder for internal crypto/subtle allowlist, please ignore. // to allow import of "crypto/subte"
	"github.com/google/tink/go/subtle/random"
)

const (
	// AESGCMSIVNonceSize is the acceptable IV size defined by RFC 8452.
	AESGCMSIVNonceSize = 12

	// aesgcmsivBlockSize is the block size that AES-GCM-SIV uses. This is the
	// size for the tag, the KDF etc.
	// Note: this value is the same as AES block size.
	aesgcmsivBlockSize = 16

	// aesgcmsivTagSize is the byte-length of the authentication tag produced by
	// AES-GCM-SIV.
	aesgcmsivTagSize = aesgcmsivBlockSize

	// aesgcmsivPolyvalSize is the byte-length of result produced by the
	// POLYVAL function.
	aesgcmsivPolyvalSize = aesgcmsivBlockSize
)

// AESGCMSIVKeySizes is an array of byte lengths of keys acceptable by the
// AES-GCM-SIV algorithm.
var AESGCMSIVKeySizes = [...]uint32{16, 32}

// AESGCMSIV is an implementation of AEAD interface.
type AESGCMSIV struct {
	Key []byte
}

// NewAESGCMSIV returns an AESGCMSIV instance.
// The key argument should be the AES key, either 16 or 32 bytes to select
// AES-128 or AES-256.
func NewAESGCMSIV(key []byte) (*AESGCMSIV, error) {
	keySize := uint32(len(key))
	if err := ValidateAESKeySize(keySize); err != nil {
		return nil, fmt.Errorf("aes_gcm_siv: %s", err)
	}
	return &AESGCMSIV{Key: key}, nil
}

// Encrypt encrypts pt with aad as additional authenticated data.
//
// The resulting ciphertext consists of three parts:
// (1) the Nonce used for encryption
// (2) the actual ciphertext
// (3) the authentication tag.
func (a *AESGCMSIV) Encrypt(pt, aad []byte) ([]byte, error) {
	if len(pt) > math.MaxInt32-AESGCMSIVNonceSize-aesgcmsivTagSize {
		return nil, fmt.Errorf("aes_gcm_siv: plaintext too long")
	}
	if len(aad) > math.MaxInt32 {
		return nil, fmt.Errorf("aes_gcm_siv: additional-data too long")
	}

	nonce := random.GetRandomBytes(uint32(AESGCMSIVNonceSize))
	authKey, encKey, err := a.deriveKeys(nonce)
	if err != nil {
		return nil, err
	}

	polyval, err := a.computePolyval(authKey, pt, aad)
	if err != nil {
		return nil, err
	}
	tag, err := a.computeTag(polyval, nonce, encKey)
	if err != nil {
		return nil, err
	}

	ct, err := a.aesCTR(encKey, tag, pt)
	if err != nil {
		return nil, err
	}

	ret := make([]byte, 0, AESGCMSIVNonceSize+aesgcmsivTagSize+len(pt))
	ret = append(ret, nonce...)
	ret = append(ret, ct...)
	ret = append(ret, tag...)

	return ret, nil
}

// Decrypt decrypts ct with aad as the additional-authenticated-data.
func (a *AESGCMSIV) Decrypt(ct, aad []byte) ([]byte, error) {
	if len(ct) < AESGCMSIVNonceSize+aesgcmsivTagSize {
		return nil, fmt.Errorf("aes_gcm_siv: ciphertext too short")
	}
	if len(ct) > math.MaxInt32 {
		return nil, fmt.Errorf("aes_gcm_siv: ciphertext too long")
	}
	if len(aad) > math.MaxInt32 {
		return nil, fmt.Errorf("aes_gcm_siv: additional-data too long")
	}

	nonce := ct[:AESGCMSIVNonceSize]
	tag := ct[len(ct)-aesgcmsivTagSize:]
	ct = ct[AESGCMSIVNonceSize : len(ct)-aesgcmsivTagSize]

	authKey, encKey, err := a.deriveKeys(nonce)
	if err != nil {
		return nil, err
	}

	pt, err := a.aesCTR(encKey, tag, ct)
	if err != nil {
		return nil, err
	}

	polyval, err := a.computePolyval(authKey, pt, aad)
	if err != nil {
		return nil, err
	}

	expectedTag, err := a.computeTag(polyval, nonce, encKey)
	if err != nil {
		return nil, err
	}

	if subtle.ConstantTimeCompare(expectedTag, tag) != 1 {
		return nil, fmt.Errorf("aes_gcm_siv: message authentication failure")
	}

	return pt, nil
}

// The KDF as described by the RFC #8452. This uses the AES-GCM-SIV key and
// nonce to generate the authentication key and the encryption key.
func (a *AESGCMSIV) deriveKeys(nonce []byte) ([]byte, []byte, error) {
	if len(nonce) != AESGCMSIVNonceSize {
		return nil, nil, fmt.Errorf("aes_gcm_siv: invalid nonce size")
	}
	nonceBlock := make([]byte, aesgcmsivBlockSize)
	copy(nonceBlock[aesgcmsivBlockSize-AESGCMSIVNonceSize:], nonce)
	block, err := aes.NewCipher(a.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("aes_gcm_siv: failed to create block cipher, error: %v", err)
	}

	encBlock := make([]byte, block.BlockSize())
	kdfAes := func(counter uint32, dst []byte) {
		binary.LittleEndian.PutUint32(nonceBlock[:4], counter)
		block.Encrypt(encBlock, nonceBlock)
		copy(dst, encBlock[0:8])
	}

	authKey := make([]byte, aesgcmsivBlockSize)
	kdfAes(0, authKey[0:8])
	kdfAes(1, authKey[8:16])

	encKey := make([]byte, len(a.Key))
	kdfAes(2, encKey[0:8])
	kdfAes(3, encKey[8:16])

	if len(a.Key) == 32 {
		kdfAes(4, encKey[16:24])
		kdfAes(5, encKey[24:32])
	}

	return authKey, encKey, nil
}

func (a *AESGCMSIV) computePolyval(authKey, pt, aad []byte) ([]byte, error) {
	lengthBlock := make([]byte, aesgcmsivBlockSize)
	binary.LittleEndian.PutUint64(lengthBlock[:8], uint64(len(aad))*8)
	binary.LittleEndian.PutUint64(lengthBlock[8:], uint64(len(pt))*8)

	p, err := NewPolyval(authKey)
	if err != nil {
		return nil, fmt.Errorf("aes_gcm_siv: failed to create polyval, error: %v", err)
	}

	p.Update(aad)
	p.Update(pt)
	p.Update(lengthBlock)
	polyval := p.Finish()

	return polyval[:], nil
}

func (a *AESGCMSIV) computeTag(polyval, nonce, encKey []byte) ([]byte, error) {
	if len(polyval) != aesgcmsivPolyvalSize {
		return nil, fmt.Errorf("aes_gcm_siv: polyval returned invalid sized response")
	}

	for i, val := range nonce {
		polyval[i] ^= val
	}
	polyval[aesgcmsivPolyvalSize-1] &= 0x7f

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("aes_gcm_siv: failed to create block cipher, error: %v", err)
	}

	tag := make([]byte, aesgcmsivTagSize)
	block.Encrypt(tag, polyval)
	return tag, nil
}

// aesCTR implements the AES-CTR operation in AES-GCM-SIV.
// Note that RFC 8452 defines AES-CTR differently compared to standard AES
// in CTR mode: the way they increment the counter block is completely different.
func (a *AESGCMSIV) aesCTR(key, tag, in []byte) ([]byte, error) {
	if len(tag) != aesgcmsivTagSize {
		return nil, fmt.Errorf("aes_gcm_siv: incorrect IV size for stream cipher")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf(
			"aes_gcm_siv: failed to create block cipher, error: %v", err)
	}

	counter := make([]byte, aesgcmsivBlockSize)
	copy(counter, tag)
	counter[aesgcmsivBlockSize-1] |= 0x80
	counterInc := binary.LittleEndian.Uint32(counter[0:4])

	output := make([]byte, len(in))
	outputIdx := 0
	keystreamBlock := make([]byte, block.BlockSize())
	for len(in) > 0 {
		block.Encrypt(keystreamBlock, counter)
		counterInc++
		binary.LittleEndian.PutUint32(counter[0:4], counterInc)

		n := xorBytes(output[outputIdx:], in, keystreamBlock)
		outputIdx += n
		in = in[n:]
	}

	return output, nil
}

// It would have been better to call xorBytes function defined in
// "crypto/cipher/xor_*.go" to make use of the architechture optimisations.
func xorBytes(dst, a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 0
	}
	for i := 0; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}

	return n
}
