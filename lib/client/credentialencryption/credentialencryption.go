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
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hpke"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
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
	key, err := NewSoftwarePrivateKey(privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plaintext, err := key.Decrypt(context.Background(), ciphertext)
	return plaintext, trace.Wrap(err)
}

// PrivateKey decrypts ciphertexts encrypted by Encrypt.
//
// Currently supports [ecdh.PrivateKey] and [hardwarekey.KeyExchanger] as
// concrete ECDH exchanger.
type PrivateKey struct {
	exchanger ecdh.KeyExchanger
}

// NewSoftwarePrivateKey creates a [PrivateKey] from an ecdsa.PrivateKey.
func NewSoftwarePrivateKey(key *ecdsa.PrivateKey) (*PrivateKey, error) {
	if key == nil {
		return nil, trace.BadParameter("key must not be nil")
	}
	exchanger, err := key.ECDH()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewPrivateKey(exchanger)
}

// NewPrivateKey returns a [PrivateKey] backed by exchanger.
func NewPrivateKey(exchanger ecdh.KeyExchanger) (*PrivateKey, error) {
	// Only P-256 for now.
	publicKey := exchanger.PublicKey()
	if publicKey == nil || publicKey.Curve() != ecdh.P256() {
		return nil, trace.BadParameter("key must be P-256")
	}
	return &PrivateKey{exchanger: exchanger}, nil
}

// Decrypt decrypts the ciphertext encrypted by Encrypt.
func (k *PrivateKey) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	exchanger := k.exchanger
	if a, ok := exchanger.(*hardwarekey.KeyExchanger); ok {
		exchanger = a.WithContext(ctx)
	}

	hpkePrivateKey, err := hpke.NewDHKEMPrivateKey(exchanger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plaintext, err := hpke.Open(hpkePrivateKey, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(hpkeInfo), ciphertext)
	return plaintext, trace.Wrap(err)
}

// PublicKey returns the PKIX, ASN.1 DER encoded public key.
func (k *PrivateKey) PublicKey() ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(k.exchanger.PublicKey())
	return der, trace.Wrap(err)
}

// PEM block types used by MarshalPrivateKey and ParsePrivateKey.
const (
	softwareKeyType   = "PRIVATE KEY"
	privateKeyRefType = "ENCRYPTION KEY AGENT REFERENCE"
)

// ParsePrivateKeyOpts configures ParsePrivateKey.
type ParsePrivateKeyOpts struct {
	// HardwareKeyService is used to decrypt through an agent-held reference.
	// Required if the encoded key is a reference rather than a raw key.
	HardwareKeyService hardwarekey.Service
}

// ParsePrivateKeyOpt is a functional option for ParsePrivateKey.
type ParsePrivateKeyOpt func(*ParsePrivateKeyOpts)

// WithHardwareKeyService sets the hardware key service used to decrypt
// through an agent-held key reference.
func WithHardwareKeyService(s hardwarekey.Service) ParsePrivateKeyOpt {
	return func(o *ParsePrivateKeyOpts) { o.HardwareKeyService = s }
}

// MarshalPrivateKey encodes key for storage, matching whatever ParsePrivateKey
// previously produced.
func MarshalPrivateKey(key *PrivateKey) ([]byte, error) {
	switch exchanger := key.exchanger.(type) {
	case *ecdh.PrivateKey:
		der, err := x509.MarshalPKCS8PrivateKey(exchanger)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: softwareKeyType, Bytes: der}), nil
	case *hardwarekey.KeyExchanger:
		encoded, err := exchanger.Encode()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: privateKeyRefType, Bytes: encoded}), nil
	default:
		return nil, trace.BadParameter("unsupported private key type %T", key.exchanger)
	}
}

// ParsePrivateKey decodes a key previously encoded by MarshalPrivateKey.
func ParsePrivateKey(keyPEM []byte, opts ...ParsePrivateKeyOpt) (*PrivateKey, error) {
	var options ParsePrivateKeyOpts
	for _, opt := range opts {
		opt(&options)
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, trace.BadParameter("expected PEM encoded private key")
	}

	switch block.Type {
	case softwareKeyType:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ecdsaKey, ok := key.(*ecdsa.PrivateKey)
		if !ok {
			return nil, trace.BadParameter("session encryption key is not an ECDSA key")
		}
		return NewSoftwarePrivateKey(ecdsaKey)
	case privateKeyRefType:
		exchangerService, ok := options.HardwareKeyService.(hardwarekey.KeyExchangerService)
		if !ok {
			return nil, trace.BadParameter("hardware key service does not support encryption")
		}
		ref, err := hardwarekey.DecodeKeyRef(block.Bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		exchanger, err := hardwarekey.NewKeyExchanger(ref, exchangerService)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewPrivateKey(exchanger)
	default:
		return nil, trace.BadParameter("unsupported PEM block type %q", block.Type)
	}
}
