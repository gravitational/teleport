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
	"context"
	"crypto"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// backend is an interface that holds private keys and provides signing and decryption
// operations.
type Backend interface {
	// generateSigner creates a new key pair and returns its identifier and a crypto.Signer. The returned
	// identifier can be passed to getSigner later to get an equivalent crypto.Signer.
	GenerateSigner(context.Context, cryptosuites.Algorithm) (keyID []byte, signer crypto.Signer, err error)

	// generateDecrypter creates a new key pair and returns its identifier and a crypto.Decrypter. The returned
	// identifier can be passed to getDecrypter later to get an equivalent crypto.Decrypter.
	GenerateDecrypter(context.Context, cryptosuites.Algorithm) (keyID []byte, decrypter crypto.Decrypter, hash crypto.Hash, err error)

	// getSigner returns a crypto.Signer for the given key identifier, if it is found.
	// The public key is passed as well so that it does not need to be fetched
	// from the underlying backend, and it is always stored in the CA anyway.
	GetSigner(ctx context.Context, keyID []byte, pub crypto.PublicKey) (crypto.Signer, error)

	// getDecrypter returns a crypto.Decrypter for the given key identifier, if it is found.
	// The public key is passed as well so that it does not need to be fetched
	// from the underlying backend.
	GetDecrypter(ctx context.Context, keyID []byte, pub crypto.PublicKey, hash crypto.Hash) (crypto.Decrypter, error)

	// canUseKey returns true if this backend is able to sign or decrypt with the
	// given key.
	CanUseKey(ctx context.Context, raw []byte, keyType types.PrivateKeyType) (bool, error)

	// deleteKey deletes the given key from the backend.
	DeleteKey(ctx context.Context, keyID []byte) error

	// deleteUnusedKeys deletes all keys from the backend if they are:
	// 1. Not included in the argument activeKeys which is meant to contain all
	//    active keys currently referenced in the backend CA.
	// 2. Created in the backend by this Teleport cluster.
	// 3. Each backend may apply extra restrictions to which keys may be deleted.
	DeleteUnusedKeys(ctx context.Context, activeKeys [][]byte) error

	// keyTypeDescription returns a human-readable description of the types of
	// keys this backend uses.
	KeyTypeDescription() string

	// name returns the name of the backend.
	Name() string
}
