/*
Copyright 2019-2022 Gravitational, Inc.

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

package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/auth/native"
)

// CreateCertificate creates a valid 2048-bit RSA certificate.
func CreateCertificate(principal string, certType uint32) (*ssh.Certificate, ssh.Signer, error) {
	// Create RSA key for CA and certificate to be signed by CA.
	caKey, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	key, err := native.GenerateRSAPrivateKey()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	cert, certSigner, err := createCertificate(principal, certType, caKey, key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cert, certSigner, nil
}

// CreateEllipticCertificate creates a valid, but not supported, ECDSA
// SSH certificate. This certificate is used to make sure Teleport rejects
// such certificates.
func CreateEllipticCertificate(principal string, certType uint32) (*ssh.Certificate, ssh.Signer, error) {
	// Create ECDSA key for CA and certificate to be signed by CA.
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	cert, certSigner, err := createCertificate(principal, certType, caKey, key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cert, certSigner, nil
}

// createCertificate creates a SSH certificate for the given key signed by the
// given CA key. This function exists here to allow easy key generation for
// some of the more core packages like "sshutils".
func createCertificate(principal string, certType uint32, caKey crypto.Signer, key crypto.Signer) (*ssh.Certificate, ssh.Signer, error) {
	// Create CA.
	caPublicKey, err := ssh.NewPublicKey(caKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	caSigner, err := ssh.NewSignerFromKey(caKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Create key.
	publicKey, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	keySigner, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Create certificate and signer.
	cert := &ssh.Certificate{
		KeyId:           principal,
		ValidPrincipals: []string{principal},
		Key:             publicKey,
		SignatureKey:    caPublicKey,
		ValidAfter:      uint64(time.Now().UTC().Add(-1 * time.Minute).Unix()),
		ValidBefore:     uint64(time.Now().UTC().Add(1 * time.Minute).Unix()),
		CertType:        certType,
	}
	err = cert.SignCert(rand.Reader, caSigner)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certSigner, err := ssh.NewCertSigner(cert, keySigner)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return cert, certSigner, nil
}
