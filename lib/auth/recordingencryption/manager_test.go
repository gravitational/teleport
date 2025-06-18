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

package recordingencryption_test

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
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
	decrypter, err := cryptosuites.GenerateDecrypterWithAlgorithm(cryptosuites.RSA2048)
	if err != nil {
		return nil, err
	}

	private, ok := decrypter.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("expected RSA private key")
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
) (context.Context, backend.Backend, services.RecordingEncryption) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewRealClock()
	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)
	bk := backend.NewSanitizer(mem)
	service, err := local.NewRecordingEncryptionService(bk)
	require.NoError(t, err)
	return ctx, bk, service
}

func newManagerConfig(bk backend.Backend, local services.RecordingEncryption, keyType types.PrivateKeyType) recordingencryption.ManagerConfig {
	return recordingencryption.ManagerConfig{
		Backend:  local,
		KeyStore: &fakeEncryptionKeyStore{keyType: keyType},
		Logger:   utils.NewSlogLoggerForTests(),
		LockConfig: backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            bk,
				LockNameComponents: []string{"recording_encryption"},
				TTL:                5 * time.Second,
				RetryInterval:      10 * time.Millisecond,
			},
		},
	}
}

func TestResolveRecordingEncryption(t *testing.T) {
	// SETUP
	ctx, bk, local := newLocalBackend(t)

	serviceAType := types.PrivateKeyType_RAW
	serviceBType := types.PrivateKeyType_AWS_KMS

	serviceA, err := recordingencryption.NewManager(newManagerConfig(bk, local, serviceAType))
	require.NoError(t, err)

	serviceB, err := recordingencryption.NewManager(newManagerConfig(bk, local, serviceBType))
	require.NoError(t, err)

	// TEST
	// CASE: service A first evaluation initializes recording encryption resource
	encryption, err := serviceA.ResolveRecordingEncryption(ctx, nil)
	require.NoError(t, err)
	activeKeys := encryption.GetSpec().GetActiveKeys()

	require.Len(t, activeKeys, 1)
	firstKey := activeKeys[0]

	// should generate a wrapped key with the initial recording encryption pair
	require.NotNil(t, firstKey.KeyEncryptionPair)
	require.NotNil(t, firstKey.RecordingEncryptionPair)

	// CASE: service B should generate an unfulfilled key since there's an existing recording encryption resource
	encryption, err = serviceB.ResolveRecordingEncryption(ctx, nil)
	require.NoError(t, err)

	activeKeys = encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == serviceAType {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// service B re-evaluting with an unfulfilled key should do nothing
	encryption, err = serviceB.ResolveRecordingEncryption(ctx, nil)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		if key.KeyEncryptionPair.PrivateKeyType == serviceAType {
			require.NotNil(t, key.RecordingEncryptionPair)
		} else {
			require.Nil(t, key.RecordingEncryptionPair)
		}
	}

	// CASE: service A evaluation should fulfill service B's key
	encryption, err = serviceA.ResolveRecordingEncryption(ctx, nil)
	require.NoError(t, err)
	activeKeys = encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	for _, key := range activeKeys {
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotNil(t, key.RecordingEncryptionPair)
	}
}

func TestResolveRecordingEncryptionConcurrent(t *testing.T) {
	// SETUP
	ctx, bk, local := newLocalBackend(t)

	serviceAType := types.PrivateKeyType_RAW
	serviceBType := types.PrivateKeyType_AWS_KMS
	serviceCType := types.PrivateKeyType_GCP_KMS

	serviceA, err := recordingencryption.NewManager(newManagerConfig(bk, local, serviceAType))
	require.NoError(t, err)

	serviceB, err := recordingencryption.NewManager(newManagerConfig(bk, local, serviceBType))
	require.NoError(t, err)

	serviceC, err := recordingencryption.NewManager(newManagerConfig(bk, local, serviceCType))
	require.NoError(t, err)

	resolveFn := func(service recordingencryption.Resolver, wg *sync.WaitGroup) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.ResolveRecordingEncryption(ctx, nil)
			require.NoError(t, err)
		}()
	}

	wg := sync.WaitGroup{}
	resolveFn(serviceA, &wg)
	resolveFn(serviceB, &wg)
	resolveFn(serviceC, &wg)
	wg.Wait()

	encryption, err := local.GetRecordingEncryption(ctx)
	require.NoError(t, err)

	activeKeys := encryption.GetSpec().ActiveKeys
	// each service should have an active wrapped key
	require.Len(t, activeKeys, 3)
	var fulfilledKeys int
	for _, key := range activeKeys {
		// all wrapped keys should have KeyEncryptionPairs
		require.NotNil(t, key.KeyEncryptionPair)
		require.NotEmpty(t, key.KeyEncryptionPair.PublicKey)
		require.NotEmpty(t, key.KeyEncryptionPair.PrivateKey)

		if key.RecordingEncryptionPair != nil {
			fulfilledKeys += 1
		}
	}

	// only the first service to run should have a fulfilled wrapped key
	require.Equal(t, 1, fulfilledKeys)
	err = local.DeleteRecordingEncryption(ctx)
	require.NoError(t, err)
}

