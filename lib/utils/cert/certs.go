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
	"crypto/rand"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

// CreateTestRSACertificate creates a valid 2048-bit RSA certificate.
func CreateTestRSACertificate(principal string, certType uint32) (*ssh.Certificate, ssh.Signer, error) {
	return createCertificate(principal, certType, cryptosuites.RSA2048)
}

// CreateTestECDSACertificate creates a valid ECDSA P-256 certificate.
func CreateTestECDSACertificate(principal string, certType uint32) (*ssh.Certificate, ssh.Signer, error) {
	return createCertificate(principal, certType, cryptosuites.ECDSAP256)
}

// CreateTestEd25519Certificate creates an Ed25519 certificate which should be
// rejected in FIPS mode.
func CreateTestEd25519Certificate(principal string, certType uint32) (*ssh.Certificate, ssh.Signer, error) {
	return createCertificate(principal, certType, cryptosuites.Ed25519)
}

// createCertificate creates a SSH certificate for the given key signed by the
// given CA key. This function exists here to allow easy key generation for
// some of the more core packages like "sshutils".
func createCertificate(principal string, certType uint32, algo cryptosuites.Algorithm) (*ssh.Certificate, ssh.Signer, error) {
	// Create CA.
	caKey, err := cryptosuites.GenerateKeyWithAlgorithm(algo)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	caPublicKey, err := ssh.NewPublicKey(caKey.Public())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	caSigner, err := ssh.NewSignerFromKey(caKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Create key.
	key, err := cryptosuites.GenerateKeyWithAlgorithm(algo)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
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
