// Copyright 2024 Gravitational, Inc.
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

package keys

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"
)

const (
	// PKCS1PublicKeyType is the PEM encoding type commonly used for PKCS#1, ASN.1 DER form public keys.
	PKCS1PublicKeyType = "RSA PUBLIC KEY"
	// PKIXPublicKeyType is the PEM encoding type commonly used for PKIX, ASN.1 DER form public keys.
	PKIXPublicKeyType = "PUBLIC KEY"
)

// MarshalPublicKey returns a PEM encoding of the given public key. Encodes RSA
// keys in PKCS1 format for backward compatibility. All other key types are
// encoded in PKIX, ASN.1 DER form. Only supports *rsa.PublicKey,
// *ecdsa.PublicKey, and ed25519.PublicKey.
func MarshalPublicKey(pub crypto.PublicKey) ([]byte, error) {
	switch pubKey := pub.(type) {
	case *rsa.PublicKey:
		pubPEM := pem.EncodeToMemory(&pem.Block{
			Type:  PKCS1PublicKeyType,
			Bytes: x509.MarshalPKCS1PublicKey(pubKey),
		})
		return pubPEM, nil
	case *ecdsa.PublicKey, ed25519.PublicKey:
		der, err := x509.MarshalPKIXPublicKey(pubKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pubPEM := pem.EncodeToMemory(&pem.Block{
			Type:  PKIXPublicKeyType,
			Bytes: der,
		})
		return pubPEM, nil
	default:
		return nil, trace.BadParameter("unsupported public key type %T", pub)
	}
}

// ParsePublicKey parses a PEM-encoded public key. Supports PEM encodings of PKCS#1 or PKIX ASN.1 DER form
// public keys.
func ParsePublicKey(keyPEM []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, trace.BadParameter("failed to decode public key PEM block")
	}

	switch block.Type {
	case PKCS1PublicKeyType, PKIXPublicKeyType:
	default:
		return nil, trace.BadParameter("unsupported public key type %q", block.Type)
	}

	// We have been known to stuff PKIX DER encoded RSA public keys into
	// "RSA PUBLIC KEY" PEM blocks, so just try to parse either.
	var preferredErr error
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		return pub, nil
	} else if block.Type == PKIXPublicKeyType {
		preferredErr = err
	}
	if pub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return pub, nil
	} else if block.Type == PKCS1PublicKeyType {
		preferredErr = err
	}
	// If both parse functions returned an error, preferedErr is guaranteed to
	// be set to the error from the parse function that usually matches the PEM
	// block type.
	return nil, trace.Wrap(preferredErr, "parsing public key PEM")
}
