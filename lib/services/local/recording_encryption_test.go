package local

import (
	"context"
	"crypto"
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestRecordingEncryption(t *testing.T) {
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewRecordingEncryptionService(backend)
	require.NoError(t, err)

	ctx := context.Background()

	initialEncryption := pb.RecordingEncryption{
		Metadata: &headerv1.Metadata{
			Name: "recording_encryption",
		},
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
	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)
	service, err := NewRecordingEncryptionService(backend)
	require.NoError(t, err)

	ctx := context.Background()

	initialEncryption := pb.RecordingEncryption{
		Metadata: &headerv1.Metadata{
			Name: "recording_encryption",
		},
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
