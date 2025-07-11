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
	"crypto"
	"io"
	"testing"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

func TestRecordingAgePlugin(t *testing.T) {
	ctx := t.Context()
	keyFinder := newFakeKeyUnwrapper()
	recordingIdentity := recordingencryption.NewRecordingIdentity(ctx, keyFinder)

	pubKey, err := keyFinder.generateIdentity()
	require.NoError(t, err)

	recipient, err := recordingencryption.ParseRecordingRecipient(pubKey)
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

	// decrypted text should match original msg
	reader, err := age.Decrypt(out, recordingIdentity)
	require.NoError(t, err)
	plaintext, err := io.ReadAll(reader)
	require.NoError(t, err)

	require.Equal(t, msg, plaintext)

	// running the same test with an unknown public key should fail
	_, pubKey, err = genKeys()
	require.NoError(t, err)

	recipient, err = recordingencryption.ParseRecordingRecipient(pubKey)
	require.NoError(t, err)
	out.Reset()
	writer, err = age.Encrypt(out, recipient)
	require.NoError(t, err)
	_, err = writer.Write(msg)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)
	_, err = age.Decrypt(out, recordingIdentity)
	require.Error(t, err)
}

type fakeKeyUnwrapper struct {
	keys map[string]crypto.Decrypter
}

func newFakeKeyUnwrapper() *fakeKeyUnwrapper {
	return &fakeKeyUnwrapper{
		keys: make(map[string]crypto.Decrypter),
	}
}

func (f *fakeKeyUnwrapper) UnwrapKey(ctx context.Context, in recordingencryption.UnwrapInput) ([]byte, error) {
	decrypter, ok := f.keys[in.Fingerprint]
	if !ok {
		return nil, trace.NotFound("no accessible decryption key found")
	}

	fileKey, err := decrypter.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	if err != nil {
		return nil, err
	}

	return fileKey, nil
}

func genKeys() (crypto.Decrypter, []byte, error) {
	decrypter, err := cryptosuites.GenerateDecrypterWithAlgorithm(cryptosuites.RSA4096)
	if err != nil {
		return nil, nil, err
	}

	publicKey, err := keys.MarshalPublicKey(decrypter.Public())
	if err != nil {
		return nil, nil, err
	}

	return decrypter, publicKey, nil
}

func (f *fakeKeyUnwrapper) generateIdentity() ([]byte, error) {
	decrypter, publicKey, err := genKeys()
	if err != nil {
		return nil, err
	}

	fp, err := recordingencryption.Fingerprint(decrypter.Public())
	if err != nil {
		return nil, err
	}

	f.keys[fp] = decrypter

	return publicKey, nil
}
