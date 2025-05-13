package recordingencryptionv1_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/recordingencryption/recordingencryptionv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
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
	keyType types.PrivateKeyType // abusing this field as a way to simulate different auth servers
}

func (f *fakeEncryptionKeyStore) NewEncryptionKeyPair(ctx context.Context, purpose cryptosuites.KeyPurpose) (*types.EncryptionKeyPair, error) {
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

	return &types.EncryptionKeyPair{
		PrivateKey:     privatePEM,
		PublicKey:      publicPEM,
		PrivateKeyType: f.keyType,
		Hash:           uint32(crypto.SHA256),
	}, nil
}

func (f *fakeEncryptionKeyStore) GetDecrypter(ctx context.Context, keyPair *types.EncryptionKeyPair) (crypto.Decrypter, error) {
	if keyPair.PrivateKeyType != f.keyType {
		return nil, errors.New("could not access decrypter")
	}

	block, _ := pem.Decode(keyPair.PrivateKey)

	private, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return oaepDecrypter{Decrypter: private, hash: crypto.Hash(keyPair.Hash)}, nil
}

func newLocalBackend(
	t *testing.T,
) (context.Context, services.RecordingEncryption) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	service, err := local.NewRecordingEncryptionService(backend.NewSanitizer(mem))
	require.NoError(t, err)
	return ctx, service
}

func newServiceConfig(backend services.RecordingEncryption, keyType types.PrivateKeyType) recordingencryptionv1.ServiceConfig {
	return recordingencryptionv1.ServiceConfig{
		Logger:     utils.NewSlogLoggerForTests(),
		Cache:      struct{ recordingencryptionv1.Cache }{},
		Authorizer: struct{ authz.Authorizer }{},
		Emitter:    struct{ events.Emitter }{},
		KeyStore:   &fakeEncryptionKeyStore{keyType: keyType},
		Backend:    backend,
	}
}

func TestResolveRecordingEncryption(t *testing.T) {
	// SETUP
	ctx, backend := newLocalBackend(t)

	serviceAType := types.PrivateKeyType_RAW
	serviceBType := types.PrivateKeyType_AWS_KMS

	serviceA, err := recordingencryptionv1.NewService(newServiceConfig(backend, serviceAType))
	require.NoError(t, err)

	serviceB, err := recordingencryptionv1.NewService(newServiceConfig(backend, serviceBType))
	require.NoError(t, err)

	// TEST
	// CASE: service A first evaluation initializes recording encryption resource
	encryption, err := serviceA.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys := encryption.GetSpec().GetKeySet().GetActiveKeys()

	require.Equal(t, 1, len(activeKeys))
	firstKey := activeKeys[0]

	// should generate a wrapped key with the initial recording encryption pair
	require.NotNil(t, firstKey.KeyEncryptionPair)
	require.NotNil(t, firstKey.RecordingEncryptionPair)

	// CASE: service B should generate an unfulfilled key since there's an existing recording encryption resource
	encryption, err = serviceB.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)

	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == serviceAType {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// service B re-evaluting with an unfulfilled key should do nothing
	encryption, err = serviceB.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == serviceAType {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// CASE: service A evaluation should fulfill service B's key
	encryption, err = serviceA.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotNil(t, key.RecordingEncryptionPair)
	}

	// marking service A key as rotating
	activeKeys[0].State = pb.KeyState_KEY_STATE_ROTATING
	encryption, err = backend.UpdateRecordingEncryption(ctx, encryption)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotNil(t, key.RecordingEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == serviceAType {
			require.Equal(t, pb.KeyState_KEY_STATE_ROTATING, key.State)
		}
	}

	// CASE: service B evaluation should do nothing when service A key is marked as rotating
	encryption, err = serviceB.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 2, len(activeKeys))
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotNil(t, key.RecordingEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == serviceAType {
			require.Equal(t, pb.KeyState_KEY_STATE_ROTATING, key.State)
		}
	}

	// CASE: service A evaluation should create an unfulfilled key to replace rotating key
	encryption, err = serviceA.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))
	require.Equal(t, serviceAType, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ROTATING, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, serviceBType, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, serviceAType, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.Nil(t, activeKeys[2].RecordingEncryptionPair)

	// service A re-evaluation should not provision another unfulfilled key
	encryption, err = serviceA.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))
	require.Equal(t, serviceAType, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ROTATING, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, serviceBType, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, serviceAType, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.Nil(t, activeKeys[2].RecordingEncryptionPair)

	// CASE: service B evaluation should fulfill service A's key but not mark the original key as rotated
	encryption, err = serviceB.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))
	require.Equal(t, serviceAType, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ROTATING, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, serviceBType, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, serviceAType, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.NotNil(t, activeKeys[2].RecordingEncryptionPair)

	// CASE: service A evaluation should mark original key as rotated
	encryption, err = serviceA.ResolveRecordingEncryption(ctx)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().GetKeySet().GetActiveKeys()
	require.Equal(t, 3, len(activeKeys))

	require.Equal(t, serviceAType, activeKeys[0].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ROTATED, activeKeys[0].State)
	require.NotNil(t, activeKeys[0].RecordingEncryptionPair)

	require.Equal(t, serviceBType, activeKeys[1].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[1].State)
	require.NotNil(t, activeKeys[1].RecordingEncryptionPair)

	require.Equal(t, serviceAType, activeKeys[2].KeyEncryptionPair.PrivateKeyType)
	require.Equal(t, pb.KeyState_KEY_STATE_ACTIVE, activeKeys[2].State)
	require.NotNil(t, activeKeys[2].RecordingEncryptionPair)
}
