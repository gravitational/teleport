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

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
)

func TestDecryptReaderIfNeeded(t *testing.T) {
	ctx := t.Context()

	keyStore := newFakeKeyStore(types.PrivateKeyType_RAW)
	_, publicKey, err := keyStore.createKey()
	require.NoError(t, err)

	srcGetter, err := newFakeSRCGetter(true, []*types.AgeEncryptionKey{
		{PublicKey: publicKey},
	})
	require.NoError(t, err)

	encryptedIO, err := recordingencryption.NewEncryptedIO(srcGetter, keyStore)
	require.NoError(t, err)

	encryptData := func(t *testing.T, plaintext []byte) []byte {
		t.Helper()
		out := bytes.NewBuffer(nil)
		writer, err := encryptedIO.WithEncryption(ctx, &writeCloser{Writer: out})
		require.NoError(t, err)
		_, err = writer.Write(plaintext)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		return out.Bytes()
	}

	t.Run("plaintext passthrough", func(t *testing.T) {
		plaintext := []byte("not encrypted data")
		rc := io.NopCloser(bytes.NewReader(plaintext))
		result, err := recordingencryption.DecryptReaderIfEncrypted(ctx, rc, encryptedIO)
		require.NoError(t, err)
		defer result.Close()

		got, err := io.ReadAll(result)
		require.NoError(t, err)
		require.Equal(t, plaintext, got)
	})

	t.Run("plaintext shorter than prefix", func(t *testing.T) {
		plaintext := []byte("hi")
		rc := io.NopCloser(bytes.NewReader(plaintext))
		result, err := recordingencryption.DecryptReaderIfEncrypted(ctx, rc, encryptedIO)
		require.NoError(t, err)
		defer result.Close()

		got, err := io.ReadAll(result)
		require.NoError(t, err)
		require.Equal(t, plaintext, got)
	})

	t.Run("empty reader", func(t *testing.T) {
		rc := io.NopCloser(bytes.NewReader(nil))
		result, err := recordingencryption.DecryptReaderIfEncrypted(ctx, rc, nil)
		require.NoError(t, err)
		defer result.Close()

		got, err := io.ReadAll(result)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("encrypted data is decrypted", func(t *testing.T) {
		msg := []byte("secret recording data")
		encrypted := encryptData(t, msg)

		rc := io.NopCloser(bytes.NewReader(encrypted))
		result, err := recordingencryption.DecryptReaderIfEncrypted(ctx, rc, encryptedIO)
		require.NoError(t, err)
		defer result.Close()

		got, err := io.ReadAll(result)
		require.NoError(t, err)
		require.Equal(t, msg, got)
	})

	t.Run("encrypted data without decrypter returns error", func(t *testing.T) {
		msg := []byte("secret recording data")
		encrypted := encryptData(t, msg)

		rc := io.NopCloser(bytes.NewReader(encrypted))
		_, err := recordingencryption.DecryptReaderIfEncrypted(ctx, rc, nil)
		require.Error(t, err)
	})

	t.Run("close propagates to underlying reader", func(t *testing.T) {
		plaintext := []byte("not encrypted data")
		trackingRC := &trackingReadCloser{Reader: bytes.NewReader(plaintext)}
		result, err := recordingencryption.DecryptReaderIfEncrypted(ctx, trackingRC, nil)
		require.NoError(t, err)

		require.False(t, trackingRC.closed)
		require.NoError(t, result.Close())
		require.True(t, trackingRC.closed)
	})

	t.Run("close propagates to underlying reader when encrypted", func(t *testing.T) {
		msg := []byte("secret recording data")
		encrypted := encryptData(t, msg)

		trackingRC := &trackingReadCloser{Reader: bytes.NewReader(encrypted)}
		result, err := recordingencryption.DecryptReaderIfEncrypted(ctx, trackingRC, encryptedIO)
		require.NoError(t, err)

		require.False(t, trackingRC.closed)
		require.NoError(t, result.Close())
		require.True(t, trackingRC.closed)
	})
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}
