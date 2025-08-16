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

package local

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestRecordingEncryption(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewRecordingEncryptionService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := context.Background()

	initialEncryption := pb.RecordingEncryption{
		Spec: &pb.RecordingEncryptionSpec{
			ActiveKeyPairs: nil,
		},
	}

	// get should fail when there's no recording encryption
	_, err = service.GetRecordingEncryption(ctx)
	require.Error(t, err)

	created, err := service.CreateRecordingEncryption(ctx, &initialEncryption)
	require.NoError(t, err)

	encryption, err := service.GetRecordingEncryption(ctx)
	require.NoError(t, err)

	require.Empty(t, created.Spec.ActiveKeyPairs)
	require.Empty(t, encryption.Spec.ActiveKeyPairs)

	encryption.Spec.ActiveKeyPairs = []*pb.KeyPair{
		{
			KeyPair: &types.EncryptionKeyPair{
				PrivateKey: []byte("recording encryption private"),
				PublicKey:  []byte("recording encryption public"),
				Hash:       0,
			},
		},
	}

	updated, err := service.UpdateRecordingEncryption(ctx, encryption)
	require.NoError(t, err)
	require.Len(t, updated.Spec.ActiveKeyPairs, 1)
	require.EqualExportedValues(t, encryption.Spec.ActiveKeyPairs[0], updated.Spec.ActiveKeyPairs[0])

	encryption, err = service.GetRecordingEncryption(ctx)
	require.NoError(t, err)
	require.Len(t, encryption.Spec.ActiveKeyPairs, 1)
	require.EqualExportedValues(t, updated.Spec.ActiveKeyPairs[0], encryption.Spec.ActiveKeyPairs[0])

	err = service.DeleteRecordingEncryption(ctx)
	require.NoError(t, err)
	_, err = service.GetRecordingEncryption(ctx)
	require.Error(t, err)
}

func TestRotatedKeys(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewRecordingEncryptionService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := context.Background()

	privateKey, err := keys.ParsePrivateKey(testRSA4096PrivateKeyPEM)
	require.NoError(t, err)
	publicKey := privateKey.Public()

	fingerprint, err := recordingencryption.Fingerprint(publicKey)
	require.NoError(t, err)

	publicKeyPEM, err := keys.MarshalPublicKey(publicKey)
	require.NoError(t, err)

	// get should fail when there's no rotated key
	_, err = service.GetRotatedKey(ctx, fingerprint)
	require.Error(t, err)

	created, err := service.CreateRotatedKey(ctx, &types.EncryptionKeyPair{
		PrivateKey:     testRSA4096PrivateKeyPEM,
		PublicKey:      publicKeyPEM,
		PrivateKeyType: types.PrivateKeyType_RAW,
	})
	require.NoError(t, err)
	require.Equal(t, testRSA4096PrivateKeyPEM, created.Spec.EncryptionKeyPair.PrivateKey)
	require.Equal(t, publicKeyPEM, created.Spec.EncryptionKeyPair.PublicKey)
	require.Equal(t, types.PrivateKeyType_RAW, created.Spec.EncryptionKeyPair.PrivateKeyType)

	rotatedKey, err := service.GetRotatedKey(ctx, fingerprint)
	require.NoError(t, err)

	require.Equal(t, testRSA4096PrivateKeyPEM, rotatedKey.Spec.EncryptionKeyPair.PrivateKey)
	require.Equal(t, publicKeyPEM, rotatedKey.Spec.EncryptionKeyPair.PublicKey)
	require.Equal(t, types.PrivateKeyType_RAW, rotatedKey.Spec.EncryptionKeyPair.PrivateKeyType)

	err = service.DeleteRotatedKey(ctx, fingerprint)
	require.NoError(t, err)
	_, err = service.GetRotatedKey(ctx, fingerprint)
	require.Error(t, err)
}

var testRSA4096PrivateKeyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`)
