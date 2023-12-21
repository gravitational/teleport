/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"
)

// MarshalPrivateKey will return a PEM encoded crypto.Signer. Only supports
// RSA private keys.
func MarshalPrivateKey(key crypto.Signer) ([]byte, []byte, error) {
	switch privateKey := key.(type) {
	case *rsa.PrivateKey:
		privateBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
		})

		publicKey, ok := key.Public().(*rsa.PublicKey)
		if !ok {
			return nil, nil, trace.BadParameter("invalid key type: %T", publicKey)
		}
		publicBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(publicKey),
		})

		return publicBytes, privateBytes, nil
	default:
		return nil, nil, trace.BadParameter("unsupported private key type %T", key)
	}
}

// MarshalPublicKey returns a PEM encoded public key for a given crypto.Signer
func MarshalPublicKey(signer crypto.Signer) ([]byte, error) {
	switch publicKey := signer.Public().(type) {
	case *rsa.PublicKey:
		return pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(publicKey),
		}), nil
	default:
		return nil, trace.BadParameter("unsupported public key type %T", publicKey)
	}
}

// ParsePrivateKey parses a PEM encoded private key and returns a
// crypto.Signer. Only supports RSA private keys.
func ParsePrivateKey(bytes []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("failed to decode private key PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	default:
		return nil, trace.BadParameter("unsupported private key type %q", block.Type)
	}
}

// ParsePublicKey parses a PEM encoded public key and returns a
// crypto.PublicKey. Only support RSA public keys.
func ParsePublicKey(bytes []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return nil, trace.BadParameter("failed to decode public key PEM block")
	}

	switch block.Type {
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		return nil, trace.BadParameter("unsupported public key type %q", block.Type)
	}
}
