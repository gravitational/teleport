// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"crypto/rsa"
	"crypto/x509"
	"io"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services/local"
)

type sealerClient struct {
	*local.ClusterConfigurationService
	types.Events
}

func newSealerClient(t *testing.T) (backend.Backend, sealerClient) {
	t.Helper()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bk.Close()) })

	clusterConfig, err := local.NewClusterConfigurationService(bk)
	require.NoError(t, err)
	return bk, sealerClient{clusterConfig, local.NewEventsService(bk)}
}

func upsertSRC(t *testing.T, clt sealerClient, src *types.SessionRecordingConfigV2) {
	t.Helper()
	_, err := clt.UpsertSessionRecordingConfig(t.Context(), src)
	require.NoError(t, err)
}

type blockingSRCClient struct {
	types.Events
	src   types.SessionRecordingConfig
	calls atomic.Int64
}

func (b *blockingSRCClient) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	if b.calls.Add(1) > 1 {
		<-ctx.Done()
		return nil, trace.Wrap(ctx.Err())
	}
	return b.src, nil
}

func encryptedSRC(t *testing.T, enabled bool, pubKeys ...[]byte) *types.SessionRecordingConfigV2 {
	t.Helper()
	src := &types.SessionRecordingConfigV2{
		Spec: types.SessionRecordingConfigSpecV2{
			Encryption: &types.SessionRecordingEncryptionConfig{
				Enabled: enabled,
			},
		},
	}
	keys := make([]*types.AgeEncryptionKey, 0, len(pubKeys))
	for _, pubKey := range pubKeys {
		keys = append(keys, &types.AgeEncryptionKey{PublicKey: pubKey})
	}
	src.SetEncryptionKeys(slices.Values(keys))
	return src
}

func testRSAKeyPair(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA4096)
	require.NoError(t, err)
	key, ok := signer.(*rsa.PrivateKey)
	require.True(t, ok)
	pubDER, err := x509.MarshalPKIXPublicKey(key.Public())
	require.NoError(t, err)
	return key, pubDER
}

type testKeyUnwrapper struct {
	key *rsa.PrivateKey
}

func (u *testKeyUnwrapper) UnwrapKey(ctx context.Context, in recordingencryption.UnwrapInput) ([]byte, error) {
	fileKey, err := u.key.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	return fileKey, trace.Wrap(err)
}

func TestNewAuditQueueSealer(t *testing.T) {
	plaintext := []byte("audit event payload")

	t.Run("encryption disabled", func(t *testing.T) {
		ctx := t.Context()
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, false))
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		assert.False(t, sealed)
		assert.Equal(t, plaintext, payload)
	})

	t.Run("encryption enabled with keys", func(t *testing.T) {
		ctx := t.Context()
		keyA, pubA := testRSAKeyPair(t)
		keyB, pubB := testRSAKeyPair(t)
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, true, pubA, pubB))
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		assert.True(t, sealed)
		assert.Equal(t, plaintext, decryptPayload(t, ctx, keyA, payload))
		assert.Equal(t, plaintext, decryptPayload(t, ctx, keyB, payload))
	})

	t.Run("enabled without keys fails", func(t *testing.T) {
		ctx := t.Context()
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, true))
		_, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.ErrorAs(t, err, new(*trace.BadParameterError))
	})

	t.Run("malformed key fails", func(t *testing.T) {
		ctx := t.Context()
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, true, []byte("not a public key")))
		_, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.Error(t, err)
	})

	t.Run("config fetch failure fails", func(t *testing.T) {
		ctx := t.Context()
		_, clt := newSealerClient(t)
		_, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.ErrorAs(t, err, new(*trace.NotFoundError))
	})

	t.Run("nil getter fails", func(t *testing.T) {
		ctx := t.Context()
		_, err := recordingencryption.NewAuditQueueSealer(ctx, nil)
		require.ErrorAs(t, err, new(*trace.BadParameterError))
	})
}

