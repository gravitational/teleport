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
	"testing"
	"time"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/protoadapt"

	apievents "github.com/gravitational/teleport/api/types/events"
)

type fingerprintKeyUnwrapper struct {
	keys map[string]*rsa.PrivateKey
}

func newFingerprintKeyUnwrapper(t *testing.T, keys ...*rsa.PrivateKey) *fingerprintKeyUnwrapper {
	t.Helper()
	unwrapper := &fingerprintKeyUnwrapper{keys: make(map[string]*rsa.PrivateKey, len(keys))}
	for _, key := range keys {
		fp, err := Fingerprint(key.Public())
		require.NoError(t, err)
		unwrapper.keys[fp] = key
	}
	return unwrapper
}

func (u *fingerprintKeyUnwrapper) UnwrapKey(ctx context.Context, in UnwrapInput) ([]byte, error) {
	key, ok := u.keys[in.Fingerprint]
	if !ok {
		return nil, trace.NotFound("no accessible key for fingerprint %q", in.Fingerprint)
	}
	fileKey, err := key.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	return fileKey, trace.Wrap(err)
}

func openerTestEvent(index int64) apievents.AuditEvent {
	return &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Index:       index,
			Type:        "user.login",
			ID:          "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			Code:        "T1000I",
			Time:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			ClusterName: "test-cluster.example.teleport.sh",
		},
		UserMetadata: apievents.UserMetadata{
			User:  "alice@example.com",
			Login: "alice",
		},
	}
}

func marshalEventBatch(t *testing.T, events []apievents.AuditEvent) []byte {
	t.Helper()
	var buf bytes.Buffer
	for _, event := range events {
		oneOf, err := apievents.ToOneOf(event)
		require.NoError(t, err)
		_, err = protodelim.MarshalTo(&buf, protoadapt.MessageV2Of(oneOf))
		require.NoError(t, err)
	}
	return buf.Bytes()
}

func newTestSealer(t *testing.T, pubKeys ...[]byte) *AuditQueueSealer {
	t.Helper()
	sealer, err := NewAuditQueueSealer(t.Context(), &staticSRCGetter{src: encryptedSRC(t, true, pubKeys...)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sealer.Close()) })
	return sealer
}

func newTestOpener(t *testing.T, keys ...*rsa.PrivateKey) *AuditQueueOpener {
	t.Helper()
	opener, err := NewAuditQueueOpener(newFingerprintKeyUnwrapper(t, keys...))
	require.NoError(t, err)
	return opener
}

func TestNewAuditQueueOpener(t *testing.T) {
	_, err := NewAuditQueueOpener(nil)
	require.ErrorContains(t, err, "KeyUnwrapper is required")
}

func TestDecryptBatch_RoundTrip(t *testing.T) {
	ctx := t.Context()
	key, pubDER := testRSAKeyPair(t)
	sealer := newTestSealer(t, pubDER)

	events := []apievents.AuditEvent{openerTestEvent(0), openerTestEvent(1), openerTestEvent(2)}
	sealed, encrypted, err := sealer.Seal(ctx, marshalEventBatch(t, events))
	require.NoError(t, err)
	require.True(t, encrypted)

	decrypted, err := newTestOpener(t, key).DecryptBatch(ctx, sealed)
	require.NoError(t, err)
	require.Equal(t, events, decrypted)
}

func TestDecryptBatch_MultiRecipient(t *testing.T) {
	ctx := t.Context()
	_, pubA := testRSAKeyPair(t)
	keyB, pubB := testRSAKeyPair(t)
	sealer := newTestSealer(t, pubA, pubB)

	events := []apievents.AuditEvent{openerTestEvent(0)}
	sealed, encrypted, err := sealer.Seal(ctx, marshalEventBatch(t, events))
	require.NoError(t, err)
	require.True(t, encrypted)

	decrypted, err := newTestOpener(t, keyB).DecryptBatch(ctx, sealed)
	require.NoError(t, err)
	require.Equal(t, events, decrypted)
}

