// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package internal

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

func SSHSignerFromCryptoSigner(cryptoSigner crypto.Signer) (ssh.Signer, error) {
	sshSigner, err := ssh.NewSignerFromSigner(cryptoSigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// [ssh.NewSignerFromSigner] currently always returns an [ssh.AlgorithmSigner].
	algorithmSigner, ok := sshSigner.(ssh.AlgorithmSigner)
	if !ok {
		return nil, trace.BadParameter("SSH CA: unsupported key type: %s", sshSigner.PublicKey().Type())
	}
	// Note: we don't actually create keys with all the algorithms supported
	// below, but customers have been known to import their own existing keys.
	switch pub := cryptoSigner.Public().(type) {
	case *rsa.PublicKey:
		// The current default hash used in ssh.(*Certificate).SignCert for an
		// RSA signer created via ssh.NewSignerFromSigner is always SHA256,
		// irrespective of the key size.
		// This was a change in golang.org/x/crypto 0.14.0, prior to that the
		// default was always SHA512.
		//
		// Due to the historical SHA512 default that existed at a time when
		// hash algorithm selection was much more difficult, there are many
		// existing GCP KMS keys that were created as 4096-bit keys using a
		// SHA512 hash. GCP KMS is very particular about RSA hash algorithms:
		// - 2048-bit or 3072-bit keys *must* use SHA256
		// - 4096-bit keys *must* use SHA256 or SHA512
		// - the hash length must be set *when the key is created* and can't be
		//   changed.
		//
		// The chosen signature algorithms below are necessary to support
		// existing GCP KMS keys, but they are also reasonable defaults for keys
		// outside of GCP KMS.
		//
		// [rsa.PublicKey.Size()] returns 256 for a 2048-bit key; more generally
		// it always returns the bit length divided by 8.
		keySize := pub.Size()
		switch {
		case keySize < 256:
			return nil, trace.BadParameter("SSH CA: RSA key size (%d) is too small", keySize)
		case keySize < 512:
			// This case matches 2048 and 3072 bit GCP KMS keys which *must* use SHA256.
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoRSASHA256})
		default:
			// This case matches existing 4096 bit GCP KMS keys which *must* use SHA512
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoRSASHA512})
		}
	case *ecdsa.PublicKey:
		// These are all the current defaults, but let's set them explicitly so
		// golang.org/x/crypto/ssh can't change them in an update and break some
		// HSM or KMS that wouldn't support the new default.
		switch pub.Curve {
		case elliptic.P256():
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoECDSA256})
		case elliptic.P384():
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoECDSA384})
		case elliptic.P521():
			return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoECDSA521})
		default:
			return nil, trace.BadParameter("SSH CA: ECDSA curve: %s", pub.Curve.Params().Name)
		}
	case ed25519.PublicKey:
		// This is the current default, but let's set it explicitly so
		// golang.org/x/crypto/ssh can't change it in an update and break some
		// HSM or KMS that wouldn't support the new default.
		return ssh.NewSignerWithAlgorithms(algorithmSigner, []string{ssh.KeyAlgoED25519})
	default:
		return nil, trace.BadParameter("SSH CA: unsupported key type: %s", sshSigner.PublicKey().Type())
	}
}