func decryptPayload(t *testing.T, ctx context.Context, key *rsa.PrivateKey, payload []byte) []byte {
	t.Helper()
	identity := recordingencryption.NewRecordingIdentity(ctx, &testKeyUnwrapper{key: key})
	reader, err := age.Decrypt(bytes.NewReader(payload), identity)
	require.NoError(t, err)
	decrypted, err := io.ReadAll(reader)
	require.NoError(t, err)
	return decrypted
}

func TestAuditQueueSealerSeal(t *testing.T) {

	const testTimeout = 5 * time.Second
	plaintext := []byte("audit event payload")

	tryDecrypt := func(ctx context.Context, key *rsa.PrivateKey, payload []byte) bool {
		identity := recordingencryption.NewRecordingIdentity(ctx, &testKeyUnwrapper{key: key})
		reader, err := age.Decrypt(bytes.NewReader(payload), identity)
		if err != nil {
			return false
		}
		decrypted, err := io.ReadAll(reader)
		return err == nil && bytes.Equal(decrypted, plaintext)
	}

	t.Run("seal round-trips when encryption is enabled", func(t *testing.T) {
		ctx := t.Context()
		key, pubDER := testRSAKeyPair(t)
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, true, pubDER))
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		assert.True(t, sealed)
		assert.NotEqual(t, plaintext, payload)
		assert.Equal(t, plaintext, decryptPayload(t, ctx, key, payload))
	})

	t.Run("seal passes through when encryption is disabled", func(t *testing.T) {
		ctx := t.Context()
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, false))
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		assert.False(t, sealed)
		assert.Equal(t, plaintext, payload)
	})

	t.Run("rotated keys apply when the watcher delivers the change", func(t *testing.T) {
		ctx := t.Context()
		keyA, pubA := testRSAKeyPair(t)
		keyB, pubB := testRSAKeyPair(t)
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, true, pubA))
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.Equal(t, plaintext, decryptPayload(t, ctx, keyA, payload))

		upsertSRC(t, clt, encryptedSRC(t, true, pubB))
		require.Eventually(t, func() bool {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			return err == nil && sealed && tryDecrypt(ctx, keyB, payload)
		}, testTimeout, 10*time.Millisecond)
	})

	t.Run("disabling encryption applies when the watcher delivers the change", func(t *testing.T) {
		ctx := t.Context()
		_, pubA := testRSAKeyPair(t)
		_, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, true, pubA))
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		upsertSRC(t, clt, encryptedSRC(t, false))
		require.Eventually(t, func() bool {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			return err == nil && !sealed && bytes.Equal(payload, plaintext)
		}, testTimeout, 10*time.Millisecond)
	})

	t.Run("resubscribe refreshes missed changes", func(t *testing.T) {
		ctx := t.Context()
		keyA, pubA := testRSAKeyPair(t)
		keyB, pubB := testRSAKeyPair(t)
		bk, clt := newSealerClient(t)
		upsertSRC(t, clt, encryptedSRC(t, true, pubA))
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, clt)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.Equal(t, plaintext, decryptPayload(t, ctx, keyA, payload))

		bk.CloseWatchers()
		upsertSRC(t, clt, encryptedSRC(t, true, pubB))
		require.Eventually(t, func() bool {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			return err == nil && sealed && tryDecrypt(ctx, keyB, payload)
		}, testTimeout, 10*time.Millisecond)
	})

	t.Run("seal does not block on a blocked getter", func(t *testing.T) {
		ctx := t.Context()
		key, pubDER := testRSAKeyPair(t)
		_, clt := newSealerClient(t)
		blocking := &blockingSRCClient{Events: clt.Events, src: encryptedSRC(t, true, pubDER)}
		sealer, err := recordingencryption.NewAuditQueueSealer(ctx, blocking)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		for range 5 {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			require.NoError(t, err)
			assert.True(t, sealed)
			assert.Equal(t, plaintext, decryptPayload(t, ctx, key, payload))
		}
	})
}
