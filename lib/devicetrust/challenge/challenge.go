// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package challenge

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	_ "crypto/sha256" // imported to register crypto.SHA256 in init func

	"github.com/gravitational/trace"
)

const challengeLength = 32

// New creates a new Device Trust challenge with the default length.
func New() ([]byte, error) {
	b := make([]byte, challengeLength)
	if _, err := rand.Read(b); err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

// Verify verifies that [sig] is a signature over [chal] by the private key
// corresponding to [pubKey].
func Verify(chal, sig []byte, pubKey crypto.PublicKey) error {
	switch pub := pubKey.(type) {
	case *ecdsa.PublicKey:
		digest, err := hash(crypto.SHA256, chal)
		if err != nil {
			return trace.Wrap(err)
		}
		if !ecdsa.VerifyASN1(pub, digest, sig) {
			return trace.BadParameter("ecdsa verification failed")
		}
		return nil

	case *rsa.PublicKey:
		digest, err := hash(crypto.SHA256, chal)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest, sig))

	case ed25519.PublicKey:
		// ed25519 is a special snowflake: the PublicKey type is not a pointer,
		// and it doesn't like to pre-hash.
		if !ed25519.Verify(pub, chal, sig) {
			return trace.BadParameter("ed25519 verification failed")
		}
		return nil

	default:
		return trace.BadParameter("unsupported key type: %T", pub)
	}
}

// Sign returns a signature over [challenge] by [signer]. A SHA256 hash is used
// for ECDSA and RSA, no pre-hash is used for Ed25519.
func Sign(challenge []byte, signer crypto.Signer) ([]byte, error) {
	switch pub := signer.Public().(type) {
	case *ecdsa.PublicKey, *rsa.PublicKey:
		digest, err := hash(crypto.SHA256, challenge)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		signature, err := signer.Sign(rand.Reader, digest, crypto.SHA256)
		return signature, trace.Wrap(err)
	case ed25519.PublicKey:
		// No pre-hash is used for Ed25519.
		signature, err := signer.Sign(rand.Reader, challenge, &ed25519.Options{})
		return signature, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unsupported key type: %T", pub)
	}
}

func hash(hash crypto.Hash, message []byte) ([]byte, error) {
	hasher := hash.New()
	if _, err := hasher.Write(message); err != nil {
		return nil, trace.Wrap(err)
	}
	return hasher.Sum(nil), nil
}
