package local

import (
	"context"
	"crypto"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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
			KeySet: &pb.KeySet{
				ActiveKeys: nil,
			},
		},
	}

	// get should fail when there's no recording encryption
	_, err = service.GetRecordingEncryption(ctx)
	require.Error(t, err)

	created, err := service.CreateRecordingEncryption(ctx, &initialEncryption)
	require.NoError(t, err)

	encryption, err := service.GetRecordingEncryption(ctx)
	require.NoError(t, err)

	require.Len(t, created.Spec.GetKeySet().ActiveKeys, 0)
	require.Len(t, encryption.Spec.GetKeySet().ActiveKeys, 0)

	encryption.Spec.KeySet.ActiveKeys = []*pb.WrappedKey{
		{
			RecordingEncryptionPair: &types.EncryptionKeyPair{
				PrivateKey: []byte("recording encryption private"),
				PublicKey:  []byte("recording encryption public"),
				Hash:       0,
			},
			KeyEncryptionPair: &types.EncryptionKeyPair{
				PrivateKey: []byte("key encryption private"),
				PublicKey:  []byte("key encryption public"),
				Hash:       uint32(crypto.SHA256),
			},
		},
	}

	updated, err := service.UpdateRecordingEncryption(ctx, encryption)
	require.NoError(t, err)
	require.Len(t, updated.Spec.GetKeySet().ActiveKeys, 1)
	require.EqualExportedValues(t, encryption.Spec.GetKeySet().ActiveKeys[0], updated.Spec.GetKeySet().ActiveKeys[0])

	encryption, err = service.GetRecordingEncryption(ctx)
	require.NoError(t, err)
	require.Len(t, encryption.Spec.GetKeySet().ActiveKeys, 1)
	require.EqualExportedValues(t, updated.Spec.GetKeySet().ActiveKeys[0], encryption.Spec.GetKeySet().ActiveKeys[0])
}

func TestRotatedKeys(t *testing.T) {
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewRecordingEncryptionService(backend.NewSanitizer(bk))
	require.NoError(t, err)

	ctx := context.Background()

	fakePublicKey := []byte("public_key")
	initialRotatedKeys := pb.RotatedKeys{
		Metadata: &headerv1.Metadata{
			Name: string(fakePublicKey),
		},
		Spec: &pb.RotatedKeysSpec{
			Keys: nil,
		},
	}

	// get should fail when there's no rotated keys for a given public key
	_, err = service.GetRotatedKeys(ctx, fakePublicKey)
	require.Error(t, err)

	created, err := service.CreateRotatedKeys(ctx, &initialRotatedKeys)
	require.NoError(t, err)

	rotatedKeys, err := service.GetRotatedKeys(ctx, fakePublicKey)
	require.NoError(t, err)

	require.Len(t, created.Spec.Keys, 0)
	require.Len(t, rotatedKeys.Spec.Keys, 0)

	rotatedKeys.Spec.Keys = []*pb.WrappedKey{
		{
			RecordingEncryptionPair: &types.EncryptionKeyPair{
				PrivateKey: []byte("recording encryption private"),
				PublicKey:  []byte("recording encryption public"),
				Hash:       0,
			},
			KeyEncryptionPair: &types.EncryptionKeyPair{
				PrivateKey: []byte("key encryption private"),
				PublicKey:  []byte("key encryption public"),
				Hash:       uint32(crypto.SHA256),
			},
		},
	}

	updated, err := service.UpdateRotatedKeys(ctx, rotatedKeys)
	require.NoError(t, err)
	require.Len(t, updated.Spec.Keys, 1)
	require.EqualExportedValues(t, rotatedKeys.Spec.Keys[0], updated.Spec.Keys[0])

	rotatedKeys, err = service.GetRotatedKeys(ctx, fakePublicKey)
	require.NoError(t, err)
	require.Len(t, rotatedKeys.Spec.Keys, 1)
	require.EqualExportedValues(t, updated.Spec.Keys[0], rotatedKeys.Spec.Keys[0])
}
