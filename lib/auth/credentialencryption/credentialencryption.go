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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// TokenData holds the sensitive token fields to be encrypted together.
type TokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

var credentialEncryptionKeyPath = backend.NewKey("credential_encryption_key")

// Encryptor encrypts and decrypts credential tokens using an RSA key pair
// managed by the keystore. With HSM/KMS-backed keystores, the private key
// never leaves the hardware.
type Encryptor struct {
	keyStore *keystore.Manager
	backend  backend.Backend

	mu      sync.Mutex
	keyPair *types.EncryptionKeyPair
}

// NewEncryptor creates a new credential encryptor.
func NewEncryptor(keyStore *keystore.Manager, backend backend.Backend) *Encryptor {
	return &Encryptor{
		keyStore: keyStore,
		backend:  backend,
	}
}

// getOrCreateKeyPair returns the encryption key pair, creating one if it
// doesn't exist. The key pair is cached in memory after first load.
func (e *Encryptor) getOrCreateKeyPair(ctx context.Context) (*types.EncryptionKeyPair, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.keyPair != nil {
		return e.keyPair, nil
	}

	item, err := e.backend.Get(ctx, credentialEncryptionKeyPath)
	if err == nil {
		var kp types.EncryptionKeyPair
		if err := json.Unmarshal(item.Value, &kp); err != nil {
			return nil, trace.Wrap(err, "unmarshaling credential encryption key")
		}
		e.keyPair = &kp
		return e.keyPair, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	kp, err := e.keyStore.NewEncryptionKeyPair(ctx, cryptosuites.RecordingKeyWrapping)
	if err != nil {
		return nil, trace.Wrap(err, "generating credential encryption key pair")
	}

	value, err := json.Marshal(kp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := e.backend.Create(ctx, backend.Item{
		Key:   credentialEncryptionKeyPath,
		Value: value,
	}); err != nil {
		if trace.IsAlreadyExists(err) {
			// Another auth server created it first, load it.
			item, err := e.backend.Get(ctx, credentialEncryptionKeyPath)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			var existing types.EncryptionKeyPair
			if err := json.Unmarshal(item.Value, &existing); err != nil {
				return nil, trace.Wrap(err)
			}
			e.keyPair = &existing
			return e.keyPair, nil
		}
		return nil, trace.Wrap(err)
	}

	e.keyPair = kp
	return e.keyPair, nil
}

// Encrypt encrypts token data using the encryption key pair's public key.
func (e *Encryptor) Encrypt(ctx context.Context, data TokenData) ([]byte, error) {
	kp, err := e.getOrCreateKeyPair(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pub, err := x509.ParsePKIXPublicKey(kp.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err, "parsing encryption public key")
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("credential encryption requires RSA key, got %T", pub)
	}

	plaintext, err := json.Marshal(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, plaintext, nil)
	if err != nil {
		return nil, trace.Wrap(err, "encrypting credentials")
	}
	return ciphertext, nil
}

// Decrypt decrypts token data using the encryption key pair's private key
// via the keystore. With HSM/KMS, the decryption happens in hardware.
func (e *Encryptor) Decrypt(ctx context.Context, ciphertext []byte) (TokenData, error) {
	kp, err := e.getOrCreateKeyPair(ctx)
	if err != nil {
		return TokenData{}, trace.Wrap(err)
	}

	decrypter, err := e.keyStore.GetDecrypter(ctx, kp)
	if err != nil {
		return TokenData{}, trace.Wrap(err, "getting decrypter")
	}

	plaintext, err := decrypter.Decrypt(rand.Reader, ciphertext, &rsa.OAEPOptions{
		Hash: crypto.SHA256,
	})
	if err != nil {
		return TokenData{}, trace.Wrap(err, "decrypting credentials")
	}

	var data TokenData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return TokenData{}, trace.Wrap(err, "unmarshaling decrypted credentials")
	}
	return data, nil
}
