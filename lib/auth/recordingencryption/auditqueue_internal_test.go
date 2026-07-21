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

package recordingencryption

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"io"
	"slices"
	"sync"
	"testing"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

type staticSRCGetter struct {
	src types.SessionRecordingConfig
	err error
}

func (s *staticSRCGetter) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return s.src, s.err
}

type swappableSRCGetter struct {
	mu  sync.Mutex
	src types.SessionRecordingConfig
	err error
}

func (s *swappableSRCGetter) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.src, s.err
}

func (s *swappableSRCGetter) set(src types.SessionRecordingConfig, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.src = src
	s.err = err
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

func testRSAPublicKeyDER(t *testing.T) []byte {
	t.Helper()
	_, pubDER := testRSAKeyPair(t)
	return pubDER
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

func (u *testKeyUnwrapper) UnwrapKey(ctx context.Context, in UnwrapInput) ([]byte, error) {
	fileKey, err := u.key.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	return fileKey, trace.Wrap(err)
}

func TestNewAuditQueueSealer(t *testing.T) {
	ctx := t.Context()

	t.Run("encryption disabled", func(t *testing.T) {
		sealer, err := NewAuditQueueSealer(ctx, &staticSRCGetter{src: encryptedSRC(t, false)})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })
		require.False(t, sealer.encrypted)
		require.Empty(t, sealer.recipients)
	})

	t.Run("encryption enabled with keys", func(t *testing.T) {
		sealer, err := NewAuditQueueSealer(ctx, &staticSRCGetter{
			src: encryptedSRC(t, true, testRSAPublicKeyDER(t), testRSAPublicKeyDER(t)),
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })
		require.True(t, sealer.encrypted)
		require.Len(t, sealer.recipients, 2)
	})

	t.Run("enabled without keys fails", func(t *testing.T) {
		_, err := NewAuditQueueSealer(ctx, &staticSRCGetter{src: encryptedSRC(t, true)})
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("malformed key fails", func(t *testing.T) {
		_, err := NewAuditQueueSealer(ctx, &staticSRCGetter{
			src: encryptedSRC(t, true, []byte("not a public key")),
		})
		require.Error(t, err)
	})

	t.Run("config fetch failure fails", func(t *testing.T) {
		_, err := NewAuditQueueSealer(ctx, &staticSRCGetter{
			err: trace.ConnectionProblem(nil, "auth unavailable"),
		})
		require.Error(t, err)
	})

	t.Run("nil getter fails", func(t *testing.T) {
		_, err := NewAuditQueueSealer(ctx, nil)
		require.True(t, trace.IsBadParameter(err))
	})
}

func requireRecipientKey(t *testing.T, recipients []age.Recipient, pubKeyDER []byte) {
	t.Helper()
	require.Len(t, recipients, 1)
	parsed, err := x509.ParsePKIXPublicKey(pubKeyDER)
	require.NoError(t, err)
	recipient, ok := recipients[0].(*RecordingRecipient)
	require.True(t, ok)
	require.True(t, recipient.PublicKey.Equal(parsed.(*rsa.PublicKey)))
}

func TestAuditQueueSealerRefresh(t *testing.T) {
	ctx := t.Context()

	newSealer := func(getter SessionRecordingConfigGetter) *AuditQueueSealer {
		return &AuditQueueSealer{srcGetter: getter}
	}

	t.Run("refresh picks up rotated keys", func(t *testing.T) {
		keyA := testRSAPublicKeyDER(t)
		keyB := testRSAPublicKeyDER(t)
		getter := &swappableSRCGetter{src: encryptedSRC(t, true, keyA)}
		sealer := newSealer(getter)

		require.NoError(t, sealer.refreshOnce(ctx))
		require.True(t, sealer.encrypted)
		requireRecipientKey(t, sealer.recipients, keyA)

		getter.set(encryptedSRC(t, true, keyB), nil)
		require.NoError(t, sealer.refreshOnce(ctx))
		requireRecipientKey(t, sealer.recipients, keyB)
	})

	t.Run("failed refresh keeps last known good keys", func(t *testing.T) {
		keyA := testRSAPublicKeyDER(t)
		getter := &swappableSRCGetter{src: encryptedSRC(t, true, keyA)}
		sealer := newSealer(getter)

		require.NoError(t, sealer.refreshOnce(ctx))

		getter.set(nil, trace.ConnectionProblem(nil, "auth unavailable"))
		require.Error(t, sealer.refreshOnce(ctx))
		require.True(t, sealer.encrypted)
		requireRecipientKey(t, sealer.recipients, keyA)

		getter.set(encryptedSRC(t, true), nil)
		require.Error(t, sealer.refreshOnce(ctx))
		require.True(t, sealer.encrypted)
		requireRecipientKey(t, sealer.recipients, keyA)
	})

	t.Run("seal passes through when encryption is disabled", func(t *testing.T) {
		getter := &swappableSRCGetter{src: encryptedSRC(t, false)}
		sealer := newSealer(getter)
		require.NoError(t, sealer.refreshOnce(ctx))

		plaintext := []byte("audit event payload")
		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.False(t, sealed)
		require.Equal(t, plaintext, payload)
	})

	t.Run("seal round-trips when encryption is enabled", func(t *testing.T) {
		key, pubDER := testRSAKeyPair(t)
		getter := &swappableSRCGetter{src: encryptedSRC(t, true, pubDER)}
		sealer := newSealer(getter)
		require.NoError(t, sealer.refreshOnce(ctx))

		plaintext := []byte("audit event payload")
		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.NotEqual(t, plaintext, payload)

		identity := NewRecordingIdentity(ctx, &testKeyUnwrapper{key: key})
		reader, err := age.Decrypt(bytes.NewReader(payload), identity)
		require.NoError(t, err)
		decrypted, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, plaintext, decrypted)
	})

	t.Run("seal fails before the first refresh", func(t *testing.T) {
		sealer := newSealer(&swappableSRCGetter{src: encryptedSRC(t, false)})

		_, _, err := sealer.Seal(ctx, []byte("audit event payload"))
		require.Error(t, err)
	})
}
