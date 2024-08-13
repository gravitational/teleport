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

package sshutils

import (
	"crypto"
	"io"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/utils/sshutils"
)

// LegacySHA1Signer always forces use of SHA-1 for signing. It should be not
// be used until necessary.
// This struct should not implement SignWithAlgorithm() method
// from ssh.AlgorithmSigner interface. This would break the SHA-1 signing.
type LegacySHA1Signer struct {
	Signer ssh.AlgorithmSigner
}

// PublicKey returns the public key from the underlying signer.
func (s *LegacySHA1Signer) PublicKey() ssh.PublicKey {
	return s.Signer.PublicKey()
}

// Sign forces the SHA-1 signature.
func (s *LegacySHA1Signer) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	return s.Signer.SignWithAlgorithm(rand, data, ssh.KeyAlgoRSA)
}

// NewSigner returns new ssh Signer from private key + certificate pair.  The
// signer can be used to create "auth methods" i.e. login into Teleport SSH
// servers.
func NewSigner(keyBytes, certBytes []byte) (ssh.Signer, error) {
	keySigner, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse SSH private key")
	}

	cert, err := sshutils.ParseCertificate(certBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse SSH certificate")
	}

	return ssh.NewCertSigner(cert, keySigner)
}

// CryptoPublicKey extracts a crypto.PublicKey from any public key in
// authorized_keys format.
func CryptoPublicKey(publicKey []byte) (crypto.PublicKey, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cryptoPubKey, ok := pubKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, trace.BadParameter("expected ssh.CryptoPublicKey, got %T", pubKey)
	}
	return cryptoPubKey.CryptoPublicKey(), nil
}
