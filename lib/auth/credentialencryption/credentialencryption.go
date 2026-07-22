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
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/json"
	"io"
	"sync"

	"filippo.io/age"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// TokenData holds the sensitive token fields to be encrypted together.
type TokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

var credentialEncryptionKeyPath = backend.NewKey("credential_encryption_key")

// Encryptor encrypts and decrypts credential tokens using envelope encryption
// via the Age library. The file key is wrapped with RSA-OAEP SHA-256 using the
// cluster's keystore.
//
// The public key is never persisted to the backend. It is derived at runtime
// from the keystore and cached in memory.
type Encryptor struct {
	keyStore *keystore.Manager
	backend  backend.Backend

	mu        sync.Mutex
	keyPair   *types.EncryptionKeyPair
	recipient *recordingencryption.RecordingRecipient
}

// NewEncryptor creates a new credential encryptor.
func NewEncryptor(keyStore *keystore.Manager, backend backend.Backend) *Encryptor {
	return &Encryptor{
		keyStore: keyStore,
		backend:  backend,
	}
}

// getOrCreateKeyPair returns the encryption key pair, creating one if it
// doesn't exist. The key pair is cached in memory after first load. The
// public key is derived from the keystore, not from the backend.
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
		if err := e.deriveAndCachePublicKey(ctx, &kp); err != nil {
			return nil, trace.Wrap(err)
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

	stored := &types.EncryptionKeyPair{
		PrivateKey:     kp.PrivateKey,
		PrivateKeyType: kp.PrivateKeyType,
		Hash:           kp.Hash,
	}
	value, err := json.Marshal(stored)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := e.backend.Create(ctx, backend.Item{
		Key:   credentialEncryptionKeyPath,
		Value: value,
	}); err != nil {
		if trace.IsAlreadyExists(err) {
			item, err := e.backend.Get(ctx, credentialEncryptionKeyPath)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			var existing types.EncryptionKeyPair
			if err := json.Unmarshal(item.Value, &existing); err != nil {
				return nil, trace.Wrap(err)
			}
			if err := e.deriveAndCachePublicKey(ctx, &existing); err != nil {
				return nil, trace.Wrap(err)
			}
			e.keyPair = &existing
			return e.keyPair, nil
		}
		return nil, trace.Wrap(err)
	}

	if err := e.deriveAndCachePublicKey(ctx, stored); err != nil {
		return nil, trace.Wrap(err)
	}
	e.keyPair = stored
	return e.keyPair, nil
}

// deriveAndCachePublicKey retrieves the public key from the keystore and
// caches it in memory as an Age recipient.
func (e *Encryptor) deriveAndCachePublicKey(ctx context.Context, kp *types.EncryptionKeyPair) error {
	if err := e.keyStore.DerivePublicKey(ctx, kp); err != nil {
		return trace.Wrap(err, "deriving public key")
	}
	recipient, err := recordingencryption.ParseRecordingRecipient(kp.PublicKey)
	if err != nil {
		return trace.Wrap(err, "creating Age recipient")
	}
	e.recipient = recipient
	return nil
}

// Encrypt encrypts token data using Age envelope encryption.
func (e *Encryptor) Encrypt(ctx context.Context, data TokenData) ([]byte, error) {
	if _, err := e.getOrCreateKeyPair(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	plaintext, err := json.Marshal(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, e.recipient)
	if err != nil {
		return nil, trace.Wrap(err, "creating Age encrypter")
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, trace.Wrap(err, "writing to Age encrypter")
	}
	if err := w.Close(); err != nil {
		return nil, trace.Wrap(err, "closing Age encrypter")
	}

	return buf.Bytes(), nil
}

// Decrypt decrypts token data using Age envelope encryption via the keystore.
func (e *Encryptor) Decrypt(ctx context.Context, ciphertext []byte) (TokenData, error) {
	kp, err := e.getOrCreateKeyPair(ctx)
	if err != nil {
		return TokenData{}, trace.Wrap(err)
	}

	identity := &credentialIdentity{
		ctx:     ctx,
		keyPair: kp,
		keyStore: e.keyStore,
	}

	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return TokenData{}, trace.Wrap(err, "decrypting credential")
	}

	plaintext, err := io.ReadAll(r)
	if err != nil {
		return TokenData{}, trace.Wrap(err, "reading decrypted credential")
	}

	var data TokenData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return TokenData{}, trace.Wrap(err, "unmarshaling decrypted credentials")
	}
	return data, nil
}

// credentialIdentity implements age.Identity for credential decryption.
// It unwraps the Age file key using the keystore's decrypter.
type credentialIdentity struct {
	ctx      context.Context
	keyPair  *types.EncryptionKeyPair
	keyStore *keystore.Manager
}

func (c *credentialIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	decrypter, err := c.keyStore.GetDecrypter(c.ctx, c.keyPair)
	if err != nil {
		return nil, trace.Wrap(err, "getting decrypter")
	}

	for _, stanza := range stanzas {
		if stanza.Type != recordingencryption.RecordingStanza {
			continue
		}
		fileKey, err := decrypter.Decrypt(nil, stanza.Body, &rsa.OAEPOptions{
			Hash: crypto.SHA256,
		})
		if err != nil {
			continue
		}
		return fileKey, nil
	}

	return nil, trace.NotFound("no matching stanza found")
}
