/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
		return nil, nil, trace.BadParameter("unsupported private key type %q", key)
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