func TestDecryptBatch_UnknownKey(t *testing.T) {
	ctx := t.Context()
	_, pubDER := testRSAKeyPair(t)
	sealer := newTestSealer(t, pubDER)

	sealed, _, err := sealer.Seal(ctx, marshalEventBatch(t, []apievents.AuditEvent{openerTestEvent(0)}))
	require.NoError(t, err)

	_, err = newTestOpener(t).DecryptBatch(ctx, sealed)
	require.Error(t, err)
}

func TestDecryptBatch_StanzaSeparation(t *testing.T) {
	ctx := t.Context()
	key, pubDER := testRSAKeyPair(t)
	payload := marshalEventBatch(t, []apievents.AuditEvent{openerTestEvent(0)})

	t.Run("recording recipient payload is rejected", func(t *testing.T) {
		recipient, err := ParseRecordingRecipient(pubDER)
		require.NoError(t, err)

		var sealed bytes.Buffer
		w, err := age.Encrypt(&sealed, recipient)
		require.NoError(t, err)
		_, err = w.Write(payload)
		require.NoError(t, err)
		require.NoError(t, w.Close())

		_, err = newTestOpener(t, key).DecryptBatch(ctx, sealed.Bytes())
		require.Error(t, err)
	})

	t.Run("recording identity rejects audit queue payload", func(t *testing.T) {
		sealer := newTestSealer(t, pubDER)
		sealed, _, err := sealer.Seal(ctx, payload)
		require.NoError(t, err)

		_, err = age.Decrypt(bytes.NewReader(sealed), NewRecordingIdentity(ctx, newFingerprintKeyUnwrapper(t, key)))
		require.Error(t, err)
	})
}

func TestDecryptBatch_MalformedPayload(t *testing.T) {
	ctx := t.Context()
	key, pubDER := testRSAKeyPair(t)
	sealer := newTestSealer(t, pubDER)
	opener := newTestOpener(t, key)

	sealed, _, err := sealer.Seal(ctx, marshalEventBatch(t, []apievents.AuditEvent{openerTestEvent(0)}))
	require.NoError(t, err)

	for name, payload := range map[string][]byte{
		"empty":                nil,
		"garbage":              []byte("not an age payload"),
		"truncated ciphertext": sealed[:len(sealed)-1],
		"plaintext batch":      marshalEventBatch(t, []apievents.AuditEvent{openerTestEvent(0)}),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := opener.DecryptBatch(ctx, payload)
			require.Error(t, err)
		})
	}
}

func TestDecryptBatch_NonBatchPlaintext(t *testing.T) {
	ctx := t.Context()
	key, pubDER := testRSAKeyPair(t)
	sealer := newTestSealer(t, pubDER)

	sealed, _, err := sealer.Seal(ctx, []byte("not a marshaled batch"))
	require.NoError(t, err)

	_, err = newTestOpener(t, key).DecryptBatch(ctx, sealed)
	require.ErrorContains(t, err, "unmarshaling audit event batch")
}

func TestDecryptBatch_TooManyEvents(t *testing.T) {
	ctx := t.Context()
	key, pubDER := testRSAKeyPair(t)
	sealer := newTestSealer(t, pubDER)
	opener := newTestOpener(t, key)

	atLimit := make([]apievents.AuditEvent, maxEventsPerSealedBatch)
	for i := range atLimit {
		atLimit[i] = openerTestEvent(int64(i))
	}
	sealed, encrypted, err := sealer.Seal(ctx, marshalEventBatch(t, atLimit))
	require.NoError(t, err)
	require.True(t, encrypted)
	decrypted, err := opener.DecryptBatch(ctx, sealed)
	require.NoError(t, err)
	require.Len(t, decrypted, maxEventsPerSealedBatch)

	overLimit := append(atLimit, openerTestEvent(maxEventsPerSealedBatch))
	sealed, encrypted, err = sealer.Seal(ctx, marshalEventBatch(t, overLimit))
	require.NoError(t, err)
	require.True(t, encrypted)
	_, err = opener.DecryptBatch(ctx, sealed)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
}
