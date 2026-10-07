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

package credentialencryption

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hpke"

	"github.com/gravitational/trace"
)

// hpkeInfo is the HPKE info for domain separation. Changing it breaks
// decryption of stored credentials.
//
// https://www.rfc-editor.org/rfc/rfc9180.html#section-5.1
const hpkeInfo = "teleport/user-external-credential/v1"

// Encrypt encrypts the plaintext with the public key using HPKE with
// DHKEM(P-256), HKDF-SHA256 and AES-256-GCM.
func Encrypt(publicKey *ecdsa.PublicKey, plaintext []byte) ([]byte, error) {
	if publicKey.Curve != elliptic.P256() {
		return nil, trace.BadParameter("key must be P-256")
	}
	ecdhPublicKey, err := publicKey.ECDH()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hpkePublicKey, err := hpke.NewDHKEMPublicKey(ecdhPublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ciphertext, err := hpke.Seal(hpkePublicKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(hpkeInfo), plaintext)
	return ciphertext, trace.Wrap(err)
}

// Decrypt decrypts the ciphertext encrypted by Encrypt with the private key.
func Decrypt(privateKey *ecdsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	if privateKey.Curve != elliptic.P256() {
		return nil, trace.BadParameter("key must be P-256")
	}
	ecdhPrivateKey, err := privateKey.ECDH()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	hpkePrivateKey, err := hpke.NewDHKEMPrivateKey(ecdhPrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plaintext, err := hpke.Open(hpkePrivateKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(hpkeInfo), ciphertext)
	return plaintext, trace.Wrap(err)
}