func TestPostProcessFn(t *testing.T) {
	ctx, bk, local := newLocalBackend(t)
	keyType := types.PrivateKeyType_RAW

	manager, err := recordingencryption.NewManager(newManagerConfig(bk, local, keyType))
	require.NoError(t, err)

	// ensure postProcessFn is called with the resolved resource
	_, err = manager.ResolveRecordingEncryption(ctx, func(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) error {
		activeKeys := encryption.GetSpec().ActiveKeys
		require.Len(t, activeKeys, 1)
		firstKey := activeKeys[0]

		// should generate a wrapped key with the initial recording encryption pair
		require.NotNil(t, firstKey.KeyEncryptionPair)
		require.NotNil(t, firstKey.RecordingEncryptionPair)
		return nil
	})
	require.NoError(t, err)

	// ensure we return the error if postProcessFn fails
	testErr := errors.New("test error")
	_, err = manager.ResolveRecordingEncryption(ctx, func(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) error {
		return testErr
	})
	require.ErrorIs(t, err, testErr)

	_, err = manager.ResolveRecordingEncryption(ctx, func(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) error {
		activeKeys := encryption.GetSpec().ActiveKeys
		require.Len(t, activeKeys, 1)
		firstKey := activeKeys[0]

		// should generate a wrapped key with the initial recording encryption pair
		require.NotNil(t, firstKey.KeyEncryptionPair)
		require.NotNil(t, firstKey.RecordingEncryptionPair)
		return nil
	})
	require.NoError(t, err)
}

func TestFindDecryptionKeyFromActiveKeys(t *testing.T) {
	// SETUP
	ctx, bk, local := newLocalBackend(t)
	keyTypeA := types.PrivateKeyType_RAW
	keyTypeB := types.PrivateKeyType_AWS_KMS

	managerA, err := recordingencryption.NewManager(newManagerConfig(bk, local, keyTypeA))
	require.NoError(t, err)

	managerB, err := recordingencryption.NewManager(newManagerConfig(bk, local, keyTypeB))
	require.NoError(t, err)

	_, err = managerA.ResolveRecordingEncryption(ctx, nil)
	require.NoError(t, err)

	encryption, err := managerB.ResolveRecordingEncryption(ctx, nil)
	require.NoError(t, err)

	activeKeys := encryption.GetSpec().ActiveKeys
	require.Len(t, activeKeys, 2)
	pubKey := activeKeys[0].RecordingEncryptionPair.PublicKey

	// fail to find private key for manager B because it is waiting for key fulfillment
	_, err = managerB.FindDecryptionKey(ctx, pubKey)
	require.Error(t, err)

	_, err = managerA.ResolveRecordingEncryption(ctx, nil)
	require.NoError(t, err)

	// find private key for manager A because it provisioned the key
	decryptionPair, err := managerA.FindDecryptionKey(ctx, pubKey)
	require.NoError(t, err)
	ident, err := age.ParseX25519Identity(string(decryptionPair.PrivateKey))
	require.NoError(t, err)
	require.Equal(t, ident.Recipient().String(), string(pubKey))

	// find private key for manager B after fulfillment
	decryptionPair, err = managerB.FindDecryptionKey(ctx, pubKey)
	require.NoError(t, err)
	ident, err = age.ParseX25519Identity(string(decryptionPair.PrivateKey))
	require.NoError(t, err)
	require.Equal(t, ident.Recipient().String(), string(pubKey))
}
