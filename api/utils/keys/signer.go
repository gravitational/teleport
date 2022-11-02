/*
Copyright 2022 Gravitational, Inc.

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

package keys

import (
	"crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/gravitational/trace"
)

// Signer implements crypto.Signer with additional helper methods.
type Signer interface {
	crypto.Signer

	// PrivateKeyPEM returns PEM encoded private key data. This may be data necessary
	// to retrieve the key, such as a Yubikey serial number and slot, or it can be a
	// PKCS marshaled private key.
	//
	// The resulting PEM encoded data should only be decoded with ParsePrivateKey to
	// prevent errors from parsing non PKCS marshaled keys, such as a PIV key.
	PrivateKeyPEM() []byte

	// TLSCertificate parses the given TLS certificate paired with the private key
	// to rerturn a tls.Certificate, ready to be used in a TLS handshake.
	TLSCertificate(tlsCert []byte) (tls.Certificate, error)
}

// StandardSigner is a shared Signer implementation for standard crypto.PrivateKey
// implemenations, which are *rsa.PrivateKey, *ecdsa.PrivateKey, and ed25519.PrivateKey.
type StandardSigner struct {
	// Signer is an *rsa.PrivateKey, *ecdsa.PrivateKey, or ed25519.PrivateKey.
	crypto.Signer
	// keyPEM is the PEM-encoded private key.
	keyPEM []byte
}

// NewStandardSigner creates a new StandardSigner from the given *rsa.PrivateKey.
func NewRSASigner(rsaKey *rsa.PrivateKey) (*StandardSigner, error) {
	// We encode the private key in PKCS #1, ASN.1 DER form
	// instead of PKCS #8 to maintain compatibility with some
	// third party clients.
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:    PKCS1PrivateKeyType,
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(rsaKey),
	})

	return newStandardSigner(rsaKey, keyPEM), nil
}

func newStandardSigner(signer crypto.Signer, keyPEM []byte) *StandardSigner {
	return &StandardSigner{
		Signer: signer,
		keyPEM: keyPEM,
	}
}

// PrivateKeyPEM returns the PEM-encoded private key.
func (s *StandardSigner) PrivateKeyPEM() []byte {
	return s.keyPEM
}

// TLSCertificate parses the given TLS certificate paired with the private key
// to return a tls.Certificate, ready to be used in a TLS handshake.
func (s *StandardSigner) TLSCertificate(certRaw []byte) (tls.Certificate, error) {
	cert, err := tls.X509KeyPair(certRaw, s.keyPEM)
	return cert, trace.Wrap(err)
}
