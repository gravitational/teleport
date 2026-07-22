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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hpke"

	"github.com/gravitational/trace"
)

// ECIESEncrypt encrypts plaintext for the given P-256 public key using HPKE
// (RFC 9180) with DHKEM(P-256), HKDF-SHA256, and AES-256-GCM.
func ECIESEncrypt(recipientPub *ecdsa.PublicKey, plaintext []byte) ([]byte, error) {
	if recipientPub.Curve != elliptic.P256() {
		return nil, trace.BadParameter("encryption requires P-256 key")
	}

	ecdhPub, err := recipientPub.ECDH()
	if err != nil {
		return nil, trace.Wrap(err, "converting recipient public key")
	}

	hpkePub, err := hpke.NewDHKEMPublicKey(ecdhPub)
	if err != nil {
		return nil, trace.Wrap(err, "creating HPKE public key")
	}

	ciphertext, err := hpke.Seal(hpkePub, hpke.HKDFSHA256(), hpke.AES256GCM(), nil, plaintext)
	if err != nil {
		return nil, trace.Wrap(err, "HPKE seal")
	}
	return ciphertext, nil
}

// ECIESDecrypt decrypts ciphertext using the given P-256 private key with HPKE
// (RFC 9180) with DHKEM(P-256), HKDF-SHA256, and AES-256-GCM.
func ECIESDecrypt(privKey *ecdsa.PrivateKey, data []byte) ([]byte, error) {
	if privKey.Curve != elliptic.P256() {
		return nil, trace.BadParameter("decryption requires P-256 key")
	}

	ecdhPriv, err := privKey.ECDH()
	if err != nil {
		return nil, trace.Wrap(err, "converting private key")
	}

	hpkePriv, err := hpke.NewDHKEMPrivateKey(ecdhPriv)
	if err != nil {
		return nil, trace.Wrap(err, "creating HPKE private key")
	}

	plaintext, err := hpke.Open(hpkePriv, hpke.HKDFSHA256(), hpke.AES256GCM(), nil, data)
	if err != nil {
		return nil, trace.Wrap(err, "HPKE open")
	}
	return plaintext, nil
}
