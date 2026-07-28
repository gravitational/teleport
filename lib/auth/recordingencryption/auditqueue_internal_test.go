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
	"sync/atomic"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

type fakeWatcher struct {
	events    chan types.Event
	done      chan struct{}
	closeOnce sync.Once
}

func (w *fakeWatcher) Events() <-chan types.Event { return w.events }

func (w *fakeWatcher) Done() <-chan struct{} { return w.done }

func (w *fakeWatcher) Close() error {
	w.closeOnce.Do(func() { close(w.done) })
	return nil
}

func (w *fakeWatcher) Error() error { return nil }

type watcherHub struct {
	hubMu  sync.Mutex
	active *fakeWatcher
}

func (h *watcherHub) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	h.hubMu.Lock()
	defer h.hubMu.Unlock()

	w := &fakeWatcher{
		events: make(chan types.Event, 16),
		done:   make(chan struct{}),
	}
	w.events <- types.Event{Type: types.OpInit}
	h.active = w
	return w, nil
}

func (h *watcherHub) activeWatcher(t *testing.T) *fakeWatcher {
	t.Helper()
	var w *fakeWatcher
	require.Eventually(t, func() bool {
		h.hubMu.Lock()
		defer h.hubMu.Unlock()
		w = h.active
		return w != nil
	}, 5*time.Second, time.Millisecond)
	return w
}

func (h *watcherHub) emit(t *testing.T, event types.Event) {
	t.Helper()
	h.activeWatcher(t).events <- event
}

func (h *watcherHub) closeActive(t *testing.T) {
	t.Helper()
	require.NoError(t, h.activeWatcher(t).Close())
}

type staticSRCGetter struct {
	watcherHub
	src types.SessionRecordingConfig
	err error
}

func (s *staticSRCGetter) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return s.src, s.err
}

type swappableSRCGetter struct {
	watcherHub
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

type blockingSRCGetter struct {
	watcherHub
	src   types.SessionRecordingConfig
	calls atomic.Int64
}

func (b *blockingSRCGetter) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
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
		state, err := sealer.encryptionState()
		require.NoError(t, err)
		require.False(t, state.encrypted)
		require.Empty(t, state.recipients)
	})

	t.Run("encryption enabled with keys", func(t *testing.T) {
		sealer, err := NewAuditQueueSealer(ctx, &staticSRCGetter{
			src: encryptedSRC(t, true, testRSAPublicKeyDER(t), testRSAPublicKeyDER(t)),
		})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })
		state, err := sealer.encryptionState()
		require.NoError(t, err)
		require.True(t, state.encrypted)
		require.Len(t, state.recipients, 2)
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

