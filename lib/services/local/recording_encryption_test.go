package local

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	recencpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

type oaepDecrypter struct {
	crypto.Decrypter
	hash crypto.Hash
}

func (d oaepDecrypter) Decrypt(rand io.Reader, msg []byte, opts crypto.DecrypterOpts) ([]byte, error) {
	return d.Decrypter.Decrypt(rand, msg, &rsa.OAEPOptions{
		Hash: d.hash,
	})
}

type fakeEncryptionKeyStore struct {
	pkeyType apitypes.PrivateKeyType // abusing this field as a way to simulate different auth servers
}

func (f *fakeEncryptionKeyStore) NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*apitypes.EncryptionKeyPair, error) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  keys.PKCS1PrivateKeyType,
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	})

	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type:  keys.PKCS1PublicKeyType,
		Bytes: x509.MarshalPKCS1PublicKey(&private.PublicKey),
	})

	return &apitypes.EncryptionKeyPair{
		PrivateKey:     privatePEM,
		PublicKey:      publicPEM,
		PrivateKeyType: f.pkeyType,
		Hash:           uint32(crypto.SHA256),
	}, nil
}

func (f *fakeEncryptionKeyStore) GetDecrypter(ctx context.Context, keyPair *apitypes.EncryptionKeyPair) (crypto.Decrypter, error) {
	if keyPair.PrivateKeyType != f.pkeyType {
		return nil, errors.New("could not access decrypter")
	}

	block, _ := pem.Decode(keyPair.PrivateKey)

	private, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return oaepDecrypter{Decrypter: private, hash: crypto.Hash(keyPair.Hash)}, nil
}

func setupRecordingEncryptionServiceTest(
	t *testing.T,
) (context.Context, clockwork.Clock, *RecordingEncryptionService) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	// TODO (eriktate): replace this with the test logger
	service, err := NewRecordingEncryptionService(backend.NewSanitizer(mem), slog.Default())
	require.NoError(t, err)
	return ctx, clock, service
}

func TestRecordingEncryptionServiceEvaluate(t *testing.T) {
	ctx, _, service := setupRecordingEncryptionServiceTest(t)
	// keytypes simulate different auth servers with access to different keys
	keyStoreRAW := &fakeEncryptionKeyStore{pkeyType: apitypes.PrivateKeyType_RAW}
	keyStoreAWS := &fakeEncryptionKeyStore{pkeyType: apitypes.PrivateKeyType_AWS_KMS}
	// keyStoreHSM := &fakeEncryptionKeyStore{pkeyType: apitypes.PrivateKeyType_PKCS11}

	encryption, err := service.EvaluateRecordingEncryption(ctx, keyStoreRAW)
	require.NoError(t, err)
	activeKeys := encryption.GetSpec().GetKeySet().GetActiveKeys()

	require.Equal(t, 1, len(activeKeys))
	firstKey := activeKeys[0]

	require.NotNil(t, firstKey.KeyEncryptionPair)
	require.NotNil(t, firstKey.RecordingEncryptionPair)

	newPair, err := keyStoreAWS.NewEncryptionKeyPair(ctx, cryptosuites.RecordingEncryption)
	require.NoError(t, err)

	// create unfulfilled key
	encryption.Spec.KeySet.ActiveKeys = append(activeKeys, &recencpb.WrappedKey{
		KeyEncryptionPair: newPair,
		State:             recencpb.KeyState_KEY_STATE_ACTIVE,
	})

	encryption, err = service.UpdateRecordingEncryption(ctx, encryption)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if slices.Equal(key.KeyEncryptionPair.PublicKey, firstKey.KeyEncryptionPair.PublicKey) {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// re-evaluting with an unfulfilled key should do nothing
	encryption, err = service.EvaluateRecordingEncryption(ctx, keyStoreAWS)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if slices.Equal(key.KeyEncryptionPair.PublicKey, firstKey.KeyEncryptionPair.PublicKey) {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// re-evaluate to fulfill key
	encryption, err = service.EvaluateRecordingEncryption(ctx, keyStoreRAW)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))

	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotNil(t, key.RecordingEncryptionPair)
	}

	// mark first key as rotating
	activeKeys[0].State = recencpb.KeyState_KEY_STATE_ROTATING
	encryption, err = service.UpdateRecordingEncryption(ctx, encryption)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))

	// re-evaluate to handle rotation
	encryption, err = service.EvaluateRecordingEncryption(ctx, keyStoreRAW)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ROTATING, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_AWS_KMS, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.Nil(t, activeKeys[2].RecordingEncryptionPair)

	// make sure that a second evaluation doesn't provision another unfulfilled key
	encryption, err = service.EvaluateRecordingEncryption(ctx, keyStoreRAW)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ROTATING, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_AWS_KMS, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.Nil(t, activeKeys[2].RecordingEncryptionPair)

	// evaluating with an active keystore should fulfill the key but not mark the old one as rotated
	encryption, err = service.EvaluateRecordingEncryption(ctx, keyStoreAWS)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ROTATING, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_AWS_KMS, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.NotNil(t, activeKeys[2].RecordingEncryptionPair)

	// evaluating with the original keystore should mark the rotated key as such
	encryption, err = service.EvaluateRecordingEncryption(ctx, keyStoreRAW)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ROTATED, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_AWS_KMS, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, apitypes.PrivateKeyType_RAW, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, recencpb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.NotNil(t, activeKeys[2].RecordingEncryptionPair)
}
