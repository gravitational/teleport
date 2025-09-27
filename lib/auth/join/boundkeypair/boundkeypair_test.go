/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package boundkeypair

import (
	"context"
	"crypto"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

// memoryFS is a trivial in memory fs backend for testing use.
type memoryFS struct {
	files  map[string][]byte
	writes uint
}

func (f *memoryFS) Read(ctx context.Context, name string) ([]byte, error) {
	data, ok := f.files[name]
	if !ok {
		return nil, trace.NotFound("not found: %s", name)
	}

	return data, nil
}

func (f *memoryFS) Write(ctx context.Context, name string, data []byte) error {
	f.writes++
	f.files[name] = data
	return nil
}

func TestClientState(t *testing.T) {
	ctx := context.Background()
	fs := &memoryFS{
		files: map[string][]byte{},
	}

	getSuite := cryptosuites.StaticAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1)

	state, err := NewUnboundClientState(ctx, fs, getSuite)
	require.NoError(t, err)

	// Nothing should be written until `Store` is called explicitly
	require.Empty(t, fs.files)

	require.NoError(t, state.Store(ctx))
	require.Len(t, fs.files, 3)
	require.Len(t, state.KeyHistory, 1)
	require.EqualValues(t, 3, fs.writes)

	require.Equal(t, fs.files[PrivateKeyPath], state.PrivateKeyBytes)

	// Keep the original key for later.
	firstKey := state.PublicKeyBytes

	prevKey := firstKey
	expectWrites := 3

	// Simulate writes up to the key history recording length.
	for i := range KeyHistoryLength - 1 {
		// We should still be able to load the original signer (< KeyHistoryLength)
		_, err := state.SignerForPublicKey(firstKey)
		require.NoError(t, err)

		// Similarly, the previous key should still be accessible.
		_, err = state.SignerForPublicKey(prevKey)
		require.NoError(t, err)

		prevKey = state.PrivateKey.MarshalSSHPublicKey()

		// Generate a new keypair. It should be added to the history, but not marked
		// as active.
		signer, err := state.GenerateKeypair(ctx, getSuite)
		require.NoError(t, err)
		require.Len(t, state.KeyHistory, 2+i)
		require.EqualValues(t, expectWrites, fs.writes, "no new writes expected")
		require.Equal(t, fs.files[PrivateKeyPath], state.PrivateKeyBytes, "active key should not change on generation")
		require.NotEqual(t, state.KeyHistory[0].PrivateKey, state.KeyHistory[1].PrivateKey)

		// Now explicitly set the new active key
		require.NoError(t, state.SetActiveKey(signer))
		require.NotEqual(t, state.PublicKeyBytes, prevKey, "public key bytes must change for new key")

		require.NoError(t, state.Store(ctx))
		expectWrites += 3

		// Load a fresh state for the next iteration.
		state, err = LoadClientState(ctx, fs)
		require.NoError(t, err)
	}

	// Generate a final keypair. This should push out the initial key. We'll
	// have reached the history limit, so the total length should not change.
	require.Len(t, state.KeyHistory, 10)
	_, err = state.GenerateKeypair(ctx, getSuite)
	require.NoError(t, err)
	require.Len(t, state.KeyHistory, 10)

	require.NoError(t, state.Store(ctx))
	state, err = LoadClientState(ctx, fs)
	require.NoError(t, err)

	// Try to load the original key again; it should fail.
	_, err = state.SignerForPublicKey(firstKey)
	require.Error(t, err)
}

func TestStaticClientState(t *testing.T) {
	getSuite := cryptosuites.StaticAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1)
	key, err := cryptosuites.GenerateKey(t.Context(), getSuite, cryptosuites.BoundKeypairJoining)
	require.NoError(t, err)

	keyEq, ok := key.Public().(interface {
		Equal(x crypto.PublicKey) bool
	})
	require.True(t, ok)

	sshPubKey, err := ssh.NewPublicKey(key.Public())
	require.NoError(t, err)

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	publicKeyString := strings.TrimSpace(string(publicKeyBytes))

	wrongKey, err := cryptosuites.GenerateKey(t.Context(), getSuite, cryptosuites.BoundKeypairJoining)
	require.NoError(t, err)

	wrongSSHPubKey, err := ssh.NewPublicKey(wrongKey.Public())
	require.NoError(t, err)
	wrongPublicKeyBytes := ssh.MarshalAuthorizedKey(wrongSSHPubKey)
	wrongPublicKeyString := strings.TrimSpace(string(wrongPublicKeyBytes))

	privateKeyBytes, err := keys.MarshalPrivateKey(key)
	require.NoError(t, err)

	static := NewStaticClientState(privateKeyBytes)

	params := static.ToJoinParams(ClientParams{
		RegistrationSecret: "test",
	})
	require.Empty(t, params.RegistrationSecret, "registration secret must not be passed through")
	require.Empty(t, params.PreviousJoinState, "previous join state must always be empty")

	signer, err := params.GetSigner(publicKeyString)
	require.NoError(t, err)

	// We should actually retrieve a signer for the public key we requested
	require.True(t, keyEq.Equal(signer.Public()))

	_, err = params.GetSigner(wrongPublicKeyString)
	require.ErrorContains(t, err, "configured static private key does match the value requested by the server")

	invalidSigner, err := params.RequestNewKeypair(t.Context(), getSuite)
	require.Nil(t, invalidSigner)
	require.ErrorContains(t, err, "static private keys do not support automatic rotation")

	// no-op, but shouldn't return an error
	require.NoError(t, static.Store(t.Context()))

	// no-op, but shouldn't return an error
	require.NoError(t, static.UpdateFromRegisterResult(&join.RegisterResult{}))
}