func TestAuditQueueSealerSeal(t *testing.T) {
	ctx := t.Context()

	const testTimeout = 5 * time.Second
	plaintext := []byte("audit event payload")

	tryDecrypt := func(key *rsa.PrivateKey, payload []byte) bool {
		identity := NewRecordingIdentity(ctx, &testKeyUnwrapper{key: key})
		reader, err := age.Decrypt(bytes.NewReader(payload), identity)
		if err != nil {
			return false
		}
		decrypted, err := io.ReadAll(reader)
		return err == nil && bytes.Equal(decrypted, plaintext)
	}

	decrypt := func(t *testing.T, key *rsa.PrivateKey, payload []byte) []byte {
		t.Helper()
		identity := NewRecordingIdentity(ctx, &testKeyUnwrapper{key: key})
		reader, err := age.Decrypt(bytes.NewReader(payload), identity)
		require.NoError(t, err)
		decrypted, err := io.ReadAll(reader)
		require.NoError(t, err)
		return decrypted
	}

	t.Run("seal round-trips when encryption is enabled", func(t *testing.T) {
		key, pubDER := testRSAKeyPair(t)
		sealer, err := NewAuditQueueSealer(ctx, &staticSRCGetter{src: encryptedSRC(t, true, pubDER)})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.NotEqual(t, plaintext, payload)
		require.Equal(t, plaintext, decrypt(t, key, payload))
	})

	t.Run("seal passes through when encryption is disabled", func(t *testing.T) {
		sealer, err := NewAuditQueueSealer(ctx, &staticSRCGetter{src: encryptedSRC(t, false)})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.False(t, sealed)
		require.Equal(t, plaintext, payload)
	})

	t.Run("rotated keys apply when the watcher delivers the change", func(t *testing.T) {
		keyA, pubA := testRSAKeyPair(t)
		keyB, pubB := testRSAKeyPair(t)
		getter := &swappableSRCGetter{src: encryptedSRC(t, true, pubA)}
		sealer, err := NewAuditQueueSealer(ctx, getter)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.Equal(t, plaintext, decrypt(t, keyA, payload))

		srcB := encryptedSRC(t, true, pubB)
		getter.set(srcB, nil)
		getter.emit(t, types.Event{Type: types.OpPut, Resource: srcB})
		require.Eventually(t, func() bool {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			return err == nil && sealed && tryDecrypt(keyB, payload)
		}, testTimeout, 10*time.Millisecond)
	})

	t.Run("disabling encryption applies when the watcher delivers the change", func(t *testing.T) {
		_, pubA := testRSAKeyPair(t)
		getter := &swappableSRCGetter{src: encryptedSRC(t, true, pubA)}
		sealer, err := NewAuditQueueSealer(ctx, getter)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		disabled := encryptedSRC(t, false)
		getter.set(disabled, nil)
		getter.emit(t, types.Event{Type: types.OpPut, Resource: disabled})
		require.Eventually(t, func() bool {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			return err == nil && !sealed && bytes.Equal(payload, plaintext)
		}, testTimeout, 10*time.Millisecond)
	})

	t.Run("resubscribe refreshes missed changes", func(t *testing.T) {
		keyA, pubA := testRSAKeyPair(t)
		keyB, pubB := testRSAKeyPair(t)
		getter := &swappableSRCGetter{src: encryptedSRC(t, true, pubA)}
		sealer, err := NewAuditQueueSealer(ctx, getter)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.Equal(t, plaintext, decrypt(t, keyA, payload))

		getter.set(encryptedSRC(t, true, pubB), nil)
		getter.closeActive(t)
		require.Eventually(t, func() bool {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			return err == nil && sealed && tryDecrypt(keyB, payload)
		}, testTimeout, 10*time.Millisecond)
	})

	t.Run("fetch failure falls back to last known keys", func(t *testing.T) {
		keyA, pubA := testRSAKeyPair(t)
		getter := &swappableSRCGetter{src: encryptedSRC(t, true, pubA)}
		sealer, err := NewAuditQueueSealer(ctx, getter)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		getter.set(nil, trace.ConnectionProblem(nil, "auth unavailable"))
		require.Error(t, sealer.refresh(ctx))
		payload, sealed, err := sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.Equal(t, plaintext, decrypt(t, keyA, payload))

		getter.set(encryptedSRC(t, true), nil)
		require.Error(t, sealer.refresh(ctx))
		payload, sealed, err = sealer.Seal(ctx, plaintext)
		require.NoError(t, err)
		require.True(t, sealed)
		require.Equal(t, plaintext, decrypt(t, keyA, payload))
	})

	t.Run("seal does not block on a blocked getter", func(t *testing.T) {
		key, pubDER := testRSAKeyPair(t)
		getter := &blockingSRCGetter{src: encryptedSRC(t, true, pubDER)}
		sealer, err := NewAuditQueueSealer(ctx, getter)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, sealer.Close()) })

		for range 5 {
			payload, sealed, err := sealer.Seal(ctx, plaintext)
			require.NoError(t, err)
			require.True(t, sealed)
			require.Equal(t, plaintext, decrypt(t, key, payload))
		}
	})

	t.Run("seal fails when keys were never resolved", func(t *testing.T) {
		sealer := &AuditQueueSealer{client: &staticSRCGetter{
			err: trace.ConnectionProblem(nil, "auth unavailable"),
		}}

		_, _, err := sealer.Seal(ctx, plaintext)
		require.Error(t, err)
	})
}
