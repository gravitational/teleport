/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package cryptoutils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"

	"github.com/gravitational/trace"
)

// ECIES wire format:
// [ephemeral_pub (65 bytes, uncompressed P-256) | nonce (12 bytes) | ciphertext + GCM tag]
const (
	eciesP256PubKeyLen = 65
	eciesNonceLen      = 12
	eciesOverhead      = eciesP256PubKeyLen + eciesNonceLen + 16 // 16 = GCM tag
)

// ECIESEncrypt encrypts plaintext using ECIES with P-256 and AES-256-GCM.
// All primitives are FIPS-approved (P-256 ECDH, HMAC-SHA256 KDF, AES-256-GCM).
func ECIESEncrypt(recipientPub *ecdsa.PublicKey, plaintext []byte) ([]byte, error) {
	if recipientPub.Curve != elliptic.P256() {
		return nil, trace.BadParameter("ECIES requires P-256 key")
	}

	recipientECDH, err := recipientPub.ECDH()
	if err != nil {
		return nil, trace.Wrap(err, "converting recipient public key")
	}

	ephemeral, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err, "generating ephemeral key")
	}

	shared, err := ephemeral.ECDH(recipientECDH)
	if err != nil {
		return nil, trace.Wrap(err, "ECDH key agreement")
	}

	aesKey := hkdfSHA256(shared, ephemeral.PublicKey().Bytes(), 32)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, trace.Wrap(err, "creating AES cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, trace.Wrap(err, "creating GCM")
	}

	nonce := make([]byte, eciesNonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, trace.Wrap(err, "generating nonce")
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Wire format: ephemeral_pub || nonce || ciphertext+tag
	out := make([]byte, 0, eciesP256PubKeyLen+eciesNonceLen+len(ciphertext))
	out = append(out, ephemeral.PublicKey().Bytes()...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// ECIESDecrypt decrypts ciphertext using ECIES with P-256 and AES-256-GCM.
func ECIESDecrypt(privKey *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	if len(data) < eciesOverhead {
		return nil, trace.BadParameter("ciphertext too short")
	}

	if privKey.Curve != elliptic.P256() {
		return nil, trace.BadParameter("ECIES requires P-256 key")
	}

	ecdhPriv, err := privKey.ECDH()
	if err != nil {
		return nil, trace.Wrap(err, "converting private key")
	}

	ephPubBytes := data[:eciesP256PubKeyLen]
	nonce := data[eciesP256PubKeyLen : eciesP256PubKeyLen+eciesNonceLen]
	ciphertext := data[eciesP256PubKeyLen+eciesNonceLen:]

	ephPub, err := ecdh.P256().NewPublicKey(ephPubBytes)
	if err != nil {
		return nil, trace.Wrap(err, "parsing ephemeral public key")
	}

	shared, err := ecdhPriv.ECDH(ephPub)
	if err != nil {
		return nil, trace.Wrap(err, "ECDH key agreement")
	}

	aesKey := hkdfSHA256(shared, ephPubBytes, 32)

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, trace.Wrap(err, "creating AES cipher")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, trace.Wrap(err, "creating GCM")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, trace.Wrap(err, "GCM decryption failed")
	}
	return plaintext, nil
}

// hkdfSHA256 derives a key using HMAC-SHA256 based HKDF-Extract + Expand.
// Uses only FIPS-approved primitives (HMAC-SHA256).
func hkdfSHA256(secret, info []byte, keyLen int) []byte {
	// HKDF-Extract: PRK = HMAC-SHA256(salt="", IKM=secret)
	h := hmac.New(sha256.New, nil)
	h.Write(secret)
	prk := h.Sum(nil)

	// HKDF-Expand: OKM = HMAC-SHA256(PRK, info || 0x01)
	h = hmac.New(sha256.New, prk)
	h.Write(info)
	h.Write([]byte{0x01})
	return h.Sum(nil)[:keyLen]
}
