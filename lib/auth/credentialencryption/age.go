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

	"filippo.io/age"
	"github.com/gravitational/trace"
)

// stanzaType is the age stanza type used for wrapping file keys of user
// external credentials.
const stanzaType = "teleport-user-credential-rsa4096"

// recipient implements age.Recipient. It wraps the file key with RSA-OAEP
// SHA-256.
type recipient struct {
	publicKey   *rsa.PublicKey
	fingerprint string
}

// Wrap implements age.Recipient.
func (r *recipient) Wrap(fileKey []byte) ([]*age.Stanza, error) {
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, r.publicKey, fileKey, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []*age.Stanza{{
		Type: stanzaType,
		Args: []string{r.fingerprint},
		Body: wrapped,
	}}, nil
}

// identity implements age.Identity. It unwraps the file key using the
// keystore decrypter of the credential encryption key.
type identity struct {
	ctx      context.Context
	keyStore KeyStore
	key      *loadedKey
}

// Unwrap implements age.Identity.
func (i *identity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	for _, stanza := range stanzas {
		if stanza.Type != stanzaType || len(stanza.Args) != 1 || stanza.Args[0] != i.key.fingerprint {
			continue
		}
		decrypter, err := i.keyStore.GetDecrypter(i.ctx, i.key.keyPair)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		fileKey, err := decrypter.Decrypt(rand.Reader, stanza.Body, &rsa.OAEPOptions{
			Hash: crypto.SHA256,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fileKey, nil
	}
	return nil, age.ErrIncorrectIdentity
}
