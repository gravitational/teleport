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

package recordingencryption

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"io"

	"filippo.io/age"
	"github.com/gravitational/trace"
)

// RecordingStanza is the type used for the identifying stanza added by RecordingRecipient.
const RecordingStanza = "teleport-recording-rsa4096"

// UnwrapInput represents a request to decrypt a wrapped file key.
type UnwrapInput struct {
	// Fingerprint of the public key used to find the related private key.
	Fingerprint string
	// WrappedKey is the encrypted file key in an encrypted recording stanza.
	WrappedKey []byte

	// Rand reader to pass to use during decryption.
	Rand io.Reader
	// Opts that should be used during decryption.
	Opts crypto.DecrypterOpts
}

// KeyUnwrapper returns an unwrapped file key given a wrapped key and a fingerprint of the encryption key.
type KeyUnwrapper interface {
	UnwrapKey(ctx context.Context, in UnwrapInput) ([]byte, error)
}

// RecordingIdentity unwraps file keys using the configured [KeyUnwrapper] and the recording stanzas
// included in the age header.
type RecordingIdentity struct {
	ctx       context.Context
	unwrapper KeyUnwrapper
}

// NewRecordingIdentity returns a new RecordingIdentity using the given [KeyUnwrapper]
// file key unwrapping.
func NewRecordingIdentity(ctx context.Context, unwrapper KeyUnwrapper) *RecordingIdentity {
	return &RecordingIdentity{
		ctx:       ctx,
		unwrapper: unwrapper,
	}
}

// Unwrap uses the additional stanzas added by [RecordingRecipient.Wrap] in order to find a matching RSA 4096
// private key.
func (i *RecordingIdentity) Unwrap(stanzas []*age.Stanza) ([]byte, error) {
	var errs []error
	for _, stanza := range stanzas {
		if stanza.Type != RecordingStanza {
			continue
		}

		if len(stanza.Args) != 1 {
			continue
		}

		fileKey, err := i.unwrapper.UnwrapKey(i.ctx, UnwrapInput{
			Rand:        rand.Reader,
			WrappedKey:  stanza.Body,
			Fingerprint: stanza.Args[0],
			Opts: &rsa.OAEPOptions{
				Hash: crypto.SHA256,
			},
		})
		if err != nil {
			if !trace.IsNotFound(err) {
				errs = append(errs, err)
			}
			continue
		}

		return fileKey, nil
	}

	if len(errs) == 0 {
		return nil, trace.Errorf("could not find an accessible decrypter for unwrapping")
	}
	return nil, trace.NewAggregate(errs...)
}

// RecordingRecipient wraps file keys using an RSA 40960public key.
type RecordingRecipient struct {
	*rsa.PublicKey
}

// ParseRecordingRecipient parses a PEM encoded RSA 4096 public key into a RecordingRecipient.
func ParseRecordingRecipient(in []byte) (*RecordingRecipient, error) {
	pubKey, err := x509.ParsePKIXPublicKey(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsaKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("recording encryption key must be a public RSA 4096")
	}

	return &RecordingRecipient{PublicKey: rsaKey}, nil
}

// Wrap a fileKey using an RSA public key. The fingerprint of the key will be included in the stanza
// to aid in fetching the correct private key during [Unwrap].
func (r *RecordingRecipient) Wrap(fileKey []byte) ([]*age.Stanza, error) {
	cipher, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, r.PublicKey, fileKey, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fp, err := Fingerprint(r.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []*age.Stanza{
		{
			Type: RecordingStanza,
			Args: []string{fp},
			Body: cipher,
		},
	}, nil
}
