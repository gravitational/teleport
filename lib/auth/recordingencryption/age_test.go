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
	"io"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
)

func TestRecordingAgePlugin(t *testing.T) {
	ctx := t.Context()
	keyStore := newFakeKeyStore(types.PrivateKeyType_RAW)
	recordingIdentity := recordingencryption.NewRecordingIdentity(ctx, keyStore)

	_, pubKey, err := keyStore.createKey()
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
	_, pubKey, err = keyStore.genKeys()
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
