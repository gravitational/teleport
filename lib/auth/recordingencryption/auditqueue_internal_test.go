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
	"context"
	"crypto/x509"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

type fakeSRCClient struct {
	src types.SessionRecordingConfig
	err error
}

func (f *fakeSRCClient) GetSessionRecordingConfig(ctx context.Context) (types.SessionRecordingConfig, error) {
	return f.src, f.err
}

func (f *fakeSRCClient) NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error) {
	return nil, trace.NotImplemented("fakeSRCClient does not support watchers")
}

func internalTestSRC(t *testing.T, enabled bool, pubKeys ...[]byte) *types.SessionRecordingConfigV2 {
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

func internalTestPublicKeyDER(t *testing.T) []byte {
	t.Helper()
	signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA4096)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(signer.Public())
	require.NoError(t, err)
	return pubDER
}

func TestAuditQueueSealerFetchFailureKeepsLastKnownKeys(t *testing.T) {
	client := &fakeSRCClient{err: trace.ConnectionProblem(nil, "auth unavailable")}
	sealer := &AuditQueueSealer{client: client}
	require.NoError(t, sealer.applyConfig(internalTestSRC(t, true, internalTestPublicKeyDER(t))))
	before, err := sealer.encryptionState()
	require.NoError(t, err)
	require.True(t, before.encrypted)
	require.Len(t, before.recipients, 1)

	require.Error(t, sealer.refresh(t.Context()))
	after, err := sealer.encryptionState()
	require.NoError(t, err)
	assert.Same(t, before, after)

	client.src, client.err = internalTestSRC(t, true), nil
	require.Error(t, sealer.refresh(t.Context()))
	after, err = sealer.encryptionState()
	require.NoError(t, err)
	assert.Same(t, before, after)
}

func TestAuditQueueSealerSealUnresolvedKeys(t *testing.T) {
	sealer := &AuditQueueSealer{}
	_, _, err := sealer.Seal(t.Context(), []byte("audit event payload"))
	require.Error(t, err)
}
