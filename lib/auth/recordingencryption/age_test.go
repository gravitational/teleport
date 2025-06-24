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

package recordingencryption_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
)

func TestRecordingAgePlugin(t *testing.T) {
	ctx := t.Context()
	keyFinder := newFakeKeyFinder()
	recordingIdentity := recordingencryption.NewRecordingIdentity(ctx, keyFinder)

	ident, err := keyFinder.generateIdentity()
	require.NoError(t, err)

	recipient, err := recordingencryption.ParseRecordingRecipient(ident.Recipient().String())
	require.NoError(t, err)

	out := bytes.NewBuffer(nil)
	writer, err := age.Encrypt(out, recipient)
	require.NoError(t, err)

	msg := []byte("testing age plugin for session recordings")
	_, err = writer.Write(msg)
	require.NoError(t, err)

	// writer must be closed to ensure data is flushed
	err = writer.Close()
	require.NoError(t, err)

	reader, err := age.Decrypt(out, recordingIdentity)
	require.NoError(t, err)
	plaintext, err := io.ReadAll(reader)
	require.NoError(t, err)

	require.Equal(t, msg, plaintext)

	// running the same test with the raw recipient should fail because the
	// the extra stanza added by RecordingRecipient won't be present and
	// the private key won't be found
	out.Reset()
	writer, err = age.Encrypt(out, ident.Recipient())
	require.NoError(t, err)
	_, err = writer.Write(msg)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)
	_, err = age.Decrypt(out, recordingIdentity)
	require.Error(t, err)
}

type fakeKeyFinder struct {
	keys map[string]string
}

func newFakeKeyFinder() *fakeKeyFinder {
	return &fakeKeyFinder{
		keys: make(map[string]string),
	}
}

func (f *fakeKeyFinder) FindDecryptionKey(ctx context.Context, publicKeys ...[]byte) (*types.EncryptionKeyPair, error) {
	for _, pubKey := range publicKeys {
		key, ok := f.keys[string(pubKey)]
		if !ok {
			continue
		}

		return &types.EncryptionKeyPair{
			PrivateKey: []byte(key),
			PublicKey:  pubKey,
		}, nil
	}

	return nil, errors.New("no accessible decryption key found")
}

func (f *fakeKeyFinder) generateIdentity() (*age.X25519Identity, error) {
	ident, err := age.GenerateX25519Identity()
	if err != nil {
		return nil, err
	}

	f.keys[ident.Recipient().String()] = ident.String()
	return ident, nil
}
