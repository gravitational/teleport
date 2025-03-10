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

package cert

import (
	"crypto"
	"crypto/elliptic"
	"crypto/rand"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/cryptopatch"
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
	caKey, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	key, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
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
