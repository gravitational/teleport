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
